// Package middleware — request metrics in-memory (TD-046).
//
// Counter untuk KPI error_rate dashboard admin: jumlah response 5xx dibagi
// total response pada window 24 jam (convention TD-046 — sama dengan window
// TotalTransactionVolume). Disimpan in-memory per proses (tanpa migration),
// window tumbling: reset saat umur window >= 24 jam pada observe berikutnya.
package middleware

import (
	"sync"
	"time"
)

// errorRateWindow adalah panjang window error rate (24 jam).
const errorRateWindow = 24 * time.Hour

// errorRateState adalah counter window aktif; dikunci mu karena di-reset oleh
// observeStatus dan dibaca GetErrorRate secara concurrent.
var errorRateState = struct {
	mu          sync.Mutex
	windowStart time.Time
	total       int64
	serverError int64
}{windowStart: time.Now()}

// observeStatus mencatat satu response ke counter window aktif; status >= 500
// dihitung sebagai error. Dipanggil Logger setelah handler selesai.
func observeStatus(status int) {
	errorRateState.mu.Lock()
	defer errorRateState.mu.Unlock()
	if time.Since(errorRateState.windowStart) >= errorRateWindow {
		errorRateState.windowStart = time.Now()
		errorRateState.total = 0
		errorRateState.serverError = 0
	}
	errorRateState.total++
	if status >= 500 {
		errorRateState.serverError++
	}
}

// GetErrorRate mengembalikan rasio 5xx/total (0..1) untuk window 24 jam
// terakhir; 0 bila window kosong atau kedaluwarsa.
func GetErrorRate() float64 {
	errorRateState.mu.Lock()
	defer errorRateState.mu.Unlock()
	if errorRateState.total == 0 || time.Since(errorRateState.windowStart) >= errorRateWindow {
		return 0
	}
	return float64(errorRateState.serverError) / float64(errorRateState.total)
}

// resetErrorRate mengosongkan counter (khusus test).
func resetErrorRate() {
	errorRateState.mu.Lock()
	defer errorRateState.mu.Unlock()
	errorRateState.windowStart = time.Now()
	errorRateState.total = 0
	errorRateState.serverError = 0
}
