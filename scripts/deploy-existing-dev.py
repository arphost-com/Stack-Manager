#!/usr/bin/env python3
"""Update only an existing, verified DEV controller; never initialize data stores."""
import hashlib
import grp
import io
import json
import os
from pathlib import Path
import pwd
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import time


def metadata(path):
    s = path.lstat()
    if stat.S_ISLNK(s.st_mode):
        raise RuntimeError('symlink in deployment path')
    if os.listxattr(path):
        raise RuntimeError('unapproved ACL/xattrs in deployment path')
    return [stat.S_IFMT(s.st_mode), s.st_uid, s.st_gid, stat.S_IMODE(s.st_mode)]


def run(args, cwd=None):
    r = subprocess.run(args, cwd=cwd, capture_output=True, text=True, timeout=300)
    if r.returncode:
        raise RuntimeError('deployment command failed: ' + args[0])
    return r.stdout


def validate_managed_name(name):
    parts = Path(name).parts
    if not parts or Path(name).is_absolute() or '..' in parts:
        raise RuntimeError('invalid managed path')
    if parts[0] in ('.stack-manager', '.git', '.claude', '.codex', 'node_modules') or Path(name).name == '.env':
        raise RuntimeError('runtime/private path cannot be promoted')


def tracked(source, revision):
    raw = subprocess.check_output(['git', '-C', str(source), 'ls-tree', '-rz', revision])
    result = {}
    for entry in raw.split(b'\0'):
        if not entry:
            continue
        info, name = entry.split(b'\t', 1)
        mode, kind, _ = info.decode().split()
        name = name.decode()
        if mode not in ('100644', '100755') or kind != 'blob':
            raise RuntimeError('unsupported source file kind')
        validate_managed_name(name)
        if name in ('AGENTS.md', 'CLAUDE.md', 'ACCESS.md', '.env'):
            continue
        result[name] = 0o775 if mode == '100755' else 0o664
    return result


def export_source(source):
    """Runner-side: send exact Git blobs, never the private checkout or its metadata."""
    source = Path(source).resolve()
    image = run(['docker', 'inspect', '--format', '{{.Image}}', 'stack-manager-server-1']).strip()
    baseline = run(['docker', 'image', 'inspect', '--format', '{{index .Config.Labels "org.opencontainers.image.revision"}}', image]).strip()
    run(['git', '-C', str(source), 'merge-base', '--is-ancestor', baseline, 'HEAD'])
    revision = run(['git', '-C', str(source), 'rev-parse', 'HEAD']).strip()
    files, old_files = tracked(source, revision), tracked(source, baseline)
    header = json.dumps({'revision': revision, 'baseline': baseline, 'files': files, 'old_files': old_files}).encode()
    with tarfile.open(fileobj=sys.stdout.buffer, mode='w|') as archive:
        def add(name, data):
            info = tarfile.TarInfo(name)
            info.size = len(data)
            archive.addfile(info, io.BytesIO(data))
        add('deployment-source.json', header)
        for name in sorted(files):
            data = subprocess.check_output(['git', '-C', str(source), 'show', revision + ':' + name])
            add('source/' + name, data)


