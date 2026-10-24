"""Frozen Loop/Child/replay audit integrity through the existing consumer."""
import copy
import json

import test_release_package
from release_package import frozen_digest


class Baseline06Tests(test_release_package.ReleasePackageTests):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.original = json.loads((cls.root / 'docs/audits/baseline-06.json').read_text())

    def test_exact_owner_assignment_and_scoped_sub_obligations(self):
        self.assertEqual(frozen_digest(self.ledger['frozen_blocks']),
                         'b20a9f33e1b58b9b184f1a3004055d12f113546764f6b89a9c103016ab94668a')
        self.assertEqual([b['line'] for b in self.ledger['frozen_blocks']][::79], [886, 1111])
        self.assertEqual(sum(len(r['obligations']) for r in self.ledger['reviews']), 282)
        for row in self.ledger['reviews']:
            self.assertEqual(row['current_line'], row['line'] + 10)
            self.assertEqual(set(row['obligations']), set(row['obligation_kinds']))
            required = {o for o, kind in row['obligation_kinds'].items()
                        if kind in ('required_v1', 'policy')}
            self.assertEqual(required, {r['obligation'] for r in row['remaining']})
        self.assertEqual(self.run_audit()['verified_requirements'], 0)

    def test_mandatory_scope_cannot_be_lost_or_silently_exempted(self):
        original = copy.deepcopy(self.ledger)
        for mutation in ('scopes', 'missing_scope', 'invalid_kind', 'unrouted', 'exempt_reason'):
            with self.subTest(mutation=mutation):
                self.ledger = copy.deepcopy(original)
                row = self.ledger['reviews'][0]
                duty = row['obligations'][0]
                if mutation == 'scopes': del row['obligation_kinds']
                if mutation == 'missing_scope': del row['obligation_kinds'][duty]
                if mutation == 'invalid_kind': row['obligation_kinds'][duty] = 'verified'
                if mutation == 'unrouted': row['remaining'].pop(0)
                if mutation == 'exempt_reason':
                    row['obligation_kinds'][duty] = 'background'
                    row['remaining'].pop(0)
                with self.assertRaises(ValueError): self.run_audit()

    def test_observed_differences_keep_independent_finite_routes(self):
        rows = {r['line']: r for r in self.ledger['reviews']}
        for line in (1009, 1102):
            self.assertEqual(rows[line]['disposition'], 'evidence_gap')
            self.assertIn('G-key', {r['gap'] for r in rows[line]['remaining']})
        self.assertEqual(self.ledger['gaps']['G-key']['issue'], 'ERII-340')
        self.assertEqual(self.ledger['gaps']['G-event']['issue'], 'ERII-341')
        self.assertEqual({r['gap'] for r in rows[892]['remaining']}, {'G-event'})
        # Unobserved compositions and Forget priority cannot inherit success
        # from constituent interfaces or the 100-Child recovery fixture.
        for line in (975, 989, 1108):
            self.assertEqual(rows[line]['disposition'], 'evidence_gap')
            self.assertTrue(rows[line]['remaining'])
        self.assertEqual(rows[1020]['remaining'][0]['gap'], 'G-model')
        report = self.run_audit()
        self.assertFalse(report['release_ready'])
        self.assertEqual(report['status'], 'inconclusive')
