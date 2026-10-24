# Frozen baseline-05 audit (ERII-235)

[baseline-05.json](baseline-05.json) retains all **80 exact source blocks** and
363 authored sub-obligations from the owner's frozen manifest. The range starts
at technical baseline §6.2.2 original L693 and ends at §6.4 original L885. It
covers invocation-local Store isolation, the JavaScript Runtime/Host protocol,
the unified Tool pipeline and runtime messages returning to Main.

There are 68 partial mechanical dispositions, five evidence-gap rows, four
policy rows and three background introductions. The sub-obligations distinguish
346 mandatory v1 duties, 13 background/optional clauses and four explicit future
design clauses. Every mandatory duty remains routed, including duties with local
mechanical evidence. No requirement is marked verified. The original ERII-30
ledger and its 122 partial/gap reviews remain unchanged and separate.

## Frozen identity

| Input | Identity |
| --- | --- |
| Original manifest | 380773 bytes; SHA256 `0120228762abfe54dd477bc27853b3b45338d0ea5f19363a3d3b742224f5c556` |
| Manifest baseline | `d11e543ea32efdf784478f5959ea6565af87b646` |
| Canonical `packages.baseline-05.blocks` digest | `3502b3aa7b733584a5ddfe624ab44b6f303201f6c752e38f650fd882ce45eb12` |
| Audited code base | `5d10262a5d5f9c5acffcaa635b39ba7e70fc2b79` |
| Current technical baseline file | SHA256 `bbb7e5db3f0bd6aa672e8704e7329d0ecd6d067769c9e49bf3997af5d9268a67` |

The maintained validator pins the package digest independently of review notes.
Local verification checks the complete manifest assignment of 1585 unique blocks.
ADR 006 / PR 45 changed Task clauses outside this package. All 80 section/content
hashes still match uniquely, with current lines equal to original lines plus 10.
Changed or ambiguous content rejects and requires coordinator resealing with an
explicit old/new hash mapping. This audit changes no normative source.

## Run and inspect

```sh
make audit-baseline-05
python3 -m unittest discover -s scripts -p 'test_release_*.py'
```

Launch and Doctor need Python, this checkout and the ledger. Drive uses the
existing `release_audit.py` CLI. Inspect `package_audit.reviews` in the read-back
report `evidence/baseline-05.json`. Cleanup may remove that disposable output.
The command contacts no network, account, model or VM. Invalid reruns remove
stale reports; output cannot overlap an input or retained native directory.
`--require-complete` still exits **2**, with status `inconclusive`,
`release_ready=false` and zero verified requirements.

For the original manifest, retained native observation bytes and full Git history:

```sh
python3 scripts/release_audit.py \
  --package-ledger docs/audits/baseline-05.json \
  --work-packages /absolute/durable/work-packages.json \
  --native-root /absolute/durable/audit-20261024 \
  --verify-git --evidence evidence/baseline-05.json
```

Inspect the explicit `manifest_checked`, `native_bytes_checked` and
`git_provenance_checked` flags. Without those inputs they remain false. Git
verification checks audited blobs and actual merged PR ancestry without fetching;
missing history rejects. Hashes bind bytes, not the truth or completeness of an
observation. Independent review must still judge the sub-obligation decomposition.

## Observed evidence and limits

Full native evidence stays in the registered durable root under
`ERII-235/5d10262a5d5f9c5acffcaa635b39ba7e70fc2b79/audit-20261024/`.
The ledger records commands, start times, exit codes, binary/source identities,
file sizes, hashes and limitations. Current reference hashes resolve in the
checkout; no historical-code fallback is used. Merged PRs 8, 10, 12, 16, 18, 20
and 26 establish provenance, not conformance.

| Observation | Inspectable result |
| --- | --- |
| Focused Go contracts | 227 passing tests/subtests; the only skipped package is the MCP fake with no tests. Indexed tests outside the command filter are not claimed as executed. |
| Focused race contracts | 112 passing tests/subtests, zero skips/failures. This is one local run, not five protected release runs. |
| JavaScript contracts | 19 real QuickJS/Node tests pass. |
| Context and Conversation contracts | Runtime origin, content normalization and compaction boundaries run through maintained Go tests. |
| Code Mode CLI | 17 checks, including Store failure/isolation/quota replay, explicit output, discovery, CUA bridge, Child aggregation, unawaited calls and two effect SIGKILL boundaries; cleanup true. |
| Tool CLI | 10 checks plus eight SIGKILL boundaries, frozen approval parameters, denial/expiry, retries, unknown effects and recovery; processes joined and disposable state removed. |
| Main CLI | Concurrent ingress, steering/frozen snapshot and ten SIGKILL boundaries; completed results reused and missing writes recovered. |
| Task CLI/PTY | Eight checks and 14 crash cases, typed runtime updates, Reporter deduplication and user-history separation; PTYs/processes joined and disposable state removed. |
| Code recovery CLI | Sidecar SIGKILL, Go SIGKILL and shutdown variants; stable mappings for 100 Child calls, completed generation reuse, suspension metrics and cancellation tree; cleanup true. |
| Computer CLI | Eight deterministic Guest-boundary checks, including private handles, action observations, consent and scoped transfer; cleanup true. |

These observations use deterministic peers. They provide no live account/model/VM
compatibility or semantic scores. No full `make verify` or protected five-run
release claim is made. The current Singleton Computer fixture cannot prove
multi-Computer concurrency, physical focus or Modal behavior. Zero active VMs
while suspended does not prove zero Goroutines, connections or Pods for every
waiting reason. Inline `Outcome.Output` versus the baseline's result-reference
contract is explicitly routed to the Runtime task. Relational nested-call trace
fields and fixed custom-message conversion retain their proof limits; names
alone do not certify or refute behavioral equivalence.

## Finite gap routes

| Route | Exact remaining responsibility |
| --- | --- |
| [ERII-338](https://multica.ai/erii-studio/issues/01a16dfe-85d9-757b-9e52-2d0de021e380) | Runtime protocol: missing `heartbeat`, per-attempt sequence/dedup versus execution-wide source requirement, missing `interrupted` observability on transport death, and result-reference versus inline Outcome contract. Existing replay success does not discharge them. One finite repair, or an owner-approved ADR/hash reseal before accepting different semantics. |
| [ERII-330](https://multica.ai/erii-studio/issues/01a16bd2-3352-7bc8-8c8c-fc6045822909) | Actual model decisions for `needs_input`, context-first questions, `unknown` handling and runtime-data authority boundaries. Scripted replies earn no semantic score. |
| [ERII-331](https://multica.ai/erii-studio/issues/01a16bd2-481b-7404-8db1-971204d4f839) | Current real Guest/DesktopLease conflict and nonconflict observations, physical UI/approval/cleanup and named human review. |
| [ERII-332](https://multica.ai/erii-studio/issues/01a16bd2-5d15-7af2-bb7f-7e5ff3c3c420) | Final current-release join of every retained mandatory duty, including unobserved protocol equivalence/resource boundaries, original race/fault/five-run/CI gates and independent review. Newly established implementation gaps require finite repair routes. |

Routes reuse existing work by semantics; the new Runtime repair stays Backlog for
coordinator registration. This audit neither implements those repairs nor changes
the frozen source. Completing the audit cannot satisfy the parent release gates.
