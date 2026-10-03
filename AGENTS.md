# Arkhiv agent instructions

Arkhiv preserves document collections and serves branded catalogs and local
readers. The configured collection is the Riot Grrrl Zine Archive. This checkout
implements the private pilot, derived from the SUBCULT Go/Vite/PostgreSQL template.

## Conventions

- Keep Go code under `backend/`, React/TypeScript under `web/`, and SQL under
  `migrations/`. Go module: `git.subcult.tv/subculture-collective/arkhiv-riot-grrrl`.
- Use pnpm 10.33.0 and commit `web/pnpm-lock.yaml` with dependency changes.
- Preserve Open Pilot issue and PR templates.
- Keep credentials, binaries, acquired documents, and generated data out of Git.
- Preserve original source metadata independently of editorial corrections.
- Do not present planned ingestion, search, or reading as implemented.
- Update README and the milestone plan when their behavior or scope changes.

## Development and verification

Before previews or tests, read
`/home/onnwee/.local/share/t3-dev-environments/AGENT-GUIDE.md`.
Use profile `arkhiv` explicitly from this checkout; snapshots are keyed by this repository root.
Do not reuse the original archive’s database or storage. Do not relocate an active thread. The isolated preview cannot
reach Archive.org; future live acquisition needs a separate bounded workflow.

Required gate: `make verify`. In T3, `t3-dev --profile arkhiv verify` runs that
command in a disposable test snapshot. It checks formatting without rewriting
source, TypeScript, lint, tests, builds, Compose configuration, and the Open Pilot
template. Browser checks and hosted CI are separate evidence.

See `docs/plans/0003-riot-grrrl-pilot.md` before implementing the archive.
