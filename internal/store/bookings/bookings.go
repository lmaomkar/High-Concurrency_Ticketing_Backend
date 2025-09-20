package bookings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
)

type Booking struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	EventID        string    `json:"event_id"`
	Status         string    `json:"status"`
	Seats          []byte    `json:"seats"` // JSON array of seat labels
	IdempotencyKey string    `json:"idempotency_key,omitempty"`
	AmountPaid     float64   `json:"amount_paid"`
	PaymentStatus  string    `json:"payment_status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Version        int       `json:"version"`
}

type BookingsRepository struct {
	db  *store.DB
	log *zap.Logger
}

func NewBookingsRepository(db *store.DB, log *zap.Logger) *BookingsRepository {
	return &BookingsRepository{db: db, log: log}
}

func (r *BookingsRepository) CreatePending(ctx context.Context, userID string, eventID string, idempotencyKey *string, seats []byte) (*Booking, error) {
	query := `
		INSERT INTO bookings (user_id, event_id, status, idempotency_key, payment_status, seats)
		VALUES ($1, $2, 'pending', $3, 'pending', $4)
		RETURNING id, created_at, updated_at, version`

	booking := &Booking{
		UserID:        userID,
		EventID:       eventID,
		Status:        "pending",
		PaymentStatus: "pending",
		Seats:         seats,
	}

	if idempotencyKey != nil {
		booking.IdempotencyKey = *idempotencyKey
	}

	err := r.db.Pool.QueryRow(ctx, query, userID, eventID, idempotencyKey, seats).
		Scan(&booking.ID, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version)
	if err != nil {
		return nil, err
	}

	return booking, nil
}

func (r *BookingsRepository) GetByID(ctx context.Context, id string) (*Booking, error) {
	query := `
		SELECT id, user_id, event_id, status, seats, idempotency_key, amount_paid, 
		       payment_status, created_at, updated_at, version
		FROM bookings
		WHERE id = $1`

	booking := &Booking{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&booking.ID, &booking.UserID, &booking.EventID, &booking.Status,
		&booking.Seats, &booking.IdempotencyKey, &booking.AmountPaid,
		&booking.PaymentStatus, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return booking, nil
}

func (r *BookingsRepository) GetByIdempotency(ctx context.Context, key string) (*Booking, error) {
	query := `
		SELECT id, user_id, event_id, status, seats, idempotency_key, amount_paid, 
		       payment_status, created_at, updated_at, version
		FROM bookings
		WHERE idempotency_key = $1`

	booking := &Booking{}
	err := r.db.Pool.QueryRow(ctx, query, key).Scan(
		&booking.ID, &booking.UserID, &booking.EventID, &booking.Status,
		&booking.Seats, &booking.IdempotencyKey, &booking.AmountPaid,
		&booking.PaymentStatus, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return booking, nil
}

func (r *BookingsRepository) ListByUser(ctx context.Context, userID string, limit, offset int) ([]*Booking, error) {
	query := `
		SELECT id, user_id, event_id, status, seats, idempotency_key, amount_paid, 
		       payment_status, created_at, updated_at, version
		FROM bookings
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*Booking
	for rows.Next() {
		booking := &Booking{}
		err := rows.Scan(
			&booking.ID, &booking.UserID, &booking.EventID, &booking.Status,
			&booking.Seats, &booking.IdempotencyKey, &booking.AmountPaid,
			&booking.PaymentStatus, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version,
		)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}

	return bookings, nil
}

