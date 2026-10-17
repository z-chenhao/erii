# JavaScript Code Mode runtime (ERII-19 / ERII-20)

The configured `builtin__codemode__execute` Tool runs one fresh QuickJS VM per
invocation through the normal Agent Loop and ToolExecutor. Its `source` argument
contains the raw async JavaScript body: top-level `await` and `return` work.
There is no second Agent Loop or JavaScript interpreter. Every nested Tool,
Agent, and Computer operation creates an ordinary fenced ToolInvocation.

## Launch and verify

Install Node **24.21.0**, then install the exact WASM dependency:

```sh
npm ci --prefix runtime/javascript --ignore-scripts
make build
python3 scripts/verify_codemode.py --binary bin/personal-agent --evidence evidence/codemode.json
python3 scripts/verify_code_recovery.py --binary bin/personal-agent --evidence evidence/code-recovery.json
```

The reusable harness owns Launch, Doctor, Drive, Evidence, and Cleanup. It
creates temporary configuration and databases, activates the explicit
`codemode` model fixture, then drives `chat send`/`chat work`. The model fixture
selects requests; the real Node/Pi/QuickJS VM executes the JavaScript. The
Computer Fake is injected only with this test fixture. Production has no Fake
Computer fallback. Its future Driver remains M6 work.

The report includes exact explicit outputs, nested invocation identities,
source/Store/event readback, a real Child Worker result with inherited frozen
model identity, approval rejection without Provider dispatch, schema version,
before/after-effect SIGKILL recovery with one durable identity and one external
receipt, and cleanup. `make verify` runs this harness five times. `make test` includes
real Sidecar transport, Promise, resource, cancellation, and security fixtures.

To enable Code Mode in an ordinary installation:

```yaml
version: 1
javascript:
  command: /absolute/path/to/node
  script: /absolute/path/to/erii/runtime/javascript/src/sidecar.mjs
```

Both paths must be trusted and outside an editable coding workspace. There is
no implicit executable discovery or default model. Capability snapshots freeze
the versioned ToolSpec for each new Run. Existing snapshots retain their
original Tool surface. A Sidecar protocol failure produces a sanitized Tool
error, never a raw exception or inherited credential.

## Script contract

```javascript
const matching = await searchTools("echo", {limit: 10});
const detail = await describeTool(matching[0].name);
const results = await Promise.allSettled([1, 2, 3].map(n =>
  tools[matching[0].name]({text: String(n)})
));
store("summary", {count: results.length});
text(load("summary"));
return results.map(result => result.status);
```

`ALL_TOOLS` contains canonical names and short descriptions. Complete schemas, TypeScript declarations and usage instructions
remain behind discovery. Canonical bracket calls always work; an escaped alias
is available only when unambiguous. Discovery searches only the frozen Run
view. Main has no `agent` global and cannot invoke the hidden Agent Tool.
Workers with the frozen capability get exactly
`agent.run({task, artifactRefs?}) -> Promise<{text, artifactRefs}>`. No Run ID,
usage, output schema, agent selector or control handle enters this API.

The CUA facade provides inventory, explicit binding, and lightweight Surface
proxies. Binding alone emits no output. `documentation()` and `capabilities`
resolve through the same nested Executor. Surface handles stay within one cell;
Store rejects them, including nested handles. Private Computer bridge helpers
are absent from public tools/discovery, so scripts cannot extract raw lease
tokens for persistence. The Fake exposes read-only
observations and documentation; it does not pretend to implement GUI actions.

Only explicit `text()`, `image()` and top-level `return` reach the parent Tool
result. Console and nested business results stay internal. Errors reject
Promises with normalized `code`, `message`, and `retryable` fields. `exit()`
ends successfully. A script failure does not roll back accepted work.

`store(key, value)` commits synchronously to invocation-local JSON state before
returning. Writes survive later script failure and process restart. Reads
return copies; a missing key returns `undefined`. Keys are 1–256 bytes, each
JSON value is at most 256 KiB, and total stored keys/values at most 1 MiB.
Repeated writes use a versioned last-write-wins update. Other cells and Child
Runs do not inherit this state. Large data belongs in Artifacts.

## Isolation and transport

Go starts a Node Sidecar with an explicit environment containing only PATH,
TZ, and LANG. It passes source, frozen accessible Tools, identities and limits;
it passes no credentials, model Provider or database handles. One execution
gets one Worker thread and one QuickJS-WASI VM. Worker termination is awaited;
the Go adapter closes pipes and reaps its exact child process before returning.
The process is currently scoped to `Start`; it is not a resident shared pool.

The Pi 1.3.0 VM/Prelude is vendored with its MIT notice and exact npm integrity.
Erii retains Pi's Promise scheduling, JSON boundary, output controls and
intrinsic lockdown. Its checked Prelude adaptations prohibit time/random and
provide synchronous Host-backed Store plus the CUA/Agent facades. Node is the
host; QuickJS is the script boundary. Scripts have no Node objects, filesystem,
network, timers, module loader, credentials, or real-time/random APIs.

The versioned JSON-RPC 2.0 stdio protocol implements initialize, start, cancel,
dispose and shutdown plus host discovery/invoke/store requests. Envelopes carry
execution, parent and Run identity, monotonic sequence and UTC time. The Go
adapter rejects forged identities, unknown methods, sequence drift and a final
RPC result without a matching authoritative terminal event. Exact duplicate
events can be acknowledged again. The sink durably records events before ack.
Nested admission atomically inserts CodeCall, ToolInvocation, Child admission
when applicable, counters and Outbox using the existing transaction.

