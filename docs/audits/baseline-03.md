# Baseline-03 audit — ERII-233

[baseline-03.json](baseline-03.json) retains all **80 exact frozen source blocks**,
from technical baseline §3.11 L343 through §5.4 L540. It records 347 authored
sub-obligations: 70 blocks have partial mechanical evidence, five have evidence
gaps, four are explanatory background and one explicitly defers future UI work.
The mixed account-auth block separately retains Device Authorization's approved
deferral under [ADR 001](../adr/001-current-openai-account-auth.md); its twelve
current obligations remain routed. None is certified as implemented or verified.

Each row binds original source/section/line/hash/text to its unique current
section/content hash and current line, sub-obligations, observed mechanics,
current code/test/harness bytes, merged PR provenance, native observations,
proof limits and finite gap routes. The schema fence at original L446 has 38
separate field/type obligations. A test reference is a navigation link; only the
native Go JSON stream establishes which tests actually executed.

The original [ERII-30 ledger](../v1-release-ledger.json) and its 122 partial/gap
reviews are unchanged. Twenty overlap this package by section/content hash;
they retain their original limitations. This package adds current dispositions
separately and does not promote a historical gap to verified. Original PR 32
is unmerged provenance. Current merged PRs are checked separately; neither a
merged PR nor green CI proves a requirement or release passed.

## Frozen identity

The owner manifest is 380773 bytes, SHA256
`0120228762abfe54dd477bc27853b3b45338d0ea5f19363a3d3b742224f5c556`, baseline
`d11e543ea32efdf784478f5959ea6565af87b646`. Reusing the retained manifest verifies
1585 blocks are assigned exactly once. The independently pinned canonical JSON
hash of `packages.baseline-03.blocks` is
`1b6367425a571b1bb0fcca0bc64a196a5f962bb28144dda17b06b23b453e7d67`.

The audited production base is `e0544b36756a9ac561c3e7ace936768951c574e3`, after
merging current main into the prescribed task branch. All 80 section/content
hashes match uniquely; original L343–540 now locate at L353–550. The three fenced
blocks are hashed as complete multi-line blocks with the maintained inventory
algorithm. ADR 006 changed only blocks outside this package. No source content
was changed or silently resealed. Changed or ambiguous package content rejects
and requires coordinator-approved old/new hash mapping.

The main integration retains baseline-01, baseline-03 and baseline-04 commands,
independent package digests and both neighboring ledgers. Baseline-03 now supplies
the explicit `obligation_kinds` and `exemptions` required by the current validator:
all 347 original duties and their routes remain unchanged. Background, future UI
work and the approved Device Authorization deferral retain their original reasons.
The production observations keep their original base SHA; merging audit metadata
does not turn those receipts into newly executed product tests.

## Reproduce and inspect

```sh
make audit-baseline-03
python3 -m unittest discover -s scripts -p 'test_release_*.py'
python3 scripts/check_docs.py
```

Launch loads the ledger through the existing audit CLI. Doctor checks the frozen
package digest, all dispositions and current reference hashes/symbols. Drive
invokes the same inventory and package validator. Evidence is
`evidence/baseline-03.json`, especially `package_audit`. Cleanup removes only that
disposable report. Input/output overlap is rejected before deletion, and an
invalid rerun removes stale output. The inherited corruption tests also run
against baseline-03: missing, duplicate, extra, changed or stale source/reference,
unmerged PR, unrouted duty, native byte tampering and Git provenance failures.

For full local integrity checks, supply independently retained inputs:

```sh
python3 scripts/release_audit.py \
  --package-ledger docs/audits/baseline-03.json \
  --work-packages /absolute/durable/work-packages.json \
  --native-root /absolute/durable/audit-20261024 \
  --verify-git --evidence evidence/baseline-03.json
```

The three explicit flags distinguish original-manifest checks, retained native
byte checks and audited Git blob/merged-commit ancestry checks. The default CI
run cannot inspect same-host native files or unavailable shallow history, so
these flags remain false there. Integrity checks bind evidence; independent
review must still assess whether the authored obligations cover the source.

`--require-complete` continues to exit **2**, with `status: inconclusive`,
`release_ready: false`, zero verified requirements and null semantic scores.
The separate release decision still requires independently acquired complete
evidence. Finishing an audit does not waive missing release obligations.

## Native observations and limits

Full raw results remain under the registered durable evidence root at
`ERII-233/e0544b36756a9ac561c3e7ace936768951c574e3/audit-20261024/`.
The ledger records exact commands, source SHA, timestamps, exit codes, file
sizes and SHA256. Browser screenshots and native reports remain local rather
than being bundled into repeated cloud archives. Production blobs referenced
by this ledger must still match both the audited base and current checkout.

