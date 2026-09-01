# G-Flow API Reference

Ringkasan endpoint utama G-Flow Super-App. Detail lengkap request/response ada di
[`docs/API CONTRACT.txt`](./API%20CONTRACT.txt) dan spesifikasi mesin-readable di
[`docs/openapi.yaml`](./openapi.yaml).

**Base URL:**
- Local: `http://localhost:8080`
- Production (asumsi): `https://api.g-flow.com`

**Autentikasi:** `Authorization: Bearer <access_token>` (JWT, 1 jam). Refresh token 7 hari
via `POST /auth/refresh-token`.

**Response envelope:**
```json
{ "success": true, "data": {}, "meta": { "timestamp": "...", "request_id": "...", "version": "1.0" } }
```
**Error envelope:**
```json
{ "success": false, "error": { "code": "ERROR_CODE", "message": "...", "details": {} } }
```

## Kode Error Standar

| Code | HTTP | Keterangan |
|------|------|------------|
| INVALID_REQUEST | 400 | Request malformed / field hilang |
| UNAUTHORIZED | 401 | Token invalid / tidak ada |
| FORBIDDEN | 403 | Token valid tapi role kurang |
| NOT_FOUND | 404 | Resource tidak ada |
| CONFLICT | 409 | Race condition / idempotency conflict |
| UNPROCESSABLE_ENTITY | 422 | Pelanggaran business logic |
| RATE_LIMIT_EXCEEDED | 429 | Terlalu banyak request |
| INTERNAL_SERVER_ERROR | 500 | Error tak terduga |
| SERVICE_UNAVAILABLE | 503 | Database down |

## Header Umum

| Header | Dibutuhkan | Keterangan |
|--------|-----------|------------|
| `Authorization` | Wajib (kecuali public) | `Bearer <token>` |
| `X-Idempotency-Key` | Untuk POST tulis | Mencegah duplikasi (topup, transfer, booking) |
| `X-Admin-2FA-Token` | Khusus admin reversal | TOTP token |
| `Idempotency-Key` | Webhook | Key idempotensi |

---

## Auth

| Method | Path | Deskripsi |
|--------|------|-----------|
| POST | `/api/v1/auth/register` | Daftar user (customer/driver/merchant) |
| POST | `/api/v1/auth/login` | Login → `{ access_token, refresh_token, user_id, user_type }` |
| POST | `/api/v1/auth/logout` | Logout / revoke token |
| POST | `/api/v1/auth/refresh-token` | Refresh access token |

---

## Wallet (PayPulse)

| Method | Path | Deskripsi |
|--------|------|-----------|
| POST | `/api/v1/wallets/{id}/topup` | Top-up wallet (idempotent) |
| POST | `/api/v1/wallets/{id}/transfer` | Transfer P2P (idempotent) |
| GET | `/api/v1/wallets/{id}/balance` | Cek saldo |
| POST | `/api/v1/wallets/{id}/withdrawal` | Withdrawal (driver/merchant) |
| GET | `/api/v1/ledger/audit` | Audit trail ledger (admin) |

---

## Ride (G-Ride)

| Method | Path | Deskripsi |
|--------|------|-----------|
| POST | `/api/v1/rides/book` | Booking ride (escrow) |
| POST | `/api/v1/rides/{id}/accept` | Driver accept |
| PATCH | `/api/v1/rides/{id}/status` | Update status (ARRIVED/STARTED/COMPLETED/CANCELLED) |
| GET | `/api/v1/rides/{id}` | Detail ride |
| GET | `/api/v1/rides/available` | Order tersedia untuk driver |

State machine: `CREATED → SEARCHING_DRIVER → DRIVER_ASSIGNED → DRIVER_ARRIVED → TRIP_STARTED → COMPLETED → SETTLED`

---

## Food (G-Food)

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET | `/api/v1/merchants` | List merchant (optional auth) |
| GET | `/api/v1/merchants/{id}/menu` | Menu + variant option groups |
| POST | `/api/v1/food-orders` | Buat order makanan |
| GET | `/api/v1/food-orders/{id}` | Detail order |
| GET | `/api/v1/food-orders/available` | Order tersedia untuk driver |

Settlement 4-way: customer → merchant / driver / platform (zero-discrepancy).

---

## Send (G-Send)

| Method | Path | Deskripsi |
|--------|------|-----------|
| POST | `/api/v1/send-orders` | Buat order pengiriman multi-stop |
| GET | `/api/v1/send-orders/{id}` | Detail order + stop (RTS status) |
| GET | `/api/v1/send-orders/available` | Order tersedia untuk driver |

Settlement 3-way + far breakdown per stop (allocated_fare). RTS: `RETURN_REQUIRED` / `RETURNED_TO_SENDER`.

---

## Driver

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET | `/api/v1/drivers/{id}/orders` | List order aktif driver |

---

## Admin (2FA untuk reversal)

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET | `/api/v1/admin/dashboard/kpis` | KPI dashboard |
| GET | `/api/v1/admin/dashboard/transactions` | Transaksi terbaru |
| GET | `/api/v1/admin/ledger` | Audit ledger |
| GET | `/api/v1/admin/ledger/verify/{wallet_id}` | Verifikasi saldo |
| POST | `/api/v1/admin/ledger/export` | Export CSV/JSON |
| GET | `/api/v1/admin/users` | List user |
| PATCH | `/api/v1/admin/users/{id}/freeze` / `/suspend` / `/ban` | Kelola user |
| POST | `/api/v1/admin/transactions/{id}/reverse` | Reversal (butuh 2FA + lockout 3x→15m) |

---

## Health

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET | `/health` | Liveness |
| GET | `/healthz/liveness` | Liveness probe (HTTP) |
| GET | `/healthz/readiness` | Readiness (DB + Redis); 503 jika tidak siap |

---

## Referensi Tambahan

- [Spesifikasi OpenAPI 3.0](./openapi.yaml)
- [API Contract lengkap](./API%20CONTRACT.txt)
- [Panduan Deployment](./DEPLOYMENT_GUIDE.md)
