package seats

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type Seat struct {
	ID            string     `json:"id"`
	EventID       string     `json:"event_id"`
	SeatLabel     string     `json:"seat_label"`
	Status        string     `json:"status"`
	HeldUntil     *time.Time `json:"held_until,omitempty"`
	HeldByBooking *string    `json:"held_by_booking,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type SeatsRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewSeatsRepository(db *store.DB, log *zap.Logger) *SeatsRepository {
	return &SeatsRepository{db: db, log: log}
}

func (r *SeatsRepository) CreateSeats(ctx context.Context, eventID string, seatLabels []string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		for _, label := range seatLabels {
			_, err := tx.Exec(ctx, `
				INSERT INTO seats (event_id, seat_label, status)
				VALUES ($1, $2, 'available')
			`, eventID, label)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SeatsRepository) GetSeatsByEvent(ctx context.Context, eventID string) ([]*Seat, error) {
	query := `
		SELECT id, event_id, seat_label, status, held_until, held_by_booking, created_at, updated_at
		FROM seats
		WHERE event_id = $1
		ORDER BY seat_label`

	rows, err := r.db.Pool.Query(ctx, query, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seats []*Seat
	for rows.Next() {
		seat := &Seat{}
		err := rows.Scan(
			&seat.ID, &seat.EventID, &seat.SeatLabel, &seat.Status,
			&seat.HeldUntil, &seat.HeldByBooking, &seat.CreatedAt, &seat.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		seats = append(seats, seat)
	}

	return seats, nil
}

func (r *SeatsRepository) UpdateSeatStatus(ctx context.Context, eventID, seatLabel, status string, heldByBooking *string, heldUntil *time.Time) error {
	query := `
		UPDATE seats 
		SET status = $1, held_by_booking = $2, held_until = $3, updated_at = now()
		WHERE event_id = $4 AND seat_label = $5`

	result, err := r.db.Pool.Exec(ctx, query, status, heldByBooking, heldUntil, eventID, seatLabel)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *SeatsRepository) ReleaseSeats(ctx context.Context, eventID string, seatLabels []string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		for _, label := range seatLabels {
			_, err := tx.Exec(ctx, `
				UPDATE seats 
				SET status = 'available', held_by_booking = NULL, held_until = NULL, updated_at = now()
				WHERE event_id = $1 AND seat_label = $2
			`, eventID, label)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SeatsRepository) BookSeats(ctx context.Context, eventID string, seatLabels []string, bookingID string) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		for _, label := range seatLabels {
			_, err := tx.Exec(ctx, `
				UPDATE seats 
				SET status = 'booked', held_by_booking = $1, held_until = NULL, updated_at = now()
				WHERE event_id = $2 AND seat_label = $3 AND status = 'held'
			`, bookingID, eventID, label)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SeatsRepository) HoldSeats(ctx context.Context, eventID string, seatLabels []string, bookingID string, heldUntil time.Time) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		for _, label := range seatLabels {
			_, err := tx.Exec(ctx, `
				UPDATE seats 
				SET status = 'held', held_by_booking = $1, held_until = $2, updated_at = now()
				WHERE event_id = $3 AND seat_label = $4 AND status = 'available'
			`, bookingID, heldUntil, eventID, label)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SeatsRepository) GetAvailableSeats(ctx context.Context, eventID string) ([]string, error) {
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
