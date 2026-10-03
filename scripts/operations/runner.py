#!/usr/bin/env python3
"""Bounded homelab orchestration; document/catalog mutations remain in the Go CLI."""
import argparse
import datetime as dt
import fcntl
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
import time
import urllib.request

JOBS = ('audit', 'backup', 'monitor', 'retention', 'update', 'restore')


def atomic(path, value):
    path = Path(path)
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
    fd, name = tempfile.mkstemp(dir=path.parent, prefix='.write-')
    try:
        with os.fdopen(fd, 'w') as f:
            json.dump(value, f, indent=2)
            f.write('\n')
            f.flush()
            os.fsync(f.fileno())
        os.replace(name, path)
        directory = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(directory)
        finally:
            os.close(directory)
    finally:
        if os.path.exists(name):
            os.unlink(name)


def digest(path):
    h = hashlib.sha256()
    with Path(path).open('rb') as f:
        for b in iter(lambda: f.read(1024 * 1024), b''):
            h.update(b)
    return h.hexdigest()


def safe_tree(root):
    root = Path(root)
    if root.is_symlink() or not root.is_dir():
        raise ValueError('unsafe archive tree')
    files = {}
    for path in root.rglob('*'):
        if path.is_symlink():
            raise ValueError('symlink in backup')
        if path.is_dir():
            continue
        if not path.is_file():
            raise ValueError('special file in backup')
        files[str(path.relative_to(root))] = digest(path)
    return files


def verify_backup(root):
    root = Path(root)
    if root.is_symlink():
        raise ValueError('symlink backup root')
    manifest_path = root / 'complete.json'
    if manifest_path.is_symlink() or manifest_path.stat().st_size > 16 * 1024 * 1024:
        raise ValueError('unsafe backup manifest')
    m = json.loads(manifest_path.read_text())
    if m.get('format') != 1 or m.get('complete') is not True or not isinstance(m.get('files'), dict):
        raise ValueError('incomplete backup')
    for name in m['files']:
        if not isinstance(name, str) or Path(name).is_absolute() or '..' in Path(name).parts:
            raise ValueError('unsafe manifest path')
    dump = root / 'catalog.dump'
    if dump.is_symlink() or not dump.is_file() or dump.stat().st_size != m['catalog_size'] or digest(dump) != m['catalog_sha256']:
        raise ValueError('catalog backup integrity mismatch')
    if safe_tree(root / 'archive') != m['files']:
        raise ValueError('archive backup integrity mismatch')
    return {'files': len(m['files']), 'catalog_sha256': m['catalog_sha256'], 'catalog_size': m['catalog_size']}


def retention_plan(root, policy, now=None):
    """Only named, completed, verified job snapshots can become deletion candidates."""
    root = Path(root)
    now = now or dt.datetime.now(dt.timezone.utc)
    snapshots, held = [], []
    for path in root.iterdir():
        if not re.fullmatch(r'auto-\d{8}T\d{6}Z', path.name) or path.is_symlink() or not path.is_dir():
            held.append(path.name)
            continue
        try:
            verify_backup(path)
            stamp = dt.datetime.strptime(path.name[5:], '%Y%m%dT%H%M%SZ').replace(tzinfo=dt.timezone.utc)
        except (ValueError, OSError, KeyError, TypeError):
            held.append(path.name)
            continue
        snapshots.append((stamp, path.name))
    snapshots.sort(reverse=True)
    keep = set(policy.get('pinned', []))
    if snapshots:
        keep.add(snapshots[0][1])
    # Keep the newest snapshot in each configured day/week/month bucket.
    for mode, count in [('daily', 7), ('weekly', 4), ('monthly', 6)]:
        count = policy.get(mode, count)
        if not isinstance(count, int) or not 1 <= count <= 365:
            raise ValueError('retention count outside bounds')
        buckets = set()
        for stamp, name in snapshots:
            bucket = stamp.strftime('%Y-%m-%d') if mode == 'daily' else stamp.strftime('%G-%V') if mode == 'weekly' else stamp.strftime('%Y-%m')
            if bucket not in buckets and len(buckets) < count:
                buckets.add(bucket)
                keep.add(name)
    return {'keep': sorted(keep), 'hold': sorted(held), 'remove': [name for _, name in snapshots if name not in keep], 'dry_run': True}


