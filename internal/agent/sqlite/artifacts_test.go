package sqlite

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/z-chenhao/erii/internal/agent"
	"github.com/z-chenhao/erii/internal/artifact"
	artifacttools "github.com/z-chenhao/erii/internal/artifact/tools"
	"github.com/z-chenhao/erii/internal/conversation"
	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/tool"
	toolfake "github.com/z-chenhao/erii/internal/tool/fake"
)

func artifactOwner(t *testing.T, s *Store) artifact.Scope {
	t.Helper()
	return artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), Owner: true}
}
func putArtifact(t *testing.T, s *Store, a artifact.Scope, text string) string {
	t.Helper()
	ref, err := s.Artifacts().Put(context.Background(), a, artifact.Input{Name: "input.txt", MediaType: "text/plain"}, strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	return ref
}
func readArtifact(t *testing.T, s *Store, a artifact.Scope, ref string) string {
	t.Helper()
	f, err := s.Artifacts().Open(context.Background(), a, ref)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
func TestArtifactStoreContractScopeIntegrityAndTombstone(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	owner := artifactOwner(t, s)
	ref := putArtifact(t, s, owner, "original bytes")
	if readArtifact(t, s, owner, ref) != "original bytes" {
		t.Fatal("bytes changed")
	}
	d, err := s.Artifacts().Describe(ctx, owner, ref)
	sum := sha256.Sum256([]byte("original bytes"))
	if err != nil || d.ContentHash != hex.EncodeToString(sum[:]) || d.Version != 1 {
		t.Fatal(d, err)
	}
	for _, a := range []artifact.Scope{{UserID: "other", AgentID: owner.AgentID, Owner: true}, {UserID: artifact.LocalUser, AgentID: "other", Owner: true}, {UserID: artifact.LocalUser, AgentID: owner.AgentID}} {
		if _, err = s.Artifacts().Open(ctx, a, ref); !errors.Is(err, artifact.ErrNotFound) {
			t.Fatal("scope admitted", a, err)
		}
	}
	for _, forged := range []string{"https://example.test/file", "/etc/passwd", "artifact:../../outside", ref + "/../secret", ref[:len(ref)-1] + "g", "artifact:" + strings.Repeat("0", 64)} {
		if _, err = s.Artifacts().Open(ctx, owner, forged); !errors.Is(err, artifact.ErrNotFound) {
			t.Fatal("forged ref", forged, err)
		}
	}
	for _, query := range []string{`UPDATE artifact SET name='new'`, `UPDATE artifact SET content_hash='new'`, `DELETE FROM artifact`, `INSERT OR REPLACE INTO artifact SELECT * FROM artifact`} {
		if _, err = s.db.Exec(query); err == nil {
			t.Fatal("mutation accepted", query)
		}
	}
	if err = s.Artifacts().Delete(ctx, owner, ref, 2); !errors.Is(err, artifact.ErrConflict) {
		t.Fatal(err)
	}
	if err = s.Artifacts().Delete(ctx, owner, ref, 1); err != nil {
		t.Fatal(err)
	}
	d, err = s.Artifacts().Describe(ctx, owner, ref)
	if err != nil || d.DeletedAt == nil || d.Version != 2 {
		t.Fatal(d, err)
	}
	if _, err = s.Artifacts().Open(ctx, owner, ref); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal(err)
	}
	report, err := s.Artifacts().Recover(ctx)
	if err != nil || report.Objects != 1 {
		t.Fatal(report, err)
	}
	if scalar(t, s, `SELECT count(*) FROM artifact`) != 1 || scalar(t, s, `SELECT count(*) FROM event_outbox WHERE type LIKE 'ai.erii.artifact.%'`) != 2 {
		t.Fatal("audit missing")
	}
}
func TestArtifactTamperSymlinkAndTraversal(t *testing.T) {
	for _, mode := range []string{"bytes", "symlink", "storage_key", "directory"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			s := openModels(t)
			a := artifactOwner(t, s)
			ref := putArtifact(t, s, a, "keep")
			var key string
			s.db.QueryRow(`SELECT storage_key FROM artifact`).Scan(&key)
			path := filepath.Join(filepath.Dir(s.path), "artifacts", key)
			switch mode {
			case "bytes":
				os.Chmod(path, 0600)
				if err := os.WriteFile(path, []byte("evil"), 0600); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				outside := filepath.Join(t.TempDir(), "private")
				os.WriteFile(outside, []byte("keep"), 0600)
				os.Remove(path)
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				os.Remove(path)
				os.Mkdir(path, 0700)
			case "storage_key":
				s.db.Exec(`DROP TRIGGER artifact_immutable`)
				s.db.Exec(`UPDATE artifact SET storage_key='../state.db'`)
			}
			if _, err := s.Artifacts().Open(ctx, a, ref); !errors.Is(err, artifact.ErrIntegrity) {
				t.Fatal("unsafe object accepted", err)
			}
		})
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "artifacts")); err != nil {
		t.Fatal(err)
	}
	if s, err := Open(context.Background(), root); err == nil {
		s.Close()
		t.Fatal("symlink root accepted")
	}
}
func TestArtifactQuotaRetentionAndReferences(t *testing.T) {
	ctx := context.Background()
	s := openModels(t)
	a := artifactOwner(t, s)
	p := s.Artifacts()
	p.Policy.MaxBytes = 5
	p.Policy.QuotaBytes = 8
	if _, err := p.Put(ctx, a, artifact.Input{Name: "ok", MediaType: "text/plain"}, strings.NewReader("123456")); !errors.Is(err, artifact.ErrLimit) {
		t.Fatal(err)
	}
	ref := putArtifact(t, s, a, "12345")
	if _, err := p.Put(ctx, a, artifact.Input{Name: "../bad", MediaType: "text/plain"}, strings.NewReader("1")); !errors.Is(err, artifact.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := p.Put(ctx, a, artifact.Input{Name: "ok", MediaType: "text/plain"}, strings.NewReader("1234")); !errors.Is(err, artifact.ErrLimit) {
		t.Fatal(err)
	}
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return p.PinTx(ctx, tx, a, ref, "conversation", "message") }); err != nil {
		t.Fatal(err)
	}
	if err := p.Delete(ctx, a, ref, 1); !errors.Is(err, artifact.ErrReferenced) {
		t.Fatal(err)
	}
	// Controlled clock fixture: the production schema never permits expiry edits.
	s.db.Exec(`DROP TRIGGER artifact_immutable`)
	s.db.Exec(`UPDATE artifact SET expires_at=1`)
	if _, err := p.Open(ctx, a, ref); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("expired read", err)
	}
	report, err := p.Cleanup(ctx, a, true)
	if err != nil || len(report.Artifacts) != 0 {
		t.Fatal("referenced cleanup", report, err)
	}
	s.db.Exec(`DELETE FROM artifact_reference`)
	report, err = p.Cleanup(ctx, a, true)
	if err != nil || !reflect.DeepEqual(report.Artifacts, []string{ref}) || scalar(t, s, `SELECT count(*) FROM artifact WHERE deleted_at IS NOT NULL`) != 0 {
		t.Fatal(report, err)
	}
	report, err = p.Cleanup(ctx, a, false)
	if err != nil || len(report.Artifacts) != 1 || report.Objects != 1 {
		t.Fatal(report, err)
	}
	if _, err = p.Describe(ctx, a, ref); err != nil {
		t.Fatal("tombstone unavailable", err)
	}
	entries, _ := os.ReadDir(filepath.Join(filepath.Dir(s.path), "artifacts"))
	if len(entries) != 0 {
		t.Fatal("temporary quota objects leaked", entries)
	}
}
func TestArtifactToolAtomicResultReplayPublicationAndFence(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []string{"binary", "large"} {
		t.Run(kind, func(t *testing.T) {
			s, r, c, e, exchange := admitPolicy(t, toolfake.Artifacts(), []llm.ToolCall{{ID: "output", Name: "mcp__fixture__" + kind, Arguments: `{}`}})
			out, err := e.Execute(ctx, exchange, []llm.ToolCall{{ID: "output", Name: "mcp__fixture__" + kind, Arguments: `{}`}})
			if err != nil || len(out) != 1 || len(out[0].ArtifactRefs) != 1 || len(out[0].ModelText()) > 1024 {
				t.Fatal(out, err)
			}
			ref := out[0].ArtifactRefs[0]
			if len(readArtifact(t, s, artifactOwner(t, s), ref)) < 16384 {
				t.Fatal("large output lost")
			}
			again, err := e.Execute(ctx, exchange, []llm.ToolCall{{ID: "output", Name: "mcp__fixture__" + kind, Arguments: `{}`}})
			if err != nil || !reflect.DeepEqual(again, out) || scalar(t, s, `SELECT count(*) FROM artifact`) != 1 {
				t.Fatal(again, err)
			}
			if scalar(t, s, `SELECT count(*) FROM artifact_publication`) != 0 {
				t.Fatal("intermediate auto-published")
			}
			if err = r.Release(ctx, c); err != nil {
				t.Fatal(err)
			}
			if _, err = e.Store.(tool.OutputStore).FinishResult(ctx, "old", tool.Result{Text: strings.Repeat("x", 65536)}); !errors.Is(err, agent.ErrClaimLost) {
				t.Fatal("old owner admitted", err)
			}
		})
	}
	s, _, _, e, exchange := admitPolicy(t, artifacttools.Definitions(), []llm.ToolCall{{ID: "save", Name: artifacttools.Save, Arguments: `{"name":"report.md","mediaType":"text/markdown","text":"confirmed"}`}})
	s.db.Exec(`CREATE TRIGGER fail_artifact_tool BEFORE UPDATE ON tool_invocation WHEN NEW.status='succeeded' BEGIN SELECT RAISE(ABORT,'fault'); END`)
	_, err := e.Execute(ctx, exchange, []llm.ToolCall{{ID: "save", Name: artifacttools.Save, Arguments: `{"name":"report.md","mediaType":"text/markdown","text":"confirmed"}`}})
	if !errors.Is(err, agent.ErrStorage) || scalar(t, s, `SELECT count(*) FROM artifact`) != 0 {
		t.Fatal("metadata survived failed Tool result", err)
	}
	report, err := s.Artifacts().Recover(ctx)
	if err != nil || report.Objects != 1 {
		t.Fatal(report, err)
	}
}
func TestArtifactTaskVisibilityConcurrentChildWritersAndPublication(t *testing.T) {
	ctx := context.Background()
	s, r, a := controlFixture(t, nil)
	controlMain(t, r, a, "create", "create")
	var taskID, rootID string
	s.db.QueryRow(`SELECT task_id,root_run_id FROM run WHERE kind='worker'`).Scan(&taskID, &rootID)
	if _, err := r.Claim(ctx, rootID); err != nil {
		t.Fatal(err)
	}
	scopes := []artifact.Scope{}
	for i := 0; i < 8; i++ {
		id := newID()
		_, err := s.db.Exec(`INSERT INTO run(run_id,agent_id,conversation_id,task_id,parent_run_id,root_run_id,kind,worker_position,status,task_epoch,depth,capability_snapshot_id,model_snapshot_id,checkpoint_json,history_boundary,created_at,updated_at,version) SELECT ?,agent_id,conversation_id,task_id,run_id,root_run_id,'worker','child','queued',task_epoch,1,capability_snapshot_id,model_snapshot_id,'{}',0,?,?,1 FROM run WHERE run_id=?`, id, time.Now().UnixMicro(), time.Now().UnixMicro(), rootID)
		if err != nil {
			t.Fatal(err)
		}
		scopes = append(scopes, artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), TaskID: taskID, RunID: id})
	}
	refs := make([]string, len(scopes))
	errs := make([]error, len(scopes))
	var wg sync.WaitGroup
	for i := range scopes {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			other, err := Open(ctx, filepath.Dir(s.path))
			if err != nil {
				errs[i] = err
				return
			}
			defer other.Close()
			refs[i], errs[i] = other.Artifacts().Put(ctx, scopes[i], artifact.Input{Name: "same.txt", MediaType: "text/plain"}, strings.NewReader(fmt.Sprint(i)))
		}(i)
	}
	wg.Wait()
	seen := map[string]bool{}
	for i, ref := range refs {
		if errs[i] != nil || seen[ref] {
			t.Fatal(ref, errs[i])
		}
		seen[ref] = true
		if readArtifact(t, s, scopes[(i+1)%len(scopes)], ref) != fmt.Sprint(i) {
			t.Fatal("sibling output overwritten")
		}
	}
	controlMain(t, r, a, "other-task", "create")
	var otherTask, otherRun string
	if err := s.db.QueryRow(`SELECT task_id,run_id FROM run WHERE kind='worker' AND worker_position='root' AND task_id!=?`, taskID).Scan(&otherTask, &otherRun); err != nil {
		t.Fatal(err)
	}
	otherScope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), TaskID: otherTask, RunID: otherRun}
	if _, err := s.Artifacts().Open(ctx, otherScope, refs[0]); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("valid unrelated Task admitted", err)
	}
	// Expiry does not make active/paused Task output a cleanup candidate.
	if _, err := s.db.Exec(`DROP TRIGGER artifact_immutable; UPDATE artifact SET expires_at=1`); err != nil {
		t.Fatal(err)
	}
	report, err := s.Artifacts().Cleanup(ctx, artifactOwner(t, s), true)
	if err != nil || len(report.Artifacts) != 0 {
		t.Fatal("active Task collected", report, err)
	}
	if _, err = s.db.Exec(`UPDATE task SET status='paused',task_epoch=task_epoch+1,version=version+1 WHERE task_id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	report, err = s.Artifacts().Cleanup(ctx, artifactOwner(t, s), false)
	if err != nil || len(report.Artifacts) != 0 {
		t.Fatal("paused Task collected", report, err)
	}
	if _, err = s.db.Exec(`UPDATE artifact SET expires_at=?`, time.Now().Add(time.Hour).UnixMicro()); err != nil {
		t.Fatal(err)
	}
	cross := scopes[0]
	cross.TaskID = "forged-task"
	if _, err := s.Artifacts().Open(ctx, cross, refs[0]); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("cross Task read", err)
	}
	main := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s)}
	s.db.QueryRow(`SELECT run_id FROM run WHERE kind='main' LIMIT 1`).Scan(&main.RunID)
	if _, err := s.Artifacts().Open(ctx, main, refs[0]); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("Main received unpublished Child output", err)
	}
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return s.Artifacts().PublishTx(ctx, tx, scopes[0], []string{refs[0]}) }); err != nil {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM artifact_publication`) != 1 {
		t.Fatal("wrong publication count")
	}
}
func TestArtifactMigrationV10Atomicity(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(root, "state.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = migrate(ctx, db, migrations[:10]); err != nil {
		t.Fatal(err)
	}
	db.Exec(`INSERT INTO agent_config VALUES(1,'prior',NULL,'setup_required',7,100,200)`)
	broken := append(append([]string{}, migrations[:10]...), artifactsSQL+`; INVALID DDL`)
	if err = migrate(ctx, db, broken); err == nil {
		t.Fatal("invalid migration succeeded")
	}
	var version, count int
	db.QueryRow(`PRAGMA user_version`).Scan(&version)
	db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE name='artifact'`).Scan(&count)
	if version != 10 || count != 0 {
		t.Fatal("partial migration", version, count)
	}
	s, err := Open(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if idAgent(t, s) != "prior" || scalar(t, s, `PRAGMA user_version`) != 17 || scalar(t, s, `SELECT config_version FROM agent_config`) != 7 {
		t.Fatal("prior state changed")
	}
}

func TestArtifactMainRunPrivacyAndInputGrant(t *testing.T) {
	ctx := context.Background()
	s, r, _, a, _ := loopFixture(t)
	receipt := mustSubmit(t, a, "first", "first")
	scope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), RunID: receipt.RunID}
	ref := putArtifact(t, s, scope, "private Run output")
	if readArtifact(t, s, scope, ref) != "private Run output" {
		t.Fatal("own output unavailable")
	}
	mustDrain(t, a)
	second := mustSubmit(t, a, "second", "second")
	other := scope
	other.RunID = second.RunID
	if _, err := s.Artifacts().Open(ctx, other, ref); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("unpublished Run output crossed Run", err)
	}
	before := scalar(t, s, `SELECT count(*) FROM main_input`)
	if _, err := r.Submit(ctx, "tool-injection", []conversation.ContentPart{{Type: "tool_call", ToolCall: &conversation.ToolCall{ID: "forged", Name: "builtin__artifact__publish", Arguments: "{}"}}}); !errors.Is(err, conversation.ErrInvalid) {
		t.Fatal("user tool call accepted", err)
	}
	if _, err := r.Submit(ctx, "forged", []conversation.ContentPart{{Type: "file", ArtifactRef: "artifact:" + strings.Repeat("0", 64)}}); !errors.Is(err, conversation.ErrInvalid) {
		t.Fatal(err)
	}
	if scalar(t, s, `SELECT count(*) FROM main_input`) != before {
		t.Fatal("forged input persisted")
	}
	if _, err := r.Submit(ctx, "grant", []conversation.ContentPart{{Type: "file", ArtifactRef: ref}}); err != nil {
		t.Fatal(err)
	}
	if readArtifact(t, s, other, ref) != "private Run output" {
		t.Fatal("explicit input grant failed")
	}
	mustDrain(t, a)
	third := mustSubmit(t, a, "third", "third")
	other.RunID = third.RunID
	if readArtifact(t, s, other, ref) != "private Run output" {
		t.Fatal("Conversation ref could not reopen in new Main")
	}
}

func TestArtifactProviderCannotSelectFinalPublication(t *testing.T) {
	ctx := context.Background()
	ref := ""
	def := tool.Definition{Spec: tool.Spec{Name: "mcp__fixture__hidden_publish", Version: 1, InputSchema: `{"type":"object","additionalProperties":false}`, ReplayPolicy: "safe", Permission: "read_only"}, Execute: func(context.Context, string) tool.Result { return tool.Result{PublishRefs: []string{ref}} }}
	call := []llm.ToolCall{{ID: "hidden", Name: def.Spec.Name, Arguments: `{}`}}
	s, _, c, e, exchange := admitPolicy(t, []tool.Definition{def}, call)
	ref = putArtifact(t, s, artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), RunID: c.RunID()}, "output")
	results, err := e.Execute(ctx, exchange, call)
	if err != nil || len(results) != 1 || results[0].Error == nil || results[0].Error.Code != "PERMISSION_DENIED" || scalar(t, s, `SELECT count(*) FROM artifact_publication`) != 0 {
		t.Fatal("provider selected final output without explicit publish", results, err)
	}
}

func TestArtifactPublicationValidatesSetBeforeMutation(t *testing.T) {
	ctx := context.Background()
	s, _, _, a, _ := loopFixture(t)
	receipt := mustSubmit(t, a, "publish", "publish")
	scope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), RunID: receipt.RunID}
	good := putArtifact(t, s, scope, "valid")
	bad := putArtifact(t, s, scope, "original")
	var key string
	if err := s.db.QueryRow(`SELECT storage_key FROM artifact WHERE artifact_ref=?`, bad).Scan(&key); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(filepath.Dir(s.path), "artifacts", key)
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, refs := range [][]string{{good, bad}, {good, "artifact:" + strings.Repeat("0", 64)}} {
		if err := s.Transaction(ctx, func(tx *sql.Tx) error {
			err := s.Artifacts().PublishTx(ctx, tx, scope, refs)
			if !errors.Is(err, artifact.ErrIntegrity) && !errors.Is(err, artifact.ErrNotFound) {
				return fmt.Errorf("invalid set accepted: %v", err)
			}
			// The Tool layer can turn a validation error into a failed result and
			// commit, without accidentally retaining a partial output selection.
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if scalar(t, s, `SELECT count(*) FROM artifact_publication`) != 0 || scalar(t, s, `SELECT count(*) FROM artifact_reference`) != 0 {
		t.Fatal("partial publication survived validation failure")
	}
}

func TestArtifactTaskLifetimeAndRetentionAfterTaskEnd(t *testing.T) {
	ctx := context.Background()
	s, r, a := controlFixture(t, nil)
	controlMain(t, r, a, "create", "create")
	var runID, taskID string
	if err := s.db.QueryRow(`SELECT run_id,task_id FROM run WHERE kind='worker'`).Scan(&runID, &taskID); err != nil {
		t.Fatal(err)
	}
	scope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), TaskID: taskID, RunID: runID}
	ref := putArtifact(t, s, scope, "recoverable Task output")
	// Advance the synthetic object clock two hours, with a frozen one-hour TTL.
	now := time.Now().UnixMicro()
	hour := time.Hour.Microseconds()
	if _, err := s.db.Exec(`DROP TRIGGER artifact_immutable; UPDATE artifact SET created_at=?,expires_at=?`, now-2*hour, now-hour); err != nil {
		t.Fatal(err)
	}
	for _, status := range []string{"active", "paused"} {
		if status == "paused" {
			if _, err := s.db.Exec(`UPDATE task SET status='paused',task_epoch=task_epoch+1,version=version+1 WHERE task_id=?`, taskID); err != nil {
				t.Fatal(err)
			}
		}
		if readArtifact(t, s, scope, ref) != "recoverable Task output" {
			t.Fatal(status, "lost output")
		}
		d, err := s.Artifacts().Describe(ctx, scope, ref)
		if err != nil || d.ExpiresAt != nil {
			t.Fatal("protected object has finite expiry", d, err)
		}
		if err := s.Transaction(ctx, func(tx *sql.Tx) error {
			list, err := s.Artifacts().ListTx(ctx, tx, scope)
			if err != nil {
				return err
			}
			if len(list) != 1 {
				return fmt.Errorf("protected object omitted from list: %v", list)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`UPDATE task SET status='active',task_epoch=task_epoch+1,version=version+1 WHERE task_id=?`, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE task SET status='completed',completed_at=?,version=version+1 WHERE task_id=?`, now, taskID); err != nil {
		t.Fatal(err)
	}
	d, err := s.Artifacts().Describe(ctx, scope, ref)
	if err != nil || d.ExpiresAt == nil || d.ExpiresAt.UnixMicro() != now+hour {
		t.Fatal("retention did not start after Task", d, err)
	}
	if readArtifact(t, s, scope, ref) != "recoverable Task output" {
		t.Fatal("retention read failed")
	}
	report, err := s.Artifacts().Cleanup(ctx, artifactOwner(t, s), true)
	if err != nil || len(report.Artifacts) != 0 {
		t.Fatal("retained Task output collected", report, err)
	}
	if _, err := s.db.Exec(`UPDATE task SET completed_at=?,version=version+1 WHERE task_id=?`, now-2*hour, taskID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Artifacts().Open(ctx, scope, ref); !errors.Is(err, artifact.ErrNotFound) {
		t.Fatal("expired Task output opened", err)
	}
	report, err = s.Artifacts().Cleanup(ctx, artifactOwner(t, s), false)
	if err != nil || len(report.Artifacts) != 1 {
		t.Fatal("expired unreferenced output not collected", report, err)
	}
}

