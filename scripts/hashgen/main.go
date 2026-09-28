// Command hashgen menghasilkan bcrypt hash dari password yang diberikan
// sebagai argumen pertama, lalu mencetak hash ke stdout.
//
// Dipakai oleh scripts/seed_admin.sh agar password admin bisa dirotasi
// per-environment lewat env (ADMIN_PASSWORD) tanpa pernah disimpan di repo
// (TD-030 / TD-031).
//
// Password dibaca dari argumen ATAU env HASHGEN_PASSWORD (argumen menang).
// Bentuk env dipakai kalau password mengandung karakter yang merepotkan di
// shell (mis. diawali "-" atau mengandung spasi).
//
// Usage:
//
//	go run ./scripts/hashgen 'password-nya'
//	HASHGEN_PASSWORD='password-nya' go run ./scripts/hashgen
//
// Output: satu baris bcrypt hash, tanpa newline tambahan.
package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := ""
	if len(os.Args) > 1 {
		password = os.Args[1]
	}
	if password == "" {
		password = os.Getenv("HASHGEN_PASSWORD")
	}
	if password == "" {
		fmt.Fprintln(os.Stderr, "hashgen: password wajib diisi (argumen atau env HASHGEN_PASSWORD)")
		os.Exit(1)
	}

	// DefaultCost (10) sama dengan yang dipakai production:
	// internal/auth/handler.go Register -> bcrypt.GenerateFromPassword(.., bcrypt.DefaultCost).
	// Migration 014 memakai cost 12; cost 12 yang dipakai supaya seed admin
	// tidak lebih lemah dari akun yang dibuat lewat API.
	cost := bcrypt.DefaultCost
	if v := os.Getenv("HASHGEN_COST"); v != "" {
		var c int
		if _, err := fmt.Sscanf(v, "%d", &c); err != nil || c < bcrypt.MinCost || c > bcrypt.MaxCost {
			fmt.Fprintf(os.Stderr, "hashgen: HASHGEN_COST=%q tidak valid (harus %d..%d)\n", v, bcrypt.MinCost, bcrypt.MaxCost)
			os.Exit(1)
		}
		cost = c
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashgen: gagal generate bcrypt hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(string(hash))
}
