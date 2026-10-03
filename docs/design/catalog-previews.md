# Riot Grrrl catalog design previews

Three temporary studies compare catalog layouts using the owner's October 3 inspiration files in [docs/inspo](../inspo/). Selection is pending. These studies are available only in Vite development mode; production builds exclude the prototype component and its styles.

| Variant | Direction | Composition |
| --- | --- | --- |
| A | Cut & Paste | Colored paper, torn edges, mismatched cutout lettering, tape, and layered publication covers. |
| B | Copy Shop | A stark photocopy poster, reversed black lettering, halftones, and dense paste-up selections. |
| C | Bedroom Press | A personal notebook with colored stock, marginal notes, and an informal zine shelf. |

## Open the previews

From this repository root, run:

```sh
pnpm --dir web install --frozen-lockfile
pnpm --dir web dev --host 127.0.0.1 --port 54432
```

Open `http://127.0.0.1:54432/?design=A`; substitute B or C for the other studies. The bottom selector and left/right arrow keys cycle designs. Text fields retain normal keyboard behavior. Search and publication/subject filters operate on six sample selections. Selecting a publication opens a native modal with sample metadata and a source link; Escape closes it.

For review from another homelab device, bind Vite to Kvant's private interface with `--host 10.0.0.50`. This frontend-only process uses no database or preserved document storage. The normal isolated launcher remains the default for application development; these studies can run independently when its two application slots are occupied.

## Content and evidence boundaries

Titles and creators come from the qualified pilot. Cover compositions are original typeset study artwork, not original scans. Anarchist numbered labels are design ordering, not historical issue numbers. Riot Grrrl VOOP dates show the pilot's 1992–1993 range, not verified dates for each edition. The samples do not demonstrate PDF reading, preservation, or live acquisition. No new rights or public-access decision is implied.

Reference images stay under docs/inspo and are not bundled into the previews. Their source URLs and licenses were not supplied. Treat them as research references; the production identity requires its own assets.

## After selection

Record the selected layout and the elements to retain, implement them against the existing catalog API and reader, then remove DesignPrototype.tsx, design-prototype.css, and the development entry gate. Keep the existing attribution, rights, source history, and access controls intact. Verify desktop and narrow layouts, keyboard controls, search, and the reader before rollout.
