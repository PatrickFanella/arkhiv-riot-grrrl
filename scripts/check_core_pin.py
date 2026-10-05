"""Verify the reviewed independent Arkhiv engine pin."""
import json
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
receipt = json.loads((root / 'deploy/core-release.json').read_text())
if (receipt['artifact_kind'] != 'core' or receipt['contract_version'] != 1
        or receipt['configuration_version'] != 1 or '1' not in receipt['api_versions']
        or receipt['modules'].get('catalog') != '1'
        or receipt['modules'].get('print') != '1'
        or receipt['schema_strategy'] != 'checksum-locked-additive-migrations'
        or receipt['engine_repository'] != 'git.subcult.tv/subculture-collective/archive-core'
        or not re.fullmatch(r'[0-9a-f]{40}', receipt['source_revision'])
        or not re.fullmatch(re.escape(receipt['engine_repository']) + r'@sha256:[0-9a-f]{64}', receipt['engine_digest'])
        or not receipt['migrations']
        or any(not re.fullmatch(r'[0-9a-f]{64}', digest) for digest in receipt['migrations'].values())):
    raise SystemExit('Incompatible or incomplete independent core receipt')
if (root / 'deploy/archive-core.env').read_text() != 'ARCHIVE_ENGINE_IMAGE=' + receipt['engine_digest'] + '\n':
    raise SystemExit('Core pin differs from the reviewed release receipt')
print('Independent Arkhiv engine pin verified:', receipt['version'], receipt['source_revision'])
