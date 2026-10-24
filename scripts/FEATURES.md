# Feature map

## Unscripted evaluation entry (ERII-230)

| Feature | Reach / authoritative entry point | Evidence and limits |
| --- | --- | --- |
| Explicit profile and frozen input execution | `unscripted_cases.py matrix/freeze/run`; locked `case_runner.variants`, production `chat send/work` and Pi Bridge | All 20 Cases have dispositions; 12 support six conversation input variants, eight lack adapters. Frozen source/binary/config/profile/input/limits reject drift. Native snapshots, requests, Tool/errors, Artifact bytes, spend/usage and restricted Judge packet retained; all current fragments remain ineligible and ungraded. |
| Offline entry verification | `verify_unscripted_cases.py --binary bin/erii --evidence evidence/unscripted.json`; providers shard | Launch private roots/maintained HTTP peer; Doctor actual Probe/Select; Drive production entry; Evidence native receipts and drift/failure/timeout rejection; Cleanup joined groups/listener and disposable roots. Strict release consumer rejects fragments. Loopback earns no semantic scores. |

See [unscripted evaluation operations](../docs/guides/unscripted-evals.md).

## Release evidence decision (ERII-224)

| Feature | Reach it | Authoritative entry point | Evidence and cleanup |
| --- | --- | --- | --- |
| Historical source inventory | `scripts/release_audit.py`; `make release-audit` | Preserved ERII-30 ledger, `inventory`, pinned historical reference index | All source text/identities and 122 partial/gap reviews survive; inventory stays inconclusive with zero certified requirements; unknown/stale references reject. Historical links do not import unmerged code or establish current proof. |
| Current normative source binding (ERII-226) | Existing audit/decision CLI after an approved baseline update | `release_audit.current_ledger`, `docs/v1-release-source-bindings.json` | Immutable original ledger bytes and all 122 review obligations retained; only exact unchanged section/content hashes relocate; changed/ambiguous/stale sources reject; new Task complete clauses remain pending and release inconclusive |
| Current release decision | `scripts/release_decision.py --pack DIR/pack.json --pack-sha256 TRUSTED_HASH --require-complete` | Private `decide`, `release_receipts.trial`, existing `case_runner.score` and `ci_gate.passed` | Exact-source code/test/merged PR/native evidence/independent review; 20 Cases × six variants × five runs × two frozen arms; dimensions/thresholds/hard failures, protected SC/safety repetitions, current L2/member review, recovery/operations/native CI and final human acceptance. External acquisition supplies trust; consumer binds bytes and recomputes scores. Actual release remains unevidenced. |
| Decision verification | `make verify-release-decision` | `test_release_decision.py:PackFixture` plus preserved inventory tests | Launch isolated synthetic file store; Doctor validates source/refs; Drive actual positive/inconclusive/rejected CLI paths; Evidence reads reports back; Cleanup removes owned stores/output. Complete joins retain all source blocks and both 20 × six × five cohorts; individual negative contracts call the production trial or `_operational_gates` validator directly. Positive combination is mechanism evidence only. No account/model/network calls, product changes or credentials. |

See [release evidence operations](../docs/guides/release-evidence.md) for trust,
private inputs, exit codes and remaining real release obligations.

## Frozen baseline-03 audit (ERII-233)

| Feature | Entry | Authoritative implementation | Evidence |
| --- | --- | --- | --- |
| Exact 80-block disposition, §3.11–§5.4 | `make audit-baseline-03` | Existing `release_audit.py`, `release_package.audit_package`, `docs/audits/baseline-03.json` | Owner package digest pinned independently; all 347 authored sub-obligations retained with current code/test hashes, merged PRs, native receipts, explicit background/deferrals and finite gap routes. Web setup gap routes to ERII-334. No verified requirements or release certification. |
| Integrity and read-only provenance | `python3 -m unittest discover -s scripts -p 'test_release_*.py'`; optional original manifest/native-root/`--verify-git` | `test_release_package.Baseline03Tests` reuses the corruption/CLI suite, plus schema-fence and mixed-deferral coverage | Launch/Doctor frozen inputs and current references; Drive maintained CLI; Evidence `evidence/baseline-03.json`, native flags and immutable local receipts; Cleanup disposable output only. Missing/duplicate/stale/unknown/unmerged/tampered evidence rejects; release-complete still exits 2. [Scope and observations](../docs/audits/baseline-03.md). |

## Frozen baseline-01 audit (ERII-231)

| Feature | Reach it | Authoritative entry point | Evidence and limits |
| --- | --- | --- | --- |
| Exact 80-block disposition | `make audit-baseline-01` | Existing `release_audit.py`, `release_package.audit_package`, `docs/audits/baseline-01.json` | Pins the owner package independently of review notes; retains source text, authored sub-obligations, current code/test hashes, merged PRs, native receipt identities and finite gap routes. Original 122 historical reviews stay separate. Always inconclusive, zero certified requirements and null semantic scores. |
| Audit integrity and stronger provenance | `python3 -m unittest discover -s scripts -p 'test_release_*.py'`; CLI optional manifest/native-root/`--verify-git` | `test_release_package.py`, existing inventory and decision tests | Isolated fixtures drive missing/duplicate/out-of-scope/stale text/ref/PR/gap rejection, native byte integrity, input/output overlap rejection and real CLI stale-output cleanup. Flags distinguish local integrity from original-manifest, retained-byte and Git-ancestry checks. No network fetch or copied credentials. |

See the [bounded audit ledger and proof limits](../docs/audits/README.md).

## Frozen baseline-04 audit (ERII-234)

| Feature | Reach it | Authoritative entry point | Evidence and limits |
| --- | --- | --- | --- |
| Account, Bridge and Code Mode dispositions | `make audit-baseline-04` | Existing `release_audit.py`, `release_package.audit_package`, `docs/audits/baseline-04.json` | Exactly 80 independently pinned source blocks, 224 authored sub-obligations with explicit mandatory/background/deferred scopes, current code/test hashes, actual merged PR provenance, seven native observation receipts and finite gap routes. All mandatory duties remain routed; no verified requirements or semantic scores. |
| Integrity and provenance | `make verify-release-decision`; optional manifest/native-root/`--verify-git` on the existing CLI | `test_release_baseline04.py` inherits maintained rejection contracts and checks explicit sub-obligation routing | Missing/duplicate/extra or changed blocks, stale references, unmerged PRs, altered native bytes and unexplained exemptions reject. CLI readback retains historical reviews and inconclusive release status; disposable reports are cleaned without touching inputs. |

See [baseline-04 observations and routes](../docs/audits/baseline-04.md).

Start with the [capability and gate directory](../docs/guides/verification.md).
`python3 scripts/verify_suite.py --list` reads the authoritative execution inventory;
`--gate NAME` drives a selected capability with its complete repetition count.
The detailed entries below preserve boundary, invariant and proof-limit records.

## Repository coherence and verification (ERII-201)

| Feature | Reach it | Authoritative entry point | Evidence and cleanup |
| --- | --- | --- | --- |
| CLI command ownership | Existing `erii model/chat/artifact/memory/skill/approval/automation/web/channel/delivery` commands | `cmd/erii/main.go` owns validation/admission/close; `models.go`, `conversation.go`, `resources.go`, `automation.go`, `channels.go` own parsing/presentation for their boundary | Existing CLI tests and complete suite drive unchanged commands, state, errors and crash/restart; every harness owns Launch/Doctor/Drive/Evidence/Cleanup. No schema or model-visible protocol changes. |
| Discover and focus verification | `verify_suite.py --list`, `--gate NAME` (repeatable); full `make verify` | `SHARDS`, `catalogue`, `select_gates` in `verify_suite.py` | `test_verification_selection.py` proves no-binary discovery, rejected unknown/duplicate/wrong-shard input, full repetition counts, stop on failure, joined process groups and isolated focused evidence; focused runs never publish complete summaries; both CI joins reject focused inputs and mark completeness only after reconstructing the entire inventory. |
| Documentation ownership | [README](../README.md), [documentation index](../docs/README.md), [contributor guide](../CONTRIBUTING.md) | README owns onboarding, docs index routes user guides, CONTRIBUTING owns development/layout/checks, verification guide maps capabilities to gates | `check_docs.py` checks repository AGENTS and local Markdown files/heading fragments, explicit HTML anchors, duplicate slugs and index reachability, including nested-list and used reference-style links; code examples, external URLs, unused reference definitions and the auto-managed runtime block are excluded. `test_project_layout.py` injects missing files/anchors and validates reference links, relative/encoded paths, list-relative code indentation and inline-code anchor exclusion, including real checker CLI failures/success. CONTRIBUTING owns the migrated workflow and history/evidence queries; historical reports retain their dates and limitations. |
| CI setup dependencies | `.github/actions/ci-inputs/action.yml`, required CI matrices | Pi/QuickJS remain universal for product gates; only unit/lint installs frontend build dependencies; unit/lint, delivery and five Browser M6 parts install Playwright and its browser, while Python compatibility installs only the pinned module | Selection/topology regressions preserve every required gate, both joins, five rounds and exact-source builds; full Ubuntu run supplies clean-runner dependency proof. Compared with 18 full installs, frontend installs remain 1; seven Browser gates install Playwright plus binaries, and Python compatibility adds a module-only install; timing claims need workflow evidence. |
| Minimum Python compatibility admission | Required `python_compat` job → stable `Ubuntu checks` | Exact-head checkout, pinned Python 3.10.22 and Node, maintained full Python discovery; `ci_gate.REQUIRED` | Launch isolated checkout; Doctor installs only the pinned Browser module needed by setup rejection tests; Drive runs the same complete `test_*.py` suite under the original ten-minute bound; Evidence retains separate SHA/attempt-bound logs; tests own disposable cleanup. Existing unit/lint, disk-backed five-run race, SDK and all other gates remain required. Topology and aggregate regressions reject missing, skipped, failed or cancelled compatibility evidence. No physical browser or copied SDK is needed by this job. |


## v1 release evidence inventory (ERII-30 assets; release incomplete)

`python3 scripts/release_audit.py --evidence evidence/release-audit.json`
indexes the frozen baseline/Golden source and checks candidate code/test/PR
references. `make release-audit` deliberately exits 2 while release completion
is inconclusive. See [scope, gaps and recovery evidence](../docs/v1-release-audit.md).
ERII-30's finite asset delivery preserves this inventory and all its limitations;
ERII-223 owns full release acceptance. The merged `release_decision.py` is a
separate evidence consumer, not an automatic upgrade of the inventory to PASS.

| Feature | Boundary | Authoritative implementation | Evidence |
| --- | --- | --- | --- |
| Inspectable release inventory | Local operator CLI above | `release_audit.py`, `docs/v1-release-ledger.json` | Launch loads the ledger; Doctor validates source hashes and references; Drive inventories each source block and binds individual partial/evidence-gap reviews to exact hashes/code/tests/unmerged PR/evidence; Evidence retains observed mechanics and unresolved subclauses, with zero verified requirements; Cleanup owns only output under `evidence/`. `test_release_audit.py` proves stale-report removal, source/ref/duplicate/PASS-claim rejection and nonzero completion status. No release or semantic/L2 PASS is inferred. |
| GC-08 exact wedding and pause/continue | `verify_gc08.py --binary bin/erii --evidence evidence/gc08.json`; communication verify gate; `prepare_release_eval.py` | MainHarness CLI → Main Task search/manage → Root → Artifact/Reporter → frozen MCP approval; same Main pause/append → new Root | Six source/wording variants retain the original three turns and add a locked continuation: C1→C2, 128 guests and unknown table count; one Task/five Roots, paused/cancelled epoch 4→5→6, all five Artifacts reopened; incomplete frozen safe pause resumes exactly one attempt without another domain transition; scoped read recovers guests without repeating them after compaction; changed wedding/deadlines; pending/denied zero-attempt mail, three actual SIGKILL/restarts and complete duplicate-input/denial/pause replay. Ten Python regressions plus Go fixture checks with the real Context Builder; native Worker → TaskUpdate → Reporter → final Main text/Artifact/delivery consistency, no repeated source/read/save/publish, and Judge rejection of failed completion; disposable state/process cleanup. Judge previews remain ineligible. Normal create still defaults to once; ongoing intent needs an owner decision. Semantic/L2, weeks/calendar/reminder variants remain due. [Scope](../docs/gc08-verification.md). |
| Steering after a crash during model preparation | `chat send/work`; Main steering and Root Task append | `Runs.Steering`, `Runs.taskSteering`, frozen `context_compaction` | `TestSteeringAfterCompactionBeforeGenerationAdmission` and `TestTaskAppendAfterCompactionBeforeGenerationAdmission` inject admission failure after context commits; the original step recovers unchanged, new input determines the later reply, with one delivery/Root result. Both enter SC-04's five-run conformance gate. `verify_main.py` launches an isolated CLI, checks setup, SIGKILLs at `after_compaction_commit`, appends steering, reads back the unchanged context/hash and final generation/Tool/delivery counts, then reaps processes and removes its root. |

## Production operations (ERII-29)

`python3 scripts/verify_operations.py --binary bin/erii --evidence evidence/operations.json`
owns isolated Launch, explicit provider/SecretStore Doctor, actual CLI Drive,
hash/state/trace Evidence, and joined-process/disposable-root Cleanup.
`make verify` runs five passes plus `verify_capacity.py` (three 1000-Child cycles in one state root); its event
gate drives a real CLI/NATS/Sidecar lifecycle. See [operations](../docs/guides/operations.md).

