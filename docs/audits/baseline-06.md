# Frozen baseline-06 audit (ERII-236)

[baseline-06.json](baseline-06.json) retains the **80 exact source blocks**
assigned by the owner manifest, from technical baseline §6.4 original L886
through §6.6.4 original L1111. Its 282 authored sub-obligations cover Loop
messages/events, durable writes, the minimal Child API, shared Tool execution,
Artifact boundaries, frozen replay and VM suspension. This audit implements no
product protocol changes and does not absorb adjacent source blocks.

There are 64 partial mechanical dispositions, six evidence gaps, three policy
rows and seven background rows. Field-level scopes retain 263 required duties,
ten policy duties and nine background statements. No new feature is declared
deferred. The ERII-17/18 historical progress notes retain their original time
context; they do not establish a current M4/M5 release result. All 273 mandatory
and policy duties remain routed. No requirement is marked verified.

## Identity and provenance

| Input | Identity |
| --- | --- |
| Original manifest | 380773 bytes; SHA256 `0120228762abfe54dd477bc27853b3b45338d0ea5f19363a3d3b742224f5c556` |
| Manifest baseline | `d11e543ea32efdf784478f5959ea6565af87b646` |
| Canonical `packages.baseline-06.blocks` digest | `b20a9f33e1b58b9b184f1a3004055d12f113546764f6b89a9c103016ab94668a` |
| Audited code base | `a84447a4f43905b7a5ae82595d82743969d7bc1c` |
| Current technical baseline file | SHA256 `bbb7e5db3f0bd6aa672e8704e7329d0ecd6d067769c9e49bf3997af5d9268a67` |

The maintained validator pins the package digest independently of the authored
reviews. Local verification checks the original 1585-block assignment for
uniqueness. ADR 006 / merged PR 45 changes Task text before this package; all 80
section/content hashes still match uniquely, at original line plus ten. No
package content changed and no source is silently substituted. A changed or
ambiguous block rejects and requires coordinator resealing with old/new hashes.

Each row preserves full source text, original/current location, all authored
sub-obligations and scopes, observations, current code/test hashes, actual
merged PR provenance, native receipt references, proof limits and remaining
duties. Indexed references are distinct from executed checks. PR ancestry is
provenance, not product acceptance. The original 122 ERII-30 partial/gap reviews
and earlier package ledgers remain separate and unchanged.

## Run and inspect

```sh
make audit-baseline-06
python3 -m unittest discover -s scripts -p 'test_release_*.py'
```

Launch and Doctor use Python, this checkout and the frozen ledger. Drive uses
the existing `release_audit.py` CLI. Evidence is `evidence/baseline-06.json`;
inspect `package_audit.reviews`, scope counts and routes. Cleanup may remove
that disposable output. The command uses no network, account, model or VM.
Invalid reruns delete stale reports; output cannot overlap input or retained
native directories. `--require-complete` still exits **2**, because audit
completion cannot certify release.

For stronger read-only verification with original inputs and retained bytes:

```sh
python3 scripts/release_audit.py \
  --package-ledger docs/audits/baseline-06.json \
  --work-packages /absolute/durable/work-packages.json \
  --native-root /absolute/durable/audit-20261024 \
  --verify-git --evidence evidence/baseline-06.json
```

The flags `manifest_checked`, `native_bytes_checked` and
`git_provenance_checked` distinguish which inputs were actually checked.
Without the optional inputs they stay false. Git validation checks audited
blobs and merged PR ancestry without fetching. Hash integrity does not prove
that authored sub-obligations are semantically complete; independent review
must assess that judgment.

## Native observations

Raw evidence remains in the registered durable root, outside worktree cleanup:
`ERII-236/a84447a4f43905b7a5ae82595d82743969d7bc1c/audit-20261024/`.
Receipts retain source SHA, commands, start times, exits, sizes, hashes and
tool versions; CLI receipts also bind the locally built binary. The QuickJS
dependency was installed from the offline lockfile cache. No historical ZIP
was downloaded. Multi-MiB state traces remain local.

| Observation | Observed result and limit |
| --- | --- |
| Focused Go race contracts | 145 passing tests/subtests, actual SQLite and QuickJS with deterministic peers. This is a selected inventory with `-race`, not the full race or release suite. The MCP Fake package has no tests. |
| Actual JavaScript contracts | 19 passing QuickJS tests, including loops/Promises, allSettled, prohibited globals/imports, Main refusal, unawaited calls, deterministic keys, delivery ordering and drift rejection. |
| Child CLI/PTY | Eight check groups and all six SIGKILL/restart boundaries. Direct Parent results, scoped Artifacts, recursive Child, independent approvals and cancellation are observed through production entry points. Owned PTYs/processes and disposable state were cleaned. |
| Fixed-source recovery CLI | Sidecar SIGKILL, Go SIGKILL and SIGTERM each at 37 completed plus eight active of 100. All 45 accepted invocation/Child identities reattach, all 100 finish, and the independent provider ledger shows no redispatch of the completed 37. Suspended observations have zero active VMs; cancellation and restart do not revive cancelled work. Cleanup passes. |

These observations supply no real-model semantic score, live account/Guest
compatibility, named human acceptance or five consecutive release runs. The
report remains `inconclusive`, `release_ready=false`, with zero verified
requirements. Examples L975/L989 were not executed verbatim as complete
compositions. The joint Forget → completed recall CodeCall → recovery scenario
was not driven in this run; row L1108 remains an evidence gap.

## Routed work

| Route | Remaining work and independent exit |
| --- | --- |
| [ERII-340](https://multica.ai/erii-studio/issues/01a16e1c-85c6-7b24-8ca6-89cca9f6caa5) | Current external keys are independently random and persisted; the baseline specifies derivation from stable invocation ID. Resolve that exact contract with recovery compatibility, or obtain owner ADR and coordinator resealing. Preserve all existing keys and unknown effects. No duplicate side effect is alleged. |
| [ERII-341](https://multica.ai/erii-studio/issues/01a16e22-c621-76f5-bcde-9a4f1d79ac65) | No production `tool_execution_update` producer/consumer contract was found. Establish the approved Agent/Tool event contract, applicable increment path and best-effort routing without fabricated empty updates. ERII-338 covers the distinct Code Mode RPC event/Outcome contract. |
| [ERII-330](https://multica.ai/erii-studio/issues/01a16bd2-3352-7bc8-8c8c-fc6045822909) | Actual-model work-first behavior, aggregation, meaningful questions, Main personality/notification and untrusted Worker-content handling. Exit with original frozen dual-arm native evaluations and independent Judge receipts; scripted peers earn no scores. |
| [ERII-331](https://multica.ai/erii-studio/issues/01a16bd2-481b-7404-8db1-971204d4f839) | Current real Guest/Computer permissions, locks, approvals and unified invocation observations, tied to release SHA and named review. Deterministic peers and historical screenshots cannot substitute. |
| [ERII-332](https://multica.ai/erii-studio/issues/01a16bd2-5d15-7af2-bb7f-7e5ff3c3c420) | Final per-duty current-head protocol, errors, examples, privacy/replay, race/repetition, native CI and independent-review evidence join. Unobserved duties remain unmet; newly observed implementation faults need finite repair routes. |

Each `remaining` item binds its precise obligation to a route and proof limit.
The two new repair records are Backlog proposals awaiting coordinator admission,
not permission to change the protocol. Their scopes were checked against the
parent work and recent ERII-335–339 tasks. This audit PR and any child being Done
cannot establish v1 release acceptance.
