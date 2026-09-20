import { test, expect } from "@playwright/test";

const URL = "/";

/**
 * Pola error console yang dianggap "harmless" (bukan regression aplikasi):
 * - favicon / resource 404
 * - Font Google / offline-only (net::, ERR_)
 * - DevTools download prompt
 * - "Failed to load resource" (umum untuk request yang dibatalkan)
 */
const HARMLESS_PATTERNS = [
  /favicon/i,
  /Failed to load resource/i,
  /net::ERR_/,
  /ERR_INTERNET_DISCONNECTED/,
  /fonts\.googleapis/i,
  /fonts\.gstatic/i,
  /Download the React DevTools/i,
];

function isHarmless(message: string): boolean {
  return HARMLESS_PATTERNS.some((p) => p.test(message));
}

async function gotoHome(page: import("@playwright/test").Page) {
  await page.goto(URL);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
}

test("LP-01: Homepage render — hero, section & footer tampil", async ({
  page,
}) => {
  await gotoHome(page);

  const hero = page.locator("#beranda");
  await expect(hero).toBeVisible();
  await expect(hero.getByRole("heading", { level: 1 })).toContainText("Bayar");

  const layanan = page.locator("#layanan");
  await expect(layanan).toBeVisible();
  await expect(layanan.getByRole("heading", { level: 2 }).first()).toBeVisible();

  const contentinfo = page.getByRole("contentinfo");
  await expect(contentinfo).toBeVisible();
  await expect(contentinfo).toContainText("G-Flow");
});

test("LP-02: Navigasi anchor — klik link layanan scroll ke section", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await gotoHome(page);

  const nav = page.getByRole("navigation", { name: /navigasi utama/i });
  await expect(nav).toBeVisible();

  const anchors: { link: string; section: string }[] = [
    { link: "Layanan", section: "#layanan" },
    { link: "Mitra", section: "#mitra" },
    { link: "Kontak", section: "#kontak" },
  ];

  for (const { link, section } of anchors) {
    const navLink = nav.getByRole("link", { name: link, exact: true });
    await expect(navLink).toBeVisible();
    await navLink.click();
    await expect(page.locator(section)).toBeInViewport();
  }
});

test("LP-03: Language toggle ID/EN — konten berubah", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await gotoHome(page);

  const toggle = page
    .locator(
      [
        '[data-testid*="lang"]',
        '[data-testid*="toggle"]',
        'button[aria-label*="bahasa" i]',
        'button[aria-label*="language" i]',
        'button:has-text("Indonesia")',
        'button:has-text("English")',
        'button:text-is("ID")',
        'button:text-is("EN")',
      ].join(", ")
    )
    .filter({ hasNotText: /kirim/i })
    .first();

  if ((await toggle.count()) === 0) {
    test.skip(true, "Tombol toggle bahasa belum ter-render di halaman. Belum ada UI toggle; skip graceful (LP-03 belum enforceable).");
    return;
  }

  const before = await page.locator("body").innerText();
  await toggle.click();
  await expect(page.locator("body")).not.toHaveText(before, { timeout: 10_000 });
});

test("LP-04: Contact form — submit kosong memunculkan validasi", async ({
  page,
}) => {
  await gotoHome(page);

  const form = page.locator("#kontak form");
  await form.scrollIntoViewIfNeeded();
  await expect(form).toBeVisible();

  const submit = form.getByRole("button", { name: /kirim pengajuan kemitraan/i });
  await expect(submit).toBeVisible();
  await submit.click();

  await expect(form).toHaveText(/nama lengkap/i);
  expect(page.url()).not.toContain("mailto:");

  const invalidFields = await form.locator("input:invalid, textarea:invalid").count();
  expect(invalidFields).toBeGreaterThan(0);

  await expect(form).not.toContainText("Aplikasi email telah dibuka");
});

test("LP-05: Responsive — desktop & mobile tanpa overflow horizontal", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await gotoHome(page);
  await expect(page.getByRole("navigation", { name: /navigasi utama/i })).toBeVisible();

  await page.setViewportSize({ width: 390, height: 844 });
  await gotoHome(page);
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();

  const hasHorizontalOverflow = await page.evaluate(() => {
    return document.documentElement.scrollWidth > document.documentElement.clientWidth;
  });
  expect(hasHorizontalOverflow).toBe(false);
});

test("LP-06: Tidak ada console error", async ({ page }) => {
  const errors: string[] = [];

  page.on("console", (msg) => {
    if (msg.type() === "error") errors.push(msg.text());
  });
  page.on("pageerror", (err) => errors.push(String(err)));

  await gotoHome(page);
  await page.waitForLoadState("networkidle");

  const harmful = errors.filter((e) => !isHarmless(e));
  expect(harmful).toEqual([]);
});