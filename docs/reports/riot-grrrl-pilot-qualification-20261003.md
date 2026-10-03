# Riot Grrrl private pilot qualification — 2026-10-03

Status: real private pilot qualified. Public release is not activated.
Repository: https://git.subcult.tv/subculture-collective/arkhiv-riot-grrrl
PR: https://git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/pulls/1

## Copied framework and runtime

The separate repository preserves Arkhiv history through
`c6a816203a418f284efeb36f2d3db2afdb34e9b7`. The new application source is
`c0da72f35a9ca5153b389da73aba763b5d445840`; operator/API, web and reader-probe
images are tagged `riot-grrrl-{api,web,reader-probe}:c0da72f`.
Operations helpers are from `a600a2be4dbc796e712ca5fae7301876d5aeb6b3` and
systemd paths from `718477d`. No documents, binaries, database dumps or credentials
are tracked in Git.

On Almaz, the project is `riot-grrrl-pilot` at
`/mnt/spektr/server/data/arkhiv-riot-grrrl`. Originals live under `pilot`,
private evidence under `qualification`, and managed-job state under
`operations/state`. The reader binds only `127.0.0.1:54620`; Kvant's retained
SSH tunnel exposes `http://127.0.0.1:54621/` for owner review.
Readiness reports `status: ok`, `stage: archive`, `public: false`, and the exact
application revision. The API mounts originals read-only and runs as UID/GID
1000; the database has no published port. Compose configuration is private mode 0600.

Runtime image IDs are recorded in private `qualification/runtime-images.json`.
API: `sha256:672ad7d16af1debf70664d6118c84d43e1ddf983785c208e399d2069fc320567`;
web: `sha256:c500f192a6288611466b77f09d97e657c65fff72ac77959a791553077c7d6219`;
reader probe: `sha256:81e548803575641fc642bd040064aae5f7326259e6f0ed3ffc0def6b9bc82f50`.

Initial setup stopped when Almaz's default Docker address pool was exhausted.
Existing networks/services were preserved. After checking Docker networks and
host routes, only the new pilot's acquisition, reader, private edge and temporary
restore networks received explicit unused /24 ranges at `10.254.20.0/24`,
`10.254.21.0/24`, `10.254.23.0/24` and `10.254.22.0/24`, respectively. The
initial archive network retains its allocated `192.168.240.0/20`. A web service
attached only to an internal network did not expose its loopback port; attaching
the web alone to its dedicated private edge resolved that observation. Neither
Docker nor the existing Arkhiv stack was restarted for this work.

## Real acquisition and catalog

Seven explicitly selected original PDFs and five available OCR companions were
acquired with source checksums and local SHA-256 verification: **12 original
assets, 255,658,268 bytes**, with seven independent raw metadata identities.
The configured caps are 10 documents, 384 MiB total, 100 MiB per file and two
concurrent transfers. Every acquired file is verified; no acquisition errors or
unfinished payloads remain. Repeating acquisition returned **12 reused files**
and no duplicate assets.

The private catalog contains **7 documents, 7 variants, 151 PDF pages**. All seven
passed extraction and private review. Text coverage is explicit:

| Document | Pages | Text coverage |
| --- | ---: | --- |
| Drop Babies - Issue 1 | 11 | document-text |
| Girl Germs 3 | 44 | page-text |
| Girl Germs 4 | 40 | document-text |
| VOOP 2 — UK Riot Grrrl zine | 16 | no-text |
| VOOP Issue 1 | 11 | document-text |
| VOOP Issue 3 UK Riot Grrl Zine | 9 | no-text |
| VOOP Issue 5 UK Riot Grrl Zine | 20 | document-text |

Five documents have searchable text: one with page mapping, four with document-level
OCR. VOOP 2 and 3 have no usable extracted text and remain searchable by catalog
metadata. Source-provided dates, creators and language are retained with their raw
metadata; this qualification does not independently establish publication dates.
Girl Germs 3's source warns that repeated photocopying lost photographic detail
and made some text difficult to read. A painted canvas is not a claim of restored
image fidelity or perfect OCR.

The editorial title `VOOP 2 — UK Riot Grrrl zine` survived repeated acquisition
and real source refresh while the original source title remained independently
stored. The first inventory established a 109-file source baseline and queued 116
review events, including seven item-metadata events. A second real refresh checked
109 files with **zero additions, content changes, metadata changes, removals or
uncertain records**. Discovery follows the seven configured item identities; this
is not a census of every Riot Grrrl archive. Updates use weekly cadence and review
policy. New candidates cannot automatically publish or expand a public allowlist.