Fixed production limits are: 30s cell deadline, 32 MiB QuickJS heap, 100,000
interrupt polls, 64 KiB source/output, 256 output items, 128 pending Host calls,
and 1 MiB RPC messages. Go ToolExecutor retains its actual concurrency,
approval, conflict, timeout and retry policies. Limits from untrusted sources
cannot expand them. Parameter drift for an admitted nested identity fails
closed. External effects retain `safe/query_before_retry/never` semantics.

The Code Mode wrapper coordinates nested calls without owning an actual Tool
execution slot. Nested Tools acquire the same global, Agent, Run, Provider and
per-Tool capacity as direct calls, including conflict exclusion. A configured
`tools.run_concurrency: 1` therefore completes an awaited nested call; parallel
calls still share that one slot. The Loop/Executor and the fixed JavaScript
limits continue to bound wrapper execution.

## Fixed-source recovery

Schema 17 adds an immutable manifest and versioned execution state behind the
ordinary Code Mode ToolInvocation. The manifest saves source and explicit
empty input JSON, hashes both, and binds the model/capability snapshots,
complete frozen Tools, limits, Runtime/Prelude versions and installed bytes.
Runtime fingerprints cover Sidecar/Worker/VM sources, package lock and WASM;
Prelude fingerprints cover both Erii adaptations and the pinned upstream file.
The registry includes installed Runtime/Prelude hashes in its ResourceHash, so
Run creation freezes them before any cell starts. Recovery rejects drift.
No heap or stack is serialized.

The trusted Prelude captures the caller's source position. Node derives each
nested key from the parent Provider call, source hash, position and dynamic
ordinal at that position. Scripts never see or supply keys. Atomic admission
binds the key to canonical name/arguments/schema/resolved Tool capabilities,
and commits CodeCall, ToolInvocation, Child/counters and Outbox together.
Completed calls reuse durable results; active Children retain their identities.
Nested effects retain ordinary `safe/query_before_retry/never` recovery;
`unknown` never becomes permission to retry. Drift fails the whole cell with
`NON_DETERMINISTIC_REPLAY`, including when script catches ordinary Tool errors.

Host invoke first accepts the call without dispatching it. If work is pending,
Node destroys the Worker/QuickJS VM. Go then drains accepted work through the
same ToolExecutor. Long approval/Child waits release both execution and Run
leases. Further progress rebuilds a fresh VM from the immutable manifest.
Persisted delivery sequence numbers restore Promise continuation order even
when transport replies arrive out of order. Store retains versioned JSON with
a value hash and update time; it is isolated to one invocation. Each Store operation has a trusted source key and an immutable receipt. Replay
reuses its original read or write, so an earlier load cannot observe a later
write and repeated replay cannot increment a saved counter. Distinct writes
to one key still use last-write-wins semantics. Events retain their audit.

Each execution claim increments an epoch and issues a new token subordinate
to the Run lease. Every Sidecar event, Store access and Host admission checks
both identities. Run renewal extends the active execution lease. The terminal
receipt is persisted with its authoritative terminal event before acknowledgement;
the ordinary ToolExecutor commits the final wrapper result under that owner.
Old callbacks cannot admit work or commit output.

Authoritative Tool, Child, approval, input, spend and stop transitions increment
`wake_seq` in their transactions. Release compares its observed sequence with
the current sequence before entering idle; a change forces another drain.
The existing Run scheduler scans expired execution claims and unprocessed
wakes and emits Outbox hints. It remains the sole scheduler.

Normal shutdown interrupts a current VM replay and preserves accepted work.
User cancellation fences the entire Run tree and settles nested effects through
the existing cancellation path. Accepted unawaited calls continue after script
completion. The chosen output is frozen while these calls drain internally;
their later results do not enter the finished output.

`code_execution_state` records replay starts/hits, epochs, wakes and leases.
Suspension events record active VM count, Node RSS and elapsed milliseconds.
`verify_code_recovery.py` kills actual Sidecar and Go processes at exactly 37
completed and 8 active Children out of 100. It checks independent Fake-provider
dispatch audits, reattached identities, quotas, final aggregation, memory
metrics and cleanup. The normal CLI harness covers unawaited work, Store
isolation, approval rejection and crashes before/after effects. Both harnesses
run five times in `make verify`. Fake-clock tests advance approval by six hours
and prove the disposed VM is not retained.

## Upgrade and operations

Install the pinned Node/QuickJS dependencies before starting `chat work`.
After a crash, run `chat work` with the same installed runtime and configuration;
a killed Go owner's five-second lease must expire before work is claimed.
Missing or changed runtime bytes fail closed and leave recovery intent intact.
Restore the original runtime to continue. Never rewrite source, hashes, claims
or accepted arguments in the database to force replay.

Migration 17 preserves v16 ledger/Store/events and all earlier state. v1 cells
lack the v2 replay manifest and source-position identities. They are **not**
automatically upgraded: finish active v1 trees with their original binary and
Runtime before deploying v2. Frozen incompatible capabilities are rejected
rather than switched. Empty/v16 migration, rollback and preservation are tested.

This slice proves deterministic protocol/recovery behavior. It does not award
Golden Case semantic scores or complete M5; ERII-21 owns the due scored cases.
The real Computer Driver and macOS compatibility remain M6 work.
