package admin

// Test untuk fitur user management admin (STEP A4, ROADMAP 4.2.3):
//   - GET   /admin/users            (ListUsers + CountUsers, filter E1)
//   - GET   /admin/users/:id        (GetUserByID, detail E2)
//   - PATCH /admin/users/:id/:action (UpdateUserStatus, E3 — freeze/suspend/
//     ban/unfreeze, audited via admin_action_logs)
//
// Pola sama seperti test reversal/ledger: pgxmock untuk DB, gin httptest untuk
// handler. Response mengikuti apps/admin_web/src/lib/types.ts (AdminUser,
// UserListResponse) dengan envelope {success, data}.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUserID = uuid.MustParse("88888888-8888-8888-8888-888888888888")

func usersRouter(h *Handler, adminID uuid.UUID) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/admin/users", h.GetUsers)
	r.GET("/admin/users/:id", h.GetUserDetail)
	r.PATCH("/admin/users/:id/:action", func(c *gin.Context) {
		c.Set("user_id", adminID.String())
		h.UpdateUserStatus(c)
	})
	return r
}

func userRows() *pgxmock.Rows {
	return pgxmock.NewRows([]string{"id", "email", "phone", "role", "status", "balance", "created_at"})
}

// TestGetUsers_Service: ListUsers (pagination) + CountUsers -> {users, total_count}.
func TestGetUsers_Service(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	id1, id2 := uuid.New(), uuid.New()
	mDB.ExpectQuery("ORDER BY u.created_at DESC LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(userRows().
			AddRow(id1, "a@g-flow.com", "0811", "CUSTOMER", "ACTIVE", decimal.NewFromInt(100000), now).
			AddRow(id2, "b@g-flow.com", "", "MERCHANT", "SUSPENDED", decimal.NewFromInt(0), now))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs().
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(2)))

	svc := newSvc(t, mDB)
	list, err := svc.GetUsers(context.Background(), UserFilter{Limit: 50})
	require.NoError(t, err)

	assert.Equal(t, int64(2), list.TotalCount)
	require.Len(t, list.Users, 2)
	assert.Equal(t, id1, list.Users[0].ID)
	assert.Equal(t, "CUSTOMER", list.Users[0].Role)
	assert.Equal(t, "ACTIVE", list.Users[0].Status)
	assert.True(t, decimal.Decimal(list.Users[0].Balance).Equal(decimal.NewFromInt(100000)))
	assert.Equal(t, "b@g-flow.com", list.Users[1].Email)
	assert.Equal(t, "", list.Users[1].Phone)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUsers_Filtered: filter role/status/search diteruskan ke ListUsers dan
