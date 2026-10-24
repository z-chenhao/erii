# Bounded baseline audits

[baseline-05](baseline-05.md) records ERII-235's separate 80-block audit of Store
isolation, the JavaScript Host protocol, ToolInvocation and Main runtime messages.
Its 363 sub-obligations retain explicit scopes, evidence limits and finite routes.
Use `make audit-baseline-05`; no requirement is certified.

[baseline-04](baseline-04.md) records ERII-234's separate 80-block audit of
account, Bridge, model failure and Code Mode contracts. Its ledger and current
observations do not change baseline-01 or the preserved historical ledger.

[baseline-01.json](baseline-01.json) disposes of the **80 exact blocks** assigned
to ERII-231 in the owner's frozen work-package manifest. It records 257 authored
sub-obligations: 48 blocks have partial mechanical evidence, 17 have evidence
gaps, seven describe policy, five provide background and three explicitly defer
long-term features. No requirement is certified as implemented or verified.

Each row retains source/section/original line/hash and full source text, current
line, sub-obligations, observations, current code/test/harness references, actual
merged PR provenance, native observation references, proof limits and remaining
duties. `references` binds current file bytes, with symbols where applicable;
there is no fallback to historical code. An indexed maintenance test is not
necessarily an executed test. The raw Go JSON stream identifies the actual 171
passing tests, including subtests.

The original [ERII-30 ledger](../v1-release-ledger.json) and its 122 partial/gap
reviews remain unchanged. Package reviews are reported separately, including
overlap with historical reviews. Counts never imply newly verified requirements.
The inventory and release decision retain their existing proof limits.

## Frozen scope and changes

The original manifest is 380773 bytes, SHA256
`0120228762abfe54dd477bc27853b3b45338d0ea5f19363a3d3b742224f5c556`, from
baseline `d11e543ea32efdf784478f5959ea6565af87b646`. Its full 1585-block assignment
was checked locally for uniqueness. The canonical JSON hash of
`packages.baseline-01.blocks` is
`b95fc9fa9861ff62d5551f820ae56dac90f9ad33ecdbd2ad2ef743fcc315db95`.
The validator pins this digest independently of the authored dispositions.

ADR 006 / merged PR 45 changed Task clauses after this package's last block.
All 80 section/content hashes still match uniquely, from §0 L10 through §3.5
L164. The recorded current whole-file hash does not replace the frozen
manifest's whole-file hash. Current inventory has 1588 blocks after approved
additions. A changed/ambiguous package block rejects and needs coordinator
resealing with an explicit old/new hash mapping. Line moves require a unique
unchanged section/content hash.

## Run and inspect

```sh
make audit-baseline-01
python3 -m unittest discover -s scripts -p 'test_release_*.py'
```

The first command uses the maintained `release_audit.py` entry point and contacts
no network, account, model or VM. Doctor checks frozen identities, current
reference bytes/symbols, complete dispositions and finite gap routes. Drive reads
the ledger through the CLI. Evidence is `evidence/baseline-01.json`; inspect
`package_audit` and `source_units`. Cleanup may remove that disposable output.
Output cannot overlap an input or retained native directory. Invalid reruns
delete stale disposable reports. `--require-complete` still exits 2:
finishing this audit is not release acceptance.

For an independently acquired original manifest, retained native bytes and
full Git history, strengthen the read-only verification:

```sh
python3 scripts/release_audit.py \
  --package-ledger docs/audits/baseline-01.json \
  --work-packages /absolute/durable/work-packages.json \
  --native-root /absolute/durable/audit-20261024 \
  --verify-git --evidence evidence/baseline-01.json
```

The three inputs set explicit `manifest_checked`, `native_bytes_checked` and
`git_provenance_checked` flags. Without them the flags stay false: a shallow
CI checkout cannot inspect unavailable history or same-host files.
`--verify-git` checks audited blobs and merged PR ancestry without fetching;
missing objects reject. Byte hashes prove integrity, not the truth of a model
or VM observation. Independent review must judge semantic completeness of the
sub-obligations; the parser cannot derive that from counts or keywords.

Native observations remain under the registered durable evidence root in
`ERII-231/fd2a3534f5b0602c5bf9376c14912f45b1ff4a5c/audit-20261024/`.
The ledger records commands, source SHA, timestamps, sizes and hashes. Contracts
use real SQLite with deterministic peers. `verify_children.py` drives the actual
CLI/PTY, scoped Artifacts, approval recovery, direct Parent results, cancellation
and six SIGKILL/restart boundaries, then joins processes and removes disposable
state. Its scripted provider earns no semantic scores. Keep the multi-MiB trace
local; publish only a compact manifest.

## Remaining evidence

| Existing route | Remaining scope | Independent exit |
| --- | --- | --- |
| [ERII-330](https://multica.ai/erii-studio/issues/01a16bd2-3352-7bc8-8c8c-fc6045822909) | Real-model Task decisions, continuation, recall, aggregation and startup/delegation chain | Original frozen dual-arm evaluations, native state and independent Judge receipts; unavailable resources stay inconclusive. ERII-230 delivered the entry point, not executions. |
| [ERII-331](https://multica.ai/erii-studio/issues/01a16bd2-481b-7404-8db1-971204d4f839) | Current Web/Feishu, isolated Guest, desktop/browser/Jianying, conflict and nonconflict operation, named human observations | Current SHA/approved environment/native state/Artifact/cleanup and named review; ERII-229's snapshot and deterministic peers cannot substitute. |
| [ERII-332](https://multica.ai/erii-studio/issues/01a16bd2-5d15-7af2-bb7f-7e5ff3c3c420) | Final normative join, protocol/version compatibility, Task status truth table and unobserved standalone Run `external_event` wake | Retain unmet duties until current native evidence and independent review resolve them. Observed implementation conflicts need separate finite issues; this audit does not repair the product. |

These bounded release children are reused by semantics. Each row's `remaining`
field identifies its exact duties. Background/deferred rows retain explicit
reasons. Neither this PR nor a child being Done grants v1 release acceptance.
