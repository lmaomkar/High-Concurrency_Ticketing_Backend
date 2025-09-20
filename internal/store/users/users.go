package users

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type User struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	PasswordHash  string    `json:"-"` // Don't expose in JSON
	OAuthProvider string    `json:"oauth_provider,omitempty"`
	OAuthSub      string    `json:"oauth_sub,omitempty"`
	Role          string    `json:"role"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UsersRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewUsersRepository(db *store.DB, log *zap.Logger) *UsersRepository {
	return &UsersRepository{db: db, log: log}
}

func (r *UsersRepository) Create(ctx context.Context, user *User) (*User, error) {
	query := `
		INSERT INTO users (name, email, phone, password_hash, role)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at`

	err := r.db.Pool.QueryRow(ctx, query, user.Name, user.Email, user.Phone, user.PasswordHash, user.Role).
		Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UsersRepository) GetByID(ctx context.Context, id string) (*User, error) {
	query := `
		SELECT id, name, email, phone, password_hash, oauth_provider, oauth_sub, role, created_at, updated_at
		FROM users
		WHERE id = $1`

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.PasswordHash,
		&user.OAuthProvider, &user.OAuthSub, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

func (r *UsersRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, name, email, phone, password_hash, oauth_provider, oauth_sub, role, created_at, updated_at
		FROM users
		WHERE email = $1`

	user := &User{}
	err := r.db.Pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Name, &user.Email, &user.Phone, &user.PasswordHash,
		&user.OAuthProvider, &user.OAuthSub, &user.Role, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return user, nil
}

func (r *UsersRepository) UpdatePassword(ctx context.Context, userID, passwordHash string) error {
	query := `
		UPDATE users 
		SET password_hash = $1, updated_at = now()
		WHERE id = $2`

	result, err := r.db.Pool.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *UsersRepository) UpdateProfile(ctx context.Context, userID, name, phone string) error {
	query := `
		UPDATE users 
		SET name = $1, phone = $2, updated_at = now()
		WHERE id = $3`

	result, err := r.db.Pool.Exec(ctx, query, name, phone, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *UsersRepository) UpdateRole(ctx context.Context, userID, role string) error {
	query := `
		UPDATE users 
		SET role = $1, updated_at = now()
		WHERE id = $2`

	result, err := r.db.Pool.Exec(ctx, query, role, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *UsersRepository) Delete(ctx context.Context, userID string) error {
	query := `DELETE FROM users WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *UsersRepository) List(ctx context.Context, limit, offset int) ([]*User, error) {
	query := `
		SELECT id, name, email, phone, oauth_provider, oauth_sub, role, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		user := &User{}
		err := rows.Scan(
			&user.ID, &user.Name, &user.Email, &user.Phone,
			&user.OAuthProvider, &user.OAuthSub, &user.Role,
			&user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *UsersRepository) Count(ctx context.Context) (int, error) {
	query := `SELECT COUNT(*) FROM users`

	var count int
	err := r.db.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}
