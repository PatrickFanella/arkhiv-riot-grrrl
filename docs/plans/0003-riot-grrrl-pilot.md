# Riot Grrrl archive pilot

Date: 2026-10-03. Status: real private pilot qualified; [evidence](../reports/riot-grrrl-pilot-qualification-20261003.md).

## Sources

Dedicated holdings exist at [NYU Fales](https://findingaids.library.nyu.edu/fales/mss_354/)
and [Claremont](https://claremont.as.atlas-sys.com/repositories/3/resources/1322).
These finding aids describe archival holdings; they are not a source of unrestricted
bulk downloads. [QZAP](https://archive.qzap.org/index.php/Detail/Object/Show/object_id/153)
has digitized Riot Grrrl material, including Riot Grrrl World #2, with assumed
copyright. The initial pilot uses Internet Archive's existing metadata/file adapter.

Live Archive.org inspection found original PDF scans for:

- [Girl Germs 3](https://archive.org/details/girl-germs-3) and [4](https://archive.org/details/girl-germs-4), source creators Allison Wolfe and Molly Neuman, source year 1991.
- [VOOP 1](https://archive.org/details/voop-issue-1-uk-riot-grrl-zine), [2](https://archive.org/details/voop-issue-2-uk-riot-grrl-zine), [3](https://archive.org/details/voop-issue-3-uk-riot-grrl-zine) and [5](https://archive.org/details/voop-issue-5-uk-riot-grrl-zine), source creators Greta Drummond-Gillett and Layla Gibbon, source years 1992–1993.
- [Drop Babies 1](https://archive.org/details/drop-babies-zine-issue-1-uk-riot-grrrl), source creators Layla Gibbon, Greta Drummond-Gillett and Rosie, source year 1992.

The seven original PDFs total 255,459,265 bytes, before OCR companions.
No item inspected supplies a license URL. Download access is separate from reuse
permission. Original image PDFs are selected over smaller source-generated text
PDFs; preserve original bytes and use available OCR independently. Girl Germs 3's
uploader warns that repeated photocopying has lost photographic detail and made
some text difficult to read. Qualification must retain that limitation.

## Acceptance sequence

1. Copy the framework and history into its own private repository; replace branding,
   collection/defaults, deployment paths and identifiers. Keep upstream history attributed.
2. Pass `make verify`, PostgreSQL integration and the isolated desktop/narrow browser suite.
3. Plan and acquire the bounded manifest into separate homelab storage; retain raw
   metadata, source checksums, local SHA-256 hashes, attempts and acquisition receipt.
4. Extract all PDFs, inspect covers/pages and search coverage, review and publish
   within the private pilot. No public allowlist is inherited.
5. Repeat acquisition and verify reuse without duplicate originals. Refresh selected
   source items and confirm reviewed editions stay stable and new changes require review.
6. Verify every preserved PDF by download hash and painted reader canvas while
   Archive.org is blocked; exercise scoped search and a narrow reader.
7. Back up catalog, originals and source history to the independent NUC; fetch that
   backup into a fresh database/store, compare editorial state and hashes, and repeat reading.
8. Commit/push qualification evidence and open a linked PR; record hosted CI separately.

## Isolation and release

Use Almaz `/mnt/spektr/server/data/arkhiv-riot-grrrl`, loopback web port 54620,
and NUC `/home/onnwee/backups/arkhiv-riot-grrrl`. These separate destinations were confirmed and are now active. Do not change or restart the existing Arkhiv deployment.
Use the Arkhiv T3 recipe explicitly from this root; the original thread remains
in its own checkout. Acquisition requires a separate outbound workflow.

Public approval from the Anarchist Zine Archive applies only to its exact reviewed
manifest. This new archive remains private through qualification. A later public
release needs a reviewed variant manifest, owner access decision and selected domain.

## Catalog design follow-up

The owner selected **A — Cut & Paste** on October 3. The [selected interface](../design/catalog-previews.md) uses actual catalog metadata and local cover scans. Search, pagination, document reading, downloads, provenance and rights/contact remain available. Comparison prototypes and unused layouts are removed. Required repository verification and desktop/narrow browser checks qualify the implementation; deployment remains separate.

## Collection expansion

The owner requested a broader collection. [The bounded expansion plan](0004-riot-grrrl-expansion.md) adds sixteen reviewed source identities to the private pilot; qualification will record the resulting catalog and backup.

## Public rollout, October 4

The owner authorized rollout and selected `riot-grrrl.subcult.tv`. The public
release uses the individually hash-bound 23-PDF review manifest and a separate
restored database and archive copy. Source rights remain unknown. All public
PDF downloads match their recorded sizes and hashes; the actual HTTPS reader
paints pages without an alert. The retained private pilot is unchanged.
[Public rollout evidence](../reports/riot-grrrl-public-rollout-20261004.md) records
the large-PDF streaming fix and passed fresh independent public recovery.

## Print viewer follow-up

The viewer feedback issue adds creator/original-date filters before pagination,
grid/table browsing, collection-scoped local favorites, current document/variant/
page sharing, and reading focus with Escape. Full source dates support bounds;
partial and unknown dates remain visible when bounds are absent. Editorial date
corrections do not replace the original source date. Existing covers, PDF text,
search, downloads, rights and product-owned layout remain available.

## Bounded backup follow-up

The opt-in [bounded backup mode](../runbooks/bounded-backups.md) retains complete snapshots and shares verified read-only files between backups. Explicit snapshot/staging/independent-store caps refuse unsafe growth without pruning evidence. Source qualification and live capacity/restore requirements are tracked in Anarchist Archive #56; live jobs retain their existing behavior until reviewed activation.
