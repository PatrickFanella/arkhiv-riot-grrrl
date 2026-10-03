import importlib.util
import datetime as dt
import fcntl
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, MagicMock

spec = importlib.util.spec_from_file_location('runner', Path(__file__).with_name('runner.py'))
runner = importlib.util.module_from_spec(spec)
spec.loader.exec_module(runner)


def fixture(root):
    root.mkdir()
    (root / 'archive').mkdir()
    (root / 'archive/original').write_bytes(b'preserved')
    (root / 'catalog.dump').write_bytes(b'catalog')
    runner.atomic(root / 'complete.json', {'format': 1, 'complete': True, 'catalog_sha256': runner.digest(root / 'catalog.dump'), 'catalog_size': 7, 'files': runner.safe_tree(root / 'archive')})


class OperationsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)

    def config(self):
        return {k: str(self.root) for k in ('project', 'archive', 'staging', 'state', 'backup_root')} | {'backup_host': 'nuc', 'minimum_free_bytes': 1, 'revision': 'test'}

    def test_cli_isolated_from_original_arkhiv_project(self):
        calls = []
        r = runner.Runner(self.config(), execute=lambda args: calls.append(args) or '{}')
        r.cli('audit')
        self.assertEqual(calls[0][calls[0].index('--project-name') + 1], 'riot-grrrl-pilot')
        self.assertNotIn('arkhiv-pilot', calls[0])

    def test_verified_complete_backup_and_corruption(self):
        p = self.root / 'backup'
        fixture(p)
        self.assertEqual(runner.verify_backup(p)['files'], 1)
        (p / 'archive/original').write_bytes(b'corrupt')
        with self.assertRaises(ValueError):
            runner.verify_backup(p)

    def test_dump_corruption_and_extra_files(self):
        p = self.root / 'backup'
        fixture(p)
        (p / 'catalog.dump').write_bytes(b'bad')
        with self.assertRaises(ValueError):
            runner.verify_backup(p)
        (p / 'catalog.dump').write_bytes(b'catalog')
        (p / 'archive/extra').write_text('unexpected')
        with self.assertRaises(ValueError):
            runner.verify_backup(p)

    def test_manifest_escape_and_symlink(self):
        p = self.root / 'backup'
        fixture(p)
        (p / 'archive/link').symlink_to('/etc/passwd')
        with self.assertRaises(ValueError):
            runner.verify_backup(p)
        (p / 'archive/link').unlink()
        m = json.loads((p / 'complete.json').read_text())
        m['files'] = {'../escape': 'unsafe'}
        runner.atomic(p / 'complete.json', m)
        with self.assertRaises(ValueError):
            runner.verify_backup(p)

    def test_run_preserves_last_success_and_failure_receipt(self):
        calls = []
        r = runner.Runner(self.config(), execute=lambda args: calls.append(args) or '{}')
        good = r.run('audit')
        self.assertEqual(good['status'], 'success')
        def failure(args):
            raise RuntimeError('operation failed: docker')
        r.execute = failure
        bad = r.run('audit')
        self.assertEqual(bad['status'], 'failed')
        self.assertEqual(bad['last_success'], good['last_success'])
        self.assertEqual(len(list((self.root / 'history').glob('*.json'))), 2)

    def test_overlap_does_not_replace_success(self):
        r = runner.Runner(self.config(), execute=lambda args: '{}')
        r.run('audit')
        with (self.root / '.operation.lock').open('a') as lock:
            fcntl.flock(lock, fcntl.LOCK_EX)
            self.assertEqual(r.run('audit')['status'], 'lock-skipped')
        self.assertEqual(json.loads((self.root / 'audit.json').read_text())['status'], 'success')

    def test_low_space_fails_without_backup(self):
        config = self.config() | {'minimum_free_bytes': 10**20}
        r = runner.Runner(config, execute=lambda args: '{}')
        self.assertEqual(r.run('backup')['status'], 'failed')
        self.assertFalse((self.root / 'latest-backup.json').exists())

    def test_retention_keeps_latest_pinned_and_unknown(self):
        for day in range(1, 6):
            fixture(self.root / f'auto-2026010{day}T000000Z')
        fixture(self.root / 'pilot-qualified-20261002')
        (self.root / '.incoming-partial').mkdir()
        policy = {'daily': 1, 'weekly': 1, 'monthly': 1, 'pinned': ['auto-20260101T000000Z']}
        plan = runner.retention_plan(self.root, policy)
        self.assertIn('auto-20260105T000000Z', plan['keep'])
        self.assertIn('auto-20260101T000000Z', plan['keep'])
        self.assertIn('pilot-qualified-20261002', plan['hold'])
        self.assertIn('.incoming-partial', plan['hold'])
        self.assertTrue(plan['remove'])
        self.assertTrue(plan['dry_run'])
        self.assertTrue((self.root / plan['remove'][0]).exists())

    def test_monitor_stale_and_recovery_deduplication(self):
        r = runner.Runner(self.config() | {'stale_seconds': {'audit': 86400}}, execute=lambda args: '{}')
        first = r.run('monitor')
        self.assertEqual(first['result']['problems'][0]['job'], 'audit')
        r.run('audit')
        recovered = r.run('monitor')
        self.assertEqual(recovered['result']['problems'], [])
        before = (self.root / 'notification.json').stat().st_mtime_ns
        r.run('monitor')
        self.assertEqual((self.root / 'notification.json').stat().st_mtime_ns, before)

    def test_notifications_retry_after_configuration_and_failure(self):
        config = self.config() | {'stale_seconds': {'audit': 86400}}
        r = runner.Runner(config, execute=lambda args: '{}')
        r.run('monitor')
        r.c['notify_url'] = 'http://127.0.0.1/notice'
        response = MagicMock()
        response.__enter__.return_value.status = 200
        with patch.object(runner.urllib.request, 'urlopen', side_effect=OSError('offline')):
            self.assertEqual(r.run('monitor')['status'], 'failed')
        self.assertFalse(json.loads((self.root / 'notification.json').read_text())['sent'])
        with patch.object(runner.urllib.request, 'urlopen', return_value=response) as send:
            self.assertEqual(r.run('monitor')['status'], 'success')
            r.run('monitor')
            self.assertEqual(send.call_count, 1)
            r.run('audit')
            r.run('monitor')
            self.assertEqual(send.call_count, 2)

    def test_public_readiness_checks_mode_revision_and_failure(self):
        config = self.config() | {'public_readiness_url': 'http://127.0.0.1:54614/api/ready', 'public_revision': 'qualified'}
        r = runner.Runner(config, execute=lambda args: '{}')
        response = MagicMock()
        response.__enter__.return_value.status = 200
        def check(body):
            response.__enter__.return_value.read.return_value = json.dumps(body).encode()
            with patch.object(runner.urllib.request, 'urlopen', return_value=response):
                return r.monitor()['problems']
        self.assertEqual(check({'status': 'ok', 'public': True, 'revision': 'qualified'}), [])
        self.assertEqual(check({'status': 'ok', 'public': False, 'revision': 'qualified'})[0]['job'], 'public-reader')
        self.assertEqual(check({'status': 'ok', 'public': True, 'revision': 'old'})[0]['status'], 'unexpected-readiness')
        with patch.object(runner.urllib.request, 'urlopen', side_effect=OSError('offline')):
            self.assertEqual(r.monitor()['problems'][0]['status'], 'unavailable')

    def test_deadline_and_safe_errors(self):
        r = runner.Runner(self.config())
        r.deadline = 0
        with self.assertRaises(TimeoutError):
            r.command(['echo', 'secret'])
        with self.assertRaises(ValueError):
            runner.Runner(self.config() | {'backup_host': 'nuc;bad'})


if __name__ == '__main__':
    unittest.main()
