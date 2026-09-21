-- MIGRATION 015: Seed data minimal untuk E2E admin web (Playwright).
-- Idempotent: aman di-re-run (ON CONFLICT DO NOTHING / WHERE NOT EXISTS).
-- Tidak menyentuh kode backend; hanya menyiapkan fixture DB agar test
-- data-dependent (AD-03 ledger, AD-04 verifikasi, AD-06 freeze, AD-07 reverse)
-- punya data deterministik yang sama di lokal maupun CI (migration runner).

-- 1) Non-admin users (customer + driver) untuk AD-05/AD-06.
--    password_hash non-valid (bukan nilai bcrypt) -> akun ini tidak bisa login,
--    hanya dipakai sebagai record manajemen pengguna.
INSERT INTO users (id, email, name, user_type, status, password_hash, kyc_status, vehicle_plate, license_number)
SELECT * FROM (VALUES
  ('70000000-0000-0000-0000-000000000001'::uuid, 'budi.santoso@g-flow.dev', 'Budi Santoso', 'customer'::user_type_enum, 'ACTIVE', 'seed-no-login', 'VERIFIED', NULL, NULL),
  ('70000000-0000-0000-0000-000000000002'::uuid, 'agus.nurbianto@g-flow.dev', 'Agus Nurbianto', 'driver'::user_type_enum, 'ACTIVE', 'seed-no-login', 'VERIFIED', 'L 1234 AB', '123456789')
) AS v(id, email, name, user_type, status, password_hash, kyc_status, vehicle_plate, license_number)
WHERE NOT EXISTS (SELECT 1 FROM users WHERE id = '70000000-0000-0000-0000-000000000001' OR id = '70000000-0000-0000-0000-000000000002')
ON CONFLICT (id) DO NOTHING;

-- 2) Wallets untuk customer + driver.
INSERT INTO wallets (id, user_id, wallet_type, balance, status)
SELECT * FROM (VALUES
  ('71000000-0000-0000-0000-000000000001'::uuid, '70000000-0000-0000-0000-000000000001'::uuid, 'CUSTOMER'::wallet_type_enum, 0, 'ACTIVE'),
  ('71000000-0000-0000-0000-000000000002'::uuid, '70000000-0000-0000-0000-000000000002'::uuid, 'DRIVER'::wallet_type_enum, 0, 'ACTIVE')
) AS v(id, user_id, wallet_type, balance, status)
WHERE NOT EXISTS (SELECT 1 FROM wallets WHERE id = '71000000-0000-0000-0000-000000000001' OR id = '71000000-0000-0000-0000-000000000002')
ON CONFLICT (id) DO NOTHING;

-- 3) Satu ride order SETTLED hari ini: memicu KPI dashboard
--    (revenue_today = platform_commission, order_status_breakdown = SETTLED:1)
--    dan pie chart Order Status Breakdown.
INSERT INTO ride_orders (
  id, customer_id, driver_id, customer_wallet_id, driver_wallet_id,
  pickup_lat, pickup_lng, pickup_address,
  dropoff_lat, dropoff_lng, dropoff_address,
  distance_km, base_fare, per_km_rate, estimated_fare, actual_fare,
  platform_commission, driver_earning, status, is_settled, settled_at
)
SELECT * FROM (VALUES (
  '60000000-0000-0000-0000-000000000001'::uuid,
  '70000000-0000-0000-0000-000000000001'::uuid,
  '70000000-0000-0000-0000-000000000002'::uuid,
  '71000000-0000-0000-0000-000000000001'::uuid,
  '71000000-0000-0000-0000-000000000002'::uuid,
  -6.20000000::numeric(10,8), 106.81666600::numeric(11,8), 'Jl. Sudirman No. 1 Jakarta',
  -6.30000000::numeric(10,8), 106.85000000::numeric(11,8), 'Jl. Thamrin No. 2 Jakarta',
  5.000::numeric(8,3), 10000.00::numeric(12,2), 4000.00::numeric(8,2),
  30000.00::numeric(12,2), 30000.00::numeric(12,2),
  6000.00::numeric(12,2), 24000.00::numeric(12,2),
  'SETTLED'::order_status_enum, TRUE, NOW()
)) AS v(
  id, customer_id, driver_id, customer_wallet_id, driver_wallet_id,
  pickup_lat, pickup_lng, pickup_address,
  dropoff_lat, dropoff_lng, dropoff_address,
  distance_km, base_fare, per_km_rate, estimated_fare, actual_fare,
  platform_commission, driver_earning, status, is_settled, settled_at
)
WHERE NOT EXISTS (SELECT 1 FROM ride_orders WHERE id = '60000000-0000-0000-0000-000000000001')
ON CONFLICT (id) DO NOTHING;

-- 4) Double-entry ledger settlement wallet:: DEBIT SYSTEM_ESCROW (fare)
--    -> CREDIT driver (80%) + CREDIT SYSTEM_PLATFORM (20%).
--    Satu statement multi-row -> constraint ledger_balance_validation
--    (DEFERRABLE INITIALLY DEFERRED) dicek saat COMMIT statement, debit == credit.
INSERT INTO ledger_entries (id, wallet_id, entry_type, amount, reference_type, reference_id, description)
SELECT * FROM (VALUES
  ('81000000-0000-0000-0000-000000000001'::uuid, '00000000-0000-0000-0000-000000000001'::uuid, 'DEBIT'::entry_type_enum, 30000.00::numeric(15,2), 'RIDE_SETTLEMENT', '60000000-0000-0000-0000-000000000001'::uuid, 'RIDE_SETTLEMENT - release escrow'),
  ('81000000-0000-0000-0000-000000000002'::uuid, '71000000-0000-0000-0000-000000000002'::uuid, 'CREDIT'::entry_type_enum, 24000.00::numeric(15,2), 'RIDE_SETTLEMENT', '60000000-0000-0000-0000-000000000001'::uuid, 'RIDE_SETTLEMENT - driver earning 80%'),
  ('81000000-0000-0000-0000-000000000003'::uuid, '00000000-0000-0000-0000-000000000002'::uuid, 'CREDIT'::entry_type_enum, 6000.00::numeric(15,2), 'RIDE_SETTLEMENT', '60000000-0000-0000-0000-000000000001'::uuid, 'RIDE_SETTLEMENT - platform commission 20%')
) AS v(id, wallet_id, entry_type, amount, reference_type, reference_id, description)
WHERE NOT EXISTS (SELECT 1 FROM ledger_entries WHERE reference_id = '60000000-0000-0000-0000-000000000001')
ON CONFLICT (id) DO NOTHING;