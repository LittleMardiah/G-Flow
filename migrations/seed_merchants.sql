-- ============================================================================
-- SEED: PHASE 3 — FOOD MERCHANTS (Task 3.1)
-- ============================================================================
-- 5 merchant seed: food_merchants + merchant_menus + merchant_items,
-- lengkap dengan users (user_type='merchant') & wallets ( MERCHANT ).
--
-- UUID FIXED deterministik (tetap) per merchant:
--   merchant i:
--     user_id    = 10000000-0000-0000-0000-00000000000{i}
--     wallet_id  = 20000000-0000-0000-0000-00000000000{i}
--     merchant_id= 30000000-0000-0000-0000-00000000000{i}
--   menu & item id memakai pola 4/5xxxxx-.... yang tetap.
-- Semua INSERT idempotent (ON CONFLICT DO NOTHING) sehingga aman diulang.
-- ============================================================================

BEGIN;

-- ============================================================================
-- 1) USERS (merchant accounts)
-- ============================================================================
INSERT INTO users (id, email, phone, name, user_type, status, password_hash, kyc_status, created_at, updated_at)
VALUES
  ('10000000-0000-0000-0000-000000000001', 'warung.nusantara@g-flow.dev',    '+6281200000001', 'Warung Nusantara',       'merchant', 'ACTIVE', 'SEED_USER_NOLOGIN', 'VERIFIED', NOW(), NOW()),
  ('10000000-0000-0000-0000-000000000002', 'bakmi.jawa.pakdhe@g-flow.dev',  '+6281200000002', 'Bakmi Jawa Pak Dhe',     'merchant', 'ACTIVE', 'SEED_USER_NOLOGIN', 'VERIFIED', NOW(), NOW()),
  ('10000000-0000-0000-0000-000000000003', 'ayam.geprek.bawang@g-flow.dev', '+6281200000003', 'Ayam Geprek Sambal Bawang', 'merchant', 'ACTIVE', 'SEED_USER_NOLOGIN', 'VERIFIED', NOW(), NOW()),
  ('10000000-0000-0000-0000-000000000004', 'seblak.jebred@g-flow.dev',      '+6281200000004', 'Seblak Jebred',          'merchant', 'ACTIVE', 'SEED_USER_NOLOGIN', 'VERIFIED', NOW(), NOW()),
  ('10000000-0000-0000-0000-000000000005', 'kopi.kita@g-flow.dev',          '+6281200000005', 'Kopi Kita',              'merchant', 'ACTIVE', 'SEED_USER_NOLOGIN', 'VERIFIED', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 2) WALLETS (MERCHANT)
