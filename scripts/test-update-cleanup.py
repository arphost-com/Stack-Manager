#!/usr/bin/env python3
"""Exercise the full CLI against fake Docker; never prune a live daemon."""
import os
from pathlib import Path
import subprocess
import tempfile
import textwrap
import unittest

ROOT = Path(__file__).resolve().parents[1]


class UpdateCleanupTest(unittest.TestCase):
    def run_cli(self, failure='', flags=(), command='update', hook=False, age='168', enabled='1'):
        with tempfile.TemporaryDirectory() as tmp:
            base = Path(tmp)
            root = base / 'stacks'
            project = root / ('stack-manager' if failure == 'controller' else 'app')
            project.mkdir(parents=True)
            (project / 'compose.yml').write_text('services:\n  app:\n    image: example.invalid/app:test\n')
            trace = base / 'trace'
            env = dict(os.environ, HOME=tmp, PATH=f'{base}:{os.environ["PATH"]}',
                       CLEANUP_TRACE=str(trace), CLEANUP_FAILURE=failure,
                       UPDATE_CLEANUP_ENABLED=enabled, UPDATE_CLEANUP_MIN_AGE_HOURS=age,
                       CLEANUP_SAMPLE=str(base / 'sample'))
            scripts = {
                'timeout': '#!/bin/sh\nshift\nexec "$@"\n',
                'sleep': '#!/bin/sh\nexit 0\n',
                'docker': r'''#!/bin/sh
printf '%s\n' "$*" >> "$CLEANUP_TRACE"
case "$1" in
 ps) case "$*" in *'{{.ID}}'*) printf 'abc123\n';; *) printf 'app\n';; esac;;
 inspect)
 if [ "$CLEANUP_FAILURE" = inspect ]; then exit 1; fi
 if [ "$CLEANUP_FAILURE" = health ]; then printf 'abc123|/app-1|restarting|1|3|unhealthy|unless-stopped\n'
 elif [ "$CLEANUP_FAILURE" = starting ]; then printf 'abc123|/app-1|running|0|0|starting|unless-stopped\n'
 elif [ "$CLEANUP_FAILURE" = restarts ]; then
 n=0; [ ! -f "$CLEANUP_SAMPLE" ] || n=$(cat "$CLEANUP_SAMPLE")
 n=$((n + 1)); printf '%s' "$n" > "$CLEANUP_SAMPLE"
 printf 'abc123|/app-1|running|0|%s|healthy|unless-stopped\n' "$n"
 elif [ "$CLEANUP_FAILURE" = init ]; then printf 'abc123|/init-1|exited|0|0|none|no\n'
 else printf 'abc123|/app-1|running|0|0|healthy|unless-stopped\n'; fi;;
 compose) case "$*" in
 *pull*) [ "$CLEANUP_FAILURE" != pull ] || exit 1;;
 *'up -d'*) [ "$CLEANUP_FAILURE" != up ] || exit 1;; esac;;
 image|builder) [ "$CLEANUP_FAILURE" != "$1" ] || exit 1
 printf 'Total reclaimed space: 123MB\n';;
esac
''',
                # Keep the established bash -lc runner inside the fake PATH.
                'bash': '#!/bin/sh\nif [ "$1" = -lc ]; then shift; exec /bin/bash -c "$1"; fi\nexec /bin/bash "$@"\n',
            }
            for name, script in scripts.items():
                path = base / name
                path.write_text(script)
                path.chmod(0o700)
            if hook:
                hooks = root / '.stack-manager/hooks'
                hooks.mkdir(parents=True)
                path = hooks / 'post-update_app.sh'
                path.write_text('#!/bin/sh\nprintf "hook\\n" >> "$CLEANUP_TRACE"\n' +
                                ('exit 1\n' if failure == 'hook' else ''))
                path.chmod(0o700)
            result = subprocess.run(['/bin/bash', str(ROOT / 'stack-manager.sh'), '--root', str(root),
                                     '--no-log', *flags, command], env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=15)
            return result, trace.read_text() if trace.exists() else ''

    def test_verified_update_reports_space_without_volume_pruning(self):
        result, calls = self.run_cli(flags=('--prune',))
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('image prune --force --filter until=168h', calls)
        self.assertIn('builder prune --all --force --filter until=168h --keep-storage 1GB', calls)
        self.assertIn('Total reclaimed space: 123MB', result.stdout)
        self.assertNotIn('volume prune', calls)
        self.assertNotIn('network prune', calls)
        self.assertLess(calls.index('inspect'), calls.index('image prune'))

    def test_failed_update_or_verification_never_prunes(self):
        for failure in ('pull', 'up', 'health', 'inspect', 'controller', 'starting', 'restarts'):
            with self.subTest(failure=failure):
                _, calls = self.run_cli(failure, flags=('--prune',))
                self.assertNotIn('prune', calls)

    def test_disabled_invalid_dry_run_and_non_update_actions(self):
        cases = [dict(flags=('--no-update-cleanup',)), dict(enabled='false'),
                 dict(age='0'), dict(enabled='invalid'), dict(flags=('--dry-run',)),
                 dict(command='pull'), dict(command='up')]
        for args in cases:
            with self.subTest(args=args):
                result, calls = self.run_cli(**args)
                self.assertNotIn('prune', calls)
                if '--dry-run' in args.get('flags', ()):
                    self.assertIn('[dry-run] docker builder prune', result.stdout)

    def test_hook_override_is_preserved(self):
        result, calls = self.run_cli(hook=True)
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('hook\n', calls)
        self.assertFalse(any(line.startswith('compose ') for line in calls.splitlines()))
        self.assertIn('builder prune', calls)
        _, failed = self.run_cli(failure='hook', hook=True)
        self.assertNotIn('prune', failed)

    def test_pipeline_preserves_cleanup_configuration(self):
        pipeline = (ROOT / '.gitlab-ci.yml').read_text()
        deployment = pipeline.split('deploy:docker02:', 1)[1].split('smoke:docker02:', 1)[0]
        self.assertIn('deploy-existing-dev.py', deployment)
        self.assertNotIn('install -m 600', deployment)
        self.assertNotIn('chown', deployment)

    def test_completed_init_service_remains_supported(self):
        result, calls = self.run_cli(failure='init')
        self.assertEqual(result.returncode, 0, result.stdout)
        self.assertIn('builder prune', calls)

    def test_cleanup_failure_warns_without_failing_update(self):
        for failure in ('image', 'builder'):
            result, calls = self.run_cli(failure)
            self.assertEqual(result.returncode, 0, result.stdout)
            self.assertIn('incomplete; update succeeded', result.stdout)
            if failure == 'image':
                self.assertNotIn('builder prune', calls)


if __name__ == '__main__':
    unittest.main()
