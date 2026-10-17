// Package tool owns the single ToolSpec -> ToolInvocation -> Executor path.
// Direct calls and future Runtime facades share this durable boundary.
package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/z-chenhao/erii/internal/llm"
)

type Error struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type Result struct {
	// HostFence is an opaque, process-local completion receipt. It is never model
	// output; only the owning Host store can construct and validate its type.
	HostFence    any           `json:"-"`
	Text         string        `json:"text"`
	ArtifactRefs []string      `json:"artifact_refs,omitempty"`
	Output       *BinaryOutput `json:"-"`
	PublishRefs  []string      `json:"-"`
	Error        *Error        `json:"error,omitempty"`
}

// BinaryOutput is an internal provider stream, never serialized into history.
// The Executor consumes it within the frozen deadline and closes io.Closers.
type BinaryOutput struct {
	Name, MediaType string
	Reader          io.Reader
}

// OutputStore atomically normalizes large/binary results and commits their
// metadata with the invocation terminal state before exposing references.
// ctx bounds result consumption; the store separately commits known failure.
type OutputStore interface {
	FinishResult(context.Context, string, Result) (Result, error)
	ReconcileResult(context.Context, string, Result) (Result, error)
}

func Failure(code string) Result {
	return Result{Error: &Error{Code: code, Message: code, Retryable: code == "PROVIDER_UNAVAILABLE"}}
}
func (r Result) ModelText() string {
	if r.Error == nil {
		if len(r.ArtifactRefs) > 0 {
			b, _ := json.Marshal(r)
			return string(b)
		}
		return r.Text
	}
	b, _ := json.Marshal(r.Error)
	return string(b)
}

type Spec struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	ResourceHash   string `json:"resource_hash,omitempty"`
	Version        int    `json:"version"`
	InputSchema    string `json:"input_schema"`
	ReplayPolicy   string `json:"replay_policy"`
	Permission     string `json:"permission"`
	Risk           string `json:"risk,omitempty"`
	TimeoutMS      int    `json:"timeout_ms,omitempty"`
	OutputBytes    int    `json:"output_bytes,omitempty"`
	Provider       string `json:"provider,omitempty"`
	ConflictPolicy string `json:"conflict_policy,omitempty"`
}

type Definition struct {
	Spec         Spec
	Host         bool // Trusted Host builtins receive the accepted invocation, never model-supplied IDs.
	Execute      func(context.Context, string) Result
	Invoke       func(context.Context, Request) Outcome
	Query        func(context.Context, Request) Evidence
	ConflictKeys func(string) []string
}
type Registry struct{ definitions map[string]Definition }

func NewRegistry(definitions []Definition) (*Registry, error) {
	r := &Registry{definitions: map[string]Definition{}}
	for _, d := range definitions {
		// Policy is frozen at admission; providers cannot choose retry safety.
		if d.Spec.Name == "" || d.Spec.Version != 1 || !validPolicy(d) || !validDefinition(d) || !json.Valid([]byte(d.Spec.InputSchema)) {
			return nil, errors.New("invalid tool definition")
		}
		if _, valid := parseSchema(d.Spec.InputSchema); !valid {
			return nil, errors.New("unsupported tool schema")
		}
		if _, exists := r.definitions[d.Spec.Name]; exists {
			return nil, errors.New("duplicate tool definition")
		}
		r.definitions[d.Spec.Name] = d
	}
	return r, nil
}
func validDefinition(d Definition) bool {
	if d.Host {
		switch d.Spec.Name {
		case "builtin__codemode__execute":
			return d.Execute == nil && d.Invoke == nil && d.Spec.Permission == "read_only"
		case "builtin__agent__run":
			return d.Execute == nil && d.Invoke == nil && d.Spec.Permission == "spawn_agents"
		case "builtin__artifact__save", "builtin__artifact__publish":
			return d.Execute == nil && d.Invoke == nil && d.Spec.Permission == "artifact_write"
		case "builtin__artifact__read", "builtin__artifact__describe", "builtin__artifact__list":
			return d.Execute == nil && d.Invoke == nil && d.Spec.Permission == "read_only"
		}
		return d.Execute == nil && ((d.Spec.Name == "builtin__task__search" && d.Spec.Permission == "read_only") || (d.Spec.Name == "builtin__task__manage" && d.Spec.Permission == "task_control"))
	}
	return (d.Execute != nil && d.Invoke == nil && d.Spec.Permission == "read_only") || (d.Invoke != nil && d.Spec.Permission == "external_effect")
}

