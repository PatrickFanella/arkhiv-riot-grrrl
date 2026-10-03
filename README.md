# Riot Grrrl Zine Archive

A separate Arkhiv application for preserving, cataloging and reading Riot Grrrl zines.
The initial selection is seven issues of **Girl Germs, VOOP and Drop Babies**, dated
1991–1993 in Internet Archive source metadata. The site uses a dark magenta ink
palette, bold paper-label masthead and local PDF reader.

This repository copies Arkhiv at `c6a816203a418f284efeb36f2d3db2afdb34e9b7`,
retaining its history, operator CLI, preservation store, PostgreSQL catalog,
full-text search, local PDF.js reader, public allowlist and recovery tooling.
The existing Anarchist Zine Archive is a separate service. Its approvals and
stored documents are not inherited by this archive.

## Source and collection

[Collection configuration](collections/riot-grrrl.yaml) selects original PDFs
explicitly across seven Archive.org items. Originals total 255,459,265 bytes;
OCR companions are acquired when available. Caps: 10 documents, 384 MiB total,
100 MiB per file, two concurrent transfers. Raw source snapshots and verified
files stay outside Git. Source rights are unknown; no reuse license is asserted.

[Source research and qualification plan](docs/plans/0003-riot-grrrl-pilot.md)
records the source choices, private qualification and release boundaries.
The discovery query follows the seven selected item identities. Weekly refresh
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

The real private pilot is qualified: seven PDFs and five OCR companions are preserved,
and the independent NUC restore passed all seven local reading/download checks.
See the [dated qualification report](docs/reports/riot-grrrl-pilot-qualification-20261003.md).
Use a separate archive root, database,
Compose project, loopback port and NUC backup destination. No public ingress or
public document approval is included in this bootstrap. The candidate Caddy
file deliberately requires an approved domain before installation.

Earlier plans and dated reports under `docs/` describe the upstream Arkhiv
project. They are retained as implementation history, not evidence that this
Riot Grrrl pilot has been qualified or publicly released. Start with the Riot
Grrrl plan and its dated qualification report for this repository's state.

## Catalog design studies

Three development-only catalog previews explore the owner-supplied [inspiration files](docs/inspo/). See [preview directions and launch instructions](docs/design/catalog-previews.md). Selection is pending; the qualified production interface is unchanged.
