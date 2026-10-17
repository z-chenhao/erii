package tool

import (
	"context"
	"errors"
	"time"
)

// Request is a trusted provider boundary, never a model-visible parameter.
// The key is generated at durable acceptance and reused across recovery.
type Request struct {
	Arguments      string
	IdempotencyKey string
}
type Outcome struct {
	Result    Result
	Transient bool
	Ambiguous bool
}

// Evidence is obtained only from an authoritative provider query by key.
// Absence permits retry only while recovering a query_before_retry call.
type Evidence struct {
	State  string
	Result Result
}

type Policy struct {
	Risk           string `json:"risk"`
	Replay         string `json:"replay"`
	TimeoutMS      int    `json:"timeout_ms"`
	OutputBytes    int    `json:"output_bytes"`
	Provider       string `json:"provider"`
	ConflictPolicy string `json:"conflict_policy"`
	Version        int    `json:"version"`
}

// Policy metadata on pre-M3 read-only/internal specs has fixed equivalent
// defaults. External tools must declare every execution boundary explicitly.
func (s Spec) Policy() Policy {
	p := Policy{Risk: s.Risk, Replay: s.ReplayPolicy, TimeoutMS: s.TimeoutMS, OutputBytes: s.OutputBytes, Provider: s.Provider, ConflictPolicy: s.ConflictPolicy, Version: 1}
	if p.Replay == "" {
		p.Replay = "never"
	}
	if s.Permission == "read_only" || s.Permission == "task_control" || s.Permission == "artifact_write" {
		if p.Risk == "" {
			p.Risk = "low"
		}
		if p.TimeoutMS == 0 {
			p.TimeoutMS = 10000
		}
		if p.OutputBytes == 0 {
			p.OutputBytes = 16384
			if s.Name == "builtin__task__search" {
				p.OutputBytes = 65536
			}
		}
		if p.Provider == "" {
			p.Provider = "builtin"
		}
		if p.ConflictPolicy == "" {
			p.ConflictPolicy = "none"
		}
	}
	return p
}
func validPolicy(d Definition) bool {
	p := d.Spec.Policy()
	if p.Version != 1 || p.TimeoutMS < 1 || p.TimeoutMS > 300000 || p.OutputBytes < 1 || p.OutputBytes > 1<<20 || p.Provider == "" {
		return false
	}
	if p.Replay != "safe" && p.Replay != "query_before_retry" && p.Replay != "never" {
		return false
	}
	if p.Risk != "low" && p.Risk != "high" {
		return false
	}
	if p.ConflictPolicy == "none" {
		if d.ConflictKeys != nil {
			return false
		}
	} else if p.ConflictPolicy == "" || d.ConflictKeys == nil {
		return false
	}
	if p.Replay == "query_before_retry" && d.Query == nil {
		return false
	}
	if d.Spec.Permission == "external_effect" {
		return p.Risk == "high" && p.Replay != "safe" && d.Invoke != nil && d.Execute == nil && !d.Host
	}
	return true
}

var ErrSuspended = errors.New("tool invocation waiting")
var ErrResource = errors.New("tool resources busy")
var ErrApprovalDenied = errors.New("APPROVAL_DENIED")
var ErrReplay = errors.New("NON_DETERMINISTIC_REPLAY")

// PolicyStore owns admission and transitions, including approval and scheduling.
// All methods operate under the same opaque Run claim as Store.
type PolicyStore interface {
	Prepare(context.Context, string) (Invocation, error)
	Reject(context.Context, string, Result) error
	Defer(context.Context, string, string, time.Time) error
	Unknown(context.Context, string) error
	Reconcile(context.Context, string, Result) error
	WaitReason(context.Context) (string, error)
}

func Normalize(r Result) Result {
	if r.Error == nil {
		return r
	}
	switch r.Error.Code {
	case "INVALID_ARGUMENT", "NOT_FOUND", "PERMISSION_DENIED", "APPROVAL_DENIED", "TIMEOUT", "CANCELLED", "PROVIDER_UNAVAILABLE", "RESOURCE_LIMIT", "UNKNOWN_SIDE_EFFECT", "NON_DETERMINISTIC_REPLAY":
		out := Failure(r.Error.Code)
		out.HostFence = r.HostFence
		return out
	default:
		out := Failure("PROVIDER_UNAVAILABLE")
		out.HostFence = r.HostFence
		return out
	}
}
func (r Result) terminal() bool { return r.Error == nil || r.Error.Code != "UNKNOWN_SIDE_EFFECT" }
