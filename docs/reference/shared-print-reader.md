# Shared print reader adoption

This frontend pins version 0.2.1 and retains its verified release receipt in
`docs/releases/archive-viewer-0.2.1.json`. It imports reader controls from the exact published
`@subcult/archive-viewer/print` dependency. The catalog, collection branding,
document routes, PDF.js runtime/assets and asset-access policy remain owned here.
The local adapter preserves variant IDs, page links and text-coverage status.

Upgrades change the dependency, lockfile and release receipt together. Check
reader behavior in the isolated frontend and existing backend before deployment.
The shared engine is a migration candidate; this component adoption keeps the
existing backend/database and does not transfer archive or worker ownership.

Run `make verify` and the separate browser gate. Local checks, hosted CI,
real reading, independent recovery and public deployment are separate evidence.
