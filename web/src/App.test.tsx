import { renderToString } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
vi.mock("./Reader", () => ({ default: () => null }));
import App, { selectVariant } from "./App";
describe("catalog before the API responds", () => {
  it("shows loading without claiming an empty or imported catalog", () => {
    vi.stubGlobal("location", {
      search: "",
      pathname: "/",
      href: "http://localhost/",
    });
    const html = renderToString(<App />);
    expect(html).toContain("Loading the archive");
    expect(html).not.toContain("No documents have been imported");
    expect(html).not.toContain("Download original PDF");
    vi.unstubAllGlobals();
  });
});
describe("edition selection", () => {
  const edition = (id: string) => ({
    id,
    filename: `${id}.pdf`,
    sha256: id,
    size: 1,
    pages: 1,
    text_status: "none",
  });
  it("does not substitute another edition for an unknown variant", () => {
    const variants = [edition("first"), edition("second")];
    expect(selectVariant(variants, "second")?.id).toBe("second");
    expect(selectVariant(variants, null)?.id).toBe("first");
    expect(selectVariant(variants, "stale")).toBeUndefined();
  });
});