// CountUsers (placeholder dinamis).
func TestGetUsers_Filtered(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectQuery("ORDER BY u.created_at DESC LIMIT").
		WithArgs("CUSTOMER", "ACTIVE", "user", pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(userRows().
			AddRow(uuid.New(), "user1@g-flow.com", "0812", "CUSTOMER", "ACTIVE", decimal.NewFromInt(50000), now))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs("CUSTOMER", "ACTIVE", "user").
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	svc := newSvc(t, mDB)
	list, err := svc.GetUsers(context.Background(), UserFilter{Limit: 10, Role: "CUSTOMER", Status: "ACTIVE", Search: "user"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), list.TotalCount)
	require.Len(t, list.Users, 1)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserDetail_Service: GetUserByID mengembalikan view user.
func TestGetUserDetail_Service(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectQuery("WHERE u.id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(userRows().
			AddRow(testUserID, "driver@g-flow.com", "0813", "DRIVER", "FROZEN", decimal.NewFromInt(250000), now))

	svc := newSvc(t, mDB)
	u, err := svc.GetUserDetail(context.Background(), testUserID)
	require.NoError(t, err)
	assert.Equal(t, testUserID, u.ID)
	assert.Equal(t, "driver@g-flow.com", u.Email)
	assert.Equal(t, "DRIVER", u.Role)
	assert.Equal(t, "FROZEN", u.Status)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserDetail_NotFound: user tidak ada -> ErrUserNotFound.
func TestGetUserDetail_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("WHERE u.id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(userRows())

	svc := newSvc(t, mDB)
	_, err = svc.GetUserDetail(context.Background(), testUserID)
	require.ErrorIs(t, err, ErrUserNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatus_Service: pemetaan 4 aksi -> status (freeze/suspend/ban/
// unfreeze) dengan audit log admin_action_logs dalam satu transaksi.
func TestUpdateUserStatus_Service(t *testing.T) {
	cases := []struct {
		action string
		want   string
	}{
		{"freeze", "FROZEN"},
		{"suspend", "SUSPENDED"},
		{"ban", "DELETED"},
		{"unfreeze", "ACTIVE"},
	}
	for _, tc := range cases {
		t.Run(tc.action, func(t *testing.T) {
			mDB, err := pgxmock.NewPool()
			require.NoError(t, err)

			now := time.Now()
			mDB.ExpectBegin()
			mDB.ExpectQuery("UPDATE users SET status").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(now))
			mDB.ExpectExec("INSERT INTO admin_action_logs").
				WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
				WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
			mDB.ExpectCommit()

			svc := newSvc(t, mDB)
			res, err := svc.UpdateUserStatus(context.Background(), UserStatusRequest{
				UserID:  testUserID,
				AdminID: testAdminID,
				Action:  tc.action,
				Reason:  "aksi tes",
			})
			require.NoError(t, err)
			assert.Equal(t, testUserID, res.UserID)
			assert.Equal(t, tc.want, res.Status)
			assert.NoError(t, mDB.ExpectationsWereMet())
		})
	}

	// Admin action log semestinya tetap ditulis walau reason kosong (opsional).
	t.Run("reason_optional", func(t *testing.T) {
		mDB, err := pgxmock.NewPool()
		require.NoError(t, err)

		now := time.Now()
		mDB.ExpectBegin()
		mDB.ExpectQuery("UPDATE users SET status").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(now))
		mDB.ExpectExec("INSERT INTO admin_action_logs").
			WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
			WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
		mDB.ExpectCommit()

		svc := newSvc(t, mDB)
		_, err = svc.UpdateUserStatus(context.Background(), UserStatusRequest{
			UserID:  testUserID,
			AdminID: testAdminID,
			Action:  "suspend",
		})
		require.NoError(t, err)
		assert.NoError(t, mDB.ExpectationsWereMet())
	})
}

// TestUpdateUserStatus_NotFound: UPDATE tanpa baris -> pgx.ErrNoRows ->
// ErrUserNotFound (tanpa menulis audit log).
func TestUpdateUserStatus_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("UPDATE users SET status").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}))

	svc := newSvc(t, mDB)
	_, err = svc.UpdateUserStatus(context.Background(), UserStatusRequest{
		UserID:  testUserID,
		AdminID: testAdminID,
		Action:  "freeze",
	})
	require.ErrorIs(t, err, ErrUserNotFound)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatus_InvalidAction: aksi tidak dikenal -> ErrInvalidAction
// tanpa menyentuh database.
func TestUpdateUserStatus_InvalidAction(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	svc := newSvc(t, mDB)
	_, err = svc.UpdateUserStatus(context.Background(), UserStatusRequest{
		UserID:  testUserID,
		AdminID: testAdminID,
		Action:  "rename",
	})
	require.ErrorIs(t, err, ErrInvalidAction)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUsersHandler: GET /admin/users via HTTP -> envelope {success, data}.
func TestGetUsersHandler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectQuery("ORDER BY u.created_at DESC LIMIT").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(userRows().
			AddRow(testUserID, "a@g-flow.com", "0811", "CUSTOMER", "ACTIVE", decimal.NewFromInt(100000), now))
	mDB.ExpectQuery("SELECT COUNT").
		WithArgs().
		WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(int64(1)))

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodGet, "/admin/users?limit=10", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"success\":true")
	assert.Contains(t, w.Body.String(), "total_count")
	assert.Contains(t, w.Body.String(), "a@g-flow.com")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUsersHandler_BadLimit: limit tidak valid -> 400 INVALID_REQUEST.
