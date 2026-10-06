# Bounded backup source qualification — October 6, 2026

The opt-in backup mode keeps format-1 complete snapshots independently restorable while reusing checksum-identical files from a read-only earlier snapshot. It never links to the mutable archive. Same-size changes with unchanged timestamps create new files. Receiving-host rsync uses checksums and `--link-dest`; no incomplete transfer becomes the latest verified receipt.

The initial full `make verify` passed in this repository's isolated `arkhiv` profile, including real disposable PostgreSQL backup/restore, frozen frontend checks and real rsync transfers. The final gate also passed after adding rejection of writable basis files without modifying their permissions. Each repository passed 20 operations tests, Go tests including its disposable PostgreSQL recovery integration, frozen-lock frontend checks/builds and Compose/release-pin checks. Hosted CI remains owner-waived while Kvant is down. Actual rsync qualification measured zero transferred payload bytes and zero transferred files for an unchanged frozen snapshot. Changed-byte and interrupted-transfer tests retained the older snapshot and latest receipt. The database test restores a deduplicated snapshot into a fresh database/archive and verifies editorial metadata and variants. These fixtures prove the mechanics, rather than full collection reader coverage or naturally scheduled operation.

Configuration defaults to the existing full-copy behavior. Enabling deduplication requires explicit snapshot, staging and independent-store limits and free-space reserve. Capacity refusal holds all evidence, including incomplete attempts. Retention remains dry-run; no deletion activation is included.

Issue [Anarchist Archive #56](https://git.subcult.tv/subculture-collective/arkhiv-anarchist/issues/56) remains open pending measured live allocation, a 60-day projection, real collection receiving-host deduplication and fresh independent restore/reader evidence, and separately reviewed live activation. Existing backup stores, qualification pins, timers, notification destinations and live images have not been changed.

## Full collection transport and cleanup follow-up

A separate October 6 qualification copied each retained October 5 full snapshot into new Almaz staging and a new independent NUC `/srv` store. Complete manifest and catalog hashes passed before and after transport. The original snapshots were reverified without permission or content changes. A second unchanged snapshot transferred zero files and zero payload bytes on both hosts.

| Collection | Archive files in manifest | First independent allocation | Allocation after two snapshots | Added allocation for the unchanged second snapshot |
|---|---:|---:|---:|---:|
| Anarchist | 4,600 | 3,860,316,160 B | 3,864,629,248 B | 4,313,088 B |
| Riot Grrrl | 260 | 950,636,544 B | 950,956,032 B | 319,488 B |

The second snapshot's additional allocation is directory overhead. Logical payload doubled, while file content remained shared between immutable snapshots. Each snapshot still has its complete file inventory and independently verified catalog. Across both stores, 60 unchanged snapshots would add approximately 278 MB of measured directory allocation to the first full baseline. Database changes and new archive content require additional unique allocation, so this does not replace the growth assumptions or caps in the capacity proposal.

A longer private Riot Grrrl recovery experiment restored the independent full snapshot and completed two Go CLI backups. All 260 archive files in the second backup shared inodes with the frozen first backup. Its second fresh restore exceeded the original 1,800-second qualification deadline. That attempt remains a recorded failure, with both completed backups and partial restore retained. A separate continuation uses a 7,200-second qualification-only budget to test the remaining restore and reader behavior. It does not qualify the live 1,800-second deadline or change its configuration.

The failed experiment exposed a cleanup defect. Compose teardown left its running one-off restore container behind. The exact owned container was removed and its process was confirmed absent. The source fix gives temporary operator containers an init process, tears down the generated project with orphan removal, then checks for remaining containers by its exact project label and removes only returned 64-character Docker IDs. The project-name guard rejects live deployment names. Evidence directories and restored bytes remain on disk. Regression tests cover leftover one-offs, failed teardown and refusal to target live projects or unexpected container identities.

The Anarchist full recovery experiment and Riot Grrrl continuation have not yet produced complete restore/reader receipts at this report revision. Neither collection's live configuration, image pointer, timers, backup destination or retention behavior has been changed.

The cleanup follow-up passed `make verify` from each native worktree using its isolated `arkhiv` profile on Sputnik. Both passed 24 operations tests, Go checks, two frontend tests, frozen-lock builds, Compose validation and engine/SDK receipt checks. Hosted CI remains owner-waived.

Compose teardown has a 30-second sub-budget inside the 60-second cleanup deadline. A timeout leaves the remaining 30 seconds for exact-label lookup and force removal. The fourth cleanup regression verifies that split, and both full gates passed again.