| Feature | Boundary | Authoritative implementation | Evidence |
| --- | --- | --- | --- |
| Staged production startup and graceful close | TUI, `web serve`, `automation work` | `app.OpenRuntime`, atomic runtime publication, `WithEvents`, existing Actor/Delivery/Memory paths; interactive selection waits for runtime admission and TUI acquires the current Actor | `TestProductionLifecycleCLI`: real binary, Provider Fake, NATS and Sidecar handshake, ordered stages, broker outage/reconnect, committed Inbox, active work completion, SIGTERM without cancellation/lease leaks; real TUI/Web OAuth harnesses share `event_broker.py` and verify immediate first input after selection, without inserted sleeps |
| Quiet poison isolation and developer alerts | `ops health/broker`, `ops retry-event ID ATTEMPTS` | Migration 026, existing Outbox publisher and Inbox; thin NATS adapter | Real authenticated NATS test: missing/wrong token refusal, durable content-free DLQ after threshold, good message before/after poison and restart; deterministic Outbox outage/non-transient distinction, no global head-of-line block, malformed envelope rejection, CAS requeue; local warning only |
| Bounded broker policy and forward adoption | NATS config, `ops upgrade-broker` | SecretStore token resolution, TLS requirement, explicit validated Stream adoption | Remote plaintext/no-token refusal; changed bounds and additional Stream configuration drift fail closed; old unbounded Stream upgrades with retained message; reopen/restart reuses existing reservations within a 2 GiB broker quota; test-broker capacity stays fixed across restart independently of shared CI tmpfs free space |
| Operator diagnostics and shareable report | `ops health/report/trace RUN_ID` | Typed operator projection over existing RunTrace, scheduler and recovery ledgers; bounded multiprocess log rotation | CLI content/credential sentinel exclusion, correlated Run/Parent/Root and Tool IDs; counts/read-only state; Go concurrent rotation JSON and size bounds; no model Tool or user notification |
| Encrypted cold backup and isolated restore | `ops backup/restore ARCHIVE KEY_REF`, explicit `ops activate` cutover | `maintenance`, shared/exclusive root lock, authenticated streaming archive, schema/hash/integrity/Artifact checks | Real held process blocks backup; SIGKILL releases lock; every restored file hash matches; Memory ciphertext/receipts and credential refs retained; frozen approval identity/hash preserved, denial recovers without an effect; wrong key, tamper/truncate/append, existing-target, symlink refusal; automatic cleanup |
| Compatible upgrade and capacity | Startup preflight; existing batch fixture | All-DB version preflight, forward migrations, existing scheduler/breaker/account approval | Populated v25 accepted work/snapshot retention and one completion; partial DDL rollback; future Memory version blocks state migration; every-prior migrations; default 1000 Children completes 998 + two business failures, one aggregate blocker, no safety fuse/orphan; `measure_capacity.py` retains same-machine samples/noise for unchanged 100-item fairness/cancel/restart workload |

Release model quality, semantic scores and live accounts remain separate claims.

## Installation and project layout (ERII-183)

