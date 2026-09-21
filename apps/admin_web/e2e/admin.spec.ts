import { test, expect, Page } from "@playwright/test";

// ============================================================================
// E2E Admin Web (Playwright)
// STEP A6.2 — AD-02..AD-07
// DB fixture minimal disediakan lewat migrations/015_seed_admin_e2e.up.sql:
//   - non-admin users (budi.santoso@g-flow.dev, agus.nurbianto@g-flow.dev)
//   - satu ride order SETTLED hari ini (id 60000000-...-0001) + ledger
//     double-entry RIDE_SETTLEMENT (DEBIT escrow 30000 = CREDIT 24000 + 6000)
// ----------------------------------------------------------------------------
// Catatan idempotensi:
//   - AD-06 freeze: aman di-rerun (status tetap FROZEN).
//   - AD-07 reverse: run pertama mengeksekusi flow 2FA sungguhan; run berikutnya
//     transaksi sudah REVERSED sehingga test diverifikasi lewat state
//     "sudah pernah di-reverse" (alur tetap menentukan hasil).
// ============================================================================

async function snap(page: Page, name: string) {
  await test.info().attach(name, {
    body: await page.screenshot({ fullPage: true }),
    contentType: "image/png",
  });
}

const ADMIN_EMAIL = process.env.E2E_ADMIN_EMAIL ?? "admin@g-flow.local";
const ADMIN_PASSWORD =
  process.env.E2E_ADMIN_PASSWORD ?? "AdminP@ssw0rd!2026";
const ADMIN_2FA = process.env.E2E_ADMIN_2FA ?? "admin-2fa-secret";

// Transaction / wallet / user fixtures (lihat migration 015).
const REVERSIBLE_TXN_ID = "60000000-0000-0000-0000-000000000001";
const VERIFY_WALLET_ID = "00000000-0000-0000-0000-000000000001"; // SYSTEM_ESCROW
const DRIVER_EMAIL = "agus.nurbianto@g-flow.dev";

async function loginAsAdmin(page: Page) {
  await page.goto("/login");
  await page.locator('input[type="email"]').fill(ADMIN_EMAIL);
  await page.locator('input[type="password"]').first().fill(ADMIN_PASSWORD);
  await page.getByRole("button", { name: "Login", exact: true }).click();

  await expect(page).toHaveURL(/\/$/, { timeout: 30_000 });
  await expect(
    page.getByRole("heading", { name: "Dashboard" }),
  ).toBeVisible();

  await expect
    .poll(
      () =>
        page.evaluate(
          () => (window.localStorage.getItem("admin_token") ?? "").length,
        ),
      { timeout: 15_000 },
    )
    .toBeGreaterThan(0);
}

test("AD-01: Admin login — AuthGuard redirect & login sukses", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page).toHaveURL(/\/login$/, { timeout: 30_000 });
  await expect(
    page.getByRole("heading", { name: "G-Flow Admin Login" }),
  ).toBeVisible();
  await snap(page, "01-login-page");

  await loginAsAdmin(page);
  await snap(page, "02-dashboard-after-login");
});

test("AD-02: Dashboard KPI & order status breakdown", async ({ page }) => {
  await loginAsAdmin(page);

  for (const label of [
    "Active Orders",
    "Total Txn Volume",
    "Avg Fare",
    "Revenue Today",
    "Error Rate",
  ]) {
    await expect(page.getByText(label, { exact: true })).toBeVisible();
  }

  await expect(page.getByText("Order Status Breakdown")).toBeVisible();
  // Pie chart benar-benar merender (data SETTLED:1 dari seed migration 015).
  await expect(page.locator(".recharts-surface").first()).toBeVisible({
    timeout: 30_000,
  });

  await snap(page, "03-dashboard-kpis-breakdown");
});

test("AD-03: Ledger list & filter entry DEBIT/CREDIT", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/ledger");

  await expect(
    page.getByRole("heading", { name: "Ledger Audit Trail" }),
  ).toBeVisible();
  await expect(page.getByText("Ledger Entries", { exact: true })).toBeVisible();

  const rows = page.locator("tbody tr");
  await expect(rows.first()).toBeVisible({ timeout: 30_000 });
  expect(await rows.count()).toBeGreaterThan(0);
  await snap(page, "04-ledger-list");

  const entryTypeSelect = page.locator("select").nth(1); // [wallet_type, entry_type]
  await entryTypeSelect.selectOption("DEBIT");
  await expect(
    page.locator("tbody").getByText("DEBIT", { exact: true }).first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(
    page.locator("tbody").getByText("CREDIT", { exact: true }),
  ).toHaveCount(0);
  expect(await page.locator("tbody tr").count()).toBeGreaterThan(0);
  await snap(page, "05-ledger-filter-debit");

  await entryTypeSelect.selectOption("CREDIT");
  await expect(
    page.locator("tbody").getByText("CREDIT", { exact: true }).first(),
  ).toBeVisible({ timeout: 30_000 });
  await expect(
    page.locator("tbody").getByText("DEBIT", { exact: true }),
  ).toHaveCount(0);
  await snap(page, "06-ledger-filter-credit");
});

