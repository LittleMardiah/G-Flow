// Package auth — HTTP Handler (Gin) untuk Authentication & RBAC.
//
// Handler ini hanya bertugas memetakan request HTTP menjadi query DB dan
// panggilan JWTService/BlacklistService, serta men-map error ke status code +
// error code sesuai API_CONTRACT. Tidak mengandung business logic lain.
package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

// DB adalah subset operasi pool yang dipakai Handler. Dipenuhi oleh
// *pgxpool.Pool (produksi) dan pgxmock (test).
type DB interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// allowedUserTypes adalah daftar role yang boleh mendaftar langsung.
// Role 'system' tidak boleh mendaftar via public API (khusus internal).
var allowedUserTypes = map[string]bool{
	"customer": true,
	"driver":   true,
	"merchant": true,
}

// Handler menerima request HTTP terkait autentikasi.
type Handler struct {
	jwtService *JWTService
	blacklist  *BlacklistService
	db         DB
}

// NewHandler membuat Handler baru dengan dependency injection.
func NewHandler(jwtService *JWTService, blacklist *BlacklistService, db DB) *Handler {
	return &Handler{jwtService: jwtService, blacklist: blacklist, db: db}
}

// request bodies.

type loginRequestBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequestBody struct {
	Email         string  `json:"email"`
	Password      string  `json:"password"`
	Name          string  `json:"name"`
	UserType      string  `json:"user_type"`
	Phone         string  `json:"phone"`
	VehicleType   *string `json:"vehicle_type"`   // wajib untuk driver (opsional lainnya)
	VehiclePlate  *string `json:"vehicle_plate"`  // wajib untuk driver (syarat CHECK constraint schema)
	LicenseNumber *string `json:"license_number"` // wajib untuk driver (syarat CHECK constraint schema)
}

// --- endpoints ---

// Login POST /auth/login
func (h *Handler) Login(c *gin.Context) {
	var body loginRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}
	if strings.TrimSpace(body.Email) == "" || body.Password == "" {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "email dan password wajib diisi")
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))

	var (
		userID   uuid.UUID
		emailDB  string
		userType string
		status   string
		hash     string
	)
	err := h.db.QueryRow(c.Request.Context(),
		`SELECT id, email, user_type, status, password_hash FROM users WHERE email = $1`,
		email,
	).Scan(&userID, &emailDB, &userType, &status, &hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email atau password salah")
			return
		}
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "query failed")
		return
	}

	if status != "ACTIVE" {
		writeError(c, http.StatusForbidden, "ACCOUNT_INACTIVE", "akun tidak aktif (suspended/frozen)")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(body.Password)); err != nil {
		writeError(c, http.StatusUnauthorized, "INVALID_CREDENTIALS", "email atau password salah")
		return
	}

	accessToken, refreshToken, err := h.jwtService.GenerateToken(userID, emailDB, userType)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal membuat token")
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"user_id":       userID,
			"user_type":     userType,
		},
	})
}

// Register POST /auth/register
// Membuat user (kyc_status default UNVERIFIED, status ACTIVE) + wallet
// CUSTOMER dalam satu transaksi DB (atomik).
func (h *Handler) Register(c *gin.Context) {
	var body registerRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		writeError(c, http.StatusBadRequest, "INVALID_REQUEST", "invalid request body")
		return
	}

	email := strings.ToLower(strings.TrimSpace(body.Email))
	if !validEmail(email) {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_EMAIL", "email tidak valid")
		return
	}
	if len(body.Password) < 8 {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_PASSWORD", "password minimal 8 karakter")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_NAME", "name wajib diisi")
		return
	}
	userType := strings.ToLower(strings.TrimSpace(body.UserType))
	if !allowedUserTypes[userType] {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_USER_TYPE", "user_type harus customer, driver, atau merchant")
		return
	}
	if userType == "driver" &&
		(body.VehiclePlate == nil || strings.TrimSpace(*body.VehiclePlate) == "" ||
			body.LicenseNumber == nil || strings.TrimSpace(*body.LicenseNumber) == "") {
		writeError(c, http.StatusUnprocessableEntity, "INVALID_DRIVER_INFO", "driver wajib mengisi vehicle_plate dan license_number")
		return
	}

	// Cek keunikan email (race terakhir ditangani oleh unique constraint DB).
	var exists bool
	err := h.db.QueryRow(c.Request.Context(),
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email,
	).Scan(&exists)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "query failed")
		return
	}
	if exists {
		writeError(c, http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "email sudah terdaftar")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal hash password")
		return
	}

	// Phone kosong -> NULL (agar lolos CHECK phone_valid; string kosong tidak valid).
	var phone any
	if strings.TrimSpace(body.Phone) != "" {
		phone = strings.TrimSpace(body.Phone)
	}

	var vehicleType, vehiclePlate, licenseNumber any
	if userType == "driver" {
		if body.VehicleType != nil {
			vehicleType = strings.TrimSpace(*body.VehicleType)
		}
		vehiclePlate = strings.TrimSpace(*body.VehiclePlate)
		licenseNumber = strings.TrimSpace(*body.LicenseNumber)
	}

	ctx := c.Request.Context()
	tx, err := h.db.Begin(ctx)
	if err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal memulai transaksi")
		return
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var userID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, phone, name, user_type, status, kyc_status, password_hash,
		                   vehicle_type, vehicle_plate, license_number)
		 VALUES ($1, $2, $3, $4, 'ACTIVE', 'UNVERIFIED', $5, $6, $7, $8)
		 RETURNING id`,
		email, phone, strings.TrimSpace(body.Name), userType, string(hash), vehicleType, vehiclePlate, licenseNumber,
	).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			writeError(c, http.StatusConflict, "EMAIL_ALREADY_REGISTERED", "email sudah terdaftar")
			return
		}
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal membuat user")
		return
	}

	if _, err := tx.Exec(ctx,
		`INSERT INTO wallets (user_id, wallet_type, balance, status) VALUES ($1, 'CUSTOMER', 0, 'ACTIVE')`,
		userID,
	); err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal membuat wallet")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal menyimpan data")
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data": gin.H{
			"user_id":   userID,
			"user_type": userType,
		},
	})
}

// Logout POST /auth/logout (butuh AuthMiddleware)
// Menandai token aktif (jti) sebagai blacklisted sampai kedaluwarsa.
func (h *Handler) Logout(c *gin.Context) {
	claimsVal, ok := c.Get("claims")
	if !ok {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "token tidak ditemukan")
		return
	}
	claims, ok := claimsVal.(*Claims)
	if !ok || claims.Jti == "" {
		writeError(c, http.StatusUnauthorized, "UNAUTHORIZED", "token tidak valid")
		return
	}

	if err := h.blacklist.Add(c.Request.Context(), claims.Jti, claims.Exp); err != nil {
		writeError(c, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "gagal mem-blacklist token")
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// --- helpers ---

// validEmail melakukan validasi ringan format email (regex ketat di DB CHECK).
func validEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

// writeError menulis error response sesuai format API_CONTRACT:
// { "success": false, "error": { "code": "...", "message": "..." } }
func writeError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"error": gin.H{
			"code":    code,
			"message": message,
		},
	})
}
