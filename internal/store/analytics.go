package store

import (
	"context"
	"time"
)

type Analytics struct {
	TotalBookings int `json:"total_bookings"`
	Cancellations int `json:"cancellations"`
}

type AnalyticsRepository struct{ db *DB }

func NewAnalyticsRepository(db *DB) *AnalyticsRepository { return &AnalyticsRepository{db: db} }

func (r *AnalyticsRepository) Summary(ctx context.Context, from, to time.Time) (Analytics, error) {
	var a Analytics
	err := r.db.Pool.QueryRow(ctx, `
        SELECT
          COALESCE(SUM(CASE WHEN status='booked' THEN 1 ELSE 0 END),0) AS total_booked,
          COALESCE(SUM(CASE WHEN status='cancelled' THEN 1 ELSE 0 END),0) AS cancellations
        FROM bookings
        WHERE created_at >= $1 AND created_at <= $2`, from, to).Scan(&a.TotalBookings, &a.Cancellations)
	return a, err
}