class Runner:
    def __init__(self, config, execute=None):
        self.c = config
        for key in ('project', 'archive', 'staging', 'state', 'backup_root'):
            value = config.get(key, '')
            if not value or not Path(value).is_absolute() or '..' in Path(value).parts:
                raise ValueError('absolute managed paths required')
        if not re.fullmatch(r'[a-zA-Z0-9_.-]+', config.get('backup_host', '')):
            raise ValueError('invalid backup host')
        self.state = Path(config['state'])
        self.state.mkdir(parents=True, exist_ok=True, mode=0o700)
        self.execute = execute or self.command
        self.deadline = time.monotonic() + min(int(config.get('timeout_seconds', 900)), 3600)

    def command(self, args):
        remaining = self.deadline - time.monotonic()
        if remaining <= 0:
            raise TimeoutError('job deadline reached')
        try:
            r = subprocess.run(args, capture_output=True, timeout=remaining, check=False)
        except subprocess.TimeoutExpired:
            raise TimeoutError('job deadline reached') from None
        if r.returncode:
            # Raw tool errors may contain private DB connection settings.
            raise RuntimeError('operation failed: ' + Path(args[0]).name)
        if len(r.stdout) > 16 * 1024 * 1024:
            raise RuntimeError('operation output exceeded bound')
        return r.stdout.decode()

    def cli(self, *args):
        return self.execute(['docker', 'compose', '-f', self.c['project'] + '/compose.json', '--project-name', self.c.get('compose_project', 'riot-grrrl-pilot'), '--profile', 'operator', 'run', '--rm', '-T', 'operator', *args])

    def remote(self, *args):
        return self.execute(['ssh', '-o', 'BatchMode=yes', self.c['backup_host'], shlex.join(args)])

    def space(self, path):
        free = shutil.disk_usage(path).free
        if free < self.c.get('minimum_free_bytes', 2 * 1024**3):
            raise RuntimeError('low disk capacity')
        return free

    def run(self, job):
        if job not in JOBS:
            raise ValueError('unknown job')
        started = dt.datetime.now(dt.timezone.utc)
        previous_path = self.state / (job + '.json')
        previous = json.loads(previous_path.read_text()) if previous_path.exists() else {}
        receipt = {'job': job, 'started': started.isoformat(), 'revision': self.c.get('revision', 'unknown'), 'status': 'running', 'last_success': previous.get('last_success')}
        with (self.state / '.operation.lock').open('a') as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                receipt.update(status='lock-skipped', finished=dt.datetime.now(dt.timezone.utc).isoformat())
                atomic(self.state / 'last-lock-skipped.json', receipt)
                return receipt
            atomic(previous_path, receipt)
            try:
                result = getattr(self, job)()
                receipt.update(status='success', result=result, last_success=dt.datetime.now(dt.timezone.utc).isoformat())
            except Exception as error:
                receipt.update(status='failed', error=str(error) if isinstance(error, (RuntimeError, TimeoutError, ValueError)) else type(error).__name__)
            receipt['finished'] = dt.datetime.now(dt.timezone.utc).isoformat()
            receipt['next_run'] = self.c.get('schedules', {}).get(job, 'configured systemd timer')
            atomic(previous_path, receipt)
            atomic(self.state / 'history' / (started.strftime('%Y%m%dT%H%M%S%fZ') + '-' + job + '.json'), receipt)
            return receipt

    def audit(self):
        summary = json.loads(self.cli('audit'))
        return {'originals': 'verified', 'free_bytes': self.space(self.c['archive']), **summary}

    def backup(self):
        self.space(self.c['staging'])
        remote_free = int(self.remote('python3', self.c['remote_script'], 'free', '--root', self.c['backup_root']).strip())
        if remote_free < self.c.get('minimum_free_bytes', 2 * 1024**3):
            raise RuntimeError('low backup capacity')
        name = 'auto-' + dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%SZ')
        self.cli('backup', '--destination', '/backups/' + name)
        local = Path(self.c['staging']) / name
        summary = verify_backup(local)
        partial = self.c['backup_root'] + '/.incoming-' + name
        self.remote('mkdir', '-m', '700', partial)
        self.execute(['rsync', '-a', '--protect-args', '--', str(local) + '/', self.c['backup_host'] + ':' + partial + '/'])
        self.remote('python3', self.c['remote_script'], 'verify', '--root', partial)
        final = self.c['backup_root'] + '/' + name
        self.remote('python3', self.c['remote_script'], 'promote', '--root', partial, '--destination', final)
        atomic(self.state / 'latest-backup.json', {'name': name, 'destination': final, **summary})
        return {'snapshot': name, 'destination': final, 'verified_on_host': self.c['backup_host'], **summary}

    def update(self):
        results = {}
        for conf in self.c.get('collections', []):
            args = ['refresh', '--scheduled', '--config', conf]
            if self.c.get('force_source_checks', False):
                args.append('--force-source-check')
            results[Path(conf).stem] = json.loads(self.cli(*args))
        return results

    def retention(self):
        value = self.remote('python3', self.c['remote_script'], 'retain', '--root', self.c['backup_root'], '--policy', json.dumps(self.c.get('retention', {})))
        return json.loads(value)

    def restore(self):
        return json.loads(self.execute(['python3', self.c['restore_script'], self.c['config_path']]))

    def monitor(self):
        problems = []
        readiness = self.c.get('public_readiness_url')
        if readiness:
            if not readiness.startswith(('https://', 'http://127.0.0.1:')):
                raise ValueError('invalid public readiness destination')
            try:
                request = urllib.request.Request(readiness, headers={'User-Agent': 'Arkhiv/0.1 (Riot Grrrl Archive; contact info@subcult.tv)', 'Accept': 'application/json'})
                with urllib.request.urlopen(request, timeout=5) as response:
                    ready = json.loads(response.read(16384))
                    healthy = response.status == 200 and ready.get('status') == 'ok' and ready.get('public') is True and ready.get('revision') == self.c['public_revision']
                if not healthy:
                    problems.append({'job': 'public-reader', 'status': 'unexpected-readiness', 'stale': False})
            except (OSError, ValueError):
                problems.append({'job': 'public-reader', 'status': 'unavailable', 'stale': False})
        for job, max_age in self.c.get('stale_seconds', {}).items():
            path = self.state / (job + '.json')
            data = json.loads(path.read_text()) if path.exists() else {}
            success = data.get('last_success')
            stale = not success or (dt.datetime.now(dt.timezone.utc) - dt.datetime.fromisoformat(success)).total_seconds() > max_age
            if data.get('status') == 'failed' or stale:
                problems.append({'job': job, 'status': data.get('status', 'never-run'), 'stale': stale})
        signature = hashlib.sha256(json.dumps(problems, sort_keys=True).encode()).hexdigest()
        notice_path = self.state / 'notification.json'
        old = json.loads(notice_path.read_text()) if notice_path.exists() else {}
        if signature != old.get('signature') or (self.c.get('notify_url') and not old.get('sent')):
            url = self.c.get('notify_url')
            if url:
                if not url.startswith(('https://', 'http://127.0.0.1', 'http://10.0.0.')):
                    raise ValueError('invalid notification destination')
                req = urllib.request.Request(url, data=json.dumps({'archive': 'riot-grrrl', 'problems': problems, 'recovery': not problems}).encode(), headers={'Title': 'Riot Grrrl preservation status', 'Content-Type': 'application/json'}, method='POST')
                with urllib.request.urlopen(req, timeout=15) as response:
                    if response.status not in (200, 201, 202):
                        raise RuntimeError('notification rejected')
            atomic(notice_path, {'signature': signature, 'sent': bool(url), 'problems': problems})
        return {'problems': problems, 'notification_configured': bool(self.c.get('notify_url'))}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('action', choices=JOBS + ('verify', 'promote', 'retain', 'free'))
    parser.add_argument('--config')
    parser.add_argument('--root')
    parser.add_argument('--destination')
    parser.add_argument('--policy', default='{}')
    args = parser.parse_args()
    if args.action == 'verify':
        result = verify_backup(args.root)
    elif args.action == 'promote':
        verify_backup(args.root)
        if Path(args.destination).exists():
            raise ValueError('destination exists')
        os.rename(args.root, args.destination)
        result = {'complete': True}
    elif args.action == 'retain':
        result = retention_plan(args.root, json.loads(args.policy))
    elif args.action == 'free':
        print(shutil.disk_usage(args.root).free)
        return 0
    else:
        config = json.loads(Path(args.config).read_text())
        config['config_path'] = str(Path(args.config).resolve())
        runner = Runner(config)
        result = runner.run(args.action)
    print(json.dumps(result))
    return 1 if result.get('status') == 'failed' else 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as error:
        print(json.dumps({'status': 'failed', 'error': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
