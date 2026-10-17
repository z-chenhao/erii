package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/conversation"
	"github.com/z-chenhao/erii/internal/delivery"
	deliveryfake "github.com/z-chenhao/erii/internal/delivery/fake"
	deliverysqlite "github.com/z-chenhao/erii/internal/delivery/sqlite"
	deliveryterminal "github.com/z-chenhao/erii/internal/delivery/terminal"
	"github.com/z-chenhao/erii/internal/llm"
)

func terminalFixture(t *testing.T) (*Store, delivery.Service, *deliveryterminal.Adapter) {
	t.Helper()
	s, _, h, a, _ := loopFixture(t)
	mustSubmit(t, a, "one", "A durable reply")
	mustDrain(t, a)
	adapter := s.TerminalInbox()
	service := delivery.Service{Store: s.Deliveries(), History: h, Adapter: adapter, Channel: "tui", Destination: idAgent(t, s), Policy: delivery.DefaultPolicy()}
	return s, service, adapter
}
func deliveryRows(t *testing.T, s *Store) []delivery.Request {
	t.Helper()
	rows, err := s.Deliveries().List(context.Background(), idAgent(t, s))
	if err != nil {
		t.Fatal(err)
	}
	return rows
}
func wakeDelivery(t *testing.T, s *Store) {
	t.Helper()
	if _, err := s.db.Exec(`UPDATE delivery_outbox SET next_wake_at=0`); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryTransportRecoveryAndRetainedHistory(t *testing.T) {
	for _, mode := range []string{"success", "retry-once", "ambiguous", "unknown", "unavailable"} {
		t.Run(mode, func(t *testing.T) {
			s, service, adapter := terminalFixture(t)
			ctx := context.Background()
			service.Policy.MaxAttempts = 2
			service.Adapter = &deliveryfake.Adapter{Base: adapter, Mode: mode}
			for attempt := 0; attempt < 4; attempt++ {
				if err := service.Drain(ctx); err != nil {
					t.Fatal(err)
				}
				wakeDelivery(t, s)
			}
			rows := deliveryRows(t, s)
			if len(rows) != 1 {
				t.Fatal(rows)
			}
			want := "sent"
			count := 1
			if mode == "unknown" {
				want = "unknown"
				count = 0
			}
			if mode == "unavailable" {
				want = "failed"
				count = 0
			}
			if rows[0].Status != want {
				t.Fatal(rows)
			}
			inbox, err := adapter.Messages(ctx, service.Destination)
			if err != nil || len(inbox) != count {
				t.Fatal(inbox, err)
			}
			if count > 0 && (inbox[0].Request.DestinationRef != service.Destination || inbox[0].Message.MessageID != rows[0].MessageID) {
				t.Fatal("wrong destination/message", inbox)
			}
			if mode == "unknown" {
				if rows[0].AttemptCount != 1 {
					t.Fatal("unknown sent again", rows)
				}
				service.Adapter = adapter // authoritative absence still cannot retry unknown
				wakeDelivery(t, s)
				if err := service.Drain(ctx); err != nil {
					t.Fatal(err)
				}
				if deliveryRows(t, s)[0].Status != "unknown" {
					t.Fatal("unknown retried")
				}
			}
			if mode == "unavailable" && rows[0].AttemptCount != 2 {
				t.Fatal("unbounded retries", rows)
			}
			before, err := service.History.List(ctx, service.Destination)
			if err != nil {
				t.Fatal(err)
			}
			if err := service.Drain(ctx); err != nil {
				t.Fatal(err)
			}
			after, _ := service.History.List(ctx, service.Destination)
			if len(before) != 2 || len(after) != 2 || before[1].MessageID != after[1].MessageID {
				t.Fatal("send rewrote semantic history")
			}
			if want == "sent" || want == "failed" {
				if scalar(t, s, `SELECT count(*) FROM delivery_outbox`) != 0 {
					t.Fatal("terminal outbox retained")
				}
			}
		})
	}
}

func TestDeliveryClaimFencesLateAckAndQueriesAcceptedSend(t *testing.T) {
	s, service, adapter := terminalFixture(t)
	ctx := context.Background()
	now := time.Now()
	c, err := s.Deliveries().Claim(ctx, "tui", service.Destination, nil, now, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	stale := c
	stale.Request.Version--
	if err = s.Deliveries().Settle(ctx, stale, delivery.Observation{Outcome: delivery.Failed}, time.Now(), time.Now()); !errors.Is(err, delivery.ErrConflict) {
		t.Fatal("stale version accepted", err)
	}
	if _, err = s.Deliveries().Claim(ctx, "tui", service.Destination, nil, now, time.Second); !errors.Is(err, delivery.ErrEmpty) {
		t.Fatal("duplicate owner", err)
	}
	messages, _ := service.History.List(ctx, service.Destination)
	e, err := delivery.Compose(c.Request, messages[1])
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Send(ctx, e).Outcome != delivery.Delivered {
		t.Fatal("send failed")
	}
	if _, err = s.db.Exec(`UPDATE delivery_outbox SET claim_expires_at=0`); err != nil {
		t.Fatal(err)
	}
	if err = service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Deliveries().Settle(ctx, c, delivery.Observation{Outcome: delivery.Failed}, time.Now(), time.Now()); !errors.Is(err, delivery.ErrConflict) {
		t.Fatal("late acknowledgement accepted", err)
	}
	if deliveryRows(t, s)[0].Status != "sent" {
		t.Fatal("accepted send lost")
	}
	if deliveryRows(t, s)[0].AttemptCount != 1 {
		t.Fatal("query counted as another send attempt")
	}
	if adapter.Send(ctx, e).Outcome != delivery.Delivered {
		t.Fatal("duplicate provider callback not idempotent")
	}
	e.Request.DestinationRef = "someone-else"
	if adapter.Send(ctx, e).Outcome != delivery.Failed {
		t.Fatal("changed destination accepted")
	}
	inbox, _ := adapter.Messages(ctx, service.Destination)
	if len(inbox) != 1 {
		t.Fatal("duplicate semantic send", inbox)
	}
}

func TestDeliveryCrashBeforeSendReattachesLastAllowedAttempt(t *testing.T) {
	s, service, _ := terminalFixture(t)
	ctx := context.Background()
	service.Policy.MaxAttempts = 1
	if _, err := s.Deliveries().Claim(ctx, "tui", service.Destination, nil, time.Now(), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE delivery_outbox SET claim_expires_at=0`); err != nil {
		t.Fatal(err)
	}
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	rows := deliveryRows(t, s)
	if rows[0].Status != "sent" || rows[0].AttemptCount != 1 {
		t.Fatal("recovery exhausted unsent attempt", rows)
	}
}

func TestDeliveryAckEventFailureRecoversAcceptedReceipt(t *testing.T) {
	s, service, adapter := terminalFixture(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER reject_sent_event BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.delivery.sent.v1' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.Drain(ctx); !errors.Is(err, delivery.ErrStorage) {
		t.Fatal("ack event failure swallowed", err)
	}
	inbox, _ := adapter.Messages(ctx, service.Destination)
	if len(inbox) != 1 || deliveryRows(t, s)[0].Status != "sending" || scalar(t, s, `SELECT count(*) FROM delivery_outbox`) != 1 {
		t.Fatal("partial ack or accepted send lost")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_sent_event; UPDATE delivery_outbox SET claim_expires_at=0`); err != nil {
		t.Fatal(err)
	}
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	inbox, _ = adapter.Messages(ctx, service.Destination)
	if len(inbox) != 1 || deliveryRows(t, s)[0].Status != "sent" || deliveryRows(t, s)[0].AttemptCount != 1 {
		t.Fatal("ack recovery repeated send")
	}
}

func TestTerminalAcceptanceRollsBackWithItsEvent(t *testing.T) {
	s, service, adapter := terminalFixture(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`CREATE TRIGGER reject_accepted_event BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.delivery.accepted.v1' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	inbox, _ := adapter.Messages(ctx, service.Destination)
	if len(inbox) != 0 || deliveryRows(t, s)[0].Status != "retry" {
		t.Fatal("partial terminal acceptance")
	}
	if _, err := s.db.Exec(`DROP TRIGGER reject_accepted_event`); err != nil {
		t.Fatal(err)
	}
	wakeDelivery(t, s)
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	inbox, _ = adapter.Messages(ctx, service.Destination)
	if len(inbox) != 1 || deliveryRows(t, s)[0].Status != "sent" || scalar(t, s, `SELECT count(*) FROM event_outbox WHERE type='ai.erii.delivery.accepted.v1'`) != 1 {
		t.Fatal("terminal acceptance did not recover atomically")
	}
}

func TestDeliveryEnqueueIdentityRollbackAndTransitions(t *testing.T) {
	s, service, _ := terminalFixture(t)
	ctx := context.Background()
	r := deliveryRows(t, s)[0]
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return deliverysqlite.Enqueue(ctx, tx, r) }); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"destination", "message", "intent", "key"} {
		changed := r
		switch field {
		case "destination":
			changed.DestinationRef = "other"
		case "message":
			changed.MessageID = "other"
		case "intent":
			changed.Intent = "progress"
		case "key":
			changed.IdempotencyKey = "model-key"
		}
		if err := s.Transaction(ctx, func(tx *sql.Tx) error { return deliverysqlite.Enqueue(ctx, tx, changed) }); !errors.Is(err, delivery.ErrConflict) {
			t.Fatal("drift accepted", field, err)
		}
	}
	other := r
	other.DeliveryID = "delivery:new"
	other.MessageID = "message:new"
	other.IdempotencyKey = other.DeliveryID
	if _, err := s.db.Exec(`CREATE TRIGGER reject_delivery_event BEFORE INSERT ON event_outbox WHEN NEW.type='ai.erii.delivery.pending.v1' BEGIN SELECT RAISE(ABORT,'injected'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return deliverysqlite.Enqueue(ctx, tx, other) }); err == nil {
		t.Fatal("event failure swallowed")
	}
	if len(deliveryRows(t, s)) != 1 || scalar(t, s, `SELECT count(*) FROM delivery_outbox`) != 1 {
		t.Fatal("partial enqueue")
	}
	s.db.Exec(`DROP TRIGGER reject_delivery_event`)
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{`UPDATE delivery_request SET status='pending'`, `UPDATE delivery_request SET destination_ref='other'`, `DELETE FROM delivery_request`, `INSERT OR REPLACE INTO delivery_request SELECT * FROM delivery_request`} {
		if _, err := s.db.Exec(query); err == nil {
			t.Fatal("illegal transition", query)
		}
	}
}

func TestDeliveryConcurrentConsumersAndDestinations(t *testing.T) {
	s, service, adapter := terminalFixture(t)
	ctx := context.Background()
	wrong := service
	wrong.Destination = "different-conversation"
	if err := wrong.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if deliveryRows(t, s)[0].Status != "pending" {
		t.Fatal("wrong destination consumed")
	}
	var wg sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); failures <- service.Drain(ctx) }()
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	items, _ := adapter.Messages(ctx, service.Destination)
	if len(items) != 1 || deliveryRows(t, s)[0].AttemptCount != 1 {
		t.Fatal("concurrent duplicate", items)
	}
}