type slowArtifactReader struct{ read bool }

func (r *slowArtifactReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	time.Sleep(50 * time.Millisecond)
	p[0] = 'x'
	return 1, nil
}
func TestArtifactBinaryOutputHonorsFrozenToolDeadline(t *testing.T) {
	def := tool.Definition{Spec: tool.Spec{Name: "mcp__fixture__slow_binary", Version: 1, InputSchema: `{"type":"object","additionalProperties":false}`, Permission: "read_only", ReplayPolicy: "safe", TimeoutMS: 5}, Execute: func(context.Context, string) tool.Result {
		return tool.Result{Output: &tool.BinaryOutput{Name: "slow.bin", MediaType: "application/octet-stream", Reader: &slowArtifactReader{}}}
	}}
	call := []llm.ToolCall{{ID: "slow", Name: def.Spec.Name, Arguments: `{}`}}
	s, _, _, e, exchange := admitPolicy(t, []tool.Definition{def}, call)
	results, err := e.Execute(context.Background(), exchange, call)
	if err != nil || len(results) != 1 || results[0].Error == nil || results[0].Error.Code != "TIMEOUT" {
		t.Fatal("binary output bypassed frozen timeout", results, err)
	}
	if scalar(t, s, `SELECT count(*) FROM artifact`) != 0 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE status='failed'`) != 1 {
		t.Fatal("timeout published bytes or did not settle Tool")
	}
}

type blockedArtifactReader struct {
	closed, started chan struct{}
	once, start     sync.Once
}

func (r *blockedArtifactReader) Read([]byte) (int, error) {
	r.start.Do(func() { close(r.started) })
	<-r.closed
	return 0, errors.New("closed stream")
}
func (r *blockedArtifactReader) Close() error { r.once.Do(func() { close(r.closed) }); return nil }
func TestArtifactDeadlineClosesBlockedStream(t *testing.T) {
	for _, kind := range []string{"deadline", "cancel"} {
		t.Run(kind, func(t *testing.T) {
			reader := &blockedArtifactReader{closed: make(chan struct{}), started: make(chan struct{})}
			timeout := 20
			if kind == "cancel" {
				timeout = 5000
			}
			def := tool.Definition{Spec: tool.Spec{Name: "mcp__fixture__blocked_binary", Version: 1, InputSchema: `{"type":"object","additionalProperties":false}`, Permission: "read_only", ReplayPolicy: "safe", TimeoutMS: timeout}, Execute: func(context.Context, string) tool.Result {
				return tool.Result{Output: &tool.BinaryOutput{Name: "blocked.bin", MediaType: "application/octet-stream", Reader: reader}}
			}}
			call := []llm.ToolCall{{ID: "blocked", Name: def.Spec.Name, Arguments: `{}`}}
			s, _, _, e, exchange := admitPolicy(t, []tool.Definition{def}, call)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			type outcome struct {
				results []tool.Result
				err     error
			}
			done := make(chan outcome, 1)
			go func() { result, err := e.Execute(ctx, exchange, call); done <- outcome{result, err} }()
			// Always unblock and join the test's execution, even if the regression
			// breaks automatic stream closure. A failed contract cannot orphan work.
			var got outcome
			if kind == "cancel" {
				select {
				case <-reader.started:
					cancel()
				case got = <-done:
					t.Fatal("stream never started", got)
				case <-time.After(time.Second):
					cancel()
					reader.Close()
					<-done
					t.Fatal("stream admission stalled")
				}
			}
			select {
			case got = <-done:
			case <-time.After(time.Second):
				cancel()
				reader.Close()
				<-done
				t.Fatal("stream cancellation did not unblock Read")
			}
			expected := "TIMEOUT"
			if kind == "cancel" {
				expected = "CANCELLED"
			}
			if got.err != nil || len(got.results) != 1 || got.results[0].Error == nil || got.results[0].Error.Code != expected {
				t.Fatal(got.results, got.err)
			}
			select {
			case <-reader.closed:
			default:
				t.Fatal("stream not closed")
			}
			if scalar(t, s, `SELECT count(*) FROM artifact`) != 0 {
				t.Fatal("interrupted stream committed")
			}
		})
	}
}
func TestArtifactExecutorClosesConsumedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "source.bin")
	if err := os.WriteFile(path, []byte("file output"), 0600); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	def := tool.Definition{Spec: tool.Spec{Name: "mcp__fixture__file_binary", Version: 1, InputSchema: `{"type":"object","additionalProperties":false}`, Permission: "read_only", ReplayPolicy: "safe"}, Execute: func(context.Context, string) tool.Result {
		return tool.Result{Output: &tool.BinaryOutput{Name: "source.bin", MediaType: "application/octet-stream", Reader: f}}
	}}
	call := []llm.ToolCall{{ID: "file", Name: def.Spec.Name, Arguments: `{}`}}
	s, _, _, e, exchange := admitPolicy(t, []tool.Definition{def}, call)
	result, err := e.Execute(context.Background(), exchange, call)
	if err != nil || len(result) != 1 || len(result[0].ArtifactRefs) != 1 {
		t.Fatal(result, err)
	}
	if _, err = f.Stat(); !errors.Is(err, os.ErrClosed) {
		t.Fatal("consumed file remained open", err)
	}
	if readArtifact(t, s, artifactOwner(t, s), result[0].ArtifactRefs[0]) != "file output" {
		t.Fatal("file output changed")
	}
}
func TestArtifactExternalStreamTimeoutRetainsUnknownWithoutRetry(t *testing.T) {
	var invoked int
	def := tool.Definition{Spec: tool.Spec{Name: "mcp__fixture__external_binary", Version: 1, InputSchema: `{"type":"object","additionalProperties":false}`, Permission: "external_effect", ReplayPolicy: "never", Risk: "high", TimeoutMS: 5, OutputBytes: 1024, Provider: "fixture", ConflictPolicy: "none"}, Invoke: func(context.Context, tool.Request) tool.Outcome {
		invoked++
		return tool.Outcome{Result: tool.Result{Output: &tool.BinaryOutput{Name: "receipt.bin", MediaType: "application/octet-stream", Reader: &slowArtifactReader{}}}}
	}}
	call := []llm.ToolCall{{ID: "external", Name: def.Spec.Name, Arguments: `{}`}}
	s, r, _, e, exchange := admitPolicy(t, []tool.Definition{def}, call)
	if _, err := e.Execute(context.Background(), exchange, call); !errors.Is(err, tool.ErrSuspended) {
		t.Fatal(err)
	}
	approvePolicy(t, s, r)
	for i := 0; i < 2; i++ {
		results, err := e.Execute(context.Background(), exchange, call)
		if err != nil || len(results) != 1 || results[0].Error == nil || results[0].Error.Code != "UNKNOWN_SIDE_EFFECT" {
			t.Fatal(results, err)
		}
	}
	if invoked != 1 || scalar(t, s, `SELECT count(*) FROM artifact`) != 0 || scalar(t, s, `SELECT count(*) FROM tool_invocation WHERE status='unknown'`) != 1 {
		t.Fatal("lost uncertainty or retried external stream", invoked)
	}
}

func TestArtifactFinalSelectionFitsConversationEnvelope(t *testing.T) {
	ctx := context.Background()
	s, _, _, a, _ := loopFixture(t)
	receipt := mustSubmit(t, a, "publication-limit", "publish bounded outputs")
	scope := artifact.Scope{UserID: artifact.LocalUser, AgentID: idAgent(t, s), RunID: receipt.RunID}
	refs := []string{}
	for i := 0; i < artifact.MaxPublished+1; i++ {
		refs = append(refs, putArtifact(t, s, scope, ""))
	}
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return s.Artifacts().PublishTx(ctx, tx, scope, refs[:artifact.MaxPublished]) }); err != nil {
		t.Fatal(err)
	}
	// A later independent publication cannot grow an invalid final envelope.
	if err := s.Transaction(ctx, func(tx *sql.Tx) error { return s.Artifacts().PublishTx(ctx, tx, scope, refs[artifact.MaxPublished:]) }); !errors.Is(err, artifact.ErrLimit) {
		t.Fatal("aggregate selection grew beyond Conversation", err)
	}
	if scalar(t, s, `SELECT count(*) FROM artifact_publication`) != artifact.MaxPublished {
		t.Fatal("limit rejection changed selection")
	}
	parts := conversation.Text("final text")
	for _, ref := range refs[:artifact.MaxPublished] {
		parts = append(parts, conversation.ContentPart{Type: "file", ArtifactRef: ref})
	}
	if _, err := conversation.Normalize(parts); err != nil {
		t.Fatal("legal maximum cannot be delivered", err)
	}
	for media, kind := range map[string]string{"IMAGE/PNG": "image", "audio/wav; rate=16000": "audio", "video/mp4": "video", "application/pdf": "file", "text/markdown": "file"} {
		if artifact.ContentType(media) != kind {
			t.Fatal(media, artifact.ContentType(media), kind)
		}
	}
}
