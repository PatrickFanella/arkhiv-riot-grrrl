# Independent frontend and shared engine

This repository owns the original arkhiv-riot-grrrl frontend, collection policy, and
operational source workflows. Arkhiv supplies the headless print API.
`deploy/core-release.json` records the reviewed engine receipt and
`deploy/archive-core.env` pins its immutable container digest. The original
backend remains available for recovery and source maintenance during rollout.

## Prepare a release

Run `make verify` and `python3 scripts/check_core_pin.py`. Use a new metadata
volume and independently recovered archive roots. Keep the donor database,
media, previous image, and configuration until rollback has been qualified.
Never import directly over the original database.

Create a private secret directory with `database-password`, `database-url`
(a SELECT-only serving role), and `operator-database-url` (the migration and
review role). Set `SECRET_ROOT`, `ARCHIVE_ROOT`, `DERIVATIVE_ROOT`, `IMPORT_ROOT`,
and `COLLECTION=riot-grrrl`. The archive and derivative roots must match the
verified migration inputs and remain read-only for serving.

Load the engine pin with `--env-file deploy/archive-core.env` and compose file
`deploy/core-compose.yml`. Start PostgreSQL, run the operator's `migrate`
command, restore the reviewed migration inputs using the Arkhiv migration guide,
and grant the serving role SELECT on the resulting tables. Start API and web
only after the catalog and access decisions have been verified. Public serving
requires `PUBLIC_MODE=1` and the existing explicit public approval records.

## Upgrade and rollback

A compatible core upgrade changes the receipt and digest pin through a reviewed
PR. API/module versions must match and every previously applied migration
checksum must remain unchanged. Run the repository gate, back up the consumer's
metadata, apply additive migrations with the operator role, and verify the
original frontend against the new API before changing a production release.

For rollback, retain the previous frontend and engine digest, configuration,
and pre-upgrade database checkpoint. Preserve newer writes before any database
restore. Select the old release in an isolated recovery environment first;
do not assume an old binary supports every later schema.

The full original document/recording identities, metadata, source history,
access decisions, and byte hashes belong to the recovered archive. An engine
upgrade does not approve additional media or change collection policy.