def receive_source(stream, release_root, baseline):
    """Service-side: validate the stream and create only private, explicitly owned paths."""
    source = Path(tempfile.mkdtemp(prefix='source-', dir=release_root))
    try:
        with tarfile.open(fileobj=stream, mode='r|') as archive:
            first = archive.next()
            if first is None or first.name != 'deployment-source.json' or not first.isfile() or first.size > 1024 * 1024:
                raise RuntimeError('invalid deployment source header')
            header = json.load(archive.extractfile(first))
            revision = header.get('revision', '')
            if len(revision) != 40 or any(c not in '0123456789abcdef' for c in revision) or header.get('baseline') != baseline:
                raise RuntimeError('deployment source revision/baseline mismatch')
            files, old_files = header['files'], header['old_files']
            if not isinstance(files, dict) or not isinstance(old_files, dict) or not files or len(files) > 10000 or len(old_files) > 10000:
                raise RuntimeError('invalid deployment file manifest')
            for name, mode in {**files, **old_files}.items():
                validate_managed_name(name)
                if name in ('AGENTS.md', 'CLAUDE.md', 'ACCESS.md') or mode not in (0o664, 0o775):
                    raise RuntimeError('invalid deployment file metadata')
            seen = set()
            size = 0
            for member in iter(archive.next, None):
                name = member.name.removeprefix('source/')
                if not member.name.startswith('source/') or name not in files or name in seen or not member.isfile() or member.size > 32 * 1024 * 1024:
                    raise RuntimeError('unexpected deployment source entry')
                size += member.size
                if size > 128 * 1024 * 1024:
                    raise RuntimeError('deployment source exceeds size limit')
                dest = source / name
                parents_safe(dest, source)
                missing = []
                parent = dest.parent
                while parent != source and not parent.exists():
                    missing.append(parent)
                    parent = parent.parent
                for parent in reversed(missing):
                    parent.mkdir(mode=0o700)
                    os.chmod(parent, stat.S_IRWXU)
                with dest.open('xb') as output:
                    shutil.copyfileobj(archive.extractfile(member), output)
                os.chmod(dest, stat.S_IRUSR | stat.S_IWUSR)
                seen.add(name)
            if seen != set(files):
                raise RuntimeError('deployment source is incomplete')
        return source, files, old_files
    except BaseException:
        shutil.rmtree(source)
        raise


def parents_safe(path, root):
    for parent in path.parents:
        if parent == root:
            return
        if (parent.exists() or parent.is_symlink()) and not stat.S_ISDIR(parent.lstat().st_mode):
            raise RuntimeError('non-directory or symlink collision')
    raise RuntimeError('path escaped deployment root')


def promote(source, target, files, old_files, fail_after=None):
    """Normalize only new owned files; atomically replace and restore exact old metadata."""
    owner = os.getuid(), os.getgid()
    original = {}
    staging = Path(tempfile.mkdtemp(prefix='promotion-', dir=target / '.stack-manager' / 'deploy-releases'))
    changed = []
    created_dirs = []
    try:
        for name in set(files) | set(old_files):
            validate_managed_name(name)
            dest = target / name
            parents_safe(dest, target)
            if dest.exists() or dest.is_symlink():
                m = metadata(dest)
                expected_mode = old_files.get(name, files.get(name))
                if m != [stat.S_IFREG, *owner, expected_mode]:
                    raise RuntimeError('installed file metadata drift: ' + name)
                backup = staging / 'backup' / name
                backup.parent.mkdir(parents=True, exist_ok=True)
                shutil.copy2(dest, backup)
                original[name] = m
        manifest = {
            'schema_version': 1,
            'owner': {
                'uid': owner[0], 'gid': owner[1],
                'account': pwd.getpwuid(owner[0]).pw_name,
                'group': grp.getgrgid(owner[1]).gr_name,
            },
            'required_acl_xattrs': [],
            'files': original,
        }
        (staging / 'manifest.json').write_text(json.dumps(manifest, sort_keys=True))
        os.chmod(staging / 'manifest.json', 0o600)
        for name, mode in files.items():
            src = source / name
            if not stat.S_ISREG(src.lstat().st_mode):
                raise RuntimeError('unsupported source file kind')
            dest = target / name
            missing = []
            parent = dest.parent
            while not parent.exists():
                missing.append(parent)
                parent = parent.parent
            for parent in reversed(missing):
                parent.mkdir(mode=0o755)
                # Public application source directory: traversal/read is intentional; no runtime secrets.
                os.chmod(parent, 0o755)  # nosemgrep: python.lang.security.audit.insecure-file-permissions.insecure-file-permissions
                created_dirs.append(parent)
            temp = staging / 'new' / name
            temp.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(src, temp)
            os.chmod(temp, mode)
            os.replace(temp, dest)
            changed.append(name)
            if metadata(dest) != [stat.S_IFREG, *owner, mode]:
                raise RuntimeError('promoted metadata mismatch')
            if fail_after is not None and len(changed) >= fail_after:
                raise RuntimeError('injected partial promotion failure')
        for name in old_files.keys() - files.keys():
            if (target / name).exists():
                (target / name).unlink()
                changed.append(name)
        return staging, original, changed, created_dirs
    except BaseException:
        restore(target, staging, original, changed, created_dirs)
        raise