| Feature | Reach it | Authoritative entry point | Evidence and cleanup |
| --- | --- | --- | --- |
| Install and launch Erii | `./install.sh`, then `erii`; `erii doctor`; explicit `erii start` | `cmd/erii`, `install.sh`, `scripts/package.sh`, `internal/startup`; installed runtime discovery uses executable location | Required providers gate `verify_startup.py`: source build → native archive → install into a path with spaces → read-only Doctor → real PTY setup, explicit Pi Probe/select, Main/Direct Tool/reply → SIGKILL/restart without duplicated state → reinstall without changing data; unmanaged executable rejection; all temporary processes/state/packages removed. No live model/account claim. |
| Managed desktop broker | Bare terminal `erii`, or `bin/erii start`; external broker via `-config` or `ERII_NATS_URL` | `startup.OpenLocalBroker`, existing `app.OpenRuntime/WithEvents` | `TestLocalBroker*` provisions the production stream and DLQ, proves exclusive desktop ownership, shared maintenance lock, disk-backed notification reopen and full shutdown; cancelled startup releases ownership. Existing lifecycle order and replay gates remain required. |
| Frontend source and documentation | `web/`; [documentation index](../docs/README.md); `make lint` | `web/build.mjs`, embedded assets in `internal/interaction/web/assets`, `check_docs.py` | `npm run check --prefix web` rejects stale assets; maintained channels browser drive proves composition/approval/fallback; local Markdown destinations checked offline. Historical measurements live in `docs/reports/`. |
| Native release packaging | `sh scripts/package.sh "$PWD/dist"`; manual/tag package workflow | `.github/workflows/release.yml`, same installer | Native Ubuntu/macOS matrix produces archives/checksums; version tags create unpublished drafts. Local startup gate proves the host-native package; other OS/architecture results require their actual workflow evidence. PR safety gates are unchanged. |
| Conservative branch cleanup | `python3 scripts/clean_branches.py` (plan), then `--apply` | `clean_branches.select/plan/apply` | Exact merge proof, protected main/worktrees/open PRs, local Git merged guard, remote exact-tip lease, stale tracking-ref pruning; audit in `evidence/branches.json`. Unique work stays intact. |
| Locked repeatable dependency setup | `make setup`; `SETUP_FLAGS='--mode offline --measure-network'`; CI packages/browser parts | `scripts/setup.py`, `setup_browser.cjs`, native npm/Playwright caches | `setup.json` binds lock/toolchain/platform/architecture and stage timings; optional Node HTTP object/body-byte evidence. `test_setup.py` drives real npm against a local registry: cold/warm/offline, SRI corruption, missing/changed locks, platform/architecture rejection, concurrent independent installs, lock/timeout cleanup. Browser Doctor checks both locked Chromium variants and FFmpeg; offline misses never call install. Disposable fixture caches are removed; retained native caches are never wiped. [Recovery and measurement limits](../CONTRIBUTING.md#repeated-and-offline-setup). |

## M6 personal journeys (ERII-26)

`ERII_VERIFY_CASE_JOBS=4 make verify VERIFY_SHARD=personal VERIFY_JOBS=1` owns Launch/Doctor/Drive/Evidence/Cleanup for eleven exact M6 worlds × six wording/noise/order variants × five serial runs. See [M6 scope and proof limits](../docs/guides/m6-evals.md). A focused case uses `scripts/run_m6_cases.py --binary bin/erii --case GC-13 --evidence evidence/m6-booking.json`.

| Feature | User boundary | Authoritative implementation | Inspectable evidence |
| --- | --- | --- | --- |
| Personal context and correction | `chat send/work`, `memory manage/work`, `builtin__memory__search` | Existing Main/Worker loop, memory mutation/Curator, separate Memory/Conversation stores and unified recall | GC-01/02/03/05/06/10 exact fixture inputs, scoped health/time/location data, real supersede/deletion and future context readback; exact finance computation; no unsolicited Tasks or external writes |
| Monthly goals review after explicit request | `run_m6_cases.py --binary bin/erii --case GC-02 --evidence evidence/goals.json`; Memory part of maintained five-round personal gate | Four ordered chat turns → Main `builtin__automation__manage` → `automation list`; existing Memory/Automation stores | Launch isolated roots, Doctor activated Fake and fixed Case scheduling clock, Drive six wording/noise/order journeys, Evidence original events plus no pre-request Task/schedule, one Main call, Nov29 20:00 +08 first fire and duplicate-input state equality, Cleanup joined processes/temp roots; strict M6 join requires all monthly assertions in every round; semantic grades remain null |
| Travel continuation and steering | Main `task_manage(create/append/pause)` during a held Root read | Existing Task store and Root instruction boundary | GC-04 shared Recall/Main publication barriers and zero new Worker requests before publication; two existing Tasks, paused NZ, France new Root, concurrent steering before completion, immutable confirmed itineraries, booking groups and ETA Artifact |
| Network and booking sandbox | Code Mode `cua.*` and delayed Playwright with actual Chromium pages | Existing ToolExecutor, frozen approvals, DesktopLease, production browser adapter; explicit Fake Lume/SSH peer; shared `guest-browser/readiness.mjs` waits for CDP publication within Doctor's deadline | GC-07 observation/denied repair/four paths/one protocol; GC-13 exact final confirmation, denial, lost acknowledgement, `unknown`, read-only `order_781` reconciliation, one submission, unchanged frozen parameters; `guest-browser/readiness.test.mjs` covers delayed/malformed publication, deadline and filesystem failure; `test_m6_cases.py` proves cleanup preserves the primary error and rejects a bad exit |
| Multimodal source provenance | `chat send-parts` with images, Artifact save/open/describe/publish | Scoped image envelope; Fake validates actual JPEG pixels; immutable ArtifactStore | GC-15 real readable PNGs, pixel rectangles/hash/dimensions, masked return year null, openable `visa_check.md`; negative tamper/region/mask tests; optional actual Vision observations via `m6_sources.py` |
| Mixed deliverables and desktop materials | Three Main-created Tasks → Root work → Main; scoped video Artifacts and browser CUA | Same Agent Loop and Task/Tool/Artifact/Computer path | GC-18 later confirmed notes, standalone technical design, exact 416/128/16 CSV summary, three valid videos decoded/organized in real browser; screenshot and original-byte readback |
| Five-run result integrity | `make verify`, `m6_report.aggregate` | Locked fixtures, exact binary/variant/assertion inventories | `m6-cases-1…5.json`, `m6-summary.json`; CI `ci_personal.py` requires seven fixed parts and rejects missing/duplicate/failed/drifted input; retained CLI-opened files/screenshots under `m6-cases-N-assets/`; all children joined and disposable state removed; semantic/product scores remain null, current L2 unexecuted |
| Delivery gate capacity and series integrity | `verify_suite.py --shard delivery --gate-jobs 2`; existing CI matrix | `run_shard` → existing two-worker executor; independent delivery/channel harnesses | Launch/Doctor isolated reports and roots; Drive overlaps the two complete five-round series; Evidence retains canonical ten rows, original actual UI/lease/approval/SIGKILL assertions and unchanged deadlines; Cleanup drains owned groups. Real-subprocess mutual-admission regression reproduces the former serial dispatch despite a two-worker request and requires per-series 1…5 order. No production concurrency or longer job budget. |

Browser launch Doctor uses `guest-browser/readiness.mjs` in both the real
Browser contract test and M6 Computer journeys. The pipe handshake is not proof
that Chromium published `DevToolsActivePort`. Delayed, missing, empty/invalid
port files wait within the shared five-second Browser readiness budget;
non-transient filesystem errors fail immediately. `guest-browser/readiness.test.mjs`
reproduces delayed/partial publication and asserts timeout/error rejection.
The M6 fifteen-second launch deadline and every product assertion remain intact.

## Computer Use (ERII-25)

| Feature | User boundary | Authoritative implementation | Inspectable evidence |
| --- | --- | --- | --- |
| Live inventory and explicit binding | `cua.getState/list*`, `getApp/getBrowser/getTab`; explicit `launchApp/createBrowserTab` | `computer.UseRuntime`, `macosvm` SSH/Cua and Guest Playwright adapters | `verify_computer_use.py`: CLI/QuickJS/ToolExecutor/real adapter protocol peers; partial inventory failures in Go; existing-resource binding never launches; duplicate window titles retain distinct IDs and renaming retains identity |
| DesktopLease and versioned observations | Bound `getAXState/getScreenshot/getAXStateAndScreenshot`, input, secondary actions, `waitForChange` | Frozen Tool policy/conflict keys, persisted `computer_use`, common Run/Code fences | Real AppKit live fixture, screenshot Artifact and AX readback; deterministic old epoch/handle/element/capability refusal, independent image revisions, atomic Artifact/result/reference readiness, read barrier, bounded WAIT_TIMEOUT summary; controlled approval resumes same physical lease; unknown input executes once |
| Scoped Guest files and independent consent | `cua.files.importArtifact/exportArtifact`; Playwright chooser/download | Scoped ArtifactStore, `computer_file`, fixed Guest Transfer Root and `transfer.py` | Byte-for-byte roundtrip; separately frozen import/overwrite/export/upload approvals, denial before dispatch; Python traversal/root/component symlink/hardlink/FIFO refusal before truncation |
| Delayed Playwright | Tab `capabilities.list/get("playwright")`, locator and navigation waits | Every method uses ordinary nested ToolInvocation; pinned Guest Playwright Core 1.64.1; document-scoped CSS/actionability waiting | `npm test --prefix guest-browser`: actual Chromium/local website; synchronized disabled-click, readonly-fill and missing-selector waits followed by navigation leave new-document controls untouched; same-document CSS replacement succeeds and core ElementRef replacement fails; existing input/download/chooser contracts; `verify_computer_use.py --live-browser` drives the production SSH adapter; no online accounts |
| Explicit screenshot output | Verified image profile via `model test/select`, then `image(screenshot.image)` | Optional behavioral image Probe; scoped refs, Host dispatch-time read/downsample, frozen ImageInput capability, actual Pi Bridge | `verify_local_provider.py`: normal CLI setup → selection → Run → screenshot cell → Pi HTTP; unselected/selected image counts `[0,0,1]`, text-only `[0,0,0]`, image retest rejection closes ingress; generation ledger retains refs only. Storage tests reject old text-only proof for image admission; existing Artifact scope/observer contracts remain |
| Schema and replay | Open populated v21; existing `chat work` recovery | Migration 022, CAS, existing CodeCall/ToolInvocation replay | Empty/prior schema upgrades, retained accepted request, illegal version/CAS rejection; no replay of unknown input; parent scopes/old owner fences remain enforced |

Launch/Doctor/Drive/Evidence/Cleanup:
`python3 scripts/verify_computer_use.py --binary bin/erii --evidence evidence/computer-use.json`.
`make verify` repeats it five times; `make test` runs actual Chromium and transfer
security contracts. [Computer Use operations](../docs/guides/computer-use.md) documents
Guest setup, API limits, native selection restrictions and the opt-in real Lume
AppKit drive. No semantic Golden Case score is inferred from these contracts.

## Durable Computer and Live Shadow (ERII-24)

`scripts/lume_live.py --root TEST_ROOT` provides opt-in local L2 mechanics.
See [Computer acceptance](../docs/reports/computer-l2.md) for authorization, dedicated
storage, pinned releases and the full acceptance matrix. The pinned 26.5.2 Guest
combination has actual Jianying acceptance evidence. [Lifecycle operations](../docs/guides/computer.md)
describe the production adapter, configuration and retained limits.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Owned Lume environment | `doctor`, foreground `serve` | `lume_live.Live`, pinned official Lume CLI | Actual signature/version/config readback; ownership and symlink refusal; disposable host probe denied by Seatbelt while owned fixture remains readable |
| Resumable clean image | `download`, then `create`; `pull-tested-image` | `Live.download`, Lume `create` with explicit owned storage | Per-range SHA-256 receipts, exact HTTP ranges, full Apple image digest before provisioning; refuses an existing VM |
| Guest observation and cleanup | `start`, `guest`, `put`, `get`, `driver`, `vnc-session control`, `vnc-control`, `state`, `stop` | Owned loopback Lume HTTP API and Guest SSH/VNC | Actual 11.6.0 import/timeline/subtitle/dissolve/MOV+MP3 export; signed Driver consent and raw capture refusal; cold/SIGKILL restart with six matching file hashes. Host/Guest/VNC constraints are explicit in the acceptance report |
| Production logical Computer | Configured `cua.getState()` through Code Mode; read-only `computer status` | `computer.Manager`, fenced `agent/sqlite` ledger, `computer/macosvm.Provider` | `verify_computer.py` runs the real CLI/HTTP/SSH adapter with offline peers five times; stable ID, increasing epochs, old-owner refusal, exclusive use, unchanged Artifact bytes |
| Durable unit and recovery | Automatic cleanup of the accepted Code Mode invocation | Atomic disk/NVRAM/config/manifest unit; Guest OS shutdown; native lock proof; guarded cache reconciliation | Publication-boundary and post-disposal/pre-final-commit process death; frozen idle IDs survive claim takeover; Main Task cancellation at readiness and post-disposal via `ComputerHarness.cancel_at`; new invocation reconciles retained idle request before restoration; cancelled Task/Run/Code remain terminal; committed unit is verified with its source absent; fresh cell succeeds; corrupted NVRAM, ambiguous start refusal, blocked shutdown, source/target hashes; no session/heap migration; owned instance deletion and process joins |
| Real provider lifecycle | `verify_computer.py --live-root TEST_ROOT --live-config TEST_ROOT/product-live.json`; `lume_live.py --profile product serve` | Same production adapter and CLI; deterministic model fixture, actual Lume/macOS/signed Driver | Guest file/TextEdit reopening, Agent SIGKILL at readiness and after disposal, resumed/fresh cell success, epoch/identity and Artifact readback, independent full-unit SHA-256, stopped/deleted instance; foreground API joined after acceptance |

The live harness owns Launch (temporary API/new VM), Doctor (versions,
storage, Guest session and Driver permissions), Drive (synthetic Jianying
scenario), Evidence (Guest screenshots, export/project hashes, restart state),
and Cleanup (stop VM, join the owned server, detach owned installation media).
Each phase must be observed before a live acceptance claim.

`verify_computer.py` owns Launch (isolated CLI and explicit HTTP/SSH peers), Doctor
(base/config/selected fixture), Drive (nested `cua.getState()`), Evidence (SQLite,
complete unit hashes, restoration intent, Artifact readback and fault receipts),
and Cleanup (native resource/instance checks plus process joins). Its offline
five-run gate is in the Code Mode shard. The live mode uses only an explicitly
marked owned test root, requires separate foreground API cleanup, and retains
private domain state and checkpoints as inspectable evidence. Surface binding,
inventory and desktop actions are covered by the ERII-25 Computer Use entry above.

The offline SSH peer consumes the complete sudo input before acknowledging
shutdown. `TestShutdownRequiresCompleteSudoInput` rejects truncated input and
proves that only the accepted command changes its shutdown counter. This keeps
the Fake from closing the channel while the real client is still sending input.

## Memory maintenance, context and experience (ERII-23)

`python3 scripts/verify_memory_maintenance.py --binary bin/erii --evidence evidence/memory-maintenance.json`
owns isolated Launch, selected-profile/Fake Doctor, CLI/Tool Drive, persisted
Evidence and Cleanup. `make verify` repeats it five times in the security shard.
See [maintenance operations](../docs/guides/memory-maintenance.md) and [ADR 003](../docs/adr/003-memory-forget-revokes-unfinished-context.md).
Failure exits preserve bounded Run/Tool/model state and Memory/compaction readbacks
before disposable cleanup in the requested evidence JSON, with `passed: false`.
Snapshot/read/write errors retain the original failure; interrupt cleanup is unchanged.
`test_memory_failure_evidence.py` verifies capture-before-cleanup and original-error
retention when stores or the evidence destination fail.

| Feature | User boundary | Authoritative implementation | Evidence |
| --- | --- | --- | --- |
| Automatic evidence-backed Memory | Configured foreground `chat work`; explicit `memory work/status` | `Runs.observeMemory/CurateMemory`, Memory receipt/evidence transaction | Health, goal and learning reports stay Recall; no maintenance delivery; serial batches; kill after Memory commit reuses receipt |
| Remember/correct/forget | Main `builtin__memory__manage`, authenticated `memory manage REQUEST_KEY` | Unified ToolExecutor; durable mutation + privacy barrier; `memory/sqlite` supersession/tombstones | Finance correction, encrypted read-only recall, same-key receipts, lineage/source/category suppression, policy replay; remember → unrelated Forget → identical fresh remember yields one hit with new provenance; duplicate/out-of-order evidence cannot replace it |
| Forget overrides unfinished replay | Same Forget control and `chat send` | Privacy epoch, Run/Code fences, guarded Provider pipe dispatch, controlled-stop recovery; immutable ingress owns a queued successor while revoked owners join | Queued input → Forget → fresh input before settlement produces one answer from a new Run; live owner held after request freeze → Forget → fresh input → kill/restart completes the successor; Claim/scheduler wait for old owner settlement; audit retained and new requests exclude old content; v19 migration preserves input/owner; Task state not implicitly cancelled |
| Sensitive records | Explicit key reference, authorized recall | AES-GCM, keyed dedup, scoped in-memory FTS/vector RRF with selected loopback embedding; prior-data sealing/cleanup marker | Memory DB/WAL scans, private vector-only synonym, remote embedding rejection before any request; key loss/tampering fail closed; immutable originals retained |
| Hierarchical context | Shared Main/Worker generation boundary | Frozen policy; complete Tool exchange removal, bounded extractive summaries; Conversation schema 3 and state manifests | CLI long history, original IDs and provenance readback; stable input budget and runtime-origin tests; one context-length recovery with unchanged Snapshot and no Tool rerun |
| Verified Skill experience | Trusted `skill manage` Reviewer control | Candidate CAS, method-bound verification ledger, full catalog/publication admission under a directory lock, bounded active loading, staged recovery and retained retirement | Two real workspace-test Runs; punctuation metadata; 127 → 128 active publication; full rejection leaves verified v3; retire then retry same candidate/version; fresh-process status/list/view/retire with >128 retained/pending entries; contracts cover metadata, capacity, concurrent last-slot ownership, writer kill/restart and unsafe staging rejection |

These are M6 contract and GC-01/02/03/06 fragment checks, not full product scores or
Live Shadow quality. Scope-wide Forget invalidation and conservative extraction
limits are explicit in the operations notes.

## Durable context recall (ERII-22)

`python3 scripts/verify_recall.py --binary bin/erii --evidence evidence/recall.json`
owns Launch/Doctor/Drive/Evidence/Cleanup with isolated databases and explicit HTTP
embedding/LLM Fakes. `make verify` runs five repetitions. See [recall operations](../docs/guides/recall.md).

| Feature | User boundary | Authoritative implementation | Evidence |
| --- | --- | --- | --- |
| Unified scoped recall | Configured `builtin__memory__search` through Main/Root/Child | Host scope in `agent/sqlite.searchMemory`, common ToolExecutor, MemoryProvider/ConversationProvider | Three real invocations; user/Agent exclusions, source/time/author without scores, vector-only synonym in both sources; controlled success ordering and forced real CLI pending-append rejection with no partial Worker admission, then explicit caller recovery; all original assertions and five-run series retained; unexpected failures retain structural diagnostics and still fail without retry |
| Memory/Conversation authority | Existing chat ingress and separate provider databases | `memory/sqlite`, `conversation/sqlite`, state migration 018 | Original messages unchanged, immutable evidence, empty/populated-prior migration, atomic index rollback, tombstone/supersession and conflict contracts; maintained recall CLI multipart/pending-v2 and multipart-prior scenarios: 80,001-byte original, bounded UTF-8 embedding chunks, nine generations/four completed Runs, inert repeated drain; actual HTTP regression covers max 128 × 64 KiB parts and mid-chunk failure/retry without partial vector |
| Hybrid ranking | Explicit embedding configuration and read-only search | `recall/embedding`, private `sqliteindex`, provider-specific RRF ordering | Actual HTTP adapter, FTS5/BM25 + exact cosine, time/tier/rank boundaries, no partial result on embedding/index fault, concurrent independent retrieval |
| Frozen context | Shared model-generation admission | `run_recall`, `Runs.recallMessages` | SIGKILL after freeze, corpus changes, exact snapshot reused on restart; new turn sees new data, stable prefix/dynamic suffix, no Parent mutable context copied, old-owner rejection |

No embedding configuration means no recall capability; this is not a full vector
completion claim for that installation. Curator, explicit management and compaction are covered by the ERII-23 entry above;
these retrieval-only checks do not score full SC-08.

Run `make verify` from the repository root. `scripts/verify.py` drives the built
CLI and independently reads SQLite/JSONL. `internal/event/integration` owns the
JetStream and process-kill mechanics. No online account or installed broker is
required. Each harness launches disposable state, checks readiness, drives the
real boundary, records evidence, and always reaps its processes/removes state.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Complete offline CI gate | `make verify`; serial baseline with `VERIFY_JOBS=1`; isolated `VERIFY_SHARD=runtime/codemode/security/delivery/personal/providers`; focused `ci_cancel.py --entry make-step` | `verify_suite.py`, `ci_personal.py`, `fixture_pool.py`, `ci_cancel.py`, `ci_step.py`, `.github/workflows/ci.yml`, `ci_gate.py`, `ci_codemode.py`, `ci_race.py`, `ci_race_join.py`, `ci_tmpfs.py`, `.github/actions/ci-inputs` | Ten-minute workflow baseline / five-minute target; job timeouts ≤10 minutes except the approved fifteen-minute Fast/Race/Delivery outer bounds, with topology regression; measured workflow wall/queue via `ci_profile.py`; complete original inventory and serial five-run series; M6 uses seven independent Required parts, four isolated fixture workers (two for travel), compact full evidence after join, and a strict M6 join reconstructing all 330 variants; five parallel Required Code Mode parts reconstruct all 90 variants, five SC/cancel proofs and 25 logical rows; join rejects missing/duplicate/failed/drifted input; bounded independent fixture/gate scheduling; official Ubuntu HTTPS APT mirror avoids observed 8–9 minute Azure mirror prerequisite downloads without changing package selection or job limits; Go model/event checks isolated; Race CI has independent packages/SQLite parts, each inheriting main’s ERII-222 fifteen-minute outer timeout, and a Required exact-source join; Bridge remains isolated before other packages, SQLite retains its serial approval test and up to three workers, and the join re-reads native discovery/events to reject missing/duplicate/failed/drifted roots, changed commands or unjoined processes; local make test-race remains complete. SQLite race tests are auto-discovered; the unchanged five-second Child approval contract runs alone before other tests run once across up to three processes within a four-CPU budget; longest-measured-first placement uses provenance-linked Ubuntu durations only as scheduling hints, automatically admits new roots and records unmeasured roots; missing/duplicate/failed test roots reject the verdict; providers/race bounded tmpfs with executable/WAL Doctor and identity-checked cleanup; full unit, five SC race rounds and five focused receipt race runs retain disk coverage; frozen source/binary and hardware records; failure/cancel/skip rejects stable `Ubuntu checks`; 14-day exact-SHA artifacts. Shell `exec` makes each phase owner receive the Actions entry-PID signal. Atomic first-signal receipt is separate from final cleanup; repeated profiler termination notifications cannot interrupt finally cleanup; five-second escalation leaves room before the Actions hard stop. Real script-file entry, two-stubborn-session, uncooperative-child and four-real-Core Make-chain probes prove descendant/state cleanup. See [operations](../docs/ci.md). |
| Copied public CI SDK | `ci_sdk.py prepare` → `doctor` → `run -- …` → `cleanup`; Required pipeline | `ci_sdk.py`, `TestCIInstalledSDKConformance`, `.github/actions/ci-inputs` | Python 3.10-compatible streaming digests; complete Python regressions additionally required on 3.10.22. Original SDK unchanged; fixed offline public roots; immutable copied tree; denied SDK write and sibling read; uncached std program executes with private writable cache. Every job builds from its own exact source; no cross-runner build reuse. Historical bundle v2 corruption/context tests remain intact. [Operations](../docs/ci.md). |
| CI profiling | `python3 scripts/ci_profile.py --limit 30 --output evidence/ci-history.json`; `ci_compare.py --pair … --artifact-root … --profile …` / `--offline`; `ci_supersede.py push/collect` | `ci_profile.py`, `ci_compare.py`, `ci_github.py`, `ci_step.py`, `ci_supersede.py` | Current single-ci.yml supersession collector and CLI regression reject missing/duplicate/unknown workflow, source or completion proof; no shadow attempt required. Exact-attempt wall/runner-minute evidence; raw API snapshots; unknown timestamps remain null; Current workflow-derived 25-job topology, canonical suite/part/join inventories and full repetitions; clean source/binary/image/CPU and frozen profile must match; three independent pairs required, insufficient cohorts remain noncertified. Exact seven-artifact retrieval uses ERII-206 metadata/hash reuse with a shared 1 MiB payload budget. Explicit `--metadata-only` retains exact-run observations without archive transfers and cannot certify; delivered ERII-207 comparison limits are recorded in [the receipt](../docs/reports/2026-10-24-ci-comparison.md). Historical 21-job run 39915092608:1 metadata fixture retains 838s/108.4 runner-minutes; its incomplete current job inventory and missing suite evidence still reject. Explicit historical nine-job/five-pair mode never certifies current performance; retired experiment CLI rejects without GitHub mutation. No repeated experiment or shadow workflow in the Required path. See [operations](../docs/ci.md). |
| M5 exact Core case gate | `make verify VERIFY_SHARD=codemode`; focused `scripts/run_core_cases.py --binary bin/erii --evidence evidence/core-probe.json --case GC-17` (also `GC-19` or `GC-20`) | `run_core_cases`, `core_conformance`, `core_report`, `ci_cancel`; CLI → Actor/Loop → ToolExecutor/QuickJS | Six locked variants × five serial stability rounds; independent variants can overlap at capacity four. Exact seed1701 CSV/Artifact readback; real Sidecar+Go SIGKILL at 37 completed/8 waiting; F055 drift rejection; unchanged audit/dirty CLI patch; committed migration reused, independent review/repair, final one-Task readback and isolated grandchild cancellation. Each full round also cancels four held real Core fixtures and proves descendant/state cleanup; aggregation requires all five proofs. SC-01…12 exercise implemented stores/executors with explicit external-provider Fakes and ungraded semantic scores. Within each serial round, audited root partitions use at most four single-CPU workers; missing/duplicate/skipped/failed roots reject, and all owned groups join before the cancellation probe. `core-cases-1…5.json`, `core-summary.json`, state/trace/ledgers. [Scope and recovery](../docs/guides/core-evals.md). |
| Bounded GC-19 diagnosis (ERII-226) | `scripts/diagnose_gc19.py --evidence evidence/gc19-diagnostic` | Separate Go overlay binary, unchanged transaction body and normal exact-source gate | Five original rounds at four-way concurrency; opt-in disposable capability fixtures retain raw transaction code/stack and read-only SQLite backups before cleanup. No credentials/configuration/stdin copies. Diagnostic reports never replace the normal binary's acceptance evidence; `test_gc19_diagnostic.py` checks source drift, WAL capture and no retry. |
| Help/doctor | `bin/erii -help` | `cmd/erii:run` | Usage; no state created |
| Startup diagnostics | `bin/erii [-config path] status`; no-command with redirected stdin | `agent/sqlite.Open`, `Store.Diagnostics`; derived `schema.sql` | setup_required, null profile, closed ingress; stored identity. Fresh schema matches every historical table/index/trigger and empty initial data; digest drift rejects startup; prior versions upgrade normally; initialization and later migrations roll back together. Actual commit-boundary kill/restart preserves schema/version and one bootstrap event. |
| Diagnostic command | `bin/erii [-config path] status` | `cmd/erii:run` | JSON object, zero exit |
| Configuration | `-config`, `ERII_*` overrides | `config.Load` | Invalid fields/URLs/durations rejected without value echo |
| SQLite bootstrap | Either startup path | `agent/sqlite.Open` | Schema v17, WAL, one Agent, one atomic fresh-bootstrap event |
| Migration safety | Empty, real v1/v2/v3/v4/v5/v6, future schema | `agent/sqlite.migrate` | Prior Agent/config/timestamps, model metadata/snapshots and Outbox/Inbox preserved; future rejected; failed v3/v4/v5/v6/v7 DDL rolls back |
| Model metadata | `bin/erii model add` (stdin JSON), `model list`, `status` | `llm/sqlite.Store.AddProfile/ListProfiles` | New process and SIGKILL/restart preserve profile; unverified/version 1; independent SQLite/canonical JSON readback; ingress stays closed |
| Metadata validation | `model add` | `llm.ProfileInput.Validate`, CLI decoder | Unknown/credential fields, credential-bearing URLs, invalid enums, dangling/mismatched auth and pre-Probe activation rejected without value echo or writes |
| Metadata CAS | Host storage, no CLI update surface | `llm/sqlite.Store.UpdateProfile/UpdateAuthState` | One winner across DB connections; stale CAS and event failures leave metadata/Outbox unchanged; profile edits reset verification |
| Secret references | `model login [ENV_NAME]`; trusted Host only | `llm.RegisterAuth`, `llm/secret`, `llm/memory` test Fake | Put before metadata; database failure cleans new reference even after cancellation; cleanup failure explicit; DB/WAL/SHM and output exclude credential material |
| Immutable model snapshot | Main Run creation and trusted transaction fixture | `llm/sqlite.Store.FreezeProfile/Snapshot` | Ready-only, frozen profile version and canonical endpoint hash; no CredentialRef; auth/profile changes preserve snapshot; update/delete/REPLACE rejected; outer transaction rollback removes snapshot/event |
| Provider Bridge | Explicit `-test-bridge success` | `llm/bridge.Open`, `llm/fake.Serve` | Version handshake, long-lived stdio JSON-RPC, sanitized errors, process crash/restart, orderly shutdown/reap |
| Catalog and auth fixtures | `model catalog`; Bridge contracts | `ModelCatalog`, `ProviderAuth` | Candidate catalog and refresh; UI-neutral auth events, continue/refresh/status/logout |
| Capability Probe | `model test ID`; opt-in `verify_local_provider.py --live-endpoint URL --live-model ID` | `llm.Control.Test/TestWithProgress`, `llm.Prober.RunWithProgress` | Actual stream, Tool Call/Result continuation, cancellation and clipping; independent 25-second checks; 11-second cold model regression; deadline/caller-cancel distinction at every stage; timed-out cancellation cannot certify support; safe stage errors preserve normalized causes and proof invalidation. Live Shadow uses isolated state and records proof/timings without selecting a profile. Tool-less models stay rejected. |
| Atomic selection | `-test-bridge success model select ID PROFILE_VERSION CONFIG_VERSION` | `llm.Control.Select`, `llm/sqlite.Store.Select` | Version-bound Probe; profile/config/Outbox rollback; concurrent config CAS; v3 migration preserves auth/profile/snapshot/events |
| Effective readiness | `status`, foreground startup | `agent.EffectiveReadiness`, `llm.Control.Available` | Explicit test injection required after restart; absent/invalid profiles, credentials and incompatible Bridge close ingress; ready profile survives SIGKILL/restart |
| Cancelled setup | `model cancel` | CLI read-only diagnostics path | setup_required and identical SQLite dump; no default provider |
| Generation contract | Bridge tests repeated by `make verify` | `LLMProvider.Generate/Continue/Cancel/Dispose` | Sequence dedup/drift/gaps, one completion, immutable snapshot/session identity, partial-crash rejection, received completion before EOF, clipping and bounded errors |
| Burst delivery with slow sinks | Real subprocess and mailbox regressions repeated by `make verify` | `bridge.eventStream.accept`, `bridge.Client.generate` | 129-event slow sink; full 4096-event burst, duplicates and pre-ack events retained; clipping before buffering; concurrent catalog/cancel/Close progress while sink waits; overflow and changed duplicates fail closed |
| Generation rejection and explicit cancellation | Real lifecycle peer regressions repeated by `make verify` | `bridge.Client.generate/Cancel`, `bridge.eventStream.cancel` | Host rejection cancels accepted work or retires peer with original error and no replay; provider terminal error skips cleanup; ack-only Cancel settles matching local call, interrupts pre-ack wait and fences late/queued completion; independent catalog and ledger prove state |
| Cancel before dispatch | Real subprocess race and held-registration regressions repeated by `make verify` | `bridge.Client.exchange/Cancel`, `eventStream.dispatchDone` | Valid 900,000-byte Generate/Continue requests race cancellation after registration; peer observes exactly one start and cancel, no active work or replay; dispatch wait respects deadline and leaves catalog/Close available |
| Stalled Bridge and pre-ack cancellation | Bridge transport regressions repeated by `make verify` | `bridge.Client.exchange/write/cancelGeneration` | Real subprocess stops reading 512 KiB request; caller/write-admission deadlines and forced Close remain bounded; cancellation before start/continuation acknowledgement reaches peer; late ack cannot revive generation |
| Restart admission and internal RPC timeout | Bridge lifecycle regressions repeated by `make verify` | `bridge.Client.ensure/startProcess/Close/cleanupGeneration` | Concurrent restart waiters respect caller deadlines; single/concurrent Close interrupts initialization and reaps it; internal start/continuation timeout cancels accepted work while caller stays live; failed cancellation retires peer; no automatic generation replay |
| Failed Probe cancellation cleanup | Active-profile retest regressions repeated by `make verify` | `llm.Control.failedProbe`, `llm/sqlite.Store.InvalidateProbe` | Unacknowledged internal cancellation retires real subprocess, revokes tested proof and degrades active config with events; replacement catalog leaves ingress closed; actual caller cancellation preserves prior state |
| Legacy metadata execution limits | `model list`, `model test/select`; v3 migration fixture | `ProfileInput.Validate/ValidateActivation` | Above-limit v3 metadata remains unchanged/readable; one such row does not block listing; Probe/selection/readiness still enforce current Host limits |
| Local Pi setup | Configure `llm`; `model discover URL`, `model add URL MODEL_ID [AUTH_ID]`, `model catalog ID/test ID/select ID ...` | `provider-bridge/bridge.mjs`, `bridge.OpenLocal`, `llm.Control` | `evidence/local-provider.json`: all five probe behaviors, manual ID/catalog independence, activation and restart, keyed/keyless headers, failed retest closes ingress |
| Local HTTP security/errors | Node contracts in `make test`/`make verify` | Pi transport wrapper | `evidence/pi-bridge.tap`: same-origin and cross-origin redirects rejected; a separate HTTP receiver gets zero requests from keyed catalog/generation; no fabricated key, bounded retries, sanitized 400/401/404/413/429/5xx, malformed/partial SSE, cancel/dispose. [Bounded model audit](../docs/v1-model-audit.md) maps source clauses to named checks and remaining live obligations. |
| Malformed Tool arguments | Actual Node/HTTP contracts in `make test`/`make verify` | `generate` Tool Call normalization | Incomplete JSON and array/null/scalar arguments reject without invented calls, raw diagnostics, completion or retry; explicit `{}` remains valid |
| Session disposal | `LLMProvider.Dispose`; real Node and deterministic lifecycle peers | `bridge.Client.Dispose`, generation admission fence | Generate/Continue settle after ack; queued completion and late admission fenced; already published results and unrelated sessions preserved; dispatch precedes disposal; failed cleanup retires peer |
| Streaming generation deadline | Actual trickling HTTP response in Node contracts | Bridge-owned 30-second timer | Body consumption terminates, HTTP closes, one sanitized PROVIDER_UNAVAILABLE event, no completion or retry; catalog remains available |
| Frozen endpoint and usage | Model generation and v4 migration fixtures | `llm/sqlite.Resolver/RecordUsage`, schema v5 | Prior endpoint survives profile edit/restart; atomic failed migration; immutable usage/snapshot attribution and no invented token counts |
| Actual Pi process crash | `make test`, repeated in `make verify` | `bridge.TestActualPiBridgeClippingCrashAndRestart` | Kill Node after delta, no partial result, same Bridge restart/catalog, clipping and token readback |
| Native credentials | `ERII_TEST_KEYCHAIN=1 go test ./internal/llm/secret -run TestKeychainOwnedRoundTrip -v -timeout 30s` | Security.framework backend | Owned synthetic credential add/read/update/delete; optional macOS check, CI uses Fakes/env references |
| Local observability | Configured `logs/` | `observability.Telemetry.Start` | JSONL logs and correlated startup/database spans |
| Shutdown/restart | SIGTERM/SIGKILL then `status` | `cmd/erii:main` | Stable identity, timestamps, SQLite dump |
| Transactional Outbox | `Store.Transaction` + `event/sqlite.Enqueue` | `event/sqlite.Enqueue` | Domain/event rollback together; unique ID rejects duplicate/drift |
| Event relay | `bin/erii [-config path] relay` | `event.Relay`, `event.Publisher.Flush` | Actual CLI offline launch, broker recovery, same event ID, SQLite publication marker, clean shutdown |
| CloudEvents transport | `EventBus.Publish/Subscribe` | `event/nats.Open`, `event/fake.New` | Shared contract; structured wire JSON/extensions; arbitrary JSON rejected; durable redelivery |
| Transactional Inbox | `Store.Events().Handle` | `event/sqlite.Store.Handle` | Domain+Inbox commit before ack; rollback, ack loss, consumer scope, concurrent duplicates |
| Late/out-of-order snapshots | Domain handler on received CloudEvent | `event/sqlite:TestDuplicateLateAndOutOfOrderSnapshots` | New version wins; old epoch/version ignored; duplicates skip effects |
| Process crash recovery | `make verify` fault suite | `event/integration:TestProcessCrashBoundaries` | Seven SIGKILL checkpoints, full SQLite close/reopen, state readback, one handled effect |
| Broker restart | Integration contract suite | `event/integration:TestNATSBrokerRestartPreservesAcceptedUnackedEvent` | Same file store and durable cursor retain accepted/unacked event |
| Broker configuration rejection | Integration contract suite | `event/integration:TestNATSRejectsIncompatibleStreamAndConsumer` | Reject incompatible stream storage and consumer acknowledgment policy on independent brokers; retain acknowledged stream deletion without coupling consumer checks to immediate stream-name reuse |

| Current ChatGPT sign-in | `model login-chatgpt [ACCOUNT_ID]`, `model accounts` | `account.Manager.Login`, `provider-bridge/account.mjs` | `evidence/account.json`: actual CLI Browser OAuth, PKCE, random loopback, identity validation, separate account/client registration, masked labels and stable private device ID |
| Account model discovery | `model account-catalog ACCOUNT_ID`, `model account-add ACCOUNT_ID MODEL_ID` | `accountCatalog`, existing Pi Responses generation/Probe | Dynamic account catalog/visibility, rejected hidden IDs, actual Tool continuation/cancel/clipping, current trusted endpoint only |
| Credential lifecycle | `model refresh ACCOUNT_ID`, `model logout ACCOUNT_ID`, startup recovery | `account.Manager`, `llm/sqlite.AccountState`, schema v6 | Six-process single-flight, rotation failure, stale CAS, logout/proof/readiness revocation, owned Keychain or explicit memory-socket fixture, SIGKILL/restart, no rotating replay |
| Account final acceptance | `go test ./internal/agent/sqlite -run TestAccountFinalAcceptance -v`; repeated by `make verify` | `account.Manager.Observer`, `llm/sqlite.RecordUsage`, `bridge.Client.generate` | Actual Go/Node/Pi result paused after Guard; second Host/DB logout, reauthorization or auth loss rejects text/completion/usage success at transaction boundary; refresh retains acceptance; storage failure publishes no completion |
| Cancellation during account acceptance | `go test -race ./internal/agent/sqlite -run TestAccountCancellationAtFinalAcceptance -v`; repeated by `make verify` | `bridge.eventStream.finalize/cancel`, `bridge.Client.generate`, `llm/sqlite.RecordUsage` | Generate/Continue × Cancel/Dispose/context × observer/open-transaction/post-commit barriers; pre-commit cancellation stores cancellation, clears result and suppresses completion; post-commit ack preserves accepted result; existing auth fence and local cancellation contracts retained; held-commit deadline regression drives independent catalog RPC |
| Account Tool namespace replay | Node account continuation contract; CLI account Probe | `provider-bridge.context`, Pi Responses | Captured continuation wire retains fixed `erii` namespace, arguments and matching call/output IDs; local peer rejects omission |
| Deferred account device login | Not offered | ADR 001 / baseline §5.4 | Explicitly deferred by user; never counted as passing |

Reports: `evidence/verification.json`, `evidence/models.jsonl`, `evidence/events.json`,
`evidence/relay.json`. `make test-race` separately checks shared-memory access;
SQL uniqueness/transaction and protocol tests prove their respective invariants.
This includes the local-provider M1 slice, with no general Durable Replay or exactly-once claim.

## Main slice (ERII-9)

`scripts/verify_main.py --binary bin/erii --evidence evidence/main.json`
owns Launch/Doctor/Drive/Evidence/Cleanup. It runs from `make verify`, creates
disposable state, starts only explicit Fakes, and reaps all owned children.
See [Main operations](../docs/guides/main-loop.md) for recovery and limits.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Durable ingress | `chat send [REQUEST_KEY]`, stdin text | `agent.Actor.Submit`, `agent/sqlite.Runs.Submit` | 16 concurrent CLI processes, one Main, stable receipt, drift rejection, setup gate |
| Execute/recover Main | `chat work` | `agent.Actor.Drain`, `agent.RunAgentLoop/ContinueAgentLoop` | Nine SIGKILL boundaries; expired claim automatically reclaimed; terminal Runs not replayed |
| Stable steering | Submit while `chat work` is active | Loop model/tool/reply boundaries | Separate ordered user messages; stale final answer overtaken; no second Main |
| Context and model freeze | Create Run, switch profile, restart worker | `MainContext`, `Runs.Initialize/Generation` | Frozen role fragments, history boundary, request/schema hashes; new profile only on a new Run |
| Conversation | `chat history` | `conversation/sqlite.Append/List`, `Runs.Flush` | Independent DB, monotonic sequence, idempotent immutable IDs, no Tool trace; cross-DB missing writes repaired |
| Direct Tool | Explicit Fake model requests `mcp__fixture__echo` (absent in production) | `tool.Registry`, `tool.Executor`, bound invocation Store | Real pure execution, schema rejection, absent Tool error, per-exchange uniqueness, immediate completion; slow/fast order regression |
| Claims/CAS/wait/failure | Host execution boundary | `Runs.Claim/Checkpoint/Wait/Complete/Fail` | Old-token/epoch rejection, version conflict, invalid transition, terminal irreversibility, auth/usage/result atomic rollback |
| Result/trace events | Host EventSink | Loop + Executor, `Runs.Trace` | Persist completion before message/tool end; ordered trace envelope; no streaming EventQueue writes |
| Graceful shutdown | SIGTERM during a generation | Actor controlled release | Accepted model result retained, unclaimed nonterminal Run, no business cancellation, recovery reuses result |
| Local/account Main path | Configured local endpoint or account profile | Real CLI -> Go Actor/Loop -> Node/Pi -> Tool -> Conversation | `local-provider.json` and `account.json`, actual System Prompt and Tool continuation; no online account required |
| Main L0 fragments | `make verify`, `evals/fixtures/m1-v2.json` | `case_runner.py`, `run_cases.py`, actual TUI | 18 GC-01/09/10 wording/noise/order fragments; normative facts; fixed scenario clock; locked fixtures/scoring; full Cases not_applicable and semantic scores null |
| M1 storage migration | Startup from empty or v6 | `agent/sqlite` v7, `conversation/sqlite` v1 | Prior identity/config/events retained; partial DDL rolls back; future schemas rejected |

DeliveryRequests use the M3 Delivery Outbox below. No scored product-Case or M5
Durable Replay claim.

## Delivery slice (ERII-15)

`scripts/verify_delivery.py` extends the maintained MainHarness/Terminal mechanics
for isolated Launch, fixture/schema/readiness Doctor, real CLI/PTY Drive, direct
SQLite snapshots and transcripts as Evidence, and child/PTY/data Cleanup.
`make verify` runs it five consecutive times; `evidence/delivery-N.json` records
20 SIGKILL/restart cases per run and deterministic transport/approval failures.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Semantic delivery and retained history | TUI, `/history`, `delivery work/list`, `chat work` | `Runs.DeliverReply`, `delivery.Service`, `delivery/terminal.Adapter` | Main and Root final replies accepted once into the terminal projection; PTY transcript and destination/content readback; duplicate drains/callbacks preserve receipts |
| Recoverable cross-database commit | Main/Root reply, kill/restart | `Runs.PrepareReply/Flush/DeliverReply` | Deterministic reply/delivery IDs; Conversation precedes enqueue and checkpoint; ten before/after boundaries per role; no duplicate logical generation or semantic message |
| Delivery retry and reconciliation | Live TUI polling or `delivery work`, explicit `-test-delivery` | `delivery.Service.Drain`, `delivery/sqlite.Store` | Known transient backoff and exhaustion; accepted-send query; unknown stays query-only; terminal sends never rewrite Conversation; late-ack fencing and concurrent-consumer Go tests |
| Security approval notification | `/approve ID HASH`, `/deny ID HASH`; CLI `approval` | `toolStore.Prepare`, `Runs.Decide`, `tui.terminal.command` | One Security notice without Main inference; frozen hash callback wakes original Root; duplicate decision produces no notice; existing rejection/expiry/cancel/parameter-drift gates remain required |
| Notification policy | Root completion/failure; read-only `/status` | `replyIntent`, `notification`, `delivery.Request.Validate` | Only completion/failure/needs_input/approval accepted; progress cannot enqueue; Worker results still return through Main |
| Delivery persistence and configuration | Startup, delivery YAML policy | State schema v12, `config.Load` | Every prior schema migrates; populated v11 receipts/keys/status/attempts survive; failed migration rolls back; legal/illegal states, atomic event enqueue, bounded config |

Physical stdout may redisplay retained history on reconnect. No generic external
exactly-once guarantee, Web/Feishu/UISpec implementation, or full Golden Case
score is claimed. Operations and state details: `docs/guides/delivery.md`.

## Terminal slice (ERII-10)

`scripts/verify_tui.py` owns launch/doctor/drive/evidence/cleanup through a real
PTY and the existing local/account HTTP peers. `make verify` runs it five times.
See [terminal operations and proof limits](../docs/guides/tui-and-evals.md).

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Fresh setup | Default launch on terminal or `tui` | `tui.Session`, existing `llm.Control`, `App.WaitForRuntime` | Actual PTY discovery/manual URL or browser account setup → numbered/ID/one-model Enter picker → invalid numeric field repair → fixed Probe stage progress → explicit selection → owned NATS/runtime ready → immediate Main message. Cold 11-second real Pi fixture retains exact 262144/32768 limits; hidden account IDs retry; failed tests print saved profile retry/select commands. Shared `event_broker.py` owns handshake and joined cleanup. |
| Terminal conversation | Plain text, `/history` | `agent.Actor.Submit/Drain`, semantic Conversation | Actual Pi Main/Tool/reply for both paths; busy Steering enters one Main; no raw Tool trace |
| Profile controls | `/models`, `/test ID`, `/select ID`, `/setup`, `/login [ID]`, `/logout ID` | Existing Model control plane and account Manager | Dynamic account IDs; versioned activation; logout/degraded gate; new Run model switch with old Snapshot retained |
| Cancel and readiness | `/cancel`, Ctrl-C, SIGTERM, unavailable Bridge | `tui.Session`, `agent.EffectiveReadiness` | Fresh setup stays closed, active profile retained; login cancels without credentials; prompt input and executor joined; five consecutive runs |
| Terminal recovery | Restart TUI after actual SIGKILL | Existing Main recovery and Conversation intents | Identical semantic state for both providers; held generation with Steering reused; no duplicate reply |
| Terminal input byte boundary | Default launch; `verify_terminal_input.py` in `make verify` | `tui.OpenInput`, bounded `readInput`, same Actor; scoped `terminal_driver.Terminal` | 12 actual PTY cases: exact 16 KiB ASCII/UTF-8, overflow rejected without partial Main rows, recovery, editing/EOF, echo on/off, signal shutdown, flags/termios restored; macOS review reproducer retained as before/after evidence. Scoped PTY waits observe cancellation even with a retained slave observer; real parallel blocked-terminal regression proves prompt joins, no live listeners or leftover state, and no queued admission. Original I/O deadlines remain. |
| Eval result/scoring protocol | `run_cases.py`; `make test` scorer regressions | `case_runner.py`, locked `scoring-v1.json` | Real TUI drives 18 fragments; full evidence and snapshots; missing grades null, hard/mechanical failure cannot be overridden, thresholds enforced |

## Task delegation slice (ERII-11)

`scripts/verify_tasks.py` extends the existing CLI/PTY launch, doctor, drive,
evidence and cleanup harness. `scripts/run_task_cases.py` uses the shared Case
result/scoring and Terminal mechanics. Both run in `make verify` without accounts.
See [Task operations and proof limits](../docs/guides/tasks.md).

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Task discovery and mutation | Natural-language Main delegation; canonical Task tools | `task` schemas/validators; accepted `ToolInvocation` -> `tools.ExecuteBuiltin` in `internal/agent/sqlite/builtins.go` -> Task transaction in `tasks.go` | Strict union/null/unknown-field checks, defaults/50 limit, scope and hybrid ranking, ambiguous real CLI search, duplicate source/call receipts; shared routing retains Automation, Code Mode, Computer, Child, Memory and transactional Task/Artifact paths |
| Atomic delegation | Main `task_manage(create)` | `Runs.createRoot` and state transaction | Task/anchor/Root/snapshots/Outbox and receipt readback; before/after SIGKILL |
| Explicit continuing Task (ERII-226) | Main `task_manage(create)` with `completionPolicy: "ongoing"`; omitted/once retains one-shot behavior | `task.ManageSchema/ParseManage`, `tools.executeBuiltin`, immutable v9 `completion_policy`, `finishTask` | `verify_tasks.py`: real CLI create → Root success stays active → idle restart → append/pause/cancel; four additional ongoing SIGKILL boundaries; contract/race tests verify once dedup, ongoing identity, descendant completion, old epoch rejection. Owner-approved complete ends only idle active work: strict union, current-epoch live/unresolved rejection, atomic timestamp/receipt, duplicate/owner/append race tests and two completion SIGKILL boundaries; no synthetic Worker update or semantic GC-08 score claimed |
| Append and continuation | Main searches then `task_manage(append)` | `TaskInstruction`, Root Steering and `createRoot` | Active frozen-generation injection, 16 concurrent instructions, immutable anchor, new Root/epoch after completion, no terminal revival |
| Root concurrency / responsive Main | Same TUI or `chat work` | Actor scheduler, shared Loop, database lease admission | Eight Tasks/four Worker leases, reserved Main slot, live PTY receives acceptance and new answer while Root is held |
| Actual provider Task path | Local endpoint through Node/Pi | Same Task/Root/Reporter path, production Bridge conversion | `local-provider.json`: Task union reaches HTTP, Root has no Task controls/Conversation, runtime-framed follow-up returns through Main |
| Root result / Reporter | Root completes or fails | `finishTask`, TaskUpdate/Outbox, `Report`, Main Inbox | One update and consumer receipt per Root, failure leaves active Task; four result/report SIGKILL boundaries; repeated drain unchanged |
| Batched follow-up / user priority | Concurrent completions while Main runs | `promoteFollowups`, `Steering` | Go contracts: terminal-commit/idle-scan races, one batch, no current-Main injection; overtaking input defers queued update before its first generation |
| Runtime provenance | Main receives a Task update | Typed `AgentMessage`, provider conversion | `origin:runtime`, untrusted JSON boundary, named safe details; unknown type/provenance fail closed; no Worker Conversation/System entry |
| Read-only activity | `/status`, `task_search` | `Runs.Progress/Activity`, `searchTasks` | Progress read/write contract creates no update, model wake or delivery; PTY status shows completed work |
| M2 due fragments | `run_task_cases.py` | Locked `m2-v2` fixture and shared scoring protocol | 24 GC-02/04/08/18 fragments; SC-01/04/05/10/12 applicable invariants; pause/cancel followed by new Root; semantic scores remain null; Child/Artifact behavior not_applicable |
| M2 storage | Startup from empty or v8 | State schema v9 | Every prior version migrates; v8 rollback/retention; old frozen capabilities still selected; Task/Run epoch fencing and audit immutability |

## Task controls (ERII-12)

`scripts/verify_task_control.py` reuses the CLI/PTY harness for Launch, Doctor,
Drive, Evidence and Cleanup. `make verify` repeats it five times. Reports retain
actual PTY output and persisted state; all processes and disposable databases
are cleaned up. See [Task operations](../docs/guides/tasks.md) for transitions and limits.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Controlled pause/cancel | Main searches then `task_manage(pause/cancel)` | `stopTask`, Actor execution context, `SettleStop/recoverStopsTx` | `task-control-{1..5}.json`: accepted provider generation cancelled; status/epoch/desired state persisted; exact Root reason; no stale result/followup; three SIGKILL boundaries; live complete rejection before pause/cancel. `TestTaskControlPauseCancelsSafeToolBeforeNextStep` submits pause to the live Actor instead of racing its Main claim, then checks safe Tool cancellation, no next generation/update, one Main claim, paused epoch, cancelled Root and released leases; cancellation cleanup joins the Actor even after failure. Included in SC-04's five-run gate. |
| Continue stopped work | Main `task_manage(append)` | `executeTaskAdmission` waits outside `executeBuiltin` transaction in `tasks.go`; `createRoot`, immutable instruction/receipt | New Root and epoch after paused/cancelled/completed; one nonterminal Root; preserved anchor, committed Tool/model results and progress; duplicate and concurrent controls; pause-boundary regression submits through the running dispatcher, rejects a competing Main claim, proves append waits outside its transaction, then joins the old Root before completing the new one |
| Epoch/owner fencing | Current Task tools and Host persistence boundary | `fence`, `Claim`, `AcceptGeneration`, `Trace` | Go contracts repeated five times: old checkpoint/progress/generation/Tool/reply/terminal/trace writes rejected; usage rollback; out-of-order response cannot change new Task |
| Tree stop and completion | Internal stored tree relationships | `recoverStopsTx`, `finishTask` | Go synthetic-tree contracts: leaf-before-parent, waiting descendants, safe cancellation/unsafe unknown, no unknown replay, last descendant releases one-shot completion, ongoing stays active; actual Child admission/recovery evidence is in the ERII-17 entry |
| Shutdown versus user stop | SIGTERM/context cancellation; later `chat work` | Shared Loop, Actor release | Current generation retained and same Root reclaimed after shutdown; Task/epoch untouched; deterministic scan/claim/after-claim cancellation barriers preserve accepted work and return normal cancellation (`TestLoopShutdown*`); user stop gives terminal Root and explicit reason |
| Bounded stop/admission invariants | `python3 scripts/model_task_control.py`; `make test` | Private [statechart and SQL mapping](../docs/guides/task-control-model.md), `model_task_control.explore` | Exhaustive bounded safety and separate fair recovery suffixes; JSON records actual bounds/counts/depth/coverage; paired Python/SQLite `TestTaskControlModelCompletionUsesRootOutcome` traces cover both Root/Child outcomes and ending orders; `--unsafe` exits 1 with shortest fencing counterexample; `TestTaskControlModelCounterexampleRejectsOldOwner` rejects that trace through persisted model/Tool admission; five-round process gates and race remain required |

| Unified Tool feature | Reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Frozen approval and decision recovery | `approval list`, `approval approve/deny ID HASH`, `chat work` | `tool.Executor`, `sqlite.tools.Prepare/Start`, `Runs.Decide` | `evidence/tools-{1..5}.json`: persisted waiting with no active claim, CLI restart, exact hash, duplicate callback, denial and no early effect |
| Steering invalidates granted/unexecuted action | `chat send` or `task_manage(append)` while approval waits | `Runs.Submit`, `tools.ExecuteBuiltin`, `invalidateApprovals` | CLI GC-12 fragment changes Monday to Tuesday; only new frozen Tuesday action executes; Go expiry/wrong principal/hash/immutability regressions |
| External effect recovery and reconciliation | `chat work`, `approval reconcile` | `Executor.executeOne`, `Runs.ReconcileUnknown` | Nine real SIGKILL/restart scenarios per harness run; separate Fake ledger, stable keys, query-before-retry and never; oversized recovery receipt persists unknown before Worker delivery, keeps Task active, reaches Main and later reconciles without dispatch; persisted Worker regressions also cover inconsistent/nonterminal query evidence; unknown absence does not authorize retry; cancel of an approval waiter never dispatches |
| Tool capacity and independent results | Parallel model Tool batch | `sqlite.tools.Start/Defer/Finish`, `Executor.Execute` | `TestToolQuotasAndDynamicConflictKeys` (all five scopes and intersecting keys), `TestToolResultsCommitIndependentlyAndDeliverInDeclarationOrder`, race detector |
| Host errors and schema/policy boundary | Direct Tool results | `tool.NewRegistry/Validate/Normalize` | Hidden Worker Tools return NOT_FOUND before validation; strict schema, forged fields/duplicate keys rejected; bounded transient retry and sanitized raw provider errors; no model credentials available to adapters |

Tool harness lifecycle: **Launch** `MainHarness` with explicit Fake provider/tool;
**Doctor** model add/test/select and ingress readback; **Drive** ordinary chat and
approval CLI commands; **Evidence** domain state, approval records, Outbox and
separate external ledger plus kill checkpoints; **Cleanup** join every child and
remove isolated roots. `make verify` runs five consecutive repetitions. Full
GC-11/12/13 product scenarios, Code Mode and Artifact spillover are not claimed by
these Tool fragments. Operational details: `docs/guides/tools.md`.

## Artifact slice (ERII-14)

`scripts/verify_artifacts.py --binary bin/erii --evidence evidence/artifacts.json`
owns Launch (isolated data root + explicit Fakes), Doctor (selected profile,
schema/readiness), Drive (CLI and normal Main/Task/Root Tool path), Evidence
(independent SQLite, bytes/hash readback, history, trace and SIGKILL residue),
and Cleanup (join children and remove disposable roots). `make verify` repeats
SC-09/recovery five times. See [Artifact operations](../docs/guides/artifacts.md).

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Binary save/open and audit | `artifact save NAME MIME` stdin; `artifact open/describe REF` | `artifact/sqlite.Put/Open/Describe` | Exact bytes/SHA-256 across processes, opaque path-free descriptor, malformed/forged/traversal rejection |
| Immutable lifecycle | `artifact delete REF VERSION`; `artifact cleanup [apply]` | `artifact/sqlite.Delete/Cleanup/Recover` | Tombstone/CAS, expired/tampered/symlink rejection, quota/ref guards, dry-run, active/paused Task reads, retention after Task end |
| Reference inputs | `chat send-parts KEY` stdin parts; Task create/append `artifactRefs` | `Runs.Submit`, `tools.executeBuiltin` | Same transaction validates/pins refs, bounded text read, original hash unchanged, forged input creates no ingress |
| Large/binary Tool results | Normal Executor result; `chat work`, `approval reconcile` recovery | `tools.FinishResult/ReconcileResult`, `Runs.ReconcileUnknown/storeOutput`, `tool.RecoverResult` | Result ref + metadata atomic commit, no payload in history; exact receipt fixture uses a controlled virtual clock while retaining the frozen 100ms policy; deterministic expiry preserves unknown before a fresh query, without a second effect; both binary Query recovery paths through `verify_artifacts.py --recovery-only` (also in the full gate); terminal-unknown uses explicit producing-client receipt invisibility until a fresh client after restart, so automatic reconciliation cannot race its crash precondition; exact receipt bytes/hash and caller/committed/replayed equality, real post-effect SIGKILL with no second effect; race regressions for scope, frozen deadline/cancel/stream closure, invalid evidence/refs/metadata, read and commit failure retaining unknown, and terminal Run immutability |
| Final output selection | `builtin__artifact__manage(operation: publish)` | `PublishTx`, `PrepareReply`, `taskUpdate/promoteFollowups` | Draft omitted; explicit Root refs in update; Main preserves selected media/file part types and bounds final refs to the Conversation envelope; GC-15 transport fragment reopens report |
| Terminal Artifact delivery | TUI conversation and `/history` | `tui.terminal.history` | Owned PTY sees final attachment ref; CLI open verifies delivered bytes; clean terminal shutdown |
| Task visibility and parallel outputs | `builtin__artifact__search/manage(operation: read)` in scoped Worker; Host child-shaped contract fixtures | `DescribeTx`, Run-derived scope | Same Task read, cross-user/Agent/Task/Run denial, eight child-shaped writers cannot overwrite same display name |
| Object publication/recovery | Real save Tool during `chat work` | `PutTx`, startup `Recover` | Six SIGKILL boundaries, orphan cleanup, one final object, committed ref reused with no second attempt |
| Schema v11 migration | Startup on empty/v10 DB | `agent/sqlite.migrate` | Prior identity/config retained, partial DDL rollback, prior v1–v9 regressions retained |

## MCP, Skills and coding fixture (ERII-16)

`scripts/verify_capabilities.py --binary bin/erii --evidence evidence/capabilities.json`
owns Launch (private data/provider/Skills/checkout roots), Doctor (explicit Fake
model add/test/select, validated scopes), Drive (ordinary chat/Task/approval CLI),
Evidence (SQLite, real test output, full tree hashes and SIGKILL/restart), and
Cleanup (join every process and remove disposable roots). `make verify` repeats
five times. See [capability operations](../docs/guides/capabilities.md).

For focused reproduction, add `--scenario dirty-index` (approved Worker
dirty-file refusal) or `--scenario mcp-reply` (malformed post-effect stdio
reply). Both actions use the same Launch/Doctor/Drive/Evidence/Cleanup lifecycle.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Structured MCP stdio | Configure allowlisted `mcp`, normal Direct Tool call; capability harness `mcp-reply` scenario | `tool/mcp.Definitions`, common Executor | Real initialize/list/call; malformed response-shape contracts; null/missing-text replies after approved Worker effects persist unknown with one attempt and one ledger entry, including restart/reconcile; schema drift, malformed metadata/params, cancellation, provider host-read/write/network/env denial; frozen approval rejection |
| Configured scope isolation | Optional MCP/Skills/workspace configuration at CLI startup | `config.Load/overlaps` | Root, shared-provider and domain-data overlap rejection; symlink alias resolved even before a new data directory exists |
| GC-11 two-turn quote journey mechanics | `verify_capabilities.py --binary bin/erii --scenario quotes --scenario-jobs 2 --evidence evidence/gc11.json`; five rounds in existing capability gate | Locked `gc11-v1`, `quote_case.py`, stdio MCP, Main read/save/echo, ArtifactStore | Launch isolated configuration/stores per variant; Doctor explicit Fake; Drive three wording/two noise/one ordering variants through both original turns; Evidence exact three quotes/services/45:35:20 weights, transported source equality, two visible scripted replies/deliveries, reopened Artifact bytes/hash, no Task/approval/purchase and duplicate-input state equality; Cleanup joins children and deletes roots. Regression tests reject missing/drifted evidence; `TestCapabilityPlanProgressSurvivesCompactionAndIdenticalCalls` uses the real compactor to prove provider-call-ID progress after old exchanges disappear, including identical calls; invalid ordinals fail closed. Scripted recommendations remain semantically ungraded. |
| Constrained Judge/comparison preparation | `prepare_release_eval.py --input evidence/gc11.json --evidence evidence/gc11-eval-preparation.json` | Locked source and allowlisted packet builder | Launch offline report load; Doctor exact variants/source/assertions/cleanup; Drive six preview packets plus identical-condition/five-repeat two-arm plan; Evidence excludes raw plans/traces/hidden Gold and marks all previews ineligible for grading, profiles/snapshots/scores unknown; Cleanup owns output only. Missing/failed/graded Fake evidence rejects. Current VM/channel commands and consolidated environment requirements: `docs/v1-live-preflight.md`; no live execution is inferred. |
| GC-11 unscripted model trial | `gc11_driver.py --binary bin/erii --trial OWNED/erii-eval.json --evidence evidence/trial.json`; offline `verify_local_provider.py --gc11-driver-only` | Production CLI/Context Builder/Pi Bridge; `gc11_driver.Driver`, locked fixture | Launch only an owner-prepared fresh installation; Doctor exact approved profile/version/limits, immutable bounded config/read-only MCP and source; Drive two original events without plans/answers; Evidence visible replies, state/snapshots/usage, actual Artifact CLI bytes/hash, duplicate input and source-before/after witness; Cleanup joins all processes and retains approved state for operator inspection. Offline HTTP contract removes its own root and never counts as model quality. No required tool/Artifact is invented when the supplied source suffices. |
| Frozen model comparison preflight | `gc11_driver.py --binary bin/erii --compare evidence/trials --evidence evidence/comparison.json` | `preflight`, `derived_conditions`, `comparison_model` | Launch sixty receipts; Doctor two arms × six variants × five independent Runs; Drive recomputes assertions/provenance/input/Context/schema/limits and normalizes only owned namespaces; Evidence rejects missing/duplicate/reused/dirty/unjoined/graded/drifted cohorts and model/endpoint/wire/reasoning/defaults/compatibility/capability changes within either arm's thirty trials. Only installation metadata is normalized for cross-trial model identity; per-trial profiles remain exact. Retains canonical identities, separate raw snapshots and null scores/winner; live receipts also require current clean source/driver/binary. Cleanup owns output only, clears stale output before failure. Durable first/last-trial, both-arm and CLI drift regressions; actual live comparison remains unexecuted. |
| Progressive Skills | `builtin__skill__list/view` in Main or Root | `skill.Definitions`, `skill.SetHash`, Run capability freeze | Summary excludes body; view returns actual text or binary Artifact; exact asset bytes reopened through CLI; reference/scope/version hashes and drift rejection; traversal/link denial; ordinary Worker management NOT_FOUND |
| Coding edit/test | `workspace.mode: edit`, normal Main/Task/Root calls; capability harness `dirty-index` scenario | `workspace.Definitions`, common Executor/Security, `computer/sandbox.Command` | Real red missing-status test, exact approved Worker edit, real green race test; only the CLI and its regression test change; ordinary/assume-unchanged/skip-worktree dirty files refuse approved Worker replacement; complete workspace/index/hooks unchanged on refusal; protected edit rejection and old preimage denial |
| Coding audit and Git | `workspace.mode: audit`, read/search/git | `workspace.read/git`, registry selection | Reads AGENTS.md; audit records actual retry file/symbol order, trigger and effect risk; full tree hashes unchanged; hidden edit and Task control NOT_FOUND; Git filters/textconv/fsmonitor never execute; transient locks excluded and bootstrap auto-maintenance disabled |
| Coding crash boundary | SIGKILL before result commit after rename; `chat work` restart | Existing ToolStore finish boundary and never-replay policy | One edit, one attempt, persisted unknown; no repeat write or claimed success |
| Interim Computer isolation | Fixed test command and dedicated MCP executable, Host-only scopes | Seatbelt on macOS, bubblewrap on Linux | Go tests attempt host-secret read/write and network access; source checkout unchanged by test mutation; missing boundary fails closed; VM/Guest production adapter remains M6 |

## Direct Child Run slice (ERII-17)

`scripts/verify_children.py --binary bin/erii --evidence evidence/children.json`
owns Launch/Doctor/Drive/Evidence/Cleanup through the existing CLI and PTY harness.
`make verify` runs five consecutive passes. See [Child operations](../docs/guides/children.md).

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Direct delegation | TUI/`chat send` -> Main Task -> Root `builtin__agent__run`; Child may delegate again | `agent.AgentToolDefinition`, `Runs.StartChildRun`, `tools.AgentToolExecutor`, shared `RunAgentLoop` | One Task/Root, Child/Grandchild, unique invocation links, exact `{text,artifactRefs}` outputs and one Root follow-up |
| Atomic admission and limits | First accepted Worker Tool call | `tools.Accept`, `run_tree`, migration 013 | Transaction fault rollback, duplicate and concurrent admission, drift rejection, depth/descendant/call ceilings; before/after admission SIGKILL |
| Capability/context isolation | New Root and recursive Child | `WorkerTools`, `ChildContext`, `child_initial`, `taskSteering` | Main forged call, forbidden controls, real private Artifact rejection, inherited frozen profile/schema/scope/Skills, no identity/Conversation/Task steering in Child |
| Artifact handoff | `agent.run` input refs and Child publish | `StartChildRun`, `AgentToolExecutor`, scoped ArtifactStore | Explicit input pin, Child read/publish, Parent result refs, Root publication and final Main delivery; large Child text becomes a readable Artifact without entering Parent model context |
| Recoverable waiting/results | `chat work`, repeat after restart | `tools.Prepare`, `Runs.Next`, terminal Outbox | Waiting ancestors release leases, completion-before-wait scan, six kill/restart boundaries, zero replay of completed generations, stable repeated drain |
| Child approval and cancellation | TUI cancel; `approval approve/deny ID HASH` + `chat work` | Existing approval/Task stop pipeline and epoch fences | CLI exit/restart approval, separate Parent/Child approval IDs, real Child cancellation, no late old-epoch commits, `unknown` never retried or packaged as success |

The M2 Case Runner permits this local Host capability only on an authorized Worker;
Main delegation, forged names/permissions and external effects retain hard failures.
Its actual no-Child, Task-count and action assertions stay strict.

This is Direct Tool protocol evidence. Fair scheduling and batch capacity are
covered below. JavaScript CodeCall replay and complete GC-16/17/18/20 remain
later slice gates; they are not counted as passing here.

## Fair Child scheduling and batch aggregation (ERII-18)

`scripts/verify_scheduling.py --binary bin/erii --evidence evidence/scheduling.json`
owns Launch (isolated Fake/data root), Doctor (profile probe/activation), Drive
(PTY, CLI and two simultaneous Tasks), Evidence (state, event lease timeline,
usage, outputs and transcript), and Cleanup (all processes reaped, state removed).
`make verify` runs five consecutive passes. The monetary harness below covers
account-spend authorization; this entry does not establish full M4 completion.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| 100-item Direct Tool batch | TUI -> Main creates Task -> Root emits four batches | `Runs.StartChildRun`, `RunAgentLoop`, `batchReply` Fake | Exactly 1 Task/Root/100 Child; 98 successes + 2 failures; unique row calls, exact aggregate, one blocker/follow-up, five Root model calls; GC-16/17/20 transport fragments |
| Transparent fair capacity | `chat work` with concurrent independent Tasks | `Runs.nextScheduled`, `claimCapacity`, `scheduler_tree` | 16 live grandchildren with queued siblings while second Task completes; atomic global/user/Agent/provider/tree bounds across database connections; reopened scheduler preserves rotation; distinct urgent Main is admitted after initial Main acknowledgement while Workers stay blocked (20 race runs, original three-second response bound); GC-18 fragment |
| Recursive stop and fencing | Main cancel while saturated; then repeat `chat work`; `verify_scheduling.py` also freezes an allocated follow-up generation before real cancellation ingress | `stopTask`, `recoverStopsTx`, Task/Run claims; deterministic Main Fake input ordering | Queued/running/waiting descendants and invocations settle; Drain waits for stopping owners to join or expire; no cancelled-tree follow-up; old Task epoch cannot revive; prior Child harness retains late-result and approval rejection evidence. Original fair/cancel case plus forced overlap retain thirty-second phase bounds and five runs; completion notices cannot hide appended pause/cancel/continue input. Failure preserves structural state before cleanup and still exits nonzero without retry. |
| Work-first aggregation and progress | TUI `/status`; Root result -> Main | `runnable`, `Activity`, `finishTask`, Worker prompt | Child joins coalesce until current batch settles; independent ready Tools execute beside pending effect approval without granting it; aggregate five-state counts; reads cause no Outbox/Main turn; successes survive item failures; Fake deduplicates repeated blocker, semantic scores remain null |
| Tree safety and provider health | `scheduler` + `children` YAML settings | `createTree`, `Generation`, `AcceptGeneration` | Frozen default 256/hard 512 Worker turns, existing depth/descendant/call ceilings, canonical RESOURCE_LIMIT; only infrastructure streak opens recoverable provider cooldown; injected health-write failure rolls back usage/generation and stays a recoverable storage error; per-operation watchdogs remain active |
| Accounting and migration | State readback; upgrade from schema 013 | Migration 014, immutable `provider_usage` + aggregate views | Empty/every-prior upgrade, populated frozen tree preservation, failed DDL rollback, explicit missing-token counts; monetary policy is covered below |
| Restart | SIGKILL after batch admission, then `chat work` | Existing model/ToolInvocation recovery and Run claims | First 25 Child identities survive; total stays 100; completed model responses reused; exact final aggregate; no JavaScript/CodeCall replay claim |


## Account monetary authorization (ERII-18, SC-13)

`scripts/verify_spend.py --binary bin/erii --evidence evidence/spend.json`
owns Launch (isolated children Fake), Doctor (profile probe/activation), Drive
(CLI approval/deny/status, real PTY monetary notice and `/approve`, 100-item batch,
Main cancellation, SIGKILL),
Evidence (frozen actions, usage, reservations, Root/Child state, effect ledger)
and Cleanup (joined processes and removed roots). `make verify` runs five passes.
Synthetic fixture `evals/fixtures/m4-spend-v1.json` specifies prices and arithmetic.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Account monetary admission | `spend.accounts` YAML, `chat work` | `freezeSpend`, `gateSpend`, `settleSpend` in the shared model admission/acceptance transactions | Integer conversion, full model-bound projection, accumulated actuals plus reservations across Roots, cross-process atomic admission, immutable price version/currency/snapshot, no double reservation, usage/result rollback on injected storage error |
| One Root spend consent | `approval list`, `approval approve/deny ID HASH`; same TUI commands | `spend_approval`, `Runs.Decide`, existing security notification boundary | 100 Child batch, one Root approval, CLI process exit/restart, no Child prompts, exact frozen hash, denial before model dispatch atomically finalizes its model ledger; actual final/failed response crossing holds and resumes saved outcome without regeneration; denied Parents settle accepted Child invocations; independent Main grant |
| Separate Child effect consent | Approve monetary batch then Child effect | Existing ToolInvocation policy/approval/execution pipeline | No effect after monetary grant; separate frozen effect ID/hash required; SIGKILL after effect and query-before-retry recovery executes exactly once |
| Cancel and late events | Main cancels pending monetary batch | `stopTask`, `invalidateApprovals`, existing epoch/owner fences | Old approval invalidated, callback and late usage rejected, no effect/follow-up, settled repeated drain; Go old-owner/Task-epoch regression |
| Sanitized unknown costs | `spend status` | Read-only `Runs.Spend`, immutable provider usage | Explicit unpriced/unconfigured/combined state, no guessed approval, missing usage retains reserve and unknown actual, historical and setup-Probe unpriced counts; no endpoint/credential or model-visible query |
| Migration | Open empty/prior schema and restart | Migration 015 | Every-prior upgrade, retained model/profile/usage data, legacy snapshots remain unpriced/unconfigured, failed migration rolls back atomically |

## M5 Code Mode runtime and recovery — ERII-19 / ERII-20

| Feature | User boundary | Authoritative implementation | Inspectable evidence |
| --- | --- | --- | --- |
| Fresh JavaScript cells, loops and Promises | Configured `builtin__codemode__execute` through `chat send` / `chat work` | `internal/tool/codemode`, `runtime/javascript/src` | `verify_codemode.py`: selected output plus nested ToolInvocation readback; real Node fixtures |
| Nested Tool execution with one slot | `tools.run_concurrency: 1`, awaited echo and `Promise.all` through `chat work` | Atomic capacity ownership in `internal/agent/sqlite/tools.go` | `verify_codemode.py`: bounded single-slot completion, wrapper/nested attempt counts and ordered parallel results; deterministic Go tests retain all five actual Tool quotas and conflict exclusion while wrappers run |
| Progressive discovery and role exposure | `ALL_TOOLS`, `searchTools`, `describeTool`, `describeNamespace`, Worker-only `agent.run` | Frozen capability view in `internal/agent/sqlite/codemode.go` | Main hidden Agent, Worker namespace, actual Child final result and inherited model |
| Synchronous persistent cell Store | `store` / `load` | `code_store` and fenced Go sink | Write survives throw, another cell cannot read it, Surface handles rejected; Store operation receipts preserve reads and avoid repeated writes on replay; `store-quota-rejection-shrink-suspend-replay` fills quota, catches `RESOURCE_LIMIT`, shrinks, suspends, then proves the same rejection, unchanged dependent Tool arguments, one dispatch, and no rejected write; unawaited calls drain without rerunning the finished cell |
| CUA facade | `cua.getState`, `getApp`, bound `observe` | Unified Computer Tool bridge; explicit read-only Code Mode fixture (production surfaces are in ERII-25) | Three nested invocations and chosen latest observation |
| Approval and isolation | Nested effect, prohibited globals, cancellation | Existing ToolExecutor, QuickJS Worker and RPC admission | Exact rejection without dispatch; forbidden globals, CPU/memory/output limits, forged peer and duplicate event fixtures |
| Fixed-source Code Mode recovery | configured `builtin__codemode__execute` via `chat send` / `chat work` | `tools.executeCode`, `codeBridge`, `Runs.recoverCode` | `verify_code_recovery.py`: actual Sidecar/Go SIGKILL at 37 completed + 8 active of 100; independent provider dispatch audit; 45 identities reattached; 100 final Children; zero active suspended VMs, replay/latency/RSS metrics, cleanup; five runs in `make verify`. State/contract tests cover old owner, wake-before-release, args/schema/manifest drift, durable delivery order and six fake hours of approval without a VM. |

Launch/Doctor/Drive/Evidence/Cleanup:
`python3 scripts/verify_codemode.py --binary bin/erii --evidence evidence/codemode.json`.
This command owns disposable configuration, Model Fake, Computer Fake, processes
and SQLite state. `make verify` repeats it five times. `docs/guides/codemode.md` records
configuration, limits, provenance, runtime upgrade constraints and ERII-21 scoring gaps.
`verify_code_recovery.py` owns the same five harness phases for fixed-source
SIGKILL/SIGTERM recovery and recursive cancellation.

## Automation scheduling and threshold monitoring (ERII-27)

The exact GC-14 journey is maintained in `verify_gc14.py` (see
[scope and commands](../docs/gc14-verification.md)). Its communication CI gate
uses the configured MCP sandbox; it is distinct from the older scheduling
fragments and does not establish semantic or L2 acceptance.

| Feature | Reach / authoritative entry point | Inspectable evidence |
| --- | --- | --- |
| GC-14 exact monitoring journey | `verify_gc14.py --binary bin/erii --evidence evidence/gc14.json`; production CLI → Main → Automation/Actor/RunClaim → normal MCP/observe/Reporter/Delivery | Launch six isolated roots; Doctor explicit profiles/source lock; Drive original request, one 08:00 Asia/Shanghai intent, six exact prices/two independent sources per day, one ongoing Task/six Roots; Evidence Oct18/21 notices only, 07:59:59 quiet, duplicate ticks/observations/replay, SIGKILL after admission and at final scan, new profile at later admission with old snapshots retained, Main cancel and no later Root/read/delivery, native empty Approval/Artifact inventories; Cleanup joins/removes owned state. Mutation and source-validation regressions; six Judge previews ineligible/null. |

`scripts/verify_automation.py --binary bin/erii --evidence evidence/automation.json`
owns Launch (private data root and explicit Fake), Doctor (profile probe/select),
Drive (ordinary chat Tool and Automation CLI), Evidence (fire/Outbox/Inbox,
Task/Root/snapshot/Tool/Delivery state, six SIGKILL boundaries and restart), and
Cleanup (all child processes joined and disposable state removed).
`make verify` runs five consecutive passes. See [Automation operations](../docs/guides/automation.md).

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Zoned at/cron intent | Main `builtin__automation__manage`; `automation manage KEY`/`list` | `automation.Next`, `Runs.manageAutomation`, schema 023 | GC-02 first fire Nov29 20:00 +08, explicit IANA zone, month/leap/DST boundaries, create retry and schedule edit/CAS |
| Durable trigger and admission | Foreground `automation work`; `automation tick` then `chat work` | `ScheduleAutomations` -> unified Outbox -> Actor `Next`/`admitAutomations` -> existing RunClaim | Four real SIGKILL/restarts; one trigger, Inbox, ongoing Task and Root; duplicate/missed events; new model frozen at admission; completed generations reused |
| Quiet threshold monitoring | Root price evidence -> `builtin__automation__observe` | Fenced per-fire observation and `finishTask` -> ordinary Main Reporter/Delivery | GC-14 six daily prices and two same-value sources per day; only Oct18/Oct21 notices; quiet days create no Main turn; Task remains active |
| Stop and version fences | Main manage cancel or Task pause/cancel; edit with current version | Existing `stopTask`/Task epoch, Automation schedule version through `Report`/`promoteFollowups` | Delayed fire becomes skipped; no new Run after cancel; old claim rejected; edits after `after_result_commit`/`after_report_commit` suppress stale Main and delivery, then the edited Oct22 intent delivers exactly once; no external effect authorization inherited |
| Privacy-fenced admission | `memory manage KEY` Forget, next due fire, fresh chat and Automation | Immutable acceptance privacy epoch (migration 024), safe `manageAutomation` list projection, `admitAutomations` intent/Task eligibility -> skipped fire + Inbox; `createRoot` fence | `list-after-forget`: two real Main model requests exclude forgotten context, revoked rows retain cancellation controls only, fresh views stay complete, real Main cancel succeeds; linked Automation Task search stays private; `forget-next-fire-fresh-chat` plus `first-fire-forget-pending` / `first-fire-forget-not-persisted`: real health Memory tombstoned, zero revoked Task/Root/model requests, fresh Main and new Automation complete; retries/edits cannot renew intent; repeated drains stable; admission and skipped-receipt storage faults roll back and recover |
| Upgrade | Startup on empty/prior DB | Migrations 023–024 | Every-prior migration, populated v22/v23 preservation, legacy intent fails closed after Forget, immutable epoch, partial DDL rollback; historical accepted-work migration fixtures remain strict |

## Unified Web / Feishu interaction (ERII-28)

`python3 scripts/verify_channels.py --binary bin/erii --evidence evidence/channels.json`
owns isolated Launch, auth/provider Doctor, actual HTTP/browser/platform Drive,
API/SQLite/independent ledgers/screenshots Evidence and joined-process Cleanup.
`make verify` and the Delivery CI shard run five consecutive passes. See
[Channel operations](../docs/guides/channels.md) for enrollment, credentials, revocation,
recovery, supported transports and release evidence limits.

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Owner-bound channel access | `channel manage` bind/list; Web token login; signed `/feishu/callback` | `interaction/web`, `interaction/feishu.Client.Verify`, schema 025 binding/CAS | Browser setup, CSRF/DNS/auth rejection, signed/encrypted event, token/app/tenant/recipient tests; revoked binding, changed outbound app/tenant, frozen-destination drift and preference CAS |
| One shared Conversation | Web `/`, `POST /api/message`, TUI chat and Feishu v2 p2p message | `Runs.AcceptInbound` -> existing `submit`/Actor; callback/source aliases and message route in one transaction | Concurrent callback IDs, same source/new event retry, content drift, one Main/input/reply; GC-08 continuation and GC-09 agenda transport fragments; SIGKILL before/after input commit |
| Safe channel UI | Web conversation/settings, Feishu card, TUI fallback; `verify_channels.py --ui-components-only` | `delivery/ui` Composer and persisted `delivery_projection`; embedded ad-agent components, OpenIntelligentUI native DataTable and original Eri logo/avatar | Real desktop/mobile Chromium screenshots/DOM in `channels-ui/`; actual React composer and frozen approval forms; delayed-bundle handoff retains early text/file bytes, focus, selection and source ID, then submits once; escaped native table cells; unknown/empty/invalid component/version uses authoritative semantic fallback; no external frontend requests or raw Run/Tool traces; generated-asset freshness gate; CLI-scoped watcher 2.6.0 build dependency removes the braces chain with identical embedded bytes; dated full npm audit and clean install evidence in the release audit |
| Read-only progress | Web `/progress`, `GET /api/progress` | Existing `Runs.Activity` aggregate projection | Browser asserts no management buttons; underlying Task/Child aggregation tests; ordinary progress does not notify |
| Approvals and auth presentation | Web frozen confirmation; Feishu `/approve ID HASH`; Web settings sign-in | Existing `Runs.decideTx`, `account.Manager.Login` and frozen Tool pipeline | GC-12 Steering invalidates Monday approval; new recipient/time frozen; SIGKILL/restart and duplicate callbacks; one independent external Fake ledger row; real Manager/Bridge OAuth Fake, hidden-model rejection, Probe/Select, restart account picker, logout and credential scan; ephemeral auth events |
| GC-12 complete invitation mechanics | `verify_gc12.py --binary bin/erii --evidence evidence/gc12.json`; five-round `verify_suite.py --shard communication` | Actual Web/signed Feishu → Main/Context Builder → Security/ToolInvocation → configured stdio MCP; explicit meeting model Fixture | Launch/Doctor isolated profile/channel/MCP; Drive original §8.13 inputs, source-based Alex Chen selection, obsolete Monday approval invalidation before notice transport, Tuesday 11:00–11:30 exact frozen consent, SIGKILL/restart and concurrent/repeated callbacks; Evidence separate mail/calendar rows, actual status readback, one completion, snapshots/empty Artifact inventory, replay equality; denial sends nothing, lost reply stays unknown/one attempt; Cleanup joins peers/removes root. Six variants plus two safety probes, no semantic scores or L2 PASS. Details: `docs/gc12-verification.md`; mutations reject missing/duplicate/wrong effects and unknown-replay drift. The CLI clears the old aggregate before validation/startup; `test_gc12.py` checks same-path failure and Judge rejection, with `ERII_GC12_TEST_BINARY` exercising the positive run through the actual CLI. |
| Channel attachments | Browser file input/Artifact download; Feishu image/file callback and outbound file | ArtifactStore owner scope, stable `channel_resource`, fixed platform upload/resource endpoint and per-part receipt | Incoming retry retains reference and downloaded bytes; outgoing filename/SHA-256/recipient asserted at independent HTTP Fake; wrong reference rejected |
| Future notifications | Web settings or `channel manage` prefer; automatic Feishu preference | Immutable `message_route`, existing `DeliveryRequest`/Outbox/claims | GC-14 six scheduled days, two threshold notices through bound Feishu, quiet-day and duplicate-source stability; unknown remote send never resent; successful multipart receipts reused after restart |
| Upgrade and crash recovery | Startup from empty/prior DB; normal foreground `web serve`; `verify_channels.py --delivery-lease-only` | Migration 025, existing Actor/Delivery claim/renew/recovery | All-prior schema tests, populated-v24 migration rollback/preservation, CAS, frozen projection and route; real SIGKILL input/delivery windows; delayed send/upload and cumulative multipart exceed the default lease, same owner excludes competing scan; pre-transport revalidation rejects expired claims; expired owner cancels and joins without stopping ingress; competing recovery and late response/restart keep unknown query-only with one independent send attempt; cleanup |


| Feature | Reach | Authoritative boundary | Inspectable evidence |
| --- | --- | --- | --- |
| GC-09 exact agenda mechanics | `run_cases.py --binary bin/erii --evidence evidence/cases.json`; existing runtime CI gate | Locked `m1-v2` §8.10 source → CLI/TUI → Main/production Context Builder → explicit golden Bridge; `agenda_case.observations` | Launch six isolated roots; Doctor explicit profile only; Drive three wordings/two noise/order with all three calendar entries, E1–E4 and T1–T2; Evidence source and original event in every provider request, E1/T1 first and before noon, preparation window before 14:00, E4 no repeated reply, lower-priority mail excluded from first two, ≤8 short items, one completion, no Task/Automation/approval/Artifact/mail/calendar action, complete replay equality; Cleanup joins PTYs/processes and removes roots. Judge previews recompute all receipts and remain ineligible/null. The separately locked same-day Steering extension retains all original receipts, drives three follow-up wordings, omits completed E1/T1/E4, updates preparation around a personal window, and proves step-zero context-freeze SIGKILL/restart, frozen input/model/tools, final model-result/visible-delivery agreement, retained completed calls and full replay equality. Judge preparation rejects absent/forged Steering receipts. Broader §7 Worker question aggregation/answer routing, concurrent Steering, actual connectors and unscripted quality remain unverified. See `docs/gc09-verification.md`. |

## Local-first evidence collection (ERII-206)

| Feature | How to reach it | Authoritative entry point | Evidence |
| --- | --- | --- | --- |
| Durable run evidence and same-host reuse | `scripts/evidence.py collect/verify/receipt/export` | `evidence.collect`, `verify`, per-run manifest/raw/export | Command/exit/profile/SHA identity, complete selected raw hashes, exclusions, worktree removal, frozen repeated calls, corruption rejection and interruption recovery |
| Selective immutable CI archive | `scripts/evidence.py ci-list/ci-fetch`; historical `ci_compare.py` with `ERII_EVIDENCE_ROOT` | Exact-attempt GitHub metadata, `artifact_archive`, existing `ci_github.read` | One ID downloaded cold, verified warm zero payload, expiry/missing/hash/attempt/budget rejection; comparison consumers preserve their assertions |
| Transport verification harness | `make verify-evidence EVIDENCE_ROOT=/absolute/durable/fresh-root` | `scripts/evidence_harness.py`; `scripts/test_evidence.py` | Real CLI author/reader processes, Fake network object ledger, actual payload sizes versus metadata estimates, explicit staged/uploaded distinction, one complete export, private fixture cleanup |

See [evidence operations](../docs/guides/evidence.md). Supporting local evidence
never replaces independent clean exact-SHA review or the existing required CI
and final release evidence contracts.

## ERII-192 consolidated Tool surface

| Feature | Reach / authoritative entry point | Inspectable evidence |
| --- | --- | --- |
| Artifact manage/search | Direct Tools or Code Mode `tools.builtin__artifact__manage/search`; `internal/artifact/tools`, `sqlite.executeArtifact`, `ArtifactStore.SearchTx` | `verify_artifacts.py` retains save/read/publish/crash tests; `verify_codemode.py` saves bytes, searches, reads/describes, selects final output through nested ToolInvocations; its maintained Unicode filename action asserts exact/case-normalized matches, suffix/MIME controls and browse; Go contract tests reject cross-operation fields, old names and cross-scope reads, find exact/lowercase/uppercase non-ASCII names and MIME matches beyond 50 newer files while skipping newer inaccessible matches before the limit, preserve accents, and reuse committed results |
| Production tools and disabled Memory | Actual local provider HTTP Tool projection; `app.prepareRuntime`, `sqlite.manageMemory/searchMemory` | `verify_local_provider.py` asserts exactly two Artifact tools, no echo fixture, both Main Memory tools, Worker search without Memory control; unconfigured calls fail with no Mutation; `verify_recall.py` and `verify_memory_maintenance.py` retain configured scoped recall and remember/forget recovery |
| Code Mode CUA and Child guidance | `codemode.Definition`, `describeNamespace("cua"/"agent")` | `verify_codemode.py` inspects exact Artifact declaration, executes CUA discover/bind/observe and one real Child; Main delegation absent; `TestDescriptionIncludesSchemaDeclarationsAndInstructions` checks discovery |
| User content and Intelligent UI | Main final semantic AssistantMessage → DeliveryRequest/Outbox → UISpec; `delivery/ui` and existing Web native table | `verify_delivery.py`, `verify_channels.py --ui-components-only`; fixture echo never sends a user message, and Workers retain no user delivery capability |

Each named harness owns isolated Launch/Doctor/Drive/Evidence/Cleanup. Commands:
`python3 scripts/verify_codemode.py --binary bin/erii --evidence evidence/codemode.json`
and `python3 scripts/verify_local_provider.py --binary bin/erii --evidence evidence/local-provider.json`.
Compatibility and frozen-Run upgrade procedure: [ADR 005](../docs/adr/005-consolidated-tool-surface.md).

## GC-16 exact research journey (ERII-30)

| Feature | Reach / authoritative entry point | Inspectable evidence |
| --- | --- | --- |
| Primary-source research and report | `verify_gc16.py --binary bin/erii --evidence evidence/gc16.json`; CLI → Main Task → Root/Child → MCP → Artifact/Reporter/Delivery | Launch private roots/stdio corpus; Doctor explicit profile and lock; Drive original request with three wordings/order/two noise variants, two same-exchange Children with overlapping lifetimes, D2→D1→D3→D4 and independent D5; Evidence exact source transport/log, Atlas primary date/rename/extension/Aster exclusion, nearby direct citations and fact/inference/unknown separation, CLI-reopened Markdown bytes and one Main artifact delivery, empty Approval/Automation, two SIGKILL/restarts, completed-call/generation reuse, frozen snapshots, complete replay equality; Cleanup joins/removes state. See `docs/gc16-verification.md`; Judge recomputes receipts and leaves all six previews ineligible/null. Actual model research quality, longer contexts and current L2 remain unverified. |
| CI dependency setup | Pinned `ci-inputs` action → Node → direct lockfile installs | Optional hosted npm restore is absent after observed cache-egress hangs prevented tests from starting. `test_verification_selection.py` rejects reintroduction; complete native CI retains unchanged assertions and rounds, with ten-minute job bounds except the approved fifteen-minute Fast/Race/Delivery outer bounds. See `docs/ci.md`. |
