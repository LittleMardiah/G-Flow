import { test, expect, Page } from "@playwright/test";

async function snap(page: Page, name: string) {
  await test.info().attach(name, {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
}

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin@g-flow.local";
const ADMIN_PASSWORD =
  process.env.E2E_ADMIN_PASSWORD ?? "AdminP@ssw0rd!2026";

test("AD-01: Admin login — AuthGuard redirect & login sukses", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/, { timeout: 30_000 });
  await expect(
    page.getByRole("heading", { name: "G-Flow Admin Login" }),
  ).toBeVisible();
  await snap(page, "01-login-page");

  await page.locator('input[type="email"]').fill(ADMIN_EMAIL);
  await page.locator('input[type="password"]').first().fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Login", exact: true }).click();

  await expect(page).toHaveURL(/\/$/, { timeout: 30_000 });
  await expect(
    page.getByRole("heading", { name: "Dashboard" }),
  ).toBeVisible();

  const hasToken = await page.evaluate(() => {
    const token = window.localStorage.getItem("admin_token");
    return typeof token === "string" && token.length > 0;
  });
  expect(hasToken).toBe(true);

  await snap(page, "02-dashboard-after-login");
});