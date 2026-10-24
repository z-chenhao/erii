"""Bounded review integrity, drift rejection and unchanged release proof limits."""
import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

import release_audit
import release_package as package


class ReleasePackageTests(unittest.TestCase):
    ledger_path = 'docs/audits/baseline-01.json'

    @classmethod
    def setUpClass(cls):
        cls.root = release_audit.ROOT
        cls.original = json.loads((cls.root / cls.ledger_path).read_text())
        cls.units = [dict(source=source, **row)
                     for source in ('docs/technical-design.md', 'evals/golden-cases.md')
                     for row in release_audit.inventory((cls.root / source).read_text())]

    def setUp(self):
        self.ledger = copy.deepcopy(self.original)

    def run_audit(self, **kwargs):
        return package.audit_package(self.root, self.ledger, self.units, **kwargs)

    def test_all_frozen_blocks_retained_without_certification(self):
        report = self.run_audit()
        self.assertEqual(report['reviewed_blocks'], 80)
        self.assertEqual(sum(report['dispositions'].values()), 80)
        self.assertEqual(report['status'], 'inconclusive')
        self.assertFalse(report['release_ready'])
        self.assertEqual(report['verified_requirements'], 0)
        self.assertFalse(report['native_bytes_checked'])
        self.assertFalse(report['manifest_checked'])
        self.assertFalse(report['git_provenance_checked'])
        self.assertEqual({r['line'] for r in report['reviews']},
                         {r['line'] for r in self.ledger['frozen_blocks']})

    def test_cannot_redefine_frozen_scope_or_drop_duplicate_add_review(self):
        for mutation in ('frozen', 'manifest_hash', 'package', 'missing', 'duplicate', 'extra'):
            with self.subTest(mutation=mutation):
                self.setUp()
                if mutation == 'frozen': self.ledger['frozen_blocks'].pop()
                if mutation == 'manifest_hash': self.ledger['manifest_sha256'] = '0' * 64
                if mutation == 'package': self.ledger['package'] = 'unapproved'
                if mutation == 'missing': self.ledger['reviews'].pop()
                if mutation == 'duplicate': self.ledger['reviews'][-1] = self.ledger['reviews'][0]
                if mutation == 'extra': self.ledger['reviews'].append(copy.deepcopy(self.ledger['reviews'][0]))
                with self.assertRaises(ValueError): self.run_audit()

    def test_changed_ambiguous_text_or_stale_location_rejects(self):
        for mutation in ('hash', 'duplicate', 'text', 'current_line', 'original_line'):
            with self.subTest(mutation=mutation):
                units = copy.deepcopy(self.units)
                target = next(u for u in units if package.identity(u) == package.identity(self.ledger['reviews'][0]))
                self.setUp()
                if mutation == 'hash': target['sha256'] = '0' * 64
                if mutation == 'duplicate': units.append(copy.deepcopy(target))
                if mutation == 'text': self.ledger['reviews'][0]['text'] = 'replacement'
                if mutation == 'current_line': self.ledger['reviews'][0]['current_line'] += 1
                if mutation == 'original_line': self.ledger['reviews'][0]['line'] += 1
                with self.assertRaises(ValueError):
                    package.audit_package(self.root, self.ledger, units)

    def test_pass_claim_missing_duties_and_unsupported_exemption_reject(self):
        for mutation in ('verified', 'empty', 'duplicate', 'limit', 'non_v1'):
            with self.subTest(mutation=mutation):
                self.setUp()
                row = self.ledger['reviews'][0]
                if mutation == 'verified': row['disposition'] = 'verified'
                if mutation == 'empty': row['obligations'] = []
                if mutation == 'duplicate': row['obligations'] *= 2
                if mutation == 'limit': row['proof_limit'] = ''
                if mutation == 'non_v1': row['disposition'] = 'deferred'
                with self.assertRaises(ValueError): self.run_audit()

    def test_unmerged_pr_unknown_refs_and_stale_current_hash_reject(self):
        for mutation in ('pr', 'pr_identity', 'ref', 'hash', 'symbol'):
            with self.subTest(mutation=mutation):
                self.setUp()
                pr = next(iter(self.ledger['merged_prs'].values()))
                if mutation == 'pr': pr['state'] = 'OPEN'
                if mutation == 'pr_identity': pr['url'] = 'https://github.com/other/erii/pull/8'
                if mutation == 'ref': self.ledger['reviews'][0]['references'].append('missing.go')
                ref = next(iter(self.ledger['references']))
                if mutation == 'hash': self.ledger['references'][ref]['sha256'] = '0' * 64
                if mutation == 'symbol':
                    self.ledger['references'][ref + '#MissingSymbol'] = self.ledger['references'].pop(ref)
                with self.assertRaises(ValueError): self.run_audit()

    def test_every_remaining_duty_requires_a_finite_existing_route(self):
        for mutation in ('missing', 'empty_exit', 'unknown_duty', 'orphan'):
            with self.subTest(mutation=mutation):
                self.setUp()
                row = next(r for r in self.ledger['reviews'] if r['remaining'])
                key = row['remaining'][0]['gap']
                if mutation == 'missing': del self.ledger['gaps'][key]
                if mutation == 'empty_exit': self.ledger['gaps'][key]['exit_condition'] = ''
                if mutation == 'unknown_duty': row['remaining'][0]['obligation'] = 'different duty'
                if mutation == 'orphan': self.ledger['gaps']['unused'] = copy.deepcopy(self.ledger['gaps'][key])
                with self.assertRaises(ValueError): self.run_audit()

    def test_raw_observation_binding_checks_bytes_size_and_safe_location(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            data = b'SYNTHETIC integrity fixture, not native product evidence\n'
            (root / 'raw.log').write_bytes(data)
            synthetic = dict(path='raw.log', size_bytes=len(data), sha256=package.digest(data))
            for obs in self.ledger['observations'].values(): obs['files'] = [synthetic]
            self.assertTrue(self.run_audit(native_root=root)['native_bytes_checked'])
            for mutation in ('bytes', 'size', 'traversal', 'symlink'):
                with self.subTest(mutation=mutation):
                    synthetic.update(path='raw.log', size_bytes=len(data), sha256=package.digest(data))
                    (root / 'raw.log').write_bytes(data)
                    if mutation == 'bytes': (root / 'raw.log').write_bytes(data + b'!')
                    if mutation == 'size': synthetic['size_bytes'] += 1
                    if mutation == 'traversal': synthetic['path'] = '../raw.log'
                    if mutation == 'symlink':
                        (root / 'link').symlink_to(root / 'raw.log')
                        synthetic['path'] = 'link'
                    with self.assertRaises(ValueError): self.run_audit(native_root=root)

    def test_invalid_original_manifest_never_appears_checked(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'manifest.json'
            path.write_text(json.dumps(dict(packages={'baseline-01': {'blocks': self.ledger['frozen_blocks']}})))
            with self.assertRaises(ValueError): self.run_audit(manifest=path)

    def test_full_git_mode_checks_blobs_and_ancestry_without_fetch(self):
        # Synthetic local history tests only the provenance consumer mechanism.
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            paths = {ref.partition('#')[0] for ref in self.ledger['references']}
            paths.add('docs/technical-design.md')
            for name in paths:
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes((self.root / name).read_bytes())
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.DEVNULL)
            git('init', '-q')
            git('add', '.')
            git('-c', 'user.name=Synthetic Test', '-c', 'user.email=test@example.invalid',
                'commit', '-qm', 'synthetic provenance fixture')
            base = git('rev-parse', 'HEAD').decode().strip()
            self.ledger['audited_base'] = base
            for obs in self.ledger['observations'].values(): obs['source_sha'] = base
            for pr in self.ledger['merged_prs'].values(): pr['mergeCommit']['oid'] = base
            report = package.audit_package(root, self.ledger, self.units, verify_git=True)
            self.assertTrue(report['git_provenance_checked'])
            self.assertFalse(report['release_ready'])
            next(iter(self.ledger['merged_prs'].values()))['mergeCommit']['oid'] = '0' * 40
            with self.assertRaises(ValueError):
                package.audit_package(root, self.ledger, self.units, verify_git=True)

    def test_real_cli_preserves_history_and_rejects_stale_report(self):
        folder = self.root / 'evidence'
        folder.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=folder, prefix='package-test-') as temp:
            root = Path(temp)
            output, ledger = root / 'report.json', root / 'package.json'
            ledger.write_text(json.dumps(self.ledger))
            command = [sys.executable, str(self.root / 'scripts/release_audit.py'),
                       '--package-ledger', str(ledger), '--evidence', str(output), '--require-complete']
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertEqual(result.returncode, 2, result.stderr)
            report = json.loads(output.read_text())
            self.assertFalse(report['release_ready'])
            self.assertEqual(report['individually_reviewed_blocks'], 122)
            self.assertEqual(report['package_audit']['reviewed_blocks'], 80)
            self.ledger['reviews'].pop()
            ledger.write_text(json.dumps(self.ledger))
            result = subprocess.run(command, capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(output.exists(), 'failed audit retained the previous report')

    def test_cli_never_deletes_a_package_or_native_input(self):
        folder = self.root / 'evidence'
        folder.mkdir(exist_ok=True)
        with tempfile.TemporaryDirectory(dir=folder, prefix='package-input-test-') as temp:
            root = Path(temp)
            native = root / 'native'
            native.mkdir()
            raw = native / 'observation.json'
            original = b'SYNTHETIC retained native input\n'
            raw.write_bytes(original)
            ledger = root / 'package.json'
            ledger.write_text(json.dumps(self.ledger))
            command = [sys.executable, str(self.root / 'scripts/release_audit.py'),
                       '--package-ledger', str(ledger)]
            result = subprocess.run(command + ['--native-root', str(native), '--evidence', str(raw)],
                                    capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertTrue(raw.exists(), 'output validation deleted retained native input')
            self.assertEqual(raw.read_bytes(), original)
            before = ledger.read_bytes()
            result = subprocess.run(command + ['--evidence', str(ledger)], capture_output=True, text=True)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(ledger.read_bytes(), before)


class Baseline03Tests(ReleasePackageTests):
    # Exercise the same production CLI and all corruption cases on this package,
    # including its multi-line schema fences and mixed current/deferred duties.
    ledger_path = 'docs/audits/baseline-03.json'

    def test_every_current_duty_is_routed_and_only_explicit_future_work_is_deferred(self):
        for row in self.ledger['reviews']:
            if row['disposition'] in ('background', 'deferred'):
                self.assertTrue(row['reason'])
                continue
            deferred = row.get('deferred_obligations', [])
            deferred_duties = {d['obligation'] for d in deferred}
            self.assertTrue(all(d['reason'] for d in deferred))
            remaining = {d['obligation'] for d in row['remaining']}
            self.assertEqual(set(row['obligation_kinds']), set(row['obligations']))
            self.assertEqual({o for o, kind in row['obligation_kinds'].items()
                              if kind == 'deferred'}, deferred_duties)
            self.assertEqual(set(row['exemptions']), deferred_duties)
            self.assertEqual({o for o, kind in row['obligation_kinds'].items()
                              if kind == 'required_v1'}, remaining)
            self.assertFalse(remaining & deferred_duties)
            self.assertEqual(set(row['obligations']), remaining | deferred_duties,
                             f"unaccounted sub-obligation at frozen line {row['line']}")
        mixed = next(r for r in self.ledger['reviews'] if r['line'] == 539)
        self.assertEqual(len(mixed['deferred_obligations']), 1)
        self.assertIn('Device Authorization', mixed['deferred_obligations'][0]['obligation'])
        self.assertEqual(len(mixed['remaining']), 12)

    def test_schema_fence_and_observed_web_gap_keep_their_independent_duties(self):
        schema = next(r for r in self.ledger['reviews'] if r['line'] == 446)
        self.assertEqual(len(schema['obligations']), 38)
        self.assertIn('ModelCapabilities.audioInput:boolean', schema['obligations'])
        self.assertIn('AuthConnection.version:number', schema['obligations'])
        for line in (433, 434):
            row = next(r for r in self.ledger['reviews'] if r['line'] == line)
            self.assertEqual(row['disposition'], 'evidence_gap')
            self.assertEqual({d['gap'] for d in row['remaining']}, {'G-setup'})
        self.assertEqual(self.ledger['gaps']['G-setup']['issue'], 'ERII-334')


if __name__ == '__main__':
    unittest.main()
