"""Verify the independently reviewed frontend SDK receipt."""
import json
from pathlib import Path
import re

root = Path(__file__).resolve().parents[1]
receipt = json.loads((root / 'deploy/viewer-release.json').read_text())
package = json.loads((root / 'web/package.json').read_text())
assert receipt['package'] == '@subcult/archive-viewer'
assert package['dependencies'][receipt['package']] == receipt['version']
assert re.fullmatch(r'[0-9a-f]{40}', receipt['source_revision'])
assert re.fullmatch(r'[0-9a-f]{64}', receipt['package_sha256'])
assert receipt['compatible_core_receipt'] == 'core-release-0.3.0.json'
assert json.loads((root / 'deploy/core-release.json').read_text())['version'] == '0.3.0'
print('Independent frontend SDK pin verified:', receipt['version'], receipt['source_revision'])