def restore(target, staging, original, changed, created_dirs):
    for name in reversed(changed):
        dest = target / name
        if name in original:
            saved = staging / 'backup' / name
            temp = staging / 'restore'
            shutil.copy2(saved, temp)
            os.replace(temp, dest)
            if metadata(dest) != original[name]:
                raise RuntimeError('rollback metadata mismatch')
        elif dest.exists():
            dest.unlink()
    for directory in reversed(created_dirs):
        directory.rmdir()


def containers():
    names = ['stack-manager-server-1', 'stack-manager-web-1', 'stack-manager-mariadb-1', 'stack-manager-redis-1']
    # Never inspect container environments or raw full Docker objects.
    fmt = '{{.Name}}|{{.Id}}|{{.Image}}|{{.State.Status}}|{{if .State.Health}}{{.State.Health.Status}}{{end}}|{{range .Mounts}}{{.Type}}:{{.Source}}:{{.Destination}};{{end}}'
    return {line.split('|')[0].lstrip('/'): line.split('|')[1:] for line in run(['docker', 'inspect', '--format', fmt, *names]).splitlines()}


def contract(target, account):
    uid, gid = account.pw_uid, account.pw_gid
    expected = {
        target.parent: [stat.S_IFDIR, uid, gid, 0o755],
        target: [stat.S_IFDIR, uid, gid, 0o775],
        target / '.env': [stat.S_IFREG, uid, gid, 0o600],
    }
    for name in ('', 'hooks', 'backups', 'docker-config', 'jobs'):
        expected[target / '.stack-manager' / name] = [stat.S_IFDIR, uid, gid, 0o2770]
    expected[target / '.stack-manager/ssl'] = [stat.S_IFDIR, uid, gid, 0o2775]
    expected[target / '.stack-manager/mariadb'] = [stat.S_IFDIR, 999, 999, 0o2770]
    expected[target / '.stack-manager/redis'] = [stat.S_IFDIR, 999, gid, 0o2770]
    for path, value in expected.items():
        if metadata(path) != value:
            raise RuntimeError('approved DEV metadata baseline mismatch: ' + str(path))
    siblings = {str(p): metadata(p) for p in target.parent.iterdir() if p != target and not p.is_symlink()}
    return expected, siblings


def prepare_release_root(path, owner):
    if not path.exists():
        path.mkdir(mode=0o700)
        # The state parent has setgid: normalize only this newly created path.
        # Owner-only private directory. The rule uses a numeric >= 0650 heuristic,
        # which incorrectly flags secure 0700 directory permissions.
        os.chmod(path, 0o700)  # nosemgrep: python.lang.security.audit.insecure-file-permissions.insecure-file-permissions
    if metadata(path) != [stat.S_IFDIR, *owner, 0o700]:
        raise RuntimeError('release directory metadata drift')


