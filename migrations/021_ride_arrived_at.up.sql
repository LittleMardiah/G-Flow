-- Migration 021: ride_orders.arrived_at (TD-129)
-- Kolom untuk track waktu driver tiba di pickup (DRIVER_ARRIVED).
-- Dipakai untuk enforce no-show rule >5 menit (NO_SHOW cancel validation).
-- Nullable (order yang belum DRIVER_ARRIVED = NULL).

ALTER TABLE ride_orders ADD COLUMN IF NOT EXISTS arrived_at TIMESTAMPTZ;
