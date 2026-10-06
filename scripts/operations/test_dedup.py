import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest

from test_runner import fixture, runner


class DeduplicatedSnapshotsTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)
        self.addCleanup(self.make_writable)

    def make_writable(self):
        for path in [self.root, *self.root.rglob('*')]:
            if path.is_dir() and not path.is_symlink():
                path.chmod(0o700)

    def transfer(self, source, destination, basis=None):
        destination.mkdir()
        args = ['rsync', '-a', '--checksum', '--stats']
        if basis:
            args.append('--link-dest=' + str(basis))
        args += ['--', str(source) + '/', str(destination) + '/']
        result = subprocess.run(args, capture_output=True, text=True, env=dict(os.environ, LC_ALL='C'), check=True)
        runner.verify_backup(destination)
        runner.freeze_backup(destination)
        return runner.transfer_summary(result.stdout)

    def test_unchanged_transfer_shares_remote_bytes_and_changed_source_preserves_history(self):
        staging = self.root / 'staging'
        remote = self.root / 'remote'
        staging.mkdir()
        remote.mkdir()
        first = staging / 'auto-20260101T000000Z'
        fixture(first)
        runner.freeze_backup(first)
        original = remote / first.name
        self.transfer(first, original)
        before = runner.allocation(remote)['allocated_bytes']
        second = staging / 'auto-20260102T000000Z'
        shutil.copytree(first, second)
        unchanged = remote / second.name
        summary = self.transfer(second, unchanged, original)
        self.assertEqual(summary['transferred_bytes'], 0)
        self.assertEqual(summary['transferred_files'], 0)
        self.assertEqual((original / 'archive/original').stat().st_ino, (unchanged / 'archive/original').stat().st_ino)
        self.assertLess(runner.allocation(remote)['allocated_bytes'] - before, 65536)
        third = staging / 'auto-20260103T000000Z'
        shutil.copytree(second, third)
        third.chmod(0o700)
        changed = third / 'archive/original'
        old_stat = changed.stat()
        changed.chmod(0o600)
        changed.write_bytes(b'changed!!')
        os.utime(changed, ns=(old_stat.st_atime_ns, old_stat.st_mtime_ns))
        manifest = json.loads((third / 'complete.json').read_text())
        manifest['files'] = runner.safe_tree(third / 'archive')
        runner.atomic(third / 'complete.json', manifest)
        runner.freeze_backup(third)
        modified = remote / third.name
        summary = self.transfer(third, modified, unchanged)
        self.assertGreater(summary['transferred_bytes'], 0)
        self.assertNotEqual((modified / 'archive/original').stat().st_ino, (unchanged / 'archive/original').stat().st_ino)
        for old in (original, unchanged):
            self.assertEqual((old / 'archive/original').read_bytes(), b'preserved')
            runner.verify_backup(old)
        # Fresh restore copying creates independent mutable files, not links to evidence.
        restored = self.root / 'fresh-restore'
        shutil.copytree(modified, restored)
        runner.verify_backup(restored)
        self.assertEqual((restored / 'archive/original').read_bytes(), b'changed!!')
        self.assertNotEqual((restored / 'archive/original').stat().st_ino, (modified / 'archive/original').stat().st_ino)

    def test_corrupt_basis_is_not_silently_skipped_and_retention_stays_dry_run(self):
        fixture(self.root / 'auto-20260101T000000Z')
        fixture(self.root / 'auto-20260102T000000Z')
        (self.root / 'auto-20260102T000000Z/catalog.dump').write_bytes(b'corrupt')
        with self.assertRaises(ValueError):
            runner.verified_basis(self.root)
        (self.root / '.incoming-incomplete').mkdir()
        plan = runner.retention_plan(self.root, {'daily': 1, 'weekly': 1, 'monthly': 1})
        self.assertTrue(plan['dry_run'])
        self.assertIn('.incoming-incomplete', plan['hold'])
        self.assertIn('auto-20260102T000000Z', plan['hold'])

    def test_writable_completed_basis_is_rejected_without_changing_it(self):
        snapshot = self.root / "auto-20260101T000000Z"
        fixture(snapshot)
        before = (snapshot / "archive/original").stat().st_mode
        with self.assertRaises(ValueError):
            runner.verified_basis(self.root)
        self.assertEqual((snapshot / "archive/original").stat().st_mode, before)
        runner.freeze_backup(snapshot)
        self.assertEqual(runner.verified_basis(self.root), str(snapshot))

    def test_capacity_refuses_before_a_new_snapshot_and_preserves_latest(self):
        state = self.root / 'state'
        state.mkdir()
        previous = {'name': 'qualified', 'destination': '/retained/qualified'}
        runner.atomic(state / 'latest-backup.json', previous)
        calls = []
        config = {k: str(self.root) for k in ('project', 'archive', 'staging', 'backup_root')}
        config.update(state=str(state), backup_host='nuc', minimum_free_bytes=1, deduplicate_backup=True, maximum_snapshot_bytes=1024, maximum_staging_bytes=1024, maximum_backup_bytes=1024)
        result = runner.Runner(config, execute=lambda args: calls.append(args)).run('backup')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(calls, [])
        self.assertEqual(json.loads((state / 'latest-backup.json').read_text()), previous)

    def test_incomplete_remote_transfer_never_becomes_latest(self):
        staging = self.root / 'staging'
        remote = self.root / 'remote'
        state = self.root / 'state'
        for path in (staging, remote, state):
            path.mkdir()
        previous = {'name': 'qualified', 'destination': '/retained/qualified'}
        runner.atomic(state / 'latest-backup.json', previous)
        def execute(args):
            if args[0] == 'docker':
                name = Path(args[args.index('--destination') + 1]).name
                fixture(staging / name)
                return ''
            if args[0] == 'ssh':
                import shlex
                command = shlex.split(args[-1])
                if 'free' in command:
                    return str(10**12)
                if command[0] == 'mkdir':
                    Path(command[-1]).mkdir()
                    return ''
                if 'verify' in command:
                    raise RuntimeError('incomplete remote transfer')
                self.fail('unexpected remote promotion')
            if args[0] == 'env':
                actual = args[2:]
                actual[-1] = actual[-1].split(':', 1)[1]
                return subprocess.check_output(actual, text=True)
            self.fail('unexpected command')
        config = {k: str(self.root) for k in ('project', 'archive')}
        config.update(staging=str(staging), state=str(state), backup_root=str(remote), backup_host='nuc', minimum_free_bytes=1, remote_script='/private/runner.py')
        result = runner.Runner(config, execute=execute).run('backup')
        self.assertEqual(result['status'], 'failed')
        self.assertEqual(json.loads((state / 'latest-backup.json').read_text()), previous)
        self.assertTrue(list(remote.glob('.incoming-*')))


if __name__ == '__main__':
    unittest.main()
