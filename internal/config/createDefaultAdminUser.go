package config

import (
	"context"
	"fmt"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func CreateDefaultAdmin(cfg *Config, db *store.DB) error {
	ctx := context.Background()

	// Check if admin user already exists
	var count int
	err := db.Pool.QueryRow(ctx, "SELECT COUNT(*) FROM users WHERE email = $1", cfg.AdminEmail).Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check existing admin: %w", err)
	}

	if count > 0 {
		// Admin already exists, skip creation
		return nil
	}

	// Hash the admin password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(cfg.AdminSuperUserPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	// Create the admin user
	_, err = db.Pool.Exec(ctx, `
		INSERT INTO users (name, email, password_hash, role) 
		VALUES ($1, $2, $3, $4)
	`, "Admin User", cfg.AdminEmail, string(hashedPassword), "admin")

	if err != nil {
		return fmt.Errorf("failed to create admin user: %w", err)
	}

	return nil
}