## Reading, search and public boundary

The all-document browser probe verified **7 matching PDF download hashes/sizes and
7 painted reader canvases**, with Archive.org blocked and zero incidental source
requests. The narrow 390×844 check passed page navigation, 125% zoom, catalog search
and no horizontal page overflow. Scoped `VOOP` and `Girl Germs` queries returned
at least four and two documents, respectively.

The native T3 browser displayed the real branded catalog with seven records and
accent `#9b1459`. It also displayed Girl Germs 3 at page 2 with a painted 919×595
canvas, no alerts and the correct document canonical URL. Catalog/reader screenshots
are retained in Kvant's private T3 browser artifacts. The separate isolated browser
suite passed all four desktop/narrow tests, including keyboard focus and visible
outline. No physical handheld device test is claimed.

A temporary public-mode API had no published port. All **70 GET/HEAD checks** for
these documents, PDFs, downloads, covers and per-document text search returned
404 with `Cache-Control: no-store`, including conditional/range request headers.
Public collection and search listings returned zero records. Its first startup
used the container default user and could not open the private archive; rerunning
with the production pilot's UID/GID 1000 fixed the probe setup. The temporary
public-mode container was removed after evidence was saved.

Rights remain unknown, source attribution and info@subcult.tv stay visible, and
no reuse license is asserted. There are **zero public approvals and zero public
access events**. The original archive's owner approval is not transferable to
these documents. No Caddy route or public domain was activated for this pilot.

## Independent backup and fresh restore

NUC destination: `/home/onnwee/backups/arkhiv-riot-grrrl`, a separate destination
from the original Arkhiv backup. The independently verified restore input was
`auto-20261003T062512Z`: **66 archive files**, catalog dump **194,749 bytes**, SHA-256
`f5475557edcb7904799b4bf6f670de11530bf9f0c6a8b4f5e7cb93e8e31b71ce`.

The snapshot was fetched from NUC and restored into a new temporary PostgreSQL 17
database and filesystem at
`qualification/riot-grrrl-drill-20261003t062636709421z`. Audit verified 12 assets,
7 variants and 7 published private documents. The fresh restored reader repeated
all **7 hash-matching downloads and 7 painted canvases**, with Archive.org blocked
and zero incidental requests.

Full primary/restored row comparisons matched across all 17 public-schema tables,
including source snapshots, variants, metadata/overrides, review events, source
inventories and text. Evidence preserves 8 editorial/publication reviews, 14 raw
source snapshots, 48 indexed text records and zero public-access events. Exact
temporary restore containers, networks and volumes were removed after evidence;
the restored files and comparison receipts remain. Independent backup validation
is separate from offsite preservation; no offsite copy is claimed.

## Managed jobs, stability and verification

Six dedicated systemd timers are enabled: audit, independent backup, monitor,
retention dry-run, source update and isolated restore. Natural audit, backup,
monitor, retention, update and restore runs succeeded on October 3. The natural update
honored the weekly cadence and returned `not-due`; the actual source checks above
were explicit qualification runs. Retention is a dry-run and pins the qualified
`auto-20261003T062512Z` snapshot. The natural backup `auto-20261003T062901Z` was also independently fetched and
restored by the 06:32 UTC timer; that fresh reader again passed seven hashes and
seven canvases with source access blocked. No backup deletion occurred. Notifications are
not configured for this new archive; job receipts and timer status are available.

The bounded private-readiness observation ran from 2026-10-03T06:29:26.578554+00:00 to 2026-10-03T06:31:26.700641+00:00
and recorded **13 healthy samples, zero failures**, checking private mode, seven
records and no public approvals. This short window does not establish long-term
soak or recurring monthly restore history.

Local gates passed: `t3-dev --profile arkhiv verify`,
`t3-dev --profile arkhiv test --command 'make test-updates test-recovery verify'`,
15 operations tests, Go race/PostgreSQL integration, and `make test-browser`
(four tests). Hosted push and PR verification passed at `a600a2b`; final report-head
CI is recorded in PR #1. Source artifacts and actual reader/backup evidence are
separate from local and hosted code checks.

## Rollback and owner review

Stop/disable only `riot-grrrl-*.timer`, and stop only this project's API/web if a
pilot regression requires withdrawal. Keep the database, original store, raw
metadata and NUC snapshots. Do not run `down --volumes` against the retained pilot;
that operation is limited to the exact generated temporary restore project.
The inherited framework CLI supports unpublishing a private record without deleting
its original bytes. Future public release requires a reviewed exact-variant
manifest, owner access decision and selected public domain.
