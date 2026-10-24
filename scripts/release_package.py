"""Validate a bounded audit ledger, without certifying product conformance.

The frozen package digest comes from the owner manifest, independently of the
authored dispositions. Native observations remain supporting evidence; only the
release decision path can consume independently acquired release receipts.
"""
from collections import Counter
import hashlib
import json
from pathlib import Path
import subprocess

from release_receipts import strict_json

MANIFEST_HASH = '0120228762abfe54dd477bc27853b3b45338d0ea5f19363a3d3b742224f5c556'
PACKAGE_HASHES = {
    'baseline-01': 'b95fc9fa9861ff62d5551f820ae56dac90f9ad33ecdbd2ad2ef743fcc315db95',
    'baseline-04': '2e56b4c2b855c9c6a81e667f316e92444e2890a8a35568a36cdce566fb07639e',
    'baseline-06': 'b20a9f33e1b58b9b184f1a3004055d12f113546764f6b89a9c103016ab94668a',
}


def digest(data):
    return hashlib.sha256(data).hexdigest()


def frozen_digest(blocks):
    return digest(json.dumps(blocks, sort_keys=True, separators=(',', ':')).encode())


def identity(block):
    return block['source'], block['section'], block['sha256']


def local_file(root, name):
    path = root / name
    if (Path(name).is_absolute() or not path.resolve().is_relative_to(root.resolve())
            or path.is_symlink() or not path.is_file()):
        raise ValueError('invalid package file: ' + name)
    return path


def git(root, *args):
    result = subprocess.run(['git', '-C', str(root), *args], capture_output=True)
    if result.returncode:
        raise ValueError('package Git binding rejected: ' + ' '.join(args))
    return result.stdout


