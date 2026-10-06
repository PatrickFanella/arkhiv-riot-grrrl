# Backup capacity and activation proposal — October 6, 2026

Read-only inventories came from Python lstat/st_blocks inode accounting and systemctl on Almaz and NUC. The backup jobs still use full copies. Retention is a dry run: seven daily, four weekly and six monthly candidates, with all existing qualification pins held.

| Store | Almaz live archive allocation | Almaz full-copy staging allocation | NUC full-copy store allocation | Latest NUC snapshot allocation |
|---|---:|---:|---:|---:|
| Anarchist | 3,842,314,240 B | 12,788,789,248 B | 12,770,328,576 B | 3,855,966,208 B |
| Riot Grrrl | 949,714,944 B | 4,839,505,920 B | 4,837,789,696 B | 950,312,960 B |

NUC's existing home filesystem had 18,565,935,104 bytes free; `/srv` on separate `/dev/sda` ext4 had 86,068,056,064 bytes free. Almaz's Spektr `/dev/sde1` ext4 had approximately 139.9 GB free. These are point-in-time measurements before qualification copies and unrelated workload growth, rather than reserved capacity.

The existing latest full snapshots allocate a combined 4,806,279,168 bytes. Sixty more unchanged daily full copies would add approximately 288.4 GB **to each host's store**. With a 2 GiB reserve and no other growth, the home filesystem fits only three more complete combined daily copies. Both daily jobs remain scheduled at 03:00 America/Chicago, audits at 02:00, retention planning at 03:40, updates at 04:00 and monthly restore at 05:00 on the first. October 5 runs are observed successes; this is not proof of a completed 60-day soak.

## Proposed separate stores and limits

Preserve every existing staging/snapshot directory and qualification pin. After owner review, create new empty deduplicated staging directories under each existing Almaz archive project and independent NUC stores on `/srv/arkhiv-deduplicated/{anarchist,riot-grrrl}`. These are proposed live destinations; only separate qualification directories have been created so far.

| Bound | Anarchist | Riot Grrrl |
|---|---:|---:|
| Complete snapshot (catalog plus full archive) | 6 GiB | 2 GiB |
| Total staging unique allocation | 24 GiB | 12 GiB |
| Total independent unique allocation | 24 GiB | 12 GiB |

Use a 16 GiB Spektr reserve and 10 GiB NUC `/srv` reserve. The code currently has one `minimum_free_bytes` for both ends; select 16 GiB for both to conservatively satisfy both requirements. Preflight holds room for each archive's worst-case new snapshot plus 16 MiB manifest overhead. Cross-archive sequential jobs retain independent caps and recheck shared filesystem free space; no automatic pruning is allowed. Leave existing deadlines (Anarchist 14,400 seconds, Riot 1,800 seconds), overlap locking, stale monitoring, notification destinations, source acquisition limits and both public frontends unchanged.

## Sixty-day planning envelope

For an explicit planning assumption of at most 128 MiB/day of new unique archive/catalog/manifest bytes for Anarchist and 32 MiB/day for Riot, 60 daily snapshots add 9.375 GiB. The measured full baseline is approximately 4.48 GiB, yielding approximately 13.86 GiB unique snapshot payload/allocation before filesystem overhead. Add one worst-case pending complete snapshot from each archive (8 GiB plus 32 MiB metadata margin): approximately 21.89 GiB across stores. The 36 GiB combined allocation caps leave headroom under this scenario. Newly allocated directory blocks and observed transfer receipts must refine that estimate.

This envelope is an assumption, not a guarantee of source churn, compressibility or a daily limit enforced by the implementation. Faster growth, retained incomplete attempts or catalog growth can exhaust a cap earlier; the job then fails closed with the prior successful receipt retained and existing monitoring alerting. A failed/held directory is evidence, never automatically deleted. Sixty worst-case entirely changed snapshots would exceed capacity; storage must refuse them.

## Activation and rollback review

Before activation, qualify unchanged full collection snapshots on the independent host, fresh disposable database/archive restores with exact hashes and actual readers, and current free capacity after retained qualification artifacts. Build and record immutable operator revisions and receiving runner checksum from the ready backup PRs. Supply exact private configuration changes and old/new receipts for review; the live jobs must use the matching binary/helper before `deduplicate_backup: true` is set.

Cutover must preserve old job configs, images, receiving helper, all originals and backups. A first new snapshot is complete and independent; update the latest receipt only after receiving verification. Rollback restores retained config/operator/helper pointers and leaves new stores held for recovery. Neither rollback nor activation enables deletion. Existing restore inputs and pins remain valid. Issue #56 stays open while real collection restore/capacity proof and live activation remain incomplete.
