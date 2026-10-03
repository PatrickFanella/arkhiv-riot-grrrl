import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import runner
import restore_drill


class RestoreSafetyTest(unittest.TestCase):
    def config(self, root):
        (root / 'state').mkdir()
        (root / 'qualification').mkdir()
        runner.atomic(root / 'state/latest-backup.json', {'name': 'auto-20261002T000000Z', 'destination': '/independent/auto-20261002T000000Z'})
        runner.atomic(root / 'compose.json', {'services': {'operator': {'image': 'operator:test'}, 'web': {'image': 'web:test'}}})
        return {'project': str(root), 'archive': str(root), 'staging': str(root), 'state': str(root / 'state'), 'backup_host': 'nuc', 'backup_root': '/independent', 'remote_script': '/helper.py', 'reader_probe_image': 'reader:test', 'minimum_free_bytes': 1}

    def test_rejects_unexpected_snapshot_destination_before_transport(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = self.config(root)
            runner.atomic(root / 'state/latest-backup.json', {'name': 'auto-20261002T000000Z', 'destination': '/unrelated'})
            with patch.object(runner.Runner, 'remote') as remote:
                with self.assertRaises(ValueError):
                    restore_drill.drill(config)
                remote.assert_not_called()

    def test_public_role_has_no_login_and_is_created_before_catalog_restore(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = self.config(root)
            calls = []
            def execute(self, args):
                calls.append(args)
                if 'restore' in args:
                    raise RuntimeError('stop after role setup')
                return ''
            with patch.object(runner.Runner, 'remote', return_value=''), patch.object(runner.Runner, 'command', execute), patch.object(restore_drill, 'verify_backup', return_value={}):
                with self.assertRaises(RuntimeError):
                    restore_drill.drill(config)
            role = next(i for i, args in enumerate(calls) if 'CREATE ROLE arkhiv_public NOLOGIN' in args)
            restore = next(i for i, args in enumerate(calls) if 'restore' in args)
            self.assertLess(role, restore)
            self.assertEqual(calls[-1][-2:], ['down', '--volumes'])

    def test_partial_stack_start_is_cleaned_without_touching_primary(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            config = self.config(root)
            calls = []
            def execute(self, args):
                calls.append(args)
                if args[0] == 'docker' and 'up' in args:
                    raise RuntimeError('partial start')
                return ''
            with patch.object(runner.Runner, 'remote', return_value=''), patch.object(runner.Runner, 'command', execute), patch.object(restore_drill, 'verify_backup', return_value={}):
                with self.assertRaises(RuntimeError):
                    restore_drill.drill(config)
            cleanup = calls[-1]
            self.assertEqual(cleanup[-2:], ['down', '--volumes'])
            project = cleanup[cleanup.index('--project-name') + 1]
            self.assertTrue(project.startswith('riot-grrrl-drill-'))
            self.assertNotEqual(project, 'riot-grrrl-pilot')
            compose_path = Path(cleanup[cleanup.index('-f') + 1])
            composed = json.loads(compose_path.read_text())
            self.assertTrue(composed['networks']['default']['internal'])
            self.assertNotIn('ports', composed['services']['db'])


if __name__ == '__main__':
    unittest.main()
