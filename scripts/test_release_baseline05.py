"""Frozen Code Mode, Tool and Loop audit uses the shared rejection contracts."""
import copy
import json

from release_package import frozen_digest
import test_release_baseline04


class Baseline05Tests(test_release_baseline04.Baseline04Tests):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.original = json.loads((cls.root / 'docs/audits/baseline-05.json').read_text())

    def test_exact_owner_assignment_and_all_sub_obligations_stay_visible(self):
        self.assertEqual(frozen_digest(self.ledger['frozen_blocks']),
                         '3502b3aa7b733584a5ddfe624ab44b6f303201f6c752e38f650fd882ce45eb12')
        self.assertEqual([b['line'] for b in self.ledger['frozen_blocks']][::79], [693, 885])
        self.assertEqual(sum(len(r['obligations']) for r in self.ledger['reviews']), 363)
        for row in self.ledger['reviews']:
            self.assertEqual(row['current_line'], row['line'] + 10)
            self.assertEqual(set(row['obligations']), set(row['obligation_kinds']))
        report = self.run_audit()
        self.assertEqual(report['verified_requirements'], 0)
        self.assertEqual(report['dispositions']['evidence_gap'], 5)

    def test_observed_implementation_gaps_do_not_inherit_pass_from_interfaces(self):
        rows = {r['line']: r for r in self.ledger['reviews']}
        for line in (753, 787, 792, 801):
            row = rows[line]
            self.assertEqual(row['disposition'], 'evidence_gap')
            self.assertIn('G-runtime', {r['gap'] for r in row['remaining']})
        self.assertEqual(rows[751]['disposition'], 'partial_mechanical')
        self.assertIn('heartbeat', rows[753]['obligations'])
        self.assertIn('attempt_id', rows[787]['observed'])
        self.assertIn('interrupted', rows[792]['observed'])
        self.assertEqual(rows[881]['disposition'], 'evidence_gap')
        self.assertEqual({r['gap'] for r in rows[881]['remaining']}, {'G-model'})

    def test_mixed_future_design_does_not_exempt_current_desktop_serialization(self):
        row = next(r for r in self.ledger['reviews'] if r['line'] == 829)
        self.assertEqual(list(row['obligation_kinds'].values()),
                         ['deferred', 'deferred', 'required_v1', 'required_v1'])
        self.assertEqual(len(row['remaining']), 2)
        # Retaining the row and its counts alone cannot excuse a mandatory duty.
        original = copy.deepcopy(self.ledger)
        row['remaining'].pop()
        with self.assertRaises(ValueError):
            self.run_audit()
        self.ledger = original
