# Riot Grrrl public rollout — October 4, 2026

The owner authorized continuation through public rollout. The distinct Cut &
Paste interface is live at https://riot-grrrl.subcult.tv/, with 23 individually
reviewed PDFs. The public manifest has SHA-256
`7c4f288b8aa6913797fa29fe2ef3283730a0f34a20a904ca70f9fc312897b688`.
It binds each approval to its document, variant, filename, byte size and hash.
Source rights remain unknown; source attribution, correction contact and the
absence of a reuse license remain visible.

The public backend image is
`sha256:2b53afd753abfb3b1f484c36d509853f43ca42c254fc24aa73f5dc49c79bc0db`;
the frontend is
`sha256:895425a6b2f6fb155a9a432ee49e277f29d4a2aebf30f5494f2be81030959066`.
Both application artifacts come from `53cd3ec2cb2b695d67d5df27220d9547f613f6a0`
and consume shared viewer 0.2.2. The mounted public Nginx configuration additionally
includes the reviewed DNS and streaming fixes through `a8d16ff`. This consumer
keeps its print backend and branded frontend; this release does not claim a
migration of that backend to the mixed headless API.

The new public services use a separate restored `riot_grrrl` database in the
Arkhiv public deployment. Serving uses a SELECT-only role and a read-only archive
mount. Public approval ran through the actual CLI against the independent copy;
all 23 variants passed file and derivative checks. The original private database,
archive and preview remain separate. No new acquisition was performed.

All 23 full HTTP PDF downloads matched their sizes and SHA-256 hashes, totalling
936,564,784 bytes. Qualification caught an 87 MB PDF being truncated when Nginx's
16 MB temporary cache filled. Disabling proxy buffering fixed that failure while
keeping the cache bound. The initial failed attempt and complete subsequent hash
checks are retained. Foreign Anarchist documents return 404.

The actual public Violet volume 4 reader painted page 2 at 864 by 1,152 pixels
without an alert. Required isolated `make verify` passed against `a8d16ff`.
These checks prove the observed reader and download behavior; a sustained public
observation window remains a separate gate.

Runtime receipts are under
`/mnt/spektr/server/data/arkhiv-platform-20261004/qualification` on Almaz.
The fresh public checkpoint `public-20261004T084653Z` reached NUC and was restored
independently on Kuznya. Every one of the 17 Riot Grrrl table counts matched,
including 23 public access events. The exact public backend served all 23
restored PDFs with matching byte sizes and SHA-256 hashes, using a SELECT-only
role and read-only archive. Recurring public monitoring is enabled and passed
its first natural timer activation. Bounded daily backup is enabled; its first
manual copy passed, and its next daily activation remains unobserved. Retain the old private
artifacts for private recovery; qualify public access behavior before using a
historical artifact as a public rollback target.
