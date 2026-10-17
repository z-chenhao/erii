// Package codemode owns the JavaScriptRuntime consumer boundary. The Runtime
// never receives credentials, database connections, or provider implementations.
package codemode

import (
	"context"
	"encoding/json"

	"github.com/z-chenhao/erii/internal/tool"
)

const ToolName = "builtin__codemode__execute"
const RuntimeVersion = "erii-js-v2"
const PreludeVersion = "erii-prelude-v2"
const ProtocolVersion = "1"
const Schema = `{"type":"object","properties":{"source":{"type":"string","minLength":1,"maxLength":65536}},"required":["source"],"additionalProperties":false}`

type Limits struct {
	TimeoutMS       int `json:"timeoutMs"`
	MemoryBytes     int `json:"memoryBytes"`
	OutputBytes     int `json:"outputBytes"`
	PendingCalls    int `json:"pendingCalls"`
	InterruptBudget int `json:"interruptBudget"`
}

func DefaultLimits() Limits { return Limits{30000, 32 << 20, 65536, 128, 100000} }

type ExecutionRequest struct {
	ProtocolVersion    string      `json:"protocolVersion"`
	RuntimeVersion     string      `json:"runtimeVersion"`
	PreludeVersion     string      `json:"preludeVersion"`
	ExecutionID        string      `json:"executionId"`
	ClaimToken         string      `json:"claimToken"`
	ExecutionEpoch     int64       `json:"executionEpoch"`
	ProviderCallID     string      `json:"providerCallId"`
	ParentInvocationID string      `json:"parentInvocationId"`
	RunID              string      `json:"runId"`
	Kind               string      `json:"kind"`
	Source             string      `json:"source"`
	SourceHash         string      `json:"sourceHash"`
	RuntimeHash        string      `json:"runtimeHash"`
	PreludeHash        string      `json:"preludeHash"`
	Tools              []tool.Spec `json:"tools"`
	Limits             Limits      `json:"limits"`
}
type Outcome struct {
	Kind           string            `json:"kind"`
	Output         []json.RawMessage `json:"output"`
	PendingCallIDs []string          `json:"pendingCallIds"`
	Error          *tool.Error       `json:"error,omitempty"`
}
type Message struct {
	ProtocolVersion    string           `json:"protocolVersion"`
	RuntimeID          string           `json:"runtimeId"`
	ExecutionID        string           `json:"executionId"`
	ParentInvocationID string           `json:"parentInvocationId"`
	RunID              string           `json:"runId"`
	Sequence           int64            `json:"sequence"`
	OccurredAt         string           `json:"occurredAt"`
	Kind               string           `json:"kind,omitempty"`
	CallID             string           `json:"callId,omitempty"`
	CallKey            string           `json:"callKey,omitempty"`
	ClaimToken         string           `json:"claimToken"`
	ExecutionEpoch     int64            `json:"executionEpoch"`
	ToolName           string           `json:"toolName,omitempty"`
	Arguments          json.RawMessage  `json:"arguments,omitempty"`
	Outcome            *Outcome         `json:"outcome,omitempty"`
	Output             json.RawMessage  `json:"output,omitempty"`
	Metrics            map[string]int64 `json:"metrics,omitempty"`
}
type Reply struct {
	DeliverySeq int64       `json:"deliverySeq,omitempty"`
	Value       any         `json:"value,omitempty"`
	Error       *tool.Error `json:"error,omitempty"`
	Suspended   bool        `json:"suspended,omitempty"`
}
type ExecutionEventSink interface {
	// Handle persists authoritative events before returning an acknowledgment.
	Handle(context.Context, string, Message) (Reply, error)
}
type JavaScriptRuntime interface {
	Fingerprint() (string, string, error)
	Start(context.Context, ExecutionRequest, ExecutionEventSink) (Outcome, error)
	Cancel(context.Context, string, string) error
	Dispose(context.Context, string) error
}

func Definition() tool.Definition {
	return tool.Definition{Host: true, Spec: tool.Spec{Name: ToolName, Version: 1, Description: "Execute JavaScript source with top-level await/return. Discover with ALL_TOOLS, searchTools, describeTool, describeNamespace. Call tools[name](args), cua methods, and Worker-only agent.run({task,artifactRefs}). Only text(), image(), or return produces output. store(key,value)/load(key) persist bounded invocation-local JSON; exit() ends the cell. Host-accepted unawaited calls continue internally. Use structured tools first; Security and Approval apply to every call.", InputSchema: Schema, ReplayPolicy: "safe", Permission: "read_only", TimeoutMS: 60000, OutputBytes: 1 << 20, ResourceHash: RuntimeVersion + ":" + PreludeVersion}}
}