def audit_package(root, ledger, units, manifest=None, native_root=None, verify_git=False):
    name = ledger['package']
    frozen = ledger['frozen_blocks']
    if (ledger.get('version') != 1 or ledger['manifest_sha256'] != MANIFEST_HASH
            or name not in PACKAGE_HASHES or frozen_digest(frozen) != PACKAGE_HASHES[name]):
        raise ValueError('frozen package changed; coordinator must reseal it')
    if manifest is not None:
        raw = manifest.read_bytes()
        if digest(raw) != MANIFEST_HASH:
            raise ValueError('owner manifest hash mismatch')
        definition = strict_json(raw)
        assigned = [identity(b) for p in definition['packages'].values() for b in p['blocks']]
        if len(assigned) != 1585 or any(n != 1 for n in Counter(assigned).values()):
            raise ValueError('owner manifest assignment is not unique and complete')
        if definition['packages'][name]['blocks'] != frozen:
            raise ValueError('package differs from owner manifest')
    by_key = {}
    for unit in units:
        by_key.setdefault(identity(unit), []).append(unit)
    expected = {identity(b) for b in frozen}
    reviews = ledger['reviews']
    if (len(reviews) != len(frozen) or len({identity(r) for r in reviews}) != len(reviews)
            or {identity(r) for r in reviews} != expected):
        raise ValueError('missing, duplicate or out-of-package disposition')
    for block in frozen:
        if len(by_key.get(identity(block), [])) != 1:
            raise ValueError('changed or ambiguous frozen source block; reseal required: ' + str(block))
    # Current evidence candidates must resolve at the audited Git revision and
    # in this checkout. Historical fallback is deliberately unavailable here.
    base = ledger['audited_base']
    if verify_git:
        git(root, 'merge-base', '--is-ancestor', base, 'HEAD')
        if digest(git(root, 'show', base + ':docs/technical-design.md')) != ledger['source_sha256']:
            raise ValueError('audited source hash mismatch')
    references = ledger['references']
    for ref, value in references.items():
        path, _, symbol = ref.partition('#')
        data = local_file(root, path).read_bytes()
        if digest(data) != value['sha256']:
            raise ValueError('stale current reference: ' + ref)
        if verify_git and digest(git(root, 'show', base + ':' + path)) != value['sha256']:
            raise ValueError('audited Git blob mismatch: ' + ref)
        if symbol and symbol not in data.decode():
            raise ValueError('missing current symbol: ' + ref)
    for number, pr in ledger['merged_prs'].items():
        if (pr['state'] != 'MERGED' or pr['number'] != int(number)
                or pr['url'] != 'https://github.com/z-chenhao/erii/pull/' + number
                or not pr['mergedAt']):
            raise ValueError('unmerged or wrong PR provenance')
        if verify_git:
            git(root, 'merge-base', '--is-ancestor', pr['mergeCommit']['oid'], base)
    gaps = ledger['gaps']
    for key, gap in gaps.items():
        if (not gap['remaining'] or not gap['exit_condition'] or not gap['deduplication']
                or not gap['issue_url'].startswith('https://multica.ai/erii-studio/issues/')
                or not gap['issue'].startswith('ERII-')):
            raise ValueError('unrouted package gap: ' + key)
    observations = ledger['observations']
    for obs in observations.values():
        if (obs['source_sha'] != base or not obs['command'] or not obs['limits']
                or not obs['files'] or obs['exit_code'] != 0):
            raise ValueError('incomplete native observation receipt')
        for f in obs['files']:
            if native_root is not None:
                raw = local_file(native_root, f['path']).read_bytes()
                if digest(raw) != f['sha256'] or len(raw) != f['size_bytes']:
                    raise ValueError('native observation bytes changed')
    counts = Counter()
    used_gaps = set()
    for review in reviews:
        source = by_key[identity(review)][0]
        original = next(b for b in frozen if identity(b) == identity(review))
        if (review['text'] != source['text'] or review['current_line'] != source['line']
                or review['line'] != original['line']):
            raise ValueError('review source text or location drifted')
        if review['disposition'] not in ('partial_mechanical', 'evidence_gap', 'policy', 'background', 'deferred'):
            raise ValueError('package reviews cannot certify conformance')
        for field in ('obligations', 'observed', 'proof_limit', 'references', 'prs'):
            if not review.get(field):
                raise ValueError('missing package review field: ' + field)
        if len(set(review['obligations'])) != len(review['obligations']):
            raise ValueError('duplicate sub-obligation')
        if any(not isinstance(o, str) or not o.strip() for o in review['obligations']):
            raise ValueError('empty sub-obligation')
        # New audits explicitly distinguish mandatory duties from mixed-in
        # background/deferred clauses. Preserve the first package's format.
        if name != 'baseline-01' or 'obligation_kinds' in review:
            kinds = review.get('obligation_kinds', {})
            if (set(kinds) != set(review['obligations']) or
                    not set(kinds.values()) <= {'required_v1', 'policy', 'background', 'deferred'}):
                raise ValueError('missing or invalid sub-obligation scope')
            exempt = {o for o, kind in kinds.items() if kind in ('background', 'deferred')}
            reasons = review.get('exemptions', {})
            remaining = {r['obligation'] for r in review['remaining']}
            if (set(reasons) != exempt or any(not reason.strip() for reason in reasons.values())
                    or remaining != set(kinds) - exempt):
                raise ValueError('mandatory duty unrouted or exemption unexplained')
        if not set(review['references']) <= references.keys() or not set(review['prs']) <= ledger['merged_prs'].keys():
            raise ValueError('unknown current reference or PR')
        if not set(review['observations']) <= observations.keys():
            raise ValueError('unknown native observation')
        if review['disposition'] in ('background', 'deferred') and (review['remaining'] or not review.get('reason')):
            raise ValueError('non-v1 disposition needs an explicit reason')
        if review['disposition'] == 'evidence_gap' and not review['remaining']:
            raise ValueError('evidence gap has no routed obligation')
        for remaining in review['remaining']:
            if (remaining['obligation'] not in review['obligations'] or remaining['gap'] not in gaps
                    or not remaining['detail']):
                raise ValueError('unrouted sub-obligation')
            used_gaps.add(remaining['gap'])
        counts[review['disposition']] += 1
    if used_gaps != gaps.keys():
        raise ValueError('orphan gap route')
    return dict(package=name, reviewed_blocks=len(reviews), dispositions=dict(counts),
                frozen_package_sha256=PACKAGE_HASHES[name],
                manifest_checked=manifest is not None, native_bytes_checked=native_root is not None,
                git_provenance_checked=verify_git,
                status='inconclusive', release_ready=False, verified_requirements=0,
                gaps=gaps, observations=observations, reviews=reviews)