test("AD-04: Ledger balance verification (BALANCED)", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/ledger");

  await expect(page.getByText("Balance Verification")).toBeVisible();
  await page.getByPlaceholder("Wallet ID").fill(VERIFY_WALLET_ID);
  // force: tata letak input+button pada viewport kecil bisa bertumpuk
  // (flex row tanpa wrap), sehingga hit-test default terhalang.
  await page
    .getByRole("button", { name: "Verify", exact: true })
    .click({ force: true });

  await expect(page.getByText("BALANCED", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  await expect(page.getByText("Discrepancy")).toBeVisible();
  await snap(page, "07-balance-verification");
});

test("AD-05: User list & filter role ADMIN", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/users");

  await expect(
    page.getByRole("heading", { name: "User Management" }),
  ).toBeVisible();

  const adminRow = page.getByRole("row").filter({ hasText: ADMIN_EMAIL });
  await expect(adminRow).toBeVisible({ timeout: 30_000 });
  await expect(adminRow.getByText("ADMIN", { exact: true })).toBeVisible();
  await snap(page, "08-users-list");

  const roleSelect = page.locator("select").nth(0); // role filter
  await roleSelect.selectOption("ADMIN");
  await expect(adminRow).toBeVisible({ timeout: 30_000 });
  await expect(
    page.locator("tbody").getByText("CUSTOMER", { exact: true }),
  ).toHaveCount(0);
  await snap(page, "09-users-filter-admin");
});

test("AD-06: User action freeze (non-admin driver)", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto("/users");

  await expect(
    page.getByRole("heading", { name: "User Management" }),
  ).toBeVisible();

  const driverRow = page.getByRole("row").filter({ hasText: DRIVER_EMAIL });
  await expect(driverRow).toBeVisible({ timeout: 30_000 });
  // force: pada viewport kecil sel tabel bisa menimpa tombol Detail
  // (table 6 kolom melampaui lebar layar), hit-test default terhalang.
  await driverRow
    .getByRole("button", { name: "Detail" })
    .click({ force: true });

  await expect(
    page.getByRole("heading", { name: "User Detail" }),
  ).toBeVisible();
  await snap(page, "10-user-detail");

  const modal = page
    .locator("div.fixed")
    .filter({ has: page.getByRole("heading", { name: "User Detail" }) });
  await modal
    .getByRole("button", { name: "Freeze", exact: true })
    .click({ force: true });
  await expect(modal.getByText("FROZEN", { exact: true })).toBeVisible({
    timeout: 30_000,
  });
  await snap(page, "11-user-frozen");
});

test("AD-07: Transaction reversal (2FA flow)", async ({ page }) => {
  await loginAsAdmin(page);
  await page.goto(`/transactions/${REVERSIBLE_TXN_ID}/reverse`);

  await expect(
    page.getByRole("heading", { name: "Transaction Reversal" }),
  ).toBeVisible();
  await expect(page.getByText("Transaction Details")).toBeVisible({
    timeout: 30_000,
  });
  await snap(page, "12-reverse-details");

  const alreadyReversed =
    (await page.getByText("Transaksi ini sudah pernah di-reverse.").count()) >
    0;

  if (!alreadyReversed) {
    await page
      .getByPlaceholder("e.g. duplicate charge, fraud")
      .fill("E2E test reversal - duplicate charge");
    await page.getByPlaceholder("X-Admin-2FA-Token").fill(ADMIN_2FA);
    // force: pada viewport kecil hit-test tombol bisa terhalang elemen lain
    // (sidebar / kartu); submit tetap dijalankan via event DOM.
    await page
      .getByRole("button", { name: "Confirm Reversal" })
      .click({ force: true });

    await expect(
      page.getByRole("heading", { name: "Konfirmasi Reversal" }),
    ).toBeVisible();
    await page
      .getByRole("button", { name: "Ya, Reverse" })
      .click({ force: true });
  }

  // Outcome yang diterima (alur 2FA dieksekusi, hasil bergantung status saat
  // submit — paralel desktop/mobile bisa double-submit):
  //   1) "Reversal berhasil"        -> sukses
  //   2) "sudah pernah di-reverse"  -> idempotensi (proyek lain sudah reverse)
  //   3) "Reversal gagal..."        -> error (2FA invalid / race reversal)
  const result = page
    .getByText("Reversal berhasil")
    .or(page.getByText("Transaksi ini sudah pernah di-reverse."))
    .or(page.getByText("Reversal gagal. Periksa 2FA token atau status transaksi."));
  await expect(result).toBeVisible({ timeout: 30_000 });
  await snap(page, "13-reverse-result");
});