func (r *BookingsRepository) ListByEvent(ctx context.Context, eventID string, limit, offset int) ([]*Booking, error) {
	query := `
		SELECT id, user_id, event_id, status, seats, idempotency_key, amount_paid, 
		       payment_status, created_at, updated_at, version
		FROM bookings
		WHERE event_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, query, eventID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []*Booking
	for rows.Next() {
		booking := &Booking{}
		err := rows.Scan(
			&booking.ID, &booking.UserID, &booking.EventID, &booking.Status,
			&booking.Seats, &booking.IdempotencyKey, &booking.AmountPaid,
			&booking.PaymentStatus, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version,
		)
		if err != nil {
			return nil, err
		}
		bookings = append(bookings, booking)
	}

	return bookings, nil
}

func (r *BookingsRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE bookings SET status = $1, updated_at = now() WHERE id = $2`

	result, err := r.db.Pool.Exec(ctx, query, status, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *BookingsRepository) UpdatePaymentStatus(ctx context.Context, id, paymentStatus string, amountPaid float64) error {
	query := `
		UPDATE bookings 
		SET payment_status = $1, amount_paid = $2
		WHERE id = $3`

	result, err := r.db.Pool.Exec(ctx, query, paymentStatus, amountPaid, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *BookingsRepository) UpdateSeats(ctx context.Context, id string, seats []byte) error {
	query := `UPDATE bookings SET seats = $1, updated_at = now() WHERE id = $2`

	result, err := r.db.Pool.Exec(ctx, query, seats, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}

func (r *BookingsRepository) CancelBookingTx(ctx context.Context, bookingID string) (*Booking, bool, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback(ctx)

	// Get booking
	var booking Booking
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, event_id, status, seats, idempotency_key, amount_paid, 
		       payment_status, created_at, updated_at, version
		FROM bookings
		WHERE id = $1
	`, bookingID).Scan(
		&booking.ID, &booking.UserID, &booking.EventID, &booking.Status,
		&booking.Seats, &booking.IdempotencyKey, &booking.AmountPaid,
		&booking.PaymentStatus, &booking.CreatedAt, &booking.UpdatedAt, &booking.Version,
	)
	if err != nil {
		return nil, false, err
	}

	// Check if booking was actually booked (not just pending)
	wasBooked := booking.Status == "booked"

	// Update booking status
	_, err = tx.Exec(ctx, `
		UPDATE bookings 
		SET status = 'cancelled', updated_at = now() 
		WHERE id = $1
	`, bookingID)
	if err != nil {
		return nil, false, err
	}

	// If it was booked, update event reserved count and release seats
	if wasBooked {
		_, err = tx.Exec(ctx, `
			UPDATE events 
			SET reserved = reserved - 1 
			WHERE id = $1
		`, booking.EventID)
		if err != nil {
			return nil, false, err
		}

		// Release seats - mark them as available again
		var seatLabels []string
		if len(booking.Seats) > 0 {
			err = json.Unmarshal(booking.Seats, &seatLabels)
			if err != nil {
				return nil, false, err
			}

			for _, seatLabel := range seatLabels {
				_, err = tx.Exec(ctx, `
				UPDATE seats 
				SET status = 'available', held_by_booking = NULL, held_until = NULL, updated_at = now()
				WHERE event_id = $1 AND seat_label = $2
			`, booking.EventID, seatLabel)
				if err != nil {
					return nil, false, err
				}
			}
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, false, err
	}

	booking.Status = "cancelled"
	return &booking, wasBooked, nil
}

func (r *BookingsRepository) FinalizeBooking(ctx context.Context, bookingID string, seats []byte, amountPaid float64) error {
	return r.db.WithTx(ctx, func(tx pgx.Tx) error {
		// Get event_id for updating seats table
		var eventID string
		err := tx.QueryRow(ctx, `SELECT event_id FROM bookings WHERE id = $1`, bookingID).Scan(&eventID)
		if err != nil {
			return err
		}

		// Update booking
		_, err = tx.Exec(ctx, `
		UPDATE bookings 
		SET status = 'booked', seats = $1, amount_paid = $2, payment_status = 'paid', updated_at = now() 
		WHERE id = $3 AND status = 'pending'
	`, seats, amountPaid, bookingID)
		if err != nil {
			return err
		}

		// Update seats table - mark seats as booked
		// Parse seats JSON and update each seat individually
		var seatLabels []string
		if len(seats) > 0 {
			err = json.Unmarshal(seats, &seatLabels)
			if err != nil {
				return err
			}

			for _, seatLabel := range seatLabels {
				_, err = tx.Exec(ctx, `
				UPDATE seats 
				SET status = 'booked', held_by_booking = $1, held_until = NULL, updated_at = now()
				WHERE event_id = $2 AND seat_label = $3
			`, bookingID, eventID, seatLabel)
				if err != nil {
					return err
				}
			}
		}

		// Update event reserved count
		_, err = tx.Exec(ctx, `
		UPDATE events 
		SET reserved = reserved + 1 
		WHERE id = $1
	`, eventID)
		if err != nil {
			return err
		}
		return nil
	})
}

func (r *BookingsRepository) GetBookingStatus(ctx context.Context, bookingID string) (string, error) {
	query := `SELECT status FROM bookings WHERE id = $1`

	var status string
	err := r.db.Pool.QueryRow(ctx, query, bookingID).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}

	return status, nil
}