| Executed entry | Observed result | Proof limit |
| --- | --- | --- |
| Focused Go contracts and supplements | 229 passing test events in the main stream, five supplementary Conversation/auth tests, one runtime normalization test, 16 TUI events and one progress contract; counts include subtests | The raw stream identifies exact tests. This is not the full Go suite or a release run. |
| Seven focused SQLite race tests | Automation dedup/edit admission, Profile CAS, Child snapshot, account single-flight, concurrent Delivery and scoped Artifact writers passed | One race run is not the required protected five-run release evidence. |
| `verify_automation.py` | 13 scenarios; real CLI/Actor/SQLite, threshold events, privacy fencing, fire/admission SIGKILL recovery and readback | Clock/model are deterministic; real long-running observations remain due. |
| `verify_artifacts.py` | Scope/integrity/retention/publication, six SIGKILL boundaries and both binary Query recovery paths | No real Guest/media understanding or long-session capacity claim. |
| `verify_local_provider.py` and `npm test --prefix provider-bridge` | Actual Go → Node/Pi → owned HTTP peer; Probe/Select/restart, manual ID, absent/explicit key, production Tool surface, selected-image wire and failure invalidation; Bridge/auth negative contracts passed | Scripted responses do not prove actual Qwen/vLLM/SGLang/LM Studio compatibility or semantic quality. Local Responses remains rejected. |
| `verify_account.py` | Actual CLI/current OAuth/Responses code with owned OAuth/JWKS/SecretStore peers, six-process refresh, rotation SIGKILL, reauthorization and secret scans | No real account, billing, OS credential backend or live subscription compatibility was certified. Device Authorization remains deferred. |
| `verify_channels.py` | Complete real HTTP/Chromium path, desktop/mobile/no-JS evidence, controlled components/fallback, attachment bytes, callbacks/approvals, frozen route/unknown-send recovery, account setup/logout and cleanup | Feishu/OAuth/model peers are deterministic. Current authorized channels/L2 and named human observations remain due. |

Locked setup used the maintained offline command and native download/browser
caches in this private checkout. No mutable dependency directory or owner
credentials were borrowed. Its offline advisory result is not an online security
audit. Harness receipts record joined children, stopped owned HTTP/browser peers
and removal of disposable product state.

## Remaining routes

| Route | Exact remaining responsibility | Independent exit |
| --- | --- | --- |
| [ERII-334](https://multica.ai/erii-studio/issues/01a16db6-1402-7615-a620-1880b9191b3b) | Observed Web setup gap: no discovered-model picker or confirmed capability/context presentation; local add fixes context/output to 8192/1024. TUI already offers catalog and bounded local context input. | A finite Host/Web PR with catalog/manual fallback, conservative confirmed capabilities, explicit Host-bounded context, actual HTTP/Chromium readback and Probe/Select/restart/Snapshot contracts, CI and independent review. It is Backlog and unassigned, not dispatched by this audit. |
| [ERII-330](https://multica.ai/erii-studio/issues/01a16bd2-3352-7bc8-8c8c-fc6045822909) | Real model decisions and output quality, actual local/account compatibility and multimodal proofs. Ordinary reply text has no independent `MEDIA:/absolute/path` rejection contract; structural Artifact validation does not prove model text compliance. | Original frozen dual-arm evaluations, actual profiles/native traces and independent scoring. Actual observations must resolve or explicitly reroute missing behavior; no Fake scores. |
| [ERII-331](https://multica.ai/erii-studio/issues/01a16bd2-481b-7404-8db1-971204d4f839) | Current authorized channel/media/Guest and named human observations, real login/credential backend and Host/Guest network/TLS boundaries | Same-release-SHA native state, Artifacts, exact approvals, cleanup and review of the same objects. Historical environment acceptance does not satisfy these observations. |
| [ERII-332](https://multica.ai/erii-studio/issues/01a16bd2-5d15-7af2-bb7f-7e5ff3c3c420) | Final per-duty protocol/field compatibility, Automation/privacy/notification/Artifact/snapshot integration, protected repetitions and operations/recovery evidence | Final normative join against the strict release consumer, preserving unmet duties and independent review. Source-specific product conflicts require finite implementation work; this report does not fix or waive them. |

The existing release children were read and reused by semantics. Only the
concrete Web implementation gap needed a new task. Each row retains its exact
remaining sub-obligations and proof limits; these routes do not assert that
an execution or report alone repairs a product defect.
