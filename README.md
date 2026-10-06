# Riot Grrrl Zine Archive

A separate Arkhiv application for preserving, cataloging and reading Riot Grrrl zines.
The collection preserves 23 issues across **Girl Germs, VOOP, Drop Babies, Violet,
No Apologies, Personality Liberation Front** and other Riot Grrrl zines. The catalog
uses the selected Cut & Paste design and a local PDF reader.

This repository copies Arkhiv at `c6a816203a418f284efeb36f2d3db2afdb34e9b7`,
retaining its history, operator CLI, preservation store, PostgreSQL catalog,
full-text search, local PDF.js reader, public allowlist and recovery tooling.
The existing Anarchist Zine Archive is a separate service. Its approvals and
stored documents are not inherited by this archive.

## Source and collection

[Collection configuration](collections/riot-grrrl.yaml) selects original PDFs
explicitly across 23 Archive.org items. The 23 PDFs and 21 OCR companions total
937,935,490 bytes. Caps: 30 documents, 2 GiB per batch, 250 MiB per file,
two concurrent transfers and a 2 GiB free-space reserve. Raw source snapshots and verified
files stay outside Git. Source rights are unknown; no reuse license is asserted.

[Source research and qualification plan](docs/plans/0003-riot-grrrl-pilot.md)
records the source choices, private qualification and release boundaries.
The [expansion plan](docs/plans/0004-riot-grrrl-expansion.md) records the sixteen additions.
The discovery query follows the 23 selected item identities. Weekly refresh
queues changes for review; it does not discover every Riot Grrrl zine or publish
new editions automatically.

## Development

Use Go 1.26, Node 24 and pnpm 10.33.0. Read `AGENTS.md` and the shared T3 guide.
The existing Arkhiv recipe can be selected explicitly, with state isolated by root:

```sh
t3-dev --profile arkhiv verify
t3-dev --profile arkhiv start
t3-dev --profile arkhiv sync
```

The preview contains synthetic documents, not acquired zines. `make verify` checks
formatting, Go vet/tests, TypeScript, lint, frontend tests/build, Compose and
Open Pilot templates. `make test-browser` checks desktop/narrow local rendering
with a disposable PostgreSQL 17 database. Hosted CI is separate evidence.

The CLI retains the name `arkhiv`; its Go module is
`git.subcult.tv/subculture-collective/arkhiv-riot-grrrl`. The default collection
is `collections/riot-grrrl.yaml`. Local Compose defaults use ports 5434/8082/5175.
`VITE_PUBLIC_URL` is optional; canonical and contact links use the current origin
when no public domain has been configured. Correction contact: info@subcult.tv.

## Qualification and operations

The expanded private collection passed all 23 PDF download hashes and reader renders,
with Archive.org blocked. It has 786 pages; 21 PDFs have extracted text. All 44
preserved files were reused on repeated acquisition, and the independent NUC
backup passed integrity verification. A fresh isolated restore then passed all
44 asset checks and all 23 local download/reader checks. See the [expansion qualification report](docs/reports/riot-grrrl-expansion-qualification-20261003.md).
The [original pilot report](docs/reports/riot-grrrl-pilot-qualification-20261003.md)
retains the earlier seven-document restore evidence.
The public site is [riot-grrrl.subcult.tv](https://riot-grrrl.subcult.tv/). Its
[reviewed manifest](collections/riot-grrrl-public-review.json) approves these exact
23 PDFs, without asserting a reuse license. It uses a separate restored database,
read-only serving role and archive mount. The original private pilot remains
separate. See the [public rollout report](docs/reports/riot-grrrl-public-rollout-20261004.md)
for actual HTTP, reader and deployment evidence. Fresh public recovery passed all 17 table counts and all 23 PDF hashes.

Earlier plans and dated reports under `docs/` describe the upstream Arkhiv
project. They are retained as implementation history, not evidence that this
Riot Grrrl pilot has been qualified or publicly released. Start with the Riot
Grrrl plan and its dated qualification report for this repository's state.

## Catalog design

The owner selected **A — Cut & Paste** from the [inspiration studies](docs/design/catalog-previews.md). The interface now displays real catalog metadata and local cover scans in the selected layout, with existing search, reading, downloads and rights information. The comparison prototypes are removed. The selected frontend is deployed to the private Almaz service; all 23 PDFs passed download/hash/render checks against it. The owner authorized public rollout on October 4; the same distinct interface
is now live on the public hostname.

The original frontend remains in this repository. A versioned headless Arkhiv
engine can serve it through the [shared-core release runbook](docs/runbooks/shared-core-release.md),
with an immutable pin and independently verified upgrades.

## Viewer feedback, October 6

The product independently pins shared viewer 0.3.1. Its native backend and core
pins remain independent. Browser favorites are scoped to this product or selected
collection and retain removable unavailable references. Sharing has a selectable
copy fallback and social compose links. Optional account activation requires a
verified native-to-canonical identity mapping and a separate provider decision.
No acquisition, approval or deployment changes are included.

The [viewer qualification report](docs/reports/viewer-feedback-2026-10-06.md) records local verification and browser evidence.
