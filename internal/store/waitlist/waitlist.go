package waitlist

import (
	"context"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type WaitlistEntry struct {
	ID         string `json:"id"`
	EventID    string `json:"event_id"`
	UserID     string `json:"user_id"`
	Position   int    `json:"position"`
	OptedOut   bool   `json:"opted_out"`
	NotifiedAt string `json:"notified_at,omitempty"`
	CreatedAt  string `json:"created_at"`
}

type WaitlistRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewWaitlistRepository(db *store.DB, log *zap.Logger) *WaitlistRepository {
	return &WaitlistRepository{db: db, log: log}
}

func (r *WaitlistRepository) Add(ctx context.Context, eventID, userID string) (int, error) {
	// Get the next position
	var position int
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(position), 0) + 1 
		FROM waitlist 
		WHERE event_id = $1 AND opted_out = false
	`, eventID).Scan(&position)
	if err != nil {
		return 0, err
	}

	// Insert the waitlist entry
	query := `
		INSERT INTO waitlist (event_id, user_id, position, opted_out)
		VALUES ($1, $2, $3, false)
		RETURNING id`

	var id string
	err = r.db.Pool.QueryRow(ctx, query, eventID, userID, position).Scan(&id)
	if err != nil {
		return 0, err
	}

	return position, nil
}

func (r *WaitlistRepository) Remove(ctx context.Context, id string) error {
	query := `DELETE FROM waitlist WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *WaitlistRepository) OptOut(ctx context.Context, eventID, userID string) error {
	query := `
		UPDATE waitlist 
		SET opted_out = true 
		WHERE event_id = $1 AND user_id = $2`

	result, err := r.db.Pool.Exec(ctx, query, eventID, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *WaitlistRepository) NextActive(ctx context.Context, eventID string) (string, string, int, error) {
	query := `
		SELECT id, user_id, position 
		FROM waitlist 
		WHERE event_id = $1 AND opted_out = false 
		ORDER BY position ASC 
		LIMIT 1`

	var id, userID string
	var position int
	err := r.db.Pool.QueryRow(ctx, query, eventID).Scan(&id, &userID, &position)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", "", 0, nil
		}
		return "", "", 0, err
	}

	return id, userID, position, nil
}

func (r *WaitlistRepository) Count(ctx context.Context, eventID string) (int, error) {
	query := `
		SELECT COUNT(*) 
		FROM waitlist 
		WHERE event_id = $1 AND opted_out = false`

	var count int
	err := r.db.Pool.QueryRow(ctx, query, eventID).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *WaitlistRepository) ListByEvent(ctx context.Context, eventID string, limit, offset int) ([]*WaitlistEntry, error) {
	query := `
		SELECT id, event_id, user_id, position, opted_out, notified_at, created_at
		FROM waitlist 
		WHERE event_id = $1 
		ORDER BY position ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, eventID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*WaitlistEntry
	for rows.Next() {
		entry := &WaitlistEntry{}
		var notifiedAt *string
		err := rows.Scan(
			&entry.ID, &entry.EventID, &entry.UserID, &entry.Position,
			&entry.OptedOut, &notifiedAt, &entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		if notifiedAt != nil {
			entry.NotifiedAt = *notifiedAt
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

func (r *WaitlistRepository) MarkNotified(ctx context.Context, id string) error {
	query := `UPDATE waitlist SET notified_at = now() WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