func TestDeliveryMigrationV11PreservesReceiptsAndRollsBack(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(ctx, db, migrations[:11]); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO agent_config VALUES(1,'prior',NULL,'setup_required',7,100,200);
INSERT INTO delivery_request VALUES('d1','m1','local_cli','prior','pending','d1',0,NULL,100,100),('d2','m2','local_cli','prior','sent','d2',1,NULL,100,200),('d3','m3','remote','other','failed','d3',2,'rejected',100,300)`); err != nil {
		t.Fatal(err)
	}
	broken := append(append([]string{}, migrations[:11]...), deliverySQL+`; INVALID DDL`)
	if migrate(ctx, db, broken) == nil {
		t.Fatal("broken migration succeeded")
	}
	var version, count int
	db.QueryRow(`PRAGMA user_version`).Scan(&version)
	db.QueryRow(`SELECT count(*) FROM delivery_request`).Scan(&count)
	if version != 11 || count != 3 {
		t.Fatal("partial migration", version, count)
	}
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if scalar(t, s, `PRAGMA user_version`) != 17 || scalar(t, s, `SELECT count(*) FROM delivery_outbox`) != 1 || scalar(t, s, `SELECT config_version FROM agent_config`) != 7 {
		t.Fatal("migration state")
	}
	rows := deliveryRows(t, s)
	if len(rows) != 2 || rows[0].Channel != "tui" || rows[1].Status != "sent" || rows[1].AttemptCount != 1 {
		t.Fatal(rows)
	}
}

func TestDeliveryComposeRejectsWrongSemanticSource(t *testing.T) {
	_, service, _ := terminalFixture(t)
	ctx := context.Background()
	messages, _ := service.History.List(ctx, service.Destination)
	r := delivery.Request{DeliveryID: "d", MessageID: messages[0].MessageID, ConversationID: service.Destination, Channel: "tui", DestinationRef: service.Destination, Intent: "approval", IdempotencyKey: "d", CreatedAt: 1}
	if _, err := delivery.Compose(r, messages[0]); err == nil {
		t.Fatal("user message sent as assistant")
	}
	r.MessageID = messages[1].MessageID
	m := messages[1]
	m.Content = []conversation.ContentPart{{Type: "tool_call", ToolCall: &conversation.ToolCall{ID: "call", Name: "test", Arguments: "{}"}}}
	if _, err := delivery.Compose(r, m); err == nil {
		t.Fatal("Tool trace delivered")
	}
}

type uncertainTransport struct {
	sends       int
	observation delivery.Observation
}

func (*uncertainTransport) Idempotent() bool { return false }
func (a *uncertainTransport) Query(context.Context, delivery.Envelope) delivery.Observation {
	return a.observation
}
func (a *uncertainTransport) Send(context.Context, delivery.Envelope) delivery.Observation {
	a.sends++
	return delivery.Observation{Outcome: delivery.Delivered}
}

func TestDeliveryExpiredSenderWithoutIdempotencyRemainsUnknown(t *testing.T) {
	s, service, _ := terminalFixture(t)
	ctx := context.Background()
	if _, err := s.Deliveries().Claim(ctx, "tui", service.Destination, nil, time.Now(), time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE delivery_outbox SET claim_expires_at=0`); err != nil {
		t.Fatal(err)
	}
	transport := &uncertainTransport{observation: delivery.Observation{Outcome: delivery.Retry, Code: "unsafe raw provider response"}}
	service.Adapter = transport
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if deliveryRows(t, s)[0].Status != "unknown" || transport.sends != 0 {
		t.Fatal("absence authorized overlapping sender")
	}
	transport.observation = delivery.Observation{Outcome: delivery.Delivered}
	wakeDelivery(t, s)
	if err := service.Drain(ctx); err != nil {
		t.Fatal(err)
	}
	if deliveryRows(t, s)[0].Status != "sent" || transport.sends != 0 {
		t.Fatal("authoritative query did not settle unknown")
	}
}

