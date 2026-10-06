# Print viewer qualification — 2026-10-06

The product retains its branded frontend and native backend. The viewer package is pinned to 0.3.1; the backend/core pin remains 0.1.2. Optional account synchronization awaits the separately tracked retained-backend identity migration.

## Required verification

Host: sputnik. Source: `/tmp/arkhiv-consumers/arkhiv-riot-grrrl`. Base revision: `e9b1bbf2aaec1a6ca7111b8ecb975a9c30ee1360` with the viewer changes uncommitted during verification. The isolated shared launcher used profile `arkhiv`, disposable PostgreSQL, and the repository `make verify` gate. Run 23328 exited 0 after the final source changes. Go checks, database integration, frontend checks and build, and package provenance checks passed. Hosted CI is waived while Kvant is unavailable.

The new database regression verifies creator and original-date filtering before pagination (26 matching records and six records on the second page), preservation of partial/unknown dates without date bounds, source dates independent of editorial overrides, invalid date bounds returning 400, and public exclusion of unapproved variants.

## Native collaborative browser

The owned preview at loopback port 33847 used synthetic collections and original two-page PDFs. Local cover images loaded, the semantic table showed creator, original date, file and page count, and browser favorites persisted through document navigation. Reader page changes updated the URL. Denying share and clipboard access exposed a manual copy field containing the current page-2 URL. Reading focus activated and Escape restored the surrounding interface.

A same-origin 390 CSS-pixel iframe in the native browser verified the narrow layout; this is desktop-browser responsive evidence, not physical-device evidence. The document width was 375 pixels, fit-width canvas 340 by 440, and text search found the page-2 phrase “Solidarity knownword appears on page two.” Original downloads returned HTTP 200 with attachment disposition and 895 bytes, SHA-256 `a5168c95e47d5ffdaae2d137f0280a5a2dde4f5888b6b8a487500fc8548d139f`. Rights, source references, preservation details and original downloads remained available.

The synthetic preview is stopped with its development data retained. This report does not claim public activation, complete retained-corpus qualification, hosted CI, or account-provider activation.
