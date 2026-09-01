-- ============================================================================
-- MIGRATION 012: OVERDUE DEBT AUTO-SYNC TRIGGER (TD-011)
-- ============================================================================
-- Overdue debt (users.overdue_debt, Bug #47) sebelumnya hanya di-set pada
-- jalur top-up (pelunasan) dan di-gate saat booking/order (Debt Gate
-- Universal). TD-011 menstandarkan pencatatan hutang agar konsisten pada
-- SEMUA jalur settlement (ride / food / send): bila saldo wallet CUSTOMER
-- menjadi negatif (mis. delta fare / shortfall settlement), debt otomatis
-- dicatat ke users.overdue_debt.
--
-- Semantik:
--   - Hanya wallet jenis CUSTOMER yang dipantau (system/driver tidak kena).
--   - Bila NEW.balance < 0  => users.overdue_debt = max(overdue_debt, -balance)
--     (konservatif: memastikan hutang setidaknya menutup kekurangan agar
--     balance kembali ke nol saat top-up berikutnya; tidak menghapus hutang
--     lama yang belum lunas).
--   - Bila NEW.balance >= 0 => trigger tidak mengubah overdue_debt (jalur
--     top-up yang menetapkan sisa hutang).
-- ============================================================================

CREATE OR REPLACE FUNCTION fn_sync_overdue_debt_from_wallet()
RETURNS TRIGGER AS $$
DECLARE
  v_current DECIMAL(15,2);
BEGIN
  -- Hanya pantau wallet jenis CUSTOMER.
  IF NEW.wallet_type <> 'CUSTOMER' THEN
    RETURN NEW;
  END IF;

  -- Saldo tidak boleh negatif? Sebaliknya: saldo negatif justru menjadi modal
  -- hutang. Catat kekurangan (nilai absolut) ke users.overdue_debt.
  IF NEW.balance < 0 THEN
    SELECT COALESCE(overdue_debt, 0) INTO v_current FROM users WHERE id = NEW.user_id;

    IF v_current IS NOT NULL AND (-NEW.balance) > v_current THEN
      UPDATE users
         SET overdue_debt = (-NEW.balance),
             updated_at   = NOW()
       WHERE id = NEW.user_id;
    END IF;
  END IF;

  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_sync_overdue_debt ON wallets;

CREATE TRIGGER trg_sync_overdue_debt
  AFTER INSERT OR UPDATE OF balance ON wallets
  FOR EACH ROW
  EXECUTE FUNCTION fn_sync_overdue_debt_from_wallet();

-- ============================================================================
-- END OF MIGRATION 012
-- ============================================================================
