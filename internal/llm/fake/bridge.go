// Package fake implements deterministic stdio fixtures. It is reachable only
// through explicit test injection; it never performs network or secret I/O.
package fake

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"strings"

	"github.com/z-chenhao/erii/internal/llm"
	"github.com/z-chenhao/erii/internal/llm/bridge"
)

func ValidFixture(f string) bool {
	if strings.HasPrefix(f, "error:") {
		code := strings.TrimPrefix(f, "error:")
		return llm.NormalizeError(code).Error() == code
	}
	switch f {
	case "codemode", "children", "capabilities", "artifacts", "approval", "approval-never", "tasks", "terminal-input", "golden", "main", "success", "incompatible", "crash", "partial-crash", "completed-crash", "no-tools", "bad-sequence", "drift", "malformed", "auth-required", "raw-error", "event-error":
		return true
	}
	return false
}

func ServeRecorded(in io.Reader, out io.Writer, fixture, path string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return serve(in, out, fixture, f)
}
func Serve(in io.Reader, out io.Writer, fixture string) error { return serve(in, out, fixture, nil) }
func serve(in io.Reader, out io.Writer, fixture string, audit io.Writer) error {
	if !ValidFixture(fixture) {
		return llm.InvalidProviderResponse
	}
	encoder := json.NewEncoder(out)
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), bridge.MaxFrameBytes)
	status := "logged_out"
	waiting := map[string]int64{}
	for scanner.Scan() {
		var r struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      uint64          `json:"id"`
			Method  string          `json:"method"`
			Params  json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &r) != nil || r.JSONRPC != "2.0" || r.ID == 0 {
			return llm.InvalidProviderResponse
		}
		reply := func(value any) error {
			return encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": r.ID, "result": value})
		}
		failure := func(code string) error {
			return encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": r.ID, "error": map[string]any{"code": -32000, "message": code}})
		}
		event := func(e llm.GenerationEvent) error {
			return encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "generation.event", "params": e})
		}
		var err error
		switch r.Method {
		case "bridge.shutdown":
			return reply(struct{}{})
		case "bridge.initialize":
			version := bridge.ProtocolVersion
			if fixture == "incompatible" {
				version++
			}
			err = reply(map[string]int{"protocol_version": version})
		case "auth.start":
			err = reply(llm.AuthState{Status: "reauth_required", Events: []llm.AuthEvent{{Kind: "open_url", URL: "https://example.invalid/authorize"}, {Kind: "device_code", DeviceCode: "TEST-CODE"}, {Kind: "progress"}, {Kind: "choose_workspace", Choices: []string{"test-workspace"}}, {Kind: "input_required"}}})
		case "auth.continue", "auth.refresh":
			status = "connected"
			err = reply(llm.AuthState{Status: status, Events: []llm.AuthEvent{}})
		case "auth.logout":
			status = "logged_out"
			err = reply(struct{}{})
		case "auth.status":
			err = reply(llm.AuthState{Status: status, Events: []llm.AuthEvent{}})
		case "models.list", "models.refresh":
			err = reply([]llm.CatalogModel{{ModelID: "fixture-model", ContextWindow: 8192, MaxOutputTokens: 1024}})
		case "model.probe":
			err = reply(map[string]string{"status": "use_host_behavioral_probe"})
		case "generation.start", "generation.continue":
			if strings.HasPrefix(fixture, "error:") {
				err = failure(strings.TrimPrefix(fixture, "error:"))
				break
			}
			if fixture == "crash" {
				return nil
			}
			if fixture == "auth-required" {
				err = failure(string(llm.AuthRequired))
				break
			}
			if fixture == "raw-error" {
				err = failure("SYNTHETIC_PROVIDER_SECRET response body")
				break
			}
			if fixture == "malformed" {
				_, err = io.WriteString(out, "invalid JSON\n")
				break
			}
			var request llm.GenerationRequest
			if json.Unmarshal(r.Params, &request) != nil {
				return llm.InvalidProviderResponse
			}
			if audit != nil {
				if err = json.NewEncoder(audit).Encode(map[string]string{"generation_id": request.GenerationID, "snapshot_id": request.Snapshot.SnapshotID}); err != nil {
					return err
				}
			}
			if err = reply(struct{}{}); err != nil {
				return err
			}
			e := llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: 1, Kind: "text_delta", Text: "OK"}
			last := request.Messages[len(request.Messages)-1]
			if (fixture == "codemode" || fixture == "children" || fixture == "capabilities" || fixture == "artifacts" || fixture == "tasks" || fixture == "approval" || fixture == "approval-never") && hasTool(request.Tools, "builtin__text__echo") {
				// Hold an actual accepted Worker generation while keeping the RPC
				// peer responsive to cancellation. A new Root gets the appended
				// instruction as its last message and can finish normally.
				if fixture == "tasks" && strings.Contains(last.Text, "Task anchor:") && strings.Contains(last.Text, "Hold worker") {
					err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: 1, Kind: "text_delta", Text: "Worker waiting for cancellation"})
					waiting[request.GenerationID] = 2
					break
				}
				var value llm.GenerationResult
				var cause error
				if fixture == "children" && last.Text == "Hold child" {
					err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: 1, Kind: "text_delta", Text: "Child waiting for cancellation"})
					waiting[request.GenerationID] = 2
					break
				}
				if fixture == "children" {
					value, cause = childrenReply(request)
				} else if fixture == "capabilities" || fixture == "codemode" {
					value = capabilitiesReply(request)
				} else if fixture == "artifacts" {
					value = artifactsReply(request)
				} else if fixture == "tasks" {
					value, cause = tasksReply(request)
				} else {
					value = approvalReply(request)
				}
				sequence := int64(1)
				if cause != nil {
					err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "error", ErrorCode: cause.Error()})
					break
				}
				for _, call := range value.ToolCalls {
					c := call
					if err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "tool_call", ToolCall: &c}); err != nil {
						return err
					}
					sequence++
				}
				if value.Text != "" {
					if err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "text_delta", Text: value.Text}); err != nil {
						return err
					}
					sequence++
				}
				var usage *llm.TokenUsage
				if fixture == "children" && (request.Snapshot.ModelID == "priced-fixture-model" || request.Snapshot.ModelID == "overrun-fixture-model") {
					// Synthetic, explicit monetary fixture; ordinary children keep
					// their missing-usage contract. No real market price is implied.
					usage = &llm.TokenUsage{InputTokens: 100, OutputTokens: 50, CacheReadTokens: 20, CacheWriteTokens: 10}
					if request.Snapshot.ModelID == "overrun-fixture-model" {
						usage.InputTokens = 200000
					}
				}
				err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "completed", Usage: usage})
				break
			}
			if fixture == "terminal-input" && hasTool(request.Tools, "builtin__text__echo") {
				// A short model reply isolates the terminal byte boundary from
				// the separate Tool argument/output limits. Probe still runs below.
				e.Text = "Input received."
				if err = event(e); err != nil {
					return err
				}
				err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: 2, Kind: "completed"})
				break
			}
			if (fixture == "main" || fixture == "golden") && hasTool(request.Tools, "builtin__text__echo") {
				// Same call IDs on distinct exchanges deliberately exercise the
				// (run, exchange, call) association, never global call-ID dedup.
				sequence := int64(1)
				if last.Role == "user" {
					for _, id := range []string{"echo-a", "echo-b"} {
						args, _ := json.Marshal(struct {
							Text string `json:"text"`
						}{last.Text})
						call := llm.ToolCall{ID: id, Name: "builtin__text__echo", Arguments: string(args)}
						if err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "tool_call", ToolCall: &call}); err != nil {
							return err
						}
						sequence++
					}
				} else if last.Role == "tool" {
					text := "Processed: " + last.Text
					if fixture == "golden" {
						text = goldenReply(request.Messages)
					}
					if err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "text_delta", Text: text}); err != nil {
						return err
					}
					sequence++
				} else {
					return llm.InvalidProviderResponse
				}
				if err = event(llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: sequence, Kind: "completed"}); err != nil {
					return err
				}
				break
			}
			if r.Method == "generation.continue" {
				if last.Role != "tool" || last.ToolCallID != "probe-call" {
					err = failure(string(llm.InvalidProviderResponse))
					break
				}
				e.Text = last.Text
			} else if len(request.Tools) > 0 && fixture != "no-tools" {
				e.Kind = "tool_call"
				e.Text = ""
				e.ToolCall = &llm.ToolCall{ID: "probe-call", Name: "probe_echo", Arguments: `{"value":"OK"}`}
			} else if strings.Contains(last.Text, "long output") {
				e.Text = strings.Repeat("x", 128<<10)
			}
			if fixture == "bad-sequence" {
				e.Sequence = 2
			}
			if fixture == "event-error" {
				e.Kind = "error"
				e.Text = ""
				e.ErrorCode = string(llm.RateLimited)
			}
			if err = event(e); err != nil {
				return err
			}
			if fixture == "partial-crash" {
				return nil
			}
			if err = event(e); err != nil {
				return err
			} // exact redelivery fixture
			if fixture == "drift" {
				e.Text = "changed"
				err = event(e)
				if err != nil {
					return err
				}
			}
			if strings.Contains(last.Text, "until cancelled") {
				waiting[request.GenerationID] = 2
				break
			}
			e = llm.GenerationEvent{GenerationID: request.GenerationID, Sequence: 2, Kind: "completed"}
			if err = event(e); err == nil {
				err = event(e)
			} // completion commits only once
			if fixture == "completed-crash" {
				return err
			}
		case "generation.cancel":
			var params struct {
				GenerationID string `json:"generation_id"`
			}
			if json.Unmarshal(r.Params, &params) != nil {
				return llm.InvalidProviderResponse
			}
			err = reply(struct{}{})
			if next, ok := waiting[params.GenerationID]; ok && err == nil {
				delete(waiting, params.GenerationID)
				err = event(llm.GenerationEvent{GenerationID: params.GenerationID, Sequence: next, Kind: "error", ErrorCode: string(llm.GenerationCancelled)})
			}
		case "session.dispose":
			err = reply(struct{}{})
		default:
			err = failure(string(llm.InvalidProviderResponse))
		}
		if err != nil {
			return err
		}
	}
	return scanner.Err()
}

