import unittest
from unittest.mock import patch
from restore_drill import cleanup

PROJECT = 'riot-grrrl-drill-20261006t064706609209z'
IDENTITY = 'a' * 64


class RestoreCleanupTests(unittest.TestCase):
    def test_running_oneoff_is_removed_only_from_the_generated_project(self):
        commands = []
        class Operations:
            def execute(self, args):
                commands.append(args)
                return IDENTITY + '\n' if args[:2] == ['docker', 'ps'] else ''
        cleanup(Operations(), ['docker', 'compose', '--project-name', PROJECT], PROJECT)
        self.assertEqual(commands[-2][-1], 'label=com.docker.compose.project=' + PROJECT)
        self.assertEqual(commands[-1], ['docker', 'rm', '--force', IDENTITY])
        self.assertIn('--remove-orphans', commands[0])

    def test_failed_compose_teardown_still_cleans_owned_oneoffs(self):
        commands = []
        class Operations:
            def execute(self, args):
                commands.append(args)
                if 'down' in args:
                    raise RuntimeError('teardown failed')
                return IDENTITY if args[:2] == ['docker', 'ps'] else ''
        with self.assertRaisesRegex(RuntimeError, 'teardown failed'):
            cleanup(Operations(), ['docker', 'compose'], PROJECT)
        self.assertEqual(commands[-1], ['docker', 'rm', '--force', IDENTITY])

    def test_live_projects_and_unexpected_container_names_are_rejected(self):
        commands = []
        class Operations:
            def execute(self, args):
                commands.append(args)
                return 'arkhiv-pilot-api-1' if args[:2] == ['docker', 'ps'] else ''
        with self.assertRaises(ValueError):
            cleanup(Operations(), ['docker', 'compose'], 'arkhiv-pilot')
        self.assertEqual(commands, [])
        with self.assertRaises(ValueError):
            cleanup(Operations(), ['docker', 'compose'], PROJECT)
        self.assertFalse(any(args[:2] == ['docker', 'rm'] for args in commands))

    def test_compose_timeout_leaves_a_separate_cleanup_budget(self):
        deadlines = []
        class Operations:
            def execute(self, args):
                deadlines.append(self.deadline)
                if 'down' in args:
                    raise TimeoutError('compose exhausted its budget')
                return IDENTITY if args[:2] == ['docker', 'ps'] else ''
        with patch('restore_drill.time.monotonic', return_value=100), self.assertRaises(TimeoutError):
            cleanup(Operations(), ['docker', 'compose'], PROJECT)
        self.assertEqual(deadlines, [130, 160, 160])
