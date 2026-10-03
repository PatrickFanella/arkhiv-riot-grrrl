import { test, expect } from "@playwright/test";
import { createHash } from "node:crypto";
test.beforeEach(async ({ page }) => {
  await page.route(/https?:\/\/([^/]*\.)?archive\.org\//, (r) => r.abort());
});
test("catalog, page search, local PDF rendering, navigation and download", async ({
  page,
  request,
}) => {
  await page.goto("/?collection=fixture-zines");
  await expect(page.getByRole("link", { name: "Riot Grrrl Archive home" })).toBeVisible();
  await page.keyboard.press("Tab");
  await expect(page.getByRole("link", { name: "Riot Grrrl Archive home" })).toBeFocused();
  expect(await page.getByRole("link", { name: "Riot Grrrl Archive home" }).evaluate((e) => {
    const style = getComputedStyle(e);
    return style.outlineStyle !== "none" && parseFloat(style.outlineWidth) >= 2;
  })).toBeTruthy();
  await expect(
    page.getByRole("heading", { name: "Mutual aid fixture" }),
  ).toBeVisible();
  await expect(page.getByText("Private fixture", { exact: true })).toHaveCount(
    0,
  );
  await page.getByLabel("Search titles and available text").fill("knownword");
  await page.getByRole("button", { name: "Search", exact: true }).click();
  await page.getByRole("link", { name: "Page 2", exact: true }).click();
  await expect(page.getByLabel("Page number")).toHaveValue("2");
  await expect(page.getByRole("status")).toHaveCount(0);
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByLabel("Page number")).toHaveAttribute("max", "2");
  const canvas = page.getByTestId("pdf-canvas");
  await expect(canvas).toBeVisible();
  expect(
    await canvas.evaluate(
      (c: HTMLCanvasElement) => c.width > 100 && c.height > 100,
    ),
  ).toBeTruthy();
  await page.getByRole("button", { name: "Previous page" }).click();
  await expect(page.getByLabel("Page number")).toHaveValue("1");
  await page.getByLabel("Zoom", { exact: true }).selectOption("1.5");
  await page.getByLabel("Search this document").fill("knownword");
  await page.getByRole("button", { name: "Find", exact: true }).click();
  await page.getByRole("button", { name: "Page 2", exact: true }).click();
  await expect(page.getByLabel("Page number")).toHaveValue("2");
  const link = page.getByRole("link", { name: "Download original PDF" });
  const response = await request.get((await link.getAttribute("href"))!);
  expect(response.ok()).toBeTruthy();
  expect(response.headers()["content-disposition"]).toContain("attachment");
  const digest = createHash("sha256")
    .update(await response.body())
    .digest("hex");
  await page.getByText("Source and preservation details").click();
  await expect(
    page.getByText("SHA-256: " + digest, { exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBeTruthy();
});
test("empty, error, loading and second configuration", async ({ page }) => {
  await page.goto("/?collection=fixture-second");
  await expect(
    page.getByRole("heading", { name: "Mutual aid fixture" }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Second synthetic archive" }),
  ).toBeVisible();
  await expect(
    page.getByText(
      "A different editorial introduction for the second synthetic collection.",
    ),
  ).toBeVisible();
  expect(
    await page
      .locator("main")
      .evaluate((e) => (e as HTMLElement).style.getPropertyValue("--accent")),
  ).toBe("#245c42");
  await page.goto("/?collection=fixture-zines&q=nomatchesxyz");
  await expect(
    page.getByText(/No documents match these filters/),
  ).toBeVisible();
  await page.route("**/api/collections", async (r) => {
    await new Promise((resolve) => setTimeout(resolve, 200));
    await r.fulfill({
      status: 503,
      contentType: "application/json",
      body: "{}",
    });
  });
  await page.goto("/");
  await expect(page.getByRole("status")).toContainText("Loading");
  await expect(page.getByRole("alert")).toContainText("could not load");
});
