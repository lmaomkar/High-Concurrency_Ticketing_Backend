package events

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type Event struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	Venue                    string    `json:"venue"`
	StartTime                time.Time `json:"start_time"`
	EndTime                  time.Time `json:"end_time"`
	Category                 string    `json:"category"`
	Capacity                 int       `json:"capacity"`
	Reserved                 int       `json:"reserved"`
	Metadata                 []byte    `json:"metadata"`
	Status                   string    `json:"status"`
	TicketPrice              float64   `json:"ticket_price"`
	CancellationFee          float64   `json:"cancellation_fee"`
	Likes                    int       `json:"likes"`
	MaximumTicketsPerBooking int       `json:"maximum_tickets_per_booking"`
	CreatedAt                time.Time `json:"created_at"`
	UpdatedAt                time.Time `json:"updated_at"`
}

type EventsRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewEventsRepository(db *store.DB, log *zap.Logger) *EventsRepository {
	return &EventsRepository{db: db, log: log}
}

func (r *EventsRepository) Create(ctx context.Context, event *Event) (*Event, error) {
	err := r.db.WithTx(ctx, func(tx pgx.Tx) error {
		query := `
		INSERT INTO events (name, venue, start_time, end_time, category, capacity, metadata, status, ticket_price, cancellation_fee, maximum_tickets_per_booking)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`

		err := tx.QueryRow(ctx, query,
			event.Name, event.Venue, event.StartTime, event.EndTime, event.Category,
			event.Capacity, event.Metadata, event.Status, event.TicketPrice,
			event.CancellationFee, event.MaximumTicketsPerBooking).
			Scan(&event.ID, &event.CreatedAt, &event.UpdatedAt)
		if err != nil {
			return err
		}
		return err
	})
	return event, err
}

