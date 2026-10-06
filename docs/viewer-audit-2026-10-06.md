
## Shared storage patch

Viewer 0.4.1 was independently published from Arkhiv source
14dd96bb216bb6e3a1331e65dbb3f6ebadbd4dff. The registry receipt is in
deploy/viewer-release.json. Core 0.3.0 and engine 0.4.0 receipts remain intact.
Browser checks on the packed SDK showed the temporary-favorites warning after
a rejected write and warning removal after a successful write. The shared hook
also retains newer pending edits across remounts. This adoption requires the
updated consumer frozen-lock gate; it changes no live deployment.

The complete consumer make verify passed with SDK 0.4.1 and this frozen registry lock. Native print integrations used disposable PostgreSQL 15 matching the dump client. Frontend receipt verification is independent of the retained core and reference-engine image receipts.

## Unreadable-response patch

Viewer 0.4.2 replaces raw JSON parser errors with retryable archive messages
for video requests and print/audio catalog responses. Structured source errors
and caller cancellation remain intact. Its independent SDK receipt is in
deploy/viewer-release.json; Core 0.3.0 and engine 0.4.0 remain pinned separately.
Source gates and current registry locks are required for this adoption.