func TestDeliveryMainFailureAndNeedsInputNotices(t *testing.T) {
	ctx := context.Background()
	t.Run("failure", func(t *testing.T) {
		s, r, _, a, p := loopFixture(t)
		p.respond = func(llm.GenerationRequest) (llm.GenerationResult, error) {
			return llm.GenerationResult{}, llm.ModelNotFound
		}
		mustSubmit(t, a, "failure", "Please finish this work")
		if err := a.Drain(ctx); !errors.Is(err, llm.ModelNotFound) {
			t.Fatal(err)
		}
		if err := r.Flush(ctx); err != nil {
			t.Fatal(err)
		}
		requests := deliveryRows(t, s)
		if len(requests) != 1 || requests[0].Intent != "failure" {
			t.Fatal(requests)
		}
		messages, _ := r.history.List(ctx, idAgent(t, s))
		if len(messages) != 2 || messages[1].Content[0].Text != "The request could not be completed. Please retry explicitly." {
			t.Fatal(messages)
		}
		if err := a.Drain(ctx); err != nil {
			t.Fatal(err)
		}
		r.Flush(ctx)
		if len(deliveryRows(t, s)) != 1 {
			t.Fatal("failed Run repeatedly notified")
		}
	})
	t.Run("needs_input", func(t *testing.T) {
		s, r, _, a, _ := loopFixture(t)
		receipt := mustSubmit(t, a, "one", "Need input")
		c, err := r.Claim(ctx, receipt.RunID)
		if err != nil {
			t.Fatal(err)
		}
		run, err := r.Load(ctx, c)
		if err != nil {
			t.Fatal(err)
		}
		if err = r.Wait(ctx, c, run.Version, "user_input"); err != nil {
			t.Fatal(err)
		}
		if err = r.Flush(ctx); err != nil {
			t.Fatal(err)
		}
		requests := deliveryRows(t, s)
		if len(requests) != 1 || requests[0].Intent != "needs_input" {
			t.Fatal(requests)
		}
	})
}

func TestTerminalProjectionHidesUnsentAndRetainsPriorSentHistory(t *testing.T) {
	s, service, adapter := terminalFixture(t)
	ctx := context.Background()
	messages, _ := service.History.List(ctx, service.Destination)
	projected, err := adapter.History(ctx, service.Destination, messages, deliveryRows(t, s))
	if err != nil || len(projected) != 1 || projected[0].Role != "user" {
		t.Fatal("unaccepted output visible", projected, err)
	}
	requests := deliveryRows(t, s)
	requests[0].Status = "sent" // retained v11 receipt, no projection row
	projected, err = adapter.History(ctx, service.Destination, messages, requests)
	if err != nil || len(projected) != 2 {
		t.Fatal("prior accepted history disappeared", projected, err)
	}
	requests[0].DestinationRef = "another-user"
	projected, err = adapter.History(ctx, service.Destination, messages, requests)
	if err != nil || len(projected) != 1 {
		t.Fatal("cross-destination history", projected, err)
	}
}