// Scripted protocol evidence, never model-quality evidence. Select the response
// by the versioned fixture context and turn ordinal, not by exact prompt wording.
// The production Context Builder and provider request are unchanged.
func goldenReply(messages []llm.Message) string {
	var context string
	users := 0
	for _, m := range messages {
		if m.Role == "user" {
			if context == "" {
				context = m.Text
			}
			users++
		}
	}
	switch {
	case strings.Contains(context, "轻度脂肪肝"):
		switch users {
		case 2:
			return "先喝水、接触自然光，09:50到公司前缓一缓。昨晚大约睡了多久？"
		case 3:
			return "一点半才睡，睡眠不足是最直接的解释。先补水、进食和短暂活动。"
		default:
			return "工作没问题，下午适当休息，今晚提前上床。"
		}
	case strings.Contains(context, "发布风险说明"):
		return "1. 先做发布风险说明，在10:00周会前或会后立即完成，12:00前确认。\n2. 14:00面试前留准备时间，E4只核对。\n3. 新闻和已确认的酒店邮件稍后看。"
	case strings.Contains(context, "rain_probability_evening"):
		return "开车通勤可穿短袖、薄长裤，带薄卫衣。18:30散步时按体感添衣，21:00下班更凉，薄卫衣用得上。"
	default:
		return "Fixture context missing."
	}
}

func hasTool(specs []llm.ToolSpec, name string) bool {
	for _, s := range specs {
		if s.Name == name {
			return true
		}
	}
	return false
}
