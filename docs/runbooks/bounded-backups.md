# Bounded deduplicated backups

This mode is opt-in. Existing configurations continue full-copy backups. Review the [source qualification](../reports/bounded-backups-20261006.md) and complete issue #56's collection/capacity evidence before enabling it in a live job.

## Snapshot rules

`arkhiv backup --destination /private/new-snapshot --basis /private/earlier-snapshot --maximum-backup-bytes BYTES` verifies the basis and compares file content hashes. The basis must be outside the mutable archive and on the destination filesystem. Its shared files must already be read-only. A first snapshot omits `--basis`. The byte limit covers archive files plus the catalog dump; a rejected dump leaves an incomplete directory for diagnosis. Restores continue copying into a fresh archive and empty database; they never hard-link restored files to backup evidence.

The managed runner verifies and freezes only newly created snapshots, then transfers into a new `.incoming-*` directory using `rsync --checksum --link-dest`. Verification on the independent host precedes promotion and the latest receipt. Receipts record transferred files/bytes and unique physical allocation. Corrupt or writable bases fail closed. Old full-copy stores and pins must remain unchanged: start a separately reviewed empty deduplicated store rather than modifying old snapshots to make them a basis.

## Required reviewed configuration

Set `deduplicate_backup: true`, positive integer `maximum_snapshot_bytes`, `maximum_staging_bytes` and `maximum_backup_bytes`, and a free-space `minimum_free_bytes`. Snapshot limits include the complete catalog and derivatives, not only source PDFs. The runner budgets another 16 MiB for the manifest/directory overhead. Both store preflights reserve room for a worst-case complete new snapshot even when unchanged files are expected to share bytes. Limits use physical allocation counted once per inode; snapshots remain logically complete. All three caps must fit the measured stores while preserving unrelated workloads and both archives' reserves.

Deploy the matching operator binary and matching receiving-host runner before enabling the mode. The runner's existing absolute deadline, overlap lock, stale checks and failure notifications still apply. Do not replace a live configuration, image or timer as part of source qualification. Complete the host allocation inventory, bounded 60-day projection and independent real collection restore first; submit exact new store paths, caps, reserve, revisions, cutover and rollback for owner review.

## Failure and rollback

Never prune to satisfy a cap. Preserve failed `.incoming-*` and staging snapshots; retain the prior latest receipt and last success. Monitor will report the failed/stale backup through the existing notification path. Verify the receiving copy and original pins before resuming.

Rollback restores the retained configuration, operator image and receiving runner while keeping every old and new snapshot. Returning to full-copy mode also needs adequate measured capacity. Retention remains seven daily, four weekly and six monthly candidates in dry-run with all existing pins. Destructive retention activation remains a separate decision.
