package admin

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type AdminRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewAdminRepository(db *store.DB, log *zap.Logger) *AdminRepository {
	return &AdminRepository{db: db, log: log}
}

type AnalyticsSummary struct {
	TotalBookings       int            `json:"total_bookings"`
	TotalEvents         int            `json:"total_events"`
	TotalUsers          int            `json:"total_users"`
	CapacityUtilization float64        `json:"capacity_utilization"`
	MostPopularEvents   []PopularEvent `json:"most_popular_events"`
}

type PopularEvent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Bookings int    `json:"bookings"`
	Likes    int    `json:"likes"`
}

func (r *AdminRepository) GetSummary(ctx context.Context, from, to time.Time) (*AnalyticsSummary, error) {
	summary := &AnalyticsSummary{}

	// Get total bookings
	err := r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM bookings 
		WHERE created_at BETWEEN $1 AND $2 AND status = 'booked'
	`, from, to).Scan(&summary.TotalBookings)
	if err != nil {
		return nil, err
	}

	// Get total events
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM events 
		WHERE created_at BETWEEN $1 AND $2
	`, from, to).Scan(&summary.TotalEvents)
	if err != nil {
		return nil, err
	}

	// Get total users
	err = r.db.Pool.QueryRow(ctx, `
		SELECT COUNT(*) 
		FROM users 
		WHERE created_at BETWEEN $1 AND $2
	`, from, to).Scan(&summary.TotalUsers)
	if err != nil {
		return nil, err
	}

	// Get capacity utilization
	err = r.db.Pool.QueryRow(ctx, `
		SELECT 
			CASE 
				WHEN SUM(capacity) > 0 THEN 
					(SUM(reserved)::float / SUM(capacity)::float) * 100
				ELSE 0 
			END
		FROM events 
		WHERE created_at BETWEEN $1 AND $2
	`, from, to).Scan(&summary.CapacityUtilization)
	if err != nil {
		return nil, err
	}

	// Get most popular events
	rows, err := r.db.Pool.Query(ctx, `
		SELECT e.id, e.name, COUNT(b.id) as bookings, e.likes
		FROM events e
		LEFT JOIN bookings b ON e.id = b.event_id AND b.status = 'booked' AND b.created_at BETWEEN $1 AND $2
		WHERE e.created_at BETWEEN $1 AND $2
		GROUP BY e.id, e.name, e.likes
		ORDER BY bookings DESC, e.likes DESC
		LIMIT 10
	`, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var event PopularEvent
		err := rows.Scan(&event.ID, &event.Name, &event.Bookings, &event.Likes)
		if err != nil {
			return nil, err
		}
		summary.MostPopularEvents = append(summary.MostPopularEvents, event)
	}

	return summary, nil
}

func (r *AdminRepository) CancelEvent(ctx context.Context, eventID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Update event status
		_, err := tx.Exec(ctx, `
			UPDATE events 
			SET status = 'cancelled', updated_at = now() 
			WHERE id = $1
		`, eventID)
		if err != nil {
			return err
		}

		// Cancel all pending bookings
		_, err = tx.Exec(ctx, `
			UPDATE bookings 
			SET status = 'cancelled', updated_at = now() 
			WHERE event_id = $1 AND status IN ('pending', 'booked')
		`, eventID)
		if err != nil {
			return err
		}

		// Clear waitlist
		_, err = tx.Exec(ctx, `
			UPDATE waitlist 
			SET opted_out = true 
			WHERE event_id = $1
		`, eventID)
		if err != nil {
			return err
		}
		return err
	})
}

func (r *AdminRepository) UpdateEvent(ctx context.Context, eventID string, updates map[string]interface{}) error {
	// Build dynamic update query
	query := "UPDATE events SET "
	args := []interface{}{}
	argIndex := 1

	for field, value := range updates {
		if argIndex > 1 {
			query += ", "
		}
		query += field + " = $" + fmt.Sprintf("%d", argIndex)
		args = append(args, value)
		argIndex++
	}

	query += ", updated_at = now() WHERE id = $" + fmt.Sprintf("%d", argIndex)
	args = append(args, eventID)

	result, err := r.db.Pool.Exec(ctx, query, args...)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *AdminRepository) CreateAdminFromUser(ctx context.Context, userID string) error {
	query := `UPDATE users SET role = 'admin', updated_at = now() WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *AdminRepository) RemoveAdmin(ctx context.Context, userID string) error {
	query := `UPDATE users SET role = 'user', updated_at = now() WHERE id = $1 AND role = 'admin'`

	result, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *AdminRepository) RemoveUser(ctx context.Context, userID string) error {
	query := `DELETE FROM users WHERE id = $1 AND role != 'admin'`

	result, err := r.db.Pool.Exec(ctx, query, userID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
