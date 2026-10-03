#!/usr/bin/env python3
"""Restore an independently verified backup into a new, temporary isolated stack."""
import datetime as dt
import json
from pathlib import Path
import re
import sys
import time

from runner import Runner, atomic, verify_backup


def drill(config):
    bounded = dict(config)
    budget = int(config.get("timeout_seconds", 900))
    if budget < 180:
        raise ValueError("restore requires at least 180 seconds including cleanup")
    bounded["timeout_seconds"] = budget - 90
    r = Runner(bounded)
    latest = json.loads((r.state / 'latest-backup.json').read_text())
    name = latest['name']
    if not re.fullmatch(r'auto-\d{8}T\d{6}Z', name):
        raise ValueError('invalid independent snapshot name')
    source = config['backup_root'] + '/' + name
    if latest['destination'] != source:
        raise ValueError('unexpected backup destination')
    r.space(config['archive'])
    r.remote('python3', config['remote_script'], 'verify', '--root', source)
    stamp = dt.datetime.now(dt.timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')
    project = 'riot-grrrl-drill-' + stamp.lower()
    work = Path(config['project']) / 'qualification' / project
    work.mkdir(mode=0o700)
    incoming = work / 'input'
    incoming.mkdir(mode=0o700)
    r.execute(['rsync', '-a', '--protect-args', '--', config['backup_host'] + ':' + source + '/', str(incoming) + '/'])
    backup = verify_backup(incoming)
    primary = json.loads((Path(config['project']) / 'compose.json').read_text())
    image = primary['services']['operator']['image']
    web_image = primary['services']['web']['image']
    # The fresh database has synthetic credentials and no exposed port or external route.
    database = 'postgres://arkhiv:disposable-restore-only@db:5432/arkhiv?sslmode=disable'
    proof = work / 'proof'
    proof.mkdir(mode=0o700)
    services = {
        'db': {'image': 'postgres:17', 'environment': {'POSTGRES_USER': 'arkhiv', 'POSTGRES_PASSWORD': 'disposable-restore-only', 'POSTGRES_DB': 'arkhiv'}, 'tmpfs': ['/var/lib/postgresql/data'], 'healthcheck': {'test': ['CMD', 'pg_isready', '-U', 'arkhiv', '-d', 'arkhiv'], 'interval': '2s', 'timeout': '2s', 'retries': 30}, 'mem_limit': '512m', 'cpus': 0.5},
        'operator': {'image': image, 'user': '1000:1000', 'entrypoint': ['/app/arkhiv'], 'environment': {'DATABASE_URL': database, 'ARKHIV_ARCHIVE': '/drill/restored'}, 'volumes': [str(incoming) + ':/input:ro', str(work) + ':/drill'], 'mem_limit': '512m', 'cpus': 0.5},
        'api': {'image': image, 'environment': {'DATABASE_URL': database, 'ARKHIV_ARCHIVE': '/data/archive'}, 'volumes': [str(work / 'restored') + ':/data/archive:ro'], 'mem_limit': '512m', 'cpus': 0.5},
        'web': {'image': web_image, 'mem_limit': '128m', 'cpus': 0.5},
        'checks': {'image': config['reader_probe_image'], 'volumes': [str(proof) + ':/proof'], 'init': True, 'mem_limit': '2g', 'pids_limit': 256, 'cpus': 1},
    }
    compose_path = work / 'compose.json'
    network = {'internal': True}
    if config.get('restore_subnet'):
        network['ipam'] = {'config': [{'subnet': config['restore_subnet']}]}
    atomic(compose_path, {'services': services, 'networks': {'default': network}})
    compose = ['docker', 'compose', '-f', str(compose_path), '--project-name', project]
    started = False
    try:
        started = True
        atomic(work / 'phase.json', {'phase': 'database-start'})
        r.execute(compose + ['up', '-d', '--wait', 'db'])
        # Replay the public SELECT grants without copying a deployment password.
        # This role cannot log in to the isolated qualification database.
        r.execute(compose + ['exec', '-T', 'db', 'psql', '-U', 'arkhiv', '-d', 'arkhiv', '-v', 'ON_ERROR_STOP=1', '-c', 'CREATE ROLE arkhiv_public NOLOGIN'])
        atomic(work / 'phase.json', {'phase': 'catalog-restore'})
        r.execute(compose + ['run', '--rm', '-T', 'operator', 'restore', '--backup', '/input', '--destination', '/drill/restored'])
        atomic(work / 'phase.json', {'phase': 'restored-audit'})
        audit = json.loads(r.execute(compose + ['run', '--rm', '-T', 'operator', 'audit']))
        expected = {'published': audit['published'], 'queries': config.get('restore_queries', [])}
        atomic(proof / 'expected.json', expected)
        r.execute(compose + ['up', '-d', 'api', 'web'])
        atomic(work / 'phase.json', {'phase': 'reader-proof'})
        r.execute(compose + ['run', '--rm', '-T', 'checks'])
        reader = json.loads((proof / 'reader.json').read_text())
        result = {'project': project, 'work': str(work), 'independent_source': source, 'backup': backup, 'audit': audit, 'reader': reader, 'image': image, 'web_image': web_image}
        atomic(work / 'complete.json', result)
        return result
    finally:
        if started:
            # This exact generated project owns all removed resources; evidence and restored bytes remain.
            r.deadline = time.monotonic() + 60
            r.execute(compose + ['down', '--volumes'])


if __name__ == '__main__':
    try:
        print(json.dumps(drill(json.loads(Path(sys.argv[1]).read_text()))))
    except Exception as error:
        print(json.dumps({'status': 'failed', 'error': type(error).__name__}), file=sys.stderr)
        sys.exit(1)