func (r *EventsRepository) Get(ctx context.Context, id string) (*Event, error) {
	query := `
		SELECT id, name, venue, start_time, end_time, category, capacity, reserved, metadata, 
		       status, ticket_price, cancellation_fee, likes, maximum_tickets_per_booking, created_at, updated_at
		FROM events
		WHERE id = $1`

	event := &Event{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&event.ID, &event.Name, &event.Venue, &event.StartTime, &event.EndTime,
		&event.Category, &event.Capacity, &event.Reserved, &event.Metadata,
		&event.Status, &event.TicketPrice, &event.CancellationFee, &event.Likes,
		&event.MaximumTicketsPerBooking, &event.CreatedAt, &event.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return event, nil
}

func (r *EventsRepository) List(ctx context.Context, limit, offset int, q string, from, to *time.Time) ([]*Event, error) {
	query := `
		SELECT id, name, venue, start_time, end_time, category, capacity, reserved, metadata, 
		       status, ticket_price, cancellation_fee, likes, maximum_tickets_per_booking, created_at, updated_at
		FROM events
		WHERE 1=1`

	args := []interface{}{}
	argIndex := 1

	if q != "" {
		query += ` AND name ILIKE $` + fmt.Sprintf("%d", argIndex)
		args = append(args, "%"+q+"%")
		argIndex++
	}

	if from != nil {
		query += ` AND start_time >= $` + fmt.Sprintf("%d", argIndex)
		args = append(args, *from)
		argIndex++
	}

	if to != nil {
		query += ` AND start_time <= $` + fmt.Sprintf("%d", argIndex)
		args = append(args, *to)
		argIndex++
	}

	query += ` ORDER BY start_time ASC LIMIT $` + fmt.Sprintf("%d", argIndex) + ` OFFSET $` + fmt.Sprintf("%d", argIndex+1)
	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		err := rows.Scan(
			&event.ID, &event.Name, &event.Venue, &event.StartTime, &event.EndTime,
			&event.Category, &event.Capacity, &event.Reserved, &event.Metadata,
			&event.Status, &event.TicketPrice, &event.CancellationFee, &event.Likes,
			&event.MaximumTicketsPerBooking, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *EventsRepository) ListAll(ctx context.Context, limit, offset int) ([]*Event, error) {
	query := `
		SELECT id, name, venue, start_time, end_time, category, capacity, reserved, metadata, 
		       status, ticket_price, cancellation_fee, likes, maximum_tickets_per_booking, created_at, updated_at
		FROM events
		WHERE (end_time IS NULL OR end_time > NOW())
		ORDER BY start_time ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		err := rows.Scan(
			&event.ID, &event.Name, &event.Venue, &event.StartTime, &event.EndTime,
			&event.Category, &event.Capacity, &event.Reserved, &event.Metadata,
			&event.Status, &event.TicketPrice, &event.CancellationFee, &event.Likes,
			&event.MaximumTicketsPerBooking, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *EventsRepository) ListUpcoming(ctx context.Context, limit, offset int) ([]*Event, error) {
	query := `
		SELECT id, name, venue, start_time, end_time, category, capacity, reserved, metadata, 
		       status, ticket_price, cancellation_fee, likes, maximum_tickets_per_booking, created_at, updated_at
		FROM events
		WHERE start_time > NOW() AND status = 'upcoming'
		ORDER BY start_time ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		err := rows.Scan(
			&event.ID, &event.Name, &event.Venue, &event.StartTime, &event.EndTime,
			&event.Category, &event.Capacity, &event.Reserved, &event.Metadata,
			&event.Status, &event.TicketPrice, &event.CancellationFee, &event.Likes,
			&event.MaximumTicketsPerBooking, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *EventsRepository) ListPopular(ctx context.Context, limit, offset int) ([]*Event, error) {
	query := `
		SELECT id, name, venue, start_time, end_time, category, capacity, reserved, metadata, 
		       status, ticket_price, cancellation_fee, likes, maximum_tickets_per_booking, created_at, updated_at
		FROM events
		WHERE status = 'upcoming'
		ORDER BY likes DESC, start_time ASC
		LIMIT $1 OFFSET $2`

	rows, err := r.db.Pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []*Event
	for rows.Next() {
		event := &Event{}
		err := rows.Scan(
			&event.ID, &event.Name, &event.Venue, &event.StartTime, &event.EndTime,
			&event.Category, &event.Capacity, &event.Reserved, &event.Metadata,
			&event.Status, &event.TicketPrice, &event.CancellationFee, &event.Likes,
			&event.MaximumTicketsPerBooking, &event.CreatedAt, &event.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	return events, nil
}

func (r *EventsRepository) Update(ctx context.Context, event *Event) error {
	query := `
		UPDATE events 
		SET name = $1, venue = $2, start_time = $3, end_time = $4, category = $5, 
		    capacity = $6, metadata = $7, status = $8, ticket_price = $9, 
		    cancellation_fee = $10, maximum_tickets_per_booking = $11, updated_at = now()
		WHERE id = $12`

	result, err := r.db.Pool.Exec(ctx, query,
		event.Name, event.Venue, event.StartTime, event.EndTime, event.Category,
		event.Capacity, event.Metadata, event.Status, event.TicketPrice,
		event.CancellationFee, event.MaximumTicketsPerBooking, event.ID)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *EventsRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE events SET status = $1, updated_at = now() WHERE id = $2`

	result, err := r.db.Pool.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *EventsRepository) LikeEvent(ctx context.Context, eventID, userID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Insert like (idempotent)
		if _, err := tx.Exec(ctx, `
			INSERT INTO event_likes (user_id, event_id)
			VALUES ($1, $2)
			ON CONFLICT (user_id, event_id) DO NOTHING
		`, userID, eventID); err != nil {
			return err
		}

		// Increment counter only if the like exists
		_, err := tx.Exec(ctx, `
			UPDATE events
			SET likes = likes + 1
			WHERE id = $1 AND EXISTS (
				SELECT 1 FROM event_likes WHERE user_id = $2 AND event_id = $1
			)
		`, eventID, userID)
		return err
	})
}

func (r *EventsRepository) UnlikeEvent(ctx context.Context, eventID, userID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		res, err := tx.Exec(ctx, `DELETE FROM event_likes WHERE user_id = $1 AND event_id = $2`, userID, eventID)
		if err != nil {
			return err
		}
		if res.RowsAffected() == 0 {
			return nil
		}
		_, err = tx.Exec(ctx, `
			UPDATE events
			SET likes = GREATEST(likes - 1, 0)
			WHERE id = $1
		`, eventID)
		return err
	})
}

func (r *EventsRepository) IsLiked(ctx context.Context, eventID, userID string) (bool, error) {
	query := `SELECT 1 FROM event_likes WHERE user_id = $1 AND event_id = $2`

	var exists int
	err := r.db.Pool.QueryRow(ctx, query, userID, eventID).Scan(&exists)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (r *EventsRepository) GetAvailableSeats(ctx context.Context, eventID string) ([]string, error) {
	query := `
		SELECT seat_label 
		FROM seats 
		WHERE event_id = $1 AND status = 'available'
		ORDER BY seat_label`

	rows, err := r.db.Pool.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []string
	for rows.Next() {
		var seat string
		if err := rows.Scan(&seat); err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}

	return seats, nil
}

func (r *EventsRepository) UpdateExpiredEvents(ctx context.Context) (int, error) {
	query := `
		UPDATE events 
		SET status = 'expired', updated_at = now()
		WHERE status != 'expired' AND end_time < NOW()`

	result, err := r.db.Pool.Exec(ctx, query)
	if err != nil {
		return 0, err
	}

	return int(result.RowsAffected()), nil
}
