# Riot Grrrl expansion qualification — October 3, 2026

The separate private archive now catalogs 23 documents, adding sixteen original scans to the seven-document pilot. The owner requested collection expansion; this does not authorize public ingress or public PDF access. All 23 variant approval flags remain false, rights remain unknown and no reuse license is asserted. [Source selection and measured limits](../plans/0004-riot-grrrl-expansion.md) records the exact source identities.

| Check | Observed result |
| --- | --- |
| Catalog | 23 published private documents, 23 original PDF variants, 786 pages |
| Preservation | 23 PDFs plus 21 OCR companions; 44 files totaling 937,935,490 bytes |
| Acquisition | 12 earlier files reused; 32 additions verified; no failures |
| Repeated acquisition | All 44 files reused, exact sizes and hashes verified |
| Text coverage | 19 PDFs use document OCR, two use page text, two handwritten VOOP issues have no extracted text |
| Live reader | All 23 download SHA-256 hashes and all 23 PDF canvas renders passed |
| Source independence | Archive.org blocked during reader checks; no source requests before the deliberate blocking probe |
| Narrow reader | Page navigation, zoom and search passed at 390 × 844 |
| Stable refresh | 366 source files checked; zero added/content/metadata/removal/uncertain changes on repeat |
| Editorial separation | Violet volume 2 remains date unknown; VOOP 2 title correction survives refresh; raw source values retained |
| Fresh isolated restore | 44 assets audited; all 23 download hashes/renders and narrow search/reader checks passed from the NUC snapshot |
| Independent backup | NUC snapshot auto-20261003T222229Z; 218 archive files and catalog dump verified on NUC |
| Repository gate | Isolated make verify passed on Kuznya; all-document live reader probe uses explicit response disposal |

The initial expanded source baseline queued 389 review entries, including source metadata and files beyond the original PDF selection. Weekly refresh remains in review mode; it neither acquires nor publicly approves those changes. Searchable text is not guaranteed for handwriting or image-only pages. The source date “1883” for Violet volume 2 is retained in its raw metadata; the catalog does not invent a replacement date.

The NUC backup at `/home/onnwee/backups/arkhiv-riot-grrrl/auto-20261003T222229Z` has catalog dump SHA-256 `783d08d09f94e2ff872c5c457708a6f1166212f1c3f29a1a3c36b0d1fb33451e` and size 855,319 bytes. A fresh NUC-side integrity check returned the same file count and dump hash after promotion. The working archive remains `/mnt/spektr/server/data/arkhiv-riot-grrrl/pilot` on Almaz.

The first expanded restore completed file and database restoration but exhausted its 900-second job budget at the final audit. That failed receipt remains. A bounded resume verified the independent backup again, reused the restored bytes, restored the verified dump into a new isolated database and passed the 44-asset audit plus all 23 reader/hash checks. Its temporary stack was removed. Completion is recorded at `qualification/riot-grrrl-drill-20261003t223620353213z/complete.json` and `qualification/expansion-restore-resume-receipt-20261003.json`; it is a manual recovery proof, not a natural timer run. The operating job deadline is now 1,800 seconds and scheduled recovery uses reader probe image `riot-grrrl-reader-probe:f8c7fd4`, built from the committed response-disposal script and syntax-checked in its container; `qualification/operations-config-pre-expanded-deadline-20261003.json` retains the previous configuration.

The expanded configuration is retained at `pilot/config/riot-grrrl.yaml` and mounted read-only into the operator. Rollback copies are `qualification/compose-pre-expansion-20261003.json` and `qualification/riot-grrrl-config-pre-expansion-20261003.yaml`. Restoring configuration alone does not undo catalog publication; use explicit private review events if catalog withdrawal is needed. Preserve the acquired originals and backup.

Host evidence is under `/mnt/spektr/server/data/arkhiv-riot-grrrl/qualification`: `expansion-plan-20261003.json`, `expansion-acquisition-20261003.json`, `expansion-final-catalog-20261003.json`, `expansion-stable-refresh-20261003.log`, `expansion-repeat-acquisition-20261003.log`, `expansion-reader-20261003/reader.json` and `expansion-backup-20261003.log`. The repeated acquisition receipt is in `pilot/jobs/expansion-repeat-acquisition-20261003.json`. Source documents, snapshots, catalogs and screenshots stay out of Git.

The real-data Cut & Paste frontend preview is available on Kvant at `http://10.0.0.50:54432/`, using the private Almaz API through an SSH tunnel. The deployed Almaz API and frontend remain revision c0da72f; the design change is merged in the repository and previewed separately. Automatic browser qualification establishes the listed behavior, not a manual visual inspection in the T3 browser or public release.