-- ============================================================================
INSERT INTO wallets (id, user_id, wallet_type, balance, status, created_at, updated_at)
VALUES
  ('20000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001', 'MERCHANT', 0, 'ACTIVE', NOW(), NOW()),
  ('20000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002', 'MERCHANT', 0, 'ACTIVE', NOW(), NOW()),
  ('20000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000003', 'MERCHANT', 0, 'ACTIVE', NOW(), NOW()),
  ('20000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000004', 'MERCHANT', 0, 'ACTIVE', NOW(), NOW()),
  ('20000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000005', 'MERCHANT', 0, 'ACTIVE', NOW(), NOW())
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 3) FOOD MERCHANTS
-- ============================================================================
INSERT INTO food_merchants (
  id, user_id, merchant_name, merchant_description, category,
  latitude, longitude, address, phone,
  avg_rating, total_reviews, total_orders,
  opening_time, closing_time, is_open,
  status, verified_at, logo_url
) VALUES
  (
    '30000000-0000-0000-0000-000000000001', '10000000-0000-0000-0000-000000000001',
    'Warung Nusantara', 'Masakan rumahan Indonesia: ayam bakar, nasi goreng, sop iga.', 'INDONESIAN',
    -6.20000000, 106.81666667, 'Jl. Sumatera No. 21, Menteng, Jakarta Pusat', '+6281200000001',
    4.8, 1240, 9870, '08:00', '22:00', TRUE, 'ACTIVE', NOW(), 'https://media.g-flow.dev/seed/warung-nusantara.png'
  ),
  (
    '30000000-0000-0000-0000-000000000002', '10000000-0000-0000-0000-000000000002',
    'Bakmi Jawa Pak Dhe', 'Bakmi Jawa godhog & goreng dengan bumbu khas keluarga.', 'NOODLES',
    -6.21000000, 106.84500000, 'Jl. Bungur Besar No. 45, Gondangdia, Jakarta Pusat', '+6281200000002',
    4.6, 860, 6230, '10:00', '23:00', TRUE, 'ACTIVE', NOW(), 'https://media.g-flow.dev/seed/bakmi-pakdhe.png'
  ),
  (
    '30000000-0000-0000-0000-000000000003', '10000000-0000-0000-0000-000000000003',
    'Ayam Geprek Sambal Bawang', 'Ayam geprek dengan sambal bawang rahasia, level 1-10.', 'FRIED_CHICKEN',
    -6.29000000, 106.80000000, 'Jl. Kemang Timur No. 7, Kemang, Jakarta Selatan', '+6281200000003',
    4.7, 2100, 15430, '09:00', '22:30', TRUE, 'ACTIVE', NOW(), 'https://media.g-flow.dev/seed/ayam-geprek.png'
  ),
  (
    '30000000-0000-0000-0000-000000000004', '10000000-0000-0000-0000-000000000004',
    'Seblak Jebred', 'Seblak basah autentik Bandung: ceker, bakso, sosis, telur.', 'SNACKS',
    -6.32000000, 106.71800000, 'Jl. Lingkar Luar Barat, Kalideres, Jakarta Barat', '+6281200000004',
    4.5, 540, 3890, '11:00', '23:59', TRUE, 'ACTIVE', NOW(), 'https://media.g-flow.dev/seed/seblak-jebred.png'
  ),
  (
    '30000000-0000-0000-0000-000000000005', '10000000-0000-0000-0000-000000000005',
    'Kopi Kita', 'Coffee shop: specialty coffee dan non-kopi, ruang kerja nyaman.', 'COFFEE',
    -6.24000000, 106.83000000, 'Jl. Cikini IV No. 33, Cikini, Jakarta Pusat', '+6281200000005',
    4.9, 1750, 11200, '07:00', '21:00', TRUE, 'ACTIVE', NOW(), 'https://media.g-flow.dev/seed/kopi-kita.png'
  )
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 4) MERCHANT MENUS
-- ============================================================================
INSERT INTO merchant_menus (id, merchant_id, name, description, sequence_order, is_active)
VALUES
  -- Merchant 1: Warung Nusantara
  ('40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Menu Utama', 'Hidangan utama khas Nusantara', 1, TRUE),
  ('40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'Minuman', 'Minuman segar', 2, TRUE),
  -- Merchant 2: Bakmi Jawa Pak Dhe
  ('40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002', 'Menu Utama', 'Bakmi godhog & goreng', 1, TRUE),
  ('40000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000002', 'Topping & Pelengkap', 'Tambahan topping', 2, TRUE),
  -- Merchant 3: Ayam Geprek Sambal Bawang
  ('40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003', 'Paket Ayam Geprek', 'Paket ayam geprek + nasi', 1, TRUE),
  ('40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000003', 'Tambahan', 'Nasi & minuman', 2, TRUE),
  -- Merchant 4: Seblak Jebred
  ('40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000004', 'Seblak', 'Pilihan seblak basah', 1, TRUE),
  ('40000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000004', 'Cemilan & Minuman', 'Cemilan & minuman', 2, TRUE),
  -- Merchant 5: Kopi Kita
  ('40000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000005', 'Kopi', 'Specialty coffee', 1, TRUE),
  ('40000000-0000-0000-0000-00000000000A', '30000000-0000-0000-0000-000000000005', 'Non-Kopi', 'Minuman tanpa kopi', 2, TRUE)
ON CONFLICT (id) DO NOTHING;

-- ============================================================================
-- 5) MERCHANT ITEMS
-- ============================================================================
INSERT INTO merchant_items (id, menu_id, merchant_id, name, description, price, image_url, stock, is_available)
VALUES
  -- Merchant 1: Warung Nusantara — Menu Utama
  ('50000000-0000-0000-0000-000000000001', '40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Nasi + Ayam Bakar Madu', 'Ayam bakar madu, sambal terasi, lalapan', 25000, 'https://media.g-flow.dev/seed/warung-ayam-bakar.png', 50, TRUE),
  ('50000000-0000-0000-0000-000000000002', '40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Nasi Goreng Kampung', 'Nasi goreng bumbu kampung + telur ceplok', 22000, 'https://media.g-flow.dev/seed/warung-nasgor.png', 40, TRUE),
  ('50000000-0000-0000-0000-000000000003', '40000000-0000-0000-0000-000000000001', '30000000-0000-0000-0000-000000000001', 'Sop Iga Sapi', 'Sop iga empuk, wortel & kentang', 45000, 'https://media.g-flow.dev/seed/warung-sop-iga.png', 20, TRUE),
  -- Merchant 1: Warung Nusantara — Minuman
  ('50000000-0000-0000-0000-000000000004', '40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'Es Teh Manis', 'Teh manis dingin', 5000, 'https://media.g-flow.dev/seed/warung-esteh.png', 100, TRUE),
  ('50000000-0000-0000-0000-000000000005', '40000000-0000-0000-0000-000000000002', '30000000-0000-0000-0000-000000000001', 'Es Jeruk', 'Jeruk peras dingin', 7000, 'https://media.g-flow.dev/seed/warung-esjeruk.png', 100, TRUE),
  -- Merchant 2: Bakmi Jawa Pak Dhe — Menu Utama
  ('50000000-0000-0000-0000-000000000006', '40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002', 'Bakmi Godhog Jumbo', 'Bakmi godhog porsi jumbo + ayam suwir', 18000, 'https://media.g-flow.dev/seed/bakmi-godhog.png', 80, TRUE),
  ('50000000-0000-0000-0000-000000000007', '40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002', 'Bakmi Goreng Komplit', 'Bakmi goreng + ayam, telur, bakso', 20000, 'https://media.g-flow.dev/seed/bakmi-goreng.png', 80, TRUE),
  ('50000000-0000-0000-0000-000000000008', '40000000-0000-0000-0000-000000000003', '30000000-0000-0000-0000-000000000002', 'Mie Goreng + Telur', 'Mie goreng + telur dadar', 16000, 'https://media.g-flow.dev/seed/bakmi-telur.png', 80, TRUE),
  -- Merchant 2: Bakmi Jawa Pak Dhe — Topping
  ('50000000-0000-0000-0000-000000000009', '40000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000002', 'Extra Ayam Suwir', 'Tambahan ayam suwir', 8000, 'https://media.g-flow.dev/seed/bakmi-ayam.png', 100, TRUE),
  ('50000000-0000-0000-0000-00000000000A', '40000000-0000-0000-0000-000000000004', '30000000-0000-0000-0000-000000000002', 'Kerupuk', 'Kerupuk udang', 2000, 'https://media.g-flow.dev/seed/bakmi-kerupuk.png', 200, TRUE),
  -- Merchant 3: Ayam Geprek Sambal Bawang — Paket
  ('50000000-0000-0000-0000-00000000000B', '40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003', 'Ayam Geprek Original + Nasi', 'Level sambal bisa dipilih', 20000, 'https://media.g-flow.dev/seed/geprek-original.png', 100, TRUE),
  ('50000000-0000-0000-0000-00000000000C', '40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003', 'Ayam Geprek Level 5 + Nasi', 'Sambal level 5 ekstra pedas', 21000, 'https://media.g-flow.dev/seed/geprek-level5.png', 100, TRUE),
  ('50000000-0000-0000-0000-00000000000D', '40000000-0000-0000-0000-000000000005', '30000000-0000-0000-0000-000000000003', 'Ayam Geprek Mozzarella + Nasi', 'Ayam geprek + topping keju mozzarella', 28000, 'https://media.g-flow.dev/seed/geprek-mozzarella.png', 60, TRUE),
  -- Merchant 3: Ayam Geprek Sambal Bawang — Tambahan
  ('50000000-0000-0000-0000-00000000000E', '40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000003', 'Nasi Putih', 'Nasi putih hangat', 5000, 'https://media.g-flow.dev/seed/nasi.png', 300, TRUE),
  ('50000000-0000-0000-0000-00000000000F', '40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000003', 'Es Teh Manis', 'Teh manis dingin', 5000, 'https://media.g-flow.dev/seed/esteh.png', 300, TRUE),
  ('50000000-0000-0000-0000-000000000010', '40000000-0000-0000-0000-000000000006', '30000000-0000-0000-0000-000000000003', 'Perasan Jeruk Nipis', 'Perasan jeruk nipis segar', 3000, 'https://media.g-flow.dev/seed/jeruk-nipis.png', 300, TRUE),
  -- Merchant 4: Seblak Jebred — Seblak
  ('50000000-0000-0000-0000-000000000011', '40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000004', 'Seblak Basah Original', 'Seblak basah kerupuk + bakso', 15000, 'https://media.g-flow.dev/seed/seblak-original.png', 90, TRUE),
  ('50000000-0000-0000-0000-000000000012', '40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000004', 'Seblak Ceker', 'Seblak + ceker ayam empuk', 20000, 'https://media.g-flow.dev/seed/seblak-ceker.png', 70, TRUE),
  ('50000000-0000-0000-0000-000000000013', '40000000-0000-0000-0000-000000000007', '30000000-0000-0000-0000-000000000004', 'Seblak Premium + Telur & Sosis', 'Seblak lengkap + telur & sosis', 25000, 'https://media.g-flow.dev/seed/seblak-premium.png', 70, TRUE),
  -- Merchant 4: Seblak Jebred — Cemilan & Minuman
  ('50000000-0000-0000-0000-000000000014', '40000000-0000-0000-0000-000000000008', '30000000-0000-0000-0000-000000000004', 'Es Jeruk', 'Jeruk peras dingin', 7000, 'https://media.g-flow.dev/seed/seblak-esjeruk.png', 100, TRUE),
  -- Merchant 5: Kopi Kita — Kopi
  ('50000000-0000-0000-0000-000000000015', '40000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000005', 'Es Kopi Susu Gula Aren', 'Espresso + susu + gula aren', 18000, 'https://media.g-flow.dev/seed/kopi-susu.png', 150, TRUE),
  ('50000000-0000-0000-0000-000000000016', '40000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000005', 'Americano', 'Espresso + air panas', 15000, 'https://media.g-flow.dev/seed/americano.png', 150, TRUE),
  ('50000000-0000-0000-0000-000000000017', '40000000-0000-0000-0000-000000000009', '30000000-0000-0000-0000-000000000005', 'Cappuccino', 'Espresso + susu microfoam', 20000, 'https://media.g-flow.dev/seed/cappuccino.png', 150, TRUE),
  -- Merchant 5: Kopi Kita — Non-Kopi
  ('50000000-0000-0000-0000-000000000018', '40000000-0000-0000-0000-00000000000A', '30000000-0000-0000-0000-000000000005', 'Teh Tarik', 'Teh tarik creamy khas Melayu', 12000, 'https://media.g-flow.dev/seed/teh-tarik.png', 100, TRUE),
  ('50000000-0000-0000-0000-000000000019', '40000000-0000-0000-0000-00000000000A', '30000000-0000-0000-0000-000000000005', 'Matcha Latte', 'Matcha premium + susu', 22000, 'https://media.g-flow.dev/seed/matcha-latte.png', 100, TRUE)
ON CONFLICT (id) DO NOTHING;

COMMIT;