import { afterEach, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { useBrowserFavorites } from "./Preferences";

afterEach(() => vi.unstubAllGlobals());

it("keeps temporary favorites and their warning after controls remount", () => {
  let stored = '["old"]';
  let denied = true;
  vi.stubGlobal("localStorage", {
    getItem: () => stored,
    setItem: (_key: string, value: string) => {
      if (denied) throw new Error("Storage denied");
      stored = value;
    },
  });
  let favorites: ReturnType<typeof useBrowserFavorites>;
  function Controls() {
    favorites = useBrowserFavorites("remount-regression");
    return <p>{favorites.ids.join(",")} {favorites.notice}</p>;
  }
  renderToStaticMarkup(<Controls />);
  favorites!.toggle("new");
  const remounted = renderToStaticMarkup(<Controls />);
  expect(remounted).toContain("old,new");
  expect(remounted).toContain("Browser storage is unavailable");
  expect(stored).toBe('["old"]');
  denied = false;
  favorites!.toggle("old");
  const recovered = renderToStaticMarkup(<Controls />);
  expect(recovered).toContain("new");
  expect(recovered).not.toContain("Browser storage is unavailable");
  expect(JSON.parse(stored)).toEqual(["new"]);
});