def main():
    if sys.argv[1] == '--export-source':
        export_source(sys.argv[2])
        return
    streamed = sys.argv[1] == '--receive-source'
    source = None if streamed else Path(sys.argv[1]).resolve()
    target = Path(os.environ['DEPLOY_PATH']).absolute()
    if str(target) != '/home/debian/docker/stack-manager':
        raise RuntimeError('this deployment contract is Docker02 DEV only')
    account = pwd.getpwnam('debian')
    if (os.getuid(), os.getgid()) != (account.pw_uid, account.pw_gid):
        raise RuntimeError('deployment must run as the documented debian service account')
    expected, siblings = contract(target, account)
    config_hash = hashlib.sha256((target / '.env').read_bytes()).hexdigest()
    before = containers()
    if any(v[2:4] != ['running', 'healthy'] for v in before.values()):
        raise RuntimeError('existing controller/data service is not healthy')
    baseline = run(['docker', 'image', 'inspect', '--format', '{{index .Config.Labels "org.opencontainers.image.revision"}}', before['stack-manager-server-1'][1]]).strip()
    if not baseline or baseline == '<no value>':
        raise RuntimeError('deployed source baseline is unknown')
    release_root = target / '.stack-manager/deploy-releases'
    prepare_release_root(release_root, (account.pw_uid, account.pw_gid))
    pipeline = os.environ['CI_PIPELINE_ID']
    if not pipeline.isdigit():
        raise RuntimeError('invalid pipeline ID')
    compose = ['docker', 'compose', '--project-name', 'stack-manager', '--env-file', '.env', '-f', 'docker-compose.yml']
    old_images = {service: before['stack-manager-' + service + '-1'][1] for service in ('server', 'web')}
    for service, image in old_images.items():
        run(['docker', 'tag', image, 'stack-manager-rollback-' + service + ':' + pipeline])
    staging = None
    try:
        if streamed:
            source, files, old_files = receive_source(sys.stdin.buffer, release_root, baseline)
        else:
            run(['git', '-C', str(source), 'merge-base', '--is-ancestor', baseline, 'HEAD'])
            files, old_files = tracked(source, 'HEAD'), tracked(source, baseline)
        staging, original, changed, dirs = promote(source, target, files, old_files)
        # Both application components changed in this release (Go and web version metadata).
        # Reuse the exact pipeline-built/scanned images; no live rebuild or dependency restart.
        for service in ('server', 'web'):
            run(['docker', 'tag', os.environ['CI_COMPOSE_PROJECT_NAME'] + '-' + service, 'stack-manager-' + service])
        run(compose + ['up', '-d', '--no-deps', '--no-build', 'server', 'web'], target)
        after = containers()
        for _ in range(60):
            if all(after['stack-manager-' + service + '-1'][2:4] == ['running', 'healthy'] for service in ('server', 'web')):
                break
            time.sleep(2)
            after = containers()
        else:
            raise RuntimeError('application containers did not become healthy')
        if any(before[name] != after[name] for name in ('stack-manager-mariadb-1', 'stack-manager-redis-1')):
            raise RuntimeError('database/cache identity, state or mount drift')
        if hashlib.sha256((target / '.env').read_bytes()).hexdigest() != config_hash:
            raise RuntimeError('runtime configuration changed')
        if any(metadata(path) != value for path, value in {**expected, **{Path(k): v for k,v in siblings.items()}}.items()):
            raise RuntimeError('shared-root or persistent metadata drift')
        print('Scoped DEV deployment promoted pipeline images; configuration, DB/cache identity/mounts and metadata preserved.')
        print('Rollback code manifest: ' + str(staging / 'manifest.json'))
    except BaseException:
        if staging is not None:
            restore(target, staging, original, changed, dirs)
            for service, image in old_images.items():
                run(['docker', 'tag', image, 'stack-manager-' + service])
            run(compose + ['up', '-d', '--no-deps', '--no-build', 'server', 'web'], target)
            print('Application code/images rolled back; independent health readback is still required.')
        raise
    finally:
        if streamed and source is not None:
            shutil.rmtree(source)


if __name__ == '__main__':
    try:
        main()
    except Exception as error:
        # No command stderr, environment, credential file contents or exception payloads.
        print('DEV deployment held: ' + (str(error) if isinstance(error, RuntimeError) else type(error).__name__))
        raise SystemExit(1)