func TestGetUsersHandler_BadLimit(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/users?limit=abc", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserDetailHandler: GET /admin/users/:id -> 200 data user.
func TestGetUserDetailHandler(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectQuery("WHERE u.id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(userRows().
			AddRow(testUserID, "driver@g-flow.com", "0813", "DRIVER", "ACTIVE", decimal.NewFromInt(250000), now))

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/users/"+testUserID.String(), nil))
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "driver@g-flow.com")
	assert.Contains(t, w.Body.String(), "\"role\":\"DRIVER\"")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserDetailHandler_NotFound: user tidak ada -> 404 USER_NOT_FOUND.
func TestGetUserDetailHandler_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectQuery("WHERE u.id").
		WithArgs(pgxmock.AnyArg()).
		WillReturnRows(userRows())

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/users/"+testUserID.String(), nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "USER_NOT_FOUND")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestGetUserDetailHandler_InvalidID: id tidak valid -> 422 INVALID_USER_ID.
func TestGetUserDetailHandler_InvalidID(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/admin/users/not-a-uuid", nil))
	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_USER_ID")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatusHandler_Success: PATCH /admin/users/:id/freeze dengan 2FA
// tidak diminta (aksi user tidak mensyaratkan 2FA dalam scope ROADMAP 4.2.3) ->
// 200 {success, data:{user_id,status,updated_at}}.
func TestUpdateUserStatusHandler_Success(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	now := time.Now()
	mDB.ExpectBegin()
	mDB.ExpectQuery("UPDATE users SET status").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}).AddRow(now))
	mDB.ExpectExec("INSERT INTO admin_action_logs").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnResult(pgconn.NewCommandTag("INSERT 0 1"))
	mDB.ExpectCommit()

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+testUserID.String()+"/freeze", strings.NewReader(`{"reason":"aktivitas mencurigakan"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "\"success\":true")
	assert.Contains(t, w.Body.String(), "FROZEN")
	assert.Contains(t, w.Body.String(), "updated_at")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatusHandler_InvalidAction: aksi tidak dikenal -> 400.
func TestUpdateUserStatusHandler_InvalidAction(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/admin/users/"+testUserID.String()+"/launch", nil))
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "INVALID_REQUEST")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatusHandler_NotFound: user tidak ada -> 404 USER_NOT_FOUND.
func TestUpdateUserStatusHandler_NotFound(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("UPDATE users SET status").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}))

	h := newTestHandler(t, mDB, nil)
	router := usersRouter(h, testAdminID)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPatch, "/admin/users/"+testUserID.String()+"/suspend", nil))
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "USER_NOT_FOUND")
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUpdateUserStatus_RepoNoRows verifies repository returns pgx.ErrNoRows on
// empty UPDATE (foundation of ErrUserNotFound mapping).
func TestUpdateUserStatus_RepoNoRows(t *testing.T) {
	mDB, err := pgxmock.NewPool()
	require.NoError(t, err)

	mDB.ExpectBegin()
	mDB.ExpectQuery("UPDATE users SET status").
		WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg()).
		WillReturnRows(pgxmock.NewRows([]string{"updated_at"}))

	tx, err := mDB.Begin(context.Background())
	require.NoError(t, err)
	repo := NewRepository(mDB)
	_, err = repo.UpdateUserStatus(context.Background(), tx, testUserID, "FROZEN")
	require.ErrorIs(t, err, pgx.ErrNoRows)
	assert.NoError(t, mDB.ExpectationsWereMet())
}

// TestUserActionToStatus memverifikasi pemetaan aksi -> status (4 value).
func TestUserActionToStatus(t *testing.T) {
	cases := map[string]string{
		"freeze":   "FROZEN",
		"suspend":  "SUSPENDED",
		"ban":      "DELETED",
		"unfreeze": "ACTIVE",
	}
	for action, want := range cases {
		got, ok := userActionToStatus(action)
		require.True(t, ok, "aksi %s harus valid", action)
		assert.Equal(t, want, got)
	}
	if _, ok := userActionToStatus("hack"); ok {
		t.Fatal("aksi tak dikenal tidak boleh lolos")
	}
}