// Select verifies every frozen spec before constructing the role-specific view.
func (r *Registry) Select(specs []Spec) (*Registry, error) {
	defs := make([]Definition, 0, len(specs))
	for _, s := range specs {
		d, ok := r.Resolve(s.Name)
		if !ok || !reflect.DeepEqual(d.Spec, s) {
			return nil, llm.ModelIncompatible
		}
		defs = append(defs, d)
	}
	return NewRegistry(defs)
}

// HostStore executes registered internal controls under the same accepted,
// fenced ToolInvocation. It is not an alternative invocation lifecycle.
type HostStore interface {
	ExecuteBuiltin(context.Context, Invocation) (Result, error)
}

func EchoRegistry() *Registry {
	r, err := NewRegistry([]Definition{{Spec: Spec{Name: "builtin__text__echo", Version: 1, InputSchema: `{"type":"object","properties":{"text":{"type":"string","maxLength":4096}},"required":["text"],"additionalProperties":false}`, ReplayPolicy: "safe", Permission: "read_only"}, Execute: func(ctx context.Context, args string) Result {
		var value struct {
			Text *string `json:"text"`
		}
		dec := json.NewDecoder(bytes.NewBufferString(args))
		dec.DisallowUnknownFields()
		if dec.Decode(&value) != nil || value.Text == nil || len(*value.Text) > 4096 {
			return Failure("INVALID_ARGUMENT")
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			return Failure("INVALID_ARGUMENT")
		}
		return Result{Text: *value.Text}
	}}})
	if err != nil {
		panic(err)
	}
	return r
}
func (r *Registry) Describe() []Spec {
	// M1 has one Tool. Sorting keeps capability hashing stable as it grows.
	out := make([]Spec, 0, len(r.definitions))
	for _, d := range r.definitions {
		out = append(out, d.Spec)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
func (r *Registry) Resolve(name string) (Definition, bool) {
	d, ok := r.definitions[name]
	return d, ok
}

type Invocation struct {
	ID         string
	Call       llm.ToolCall
	Status     string
	WaitReason string
	Key        string
	Attempts   int
	Result     Result
}

// Store is bound by the Host to an opaque Run claim. Every mutation fences.
type Store interface {
	PolicyStore
	Accept(context.Context, string, []llm.ToolCall, []Spec) ([]Invocation, error)
	Start(context.Context, string) error
	Finish(context.Context, string, Result) error
}
type Event struct {
	Kind         string
	InvocationID string
	Call         llm.ToolCall
	Result       Result
}
type Executor struct {
	Registry    *Registry
	Store       Store
	Concurrency int
	Sink        func(Event)
}

func (e Executor) Execute(ctx context.Context, exchange string, calls []llm.ToolCall) ([]Result, error) {
	if len(calls) > 64 {
		return nil, errors.New("tool batch resource limit")
	}
	invocations, err := e.Store.Accept(ctx, exchange, calls, e.Registry.Describe())
	if err != nil {
		return nil, err
	}
	limit := e.Concurrency
	if limit < 1 {
		limit = 4
	}
	if limit > 16 {
		limit = 16
	}
	results := make([]Result, len(invocations))
	errs := make([]error, len(invocations))
	jobs := make(chan int)
	var wg sync.WaitGroup
	for n := 0; n < limit; n++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				results[i], errs[i] = e.executeOne(ctx, invocations[i])
			}
		}()
	}
	for i := range invocations {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	// Return coordination failures before suspension; every independent result
	// has already committed, regardless of original delivery order.
	for _, err := range errs {
		if err != nil && !errors.Is(err, ErrSuspended) {
			return nil, err
		}
	}
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}
func (e Executor) executeOne(ctx context.Context, inv Invocation) (Result, error) {
	policyStore := e.Store
	var err error
	inv, err = policyStore.Prepare(ctx, inv.ID)
	if err != nil {
		return Result{}, err
	}

	switch inv.Status {
	case "succeeded", "failed", "cancelled":
		return inv.Result, nil
	case "waiting":
		return Result{}, ErrSuspended
	}
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}
	d, known := e.Registry.Resolve(inv.Call.Name)
	if !known {
		d.Spec = Spec{Permission: "read_only", ReplayPolicy: "safe"}
	}
	p := d.Spec.Policy()
	request := Request{Arguments: inv.Call.Arguments, IdempotencyKey: inv.Key}
	if inv.Status == "unknown" || (inv.Status == "running" && p.Replay != "safe") {
		uncertain := func() (Result, error) {
			// Persist uncertainty before the Loop can consume the error and
			// finish its Run. Task reporting and reconciliation rely on it.
			if inv.Status != "unknown" {
				if err := policyStore.Unknown(context.WithoutCancel(ctx), inv.ID); err != nil {
					return Result{}, err
				}
			}
			return Failure("UNKNOWN_SIDE_EFFECT"), nil
		}
		if inv.Status != "unknown" && p.Replay == "never" {
			return uncertain()
		}
		if d.Query == nil {
			return uncertain()
		}
		evidence, err := RecoverResult(ctx, d, request, func(queryCtx context.Context, result Result) (Result, error) {
			if store, ok := e.Store.(OutputStore); ok {
				return store.ReconcileResult(queryCtx, inv.ID, result)
			}
			if result.Output != nil || len(result.ArtifactRefs) > 0 || result.PublishRefs != nil {
				return Failure("UNKNOWN_SIDE_EFFECT"), nil
			}
			return result, policyStore.Reconcile(context.WithoutCancel(queryCtx), inv.ID, result)
		})
		if err != nil {
			if _, persistErr := uncertain(); persistErr != nil {
				return Result{}, persistErr
			}
			return Result{}, err
		}
		if evidence.State == "succeeded" || evidence.State == "failed" {
			return evidence.Result, nil
		}
		// unknown is terminal for retry purposes. Only an incomplete running
		// query-before-retry call may proceed after authoritative absence.
		if evidence.State != "absent" || inv.Status == "unknown" {
			return uncertain()
		}
	}
	if err := e.Store.Start(ctx, inv.ID); err != nil {
		if errors.Is(err, ErrApprovalDenied) {
			result := Failure("APPROVAL_DENIED")
			if err = policyStore.Reject(ctx, inv.ID, result); err != nil {
				return Result{}, err
			}
			return result, nil
		}
		if errors.Is(err, ErrResource) {
			if err = policyStore.Defer(ctx, inv.ID, "resource", time.Time{}); err == nil {
				err = ErrSuspended
			}
			return Result{}, err
		}
		return Result{}, err
	}
	if e.Sink != nil {
		e.Sink(Event{Kind: "tool_execution_start", InvocationID: inv.ID, Call: inv.Call})
	}
	result := Failure("NOT_FOUND")
	outcome := Outcome{}
	resultCtx := ctx
	if known {
		callCtx, cancel := context.WithTimeout(ctx, time.Duration(p.TimeoutMS)*time.Millisecond)
		resultCtx = callCtx
		defer cancel()
		if d.Host {
			host, ok := e.Store.(HostStore)
			if !ok {
				result = Failure("PERMISSION_DENIED")
			} else {
				var err error
				result, err = host.ExecuteBuiltin(callCtx, inv)
				if err != nil {
					cancel()
					return Result{}, err
				}
			}
		} else if d.Invoke != nil {
			outcome = d.Invoke(callCtx, request)
			result = outcome.Result
		} else {
			result = d.Execute(callCtx, inv.Call.Arguments)
		}
		defer closeOutput(callCtx, result)()
		if callCtx.Err() != nil && !d.Host {
			if d.Spec.Permission == "external_effect" {
				outcome.Ambiguous = true
			} else if ctx.Err() != nil {
				result = Failure("CANCELLED")
			} else {
				result = Failure("TIMEOUT")
			}
		}
	}
	result = Normalize(result)
	if outcome.Ambiguous || (result.Error != nil && result.Error.Code == "UNKNOWN_SIDE_EFFECT") {

		if err := policyStore.Unknown(context.WithoutCancel(ctx), inv.ID); err != nil {
			return Result{}, err
		}
		return Failure("UNKNOWN_SIDE_EFFECT"), nil
	}
	// Providers may mark only a known pre-effect infrastructure failure as
	// transient. Timeout, business and ambiguous outcomes never enter retry.
	if outcome.Transient && result.Error != nil && result.Error.Code == "PROVIDER_UNAVAILABLE" && inv.Attempts < 2 {
		if err := policyStore.Defer(ctx, inv.ID, "retry_backoff", time.Now().Add(time.Duration(50<<inv.Attempts)*time.Millisecond)); err != nil {
			return Result{}, err
		}
		return Result{}, ErrSuspended
	}
	if store, ok := e.Store.(OutputStore); ok {
		result, err = store.FinishResult(resultCtx, inv.ID, result)
	} else {
		if len(result.Text) > p.OutputBytes || result.Output != nil {
			result = Failure("RESOURCE_LIMIT")
		}
		err = e.Store.Finish(context.WithoutCancel(ctx), inv.ID, result)
	}
	if err != nil {
		return Result{}, err
	}
	if e.Sink != nil {
		e.Sink(Event{Kind: "tool_execution_end", InvocationID: inv.ID, Call: inv.Call, Result: result})
	}
	return result, nil
}
