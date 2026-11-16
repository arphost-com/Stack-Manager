#!/usr/bin/env python3
"""Metadata/rollback regressions against isolated filesystem fixtures only."""
import importlib.util
import grp
import json
import io
import os
from pathlib import Path
import stat
import pwd
import tempfile
import tarfile
import unittest
from unittest import mock

spec = importlib.util.spec_from_file_location('deploy', Path(__file__).with_name('deploy-existing-dev.py'))
deploy = importlib.util.module_from_spec(spec)
spec.loader.exec_module(deploy)


class PromotionTest(unittest.TestCase):
    def setUp(self):
        if not hasattr(os, 'listxattr'):
            patch = mock.patch.object(os, 'listxattr', return_value=[], create=True)
            patch.start()
            self.addCleanup(patch.stop)
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.base = Path(self.tmp.name)
        self.source = self.base / 'runner-source'
        self.target = self.base / 'service-target'
        self.source.mkdir()
        self.target.mkdir()
        (self.target / '.stack-manager/deploy-releases').mkdir(parents=True)
        (self.source / 'old.txt').write_text('new')
        (self.target / 'old.txt').write_text('old')
        # Synthetic fixture reproduces the approved existing group-writable code baseline.
        os.chmod(self.target / 'old.txt', 0o664)  # nosemgrep: python.lang.security.audit.insecure-file-permissions.insecure-file-permissions
        os.chmod(self.source / 'old.txt', 0o600)  # donor mode must not leak
        (self.source / 'new').mkdir()
        (self.source / 'new/script.sh').write_text('new script')
        os.chmod(self.source / 'new/script.sh', 0o600)
        self.files = {'old.txt': 0o664, 'new/script.sh': 0o775}
        self.old = {'old.txt': 0o664}

    def test_explicit_new_metadata_and_exact_rollback(self):
        initial = deploy.metadata(self.target / 'old.txt')
        staging, original, changed, dirs = deploy.promote(self.source, self.target, self.files, self.old)
        manifest_path = staging / 'manifest.json'
        manifest = json.loads(manifest_path.read_text())
        self.assertEqual(manifest['files']['old.txt'], initial)
        self.assertEqual(manifest['owner'], {
            'uid': os.getuid(), 'gid': os.getgid(),
            'account': pwd.getpwuid(os.getuid()).pw_name,
            'group': grp.getgrgid(os.getgid()).gr_name,
        })
        self.assertEqual(manifest['required_acl_xattrs'], [])
        self.assertEqual(stat.S_IMODE(manifest_path.stat().st_mode), 0o600)
        self.assertEqual((self.target / 'old.txt').read_text(), 'new')
        self.assertEqual(deploy.metadata(self.target / 'new/script.sh'), [stat.S_IFREG, os.getuid(), os.getgid(), 0o775])
        self.assertEqual(deploy.metadata(self.target / 'new')[-1], 0o755)
        deploy.restore(self.target, staging, original, changed, dirs)
        self.assertEqual((self.target / 'old.txt').read_text(), 'old')
        self.assertEqual(deploy.metadata(self.target / 'old.txt'), initial)
        self.assertFalse((self.target / 'new').exists())

    def source_stream(self, entries, header=None):
        stream = io.BytesIO()
        header = header or {'revision': 'a' * 40, 'baseline': 'baseline',
                            'files': self.files, 'old_files': self.old}
        with tarfile.open(fileobj=stream, mode='w') as archive:
            data = json.dumps(header).encode()
            info = tarfile.TarInfo('deployment-source.json')
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
            for name, body, kind in entries:
                info = tarfile.TarInfo(name)
                info.type = kind
                info.uid = info.gid = 12345  # deliberately untrusted donor metadata
                info.mode = 0o777
                info.size = len(body)
                archive.addfile(info, io.BytesIO(body))
        stream.seek(0)
        return stream

    def test_streamed_source_promotes_without_importing_archive_ownership(self):
        entries = [('source/old.txt', b'committed old', tarfile.REGTYPE),
                   ('source/new/script.sh', b'committed script', tarfile.REGTYPE)]
        release = self.target / '.stack-manager/deploy-releases'
        source, files, old = deploy.receive_source(self.source_stream(entries), release, 'baseline')
        self.assertEqual(stat.S_IMODE(source.stat().st_mode), 0o700)
        self.assertEqual(stat.S_IMODE((source / 'new').stat().st_mode), 0o700)
        self.assertEqual(deploy.metadata(source / 'old.txt'), [stat.S_IFREG, os.getuid(), os.getgid(), 0o600])
        staging, original, changed, dirs = deploy.promote(source, self.target, files, old)
        self.assertEqual((self.target / 'old.txt').read_bytes(), b'committed old')
        self.assertEqual(deploy.metadata(self.target / 'new/script.sh'), [stat.S_IFREG, os.getuid(), os.getgid(), 0o775])
        deploy.restore(self.target, staging, original, changed, dirs)
        self.assertEqual((self.target / 'old.txt').read_text(), 'old')

    def test_invalid_source_stream_never_changes_live_files_or_retains_partial_source(self):
        entries = [('source/old.txt', b'new', tarfile.REGTYPE),
                   ('source/new/script.sh', b'script', tarfile.REGTYPE)]
        release = self.target / '.stack-manager/deploy-releases'
        for bad_entries, bad_baseline in [
            (entries, 'changed-baseline'), (entries[:1], 'baseline'),
            (entries + [entries[0]], 'baseline'),
            ([('source/old.txt', b'', tarfile.SYMTYPE)], 'baseline'),
            ([('source/../../escaped', b'bad', tarfile.REGTYPE)], 'baseline'),
            ([('source/.env', b'not a credential', tarfile.REGTYPE)], 'baseline'),
        ]:
            with self.subTest(entries=[e[0] for e in bad_entries], baseline=bad_baseline):
                before = set(release.iterdir())
                with self.assertRaises(RuntimeError):
                    deploy.receive_source(self.source_stream(bad_entries), release, bad_baseline)
                self.assertEqual(set(release.iterdir()), before)
                self.assertEqual((self.target / 'old.txt').read_text(), 'old')
                self.assertFalse((self.target / 'new').exists())

    @unittest.skipUnless(os.geteuid() == 0, 'foreign-owner fixture runs only in the isolated root test container')
    def test_foreign_runner_owner_never_leaks_to_installed_files(self):
        donor = self.source / 'old.txt'
        os.chmod(donor, 0o644)
        os.chown(donor, 12345, 12345)
        self.assertEqual(donor.stat().st_uid, 12345)
        deploy.promote(self.source, self.target, self.files, self.old)
        self.assertEqual((self.target / 'old.txt').stat().st_uid, os.getuid())
        self.assertEqual((self.target / 'old.txt').stat().st_gid, os.getgid())

    def test_partial_promotion_restores_original_bytes_and_metadata(self):
        initial = deploy.metadata(self.target / 'old.txt')
        with self.assertRaises(RuntimeError):
            deploy.promote(self.source, self.target, self.files, self.old, fail_after=1)
        self.assertEqual((self.target / 'old.txt').read_text(), 'old')
        self.assertEqual(deploy.metadata(self.target / 'old.txt'), initial)
        self.assertFalse((self.target / 'new').exists())

    def test_directory_and_symlink_collisions_fail_closed(self):
        for kind in ('directory', 'symlink', 'broken-parent'):
            with self.subTest(kind=kind):
                self.setUp()
                if kind == 'directory':
                    (self.target / 'new').mkdir()
                    (self.target / 'new/script.sh').mkdir()
                elif kind == 'symlink':
                    (self.target / 'old.txt').unlink()
                    (self.target / 'old.txt').symlink_to(self.source / 'old.txt')
                else:
                    (self.target / 'new').symlink_to(self.base / 'absent')
                with self.assertRaises(RuntimeError):
                    deploy.promote(self.source, self.target, self.files, self.old)

    def test_new_release_root_normalizes_inherited_setgid_only_on_creation(self):
        parent = self.base / 'setgid-state'
        parent.mkdir()
        os.chmod(parent, stat.S_ISGID | stat.S_IRWXU)
        release = parent / 'releases'
        owner = os.getuid(), os.getgid()
        deploy.prepare_release_root(release, owner)
        self.assertEqual(deploy.metadata(release), [stat.S_IFDIR, *owner, 0o700])
        # Deliberate isolated fixture drift: the contract must reject this mode.
        os.chmod(release, 0o755)  # nosemgrep: python.lang.security.audit.insecure-file-permissions.insecure-file-permissions
        with self.assertRaises(RuntimeError):
            deploy.prepare_release_root(release, owner)
        self.assertEqual(deploy.metadata(release)[-1], 0o755)

    def test_existing_metadata_drift_is_not_repaired(self):
        os.chmod(self.target / 'old.txt', 0o600)
        with self.assertRaises(RuntimeError):
            deploy.promote(self.source, self.target, self.files, self.old)
        self.assertEqual(deploy.metadata(self.target / 'old.txt')[-1], 0o600)
        self.assertEqual((self.target / 'old.txt').read_text(), 'old')

    def test_tracked_runtime_path_cannot_replace_persistent_data(self):
        db = self.target / '.stack-manager/mariadb'
        db.mkdir()
        (db / 'data').write_text('database')
        with self.assertRaises(RuntimeError):
            deploy.promote(self.source, self.target, {'.stack-manager/mariadb/data': 0o600}, {})
        self.assertEqual((db / 'data').read_text(), 'database')

    def test_persistent_data_and_unrelated_siblings_are_untouched(self):
        other = self.target.parent / 'other-project'
        other.mkdir()
        (other / 'data').write_text('other')
        db = self.target / '.stack-manager/mariadb'
        db.mkdir()
        (db / 'data').write_text('database')
        before = deploy.metadata(db), deploy.metadata(other)
        deploy.promote(self.source, self.target, self.files, self.old)
        self.assertEqual((db / 'data').read_text(), 'database')
        self.assertEqual((other / 'data').read_text(), 'other')
        self.assertEqual((deploy.metadata(db), deploy.metadata(other)), before)


if __name__ == '__main__':
    unittest.main()
