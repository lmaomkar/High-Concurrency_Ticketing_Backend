package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"go.uber.org/zap"

	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	mailer "github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/admin"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/seats"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
)

type AdminService struct {
	log      *zap.Logger
	events   *events.EventsRepository
	users    *users.UsersRepository
	bookings *bookings.BookingsRepository
	admin    *admin.AdminRepository
	seats    *seats.SeatsRepository
	tokens   *redisx.TokenBucket
	mailer   *mailer.MailerService
}

func NewAdminService(log *zap.Logger, events *events.EventsRepository, users *users.UsersRepository, bookings *bookings.BookingsRepository, admin *admin.AdminRepository, seats *seats.SeatsRepository, tokens *redisx.TokenBucket, mailer *mailer.MailerService) *AdminService {
	return &AdminService{log: log, events: events, users: users, bookings: bookings, admin: admin, seats: seats, tokens: tokens, mailer: mailer}
}

type AdminEvent struct {
	Name                     string          `json:"name" binding:"required"`
	Venue                    string          `json:"venue" binding:"required"`
	Category                 string          `json:"category"`
	StartTime                time.Time       `json:"start_time" binding:"required"`
	EndTime                  time.Time       `json:"end_time" binding:"required"`
	Capacity                 int             `json:"capacity" binding:"required"`
	Metadata                 json.RawMessage `json:"metadata"`
	TicketPrice              float64         `json:"ticket_price"`
	CancellationFee          float64         `json:"cancellation_fee"`
	MaximumTicketsPerBooking int             `json:"maximum_tickets_per_booking"`
	Seats                    []string        `json:"seats" binding:"required"`
}

func (a *AdminService) CreateEvent(ctx context.Context, in AdminEvent) (*events.Event, error) {
	// Validate seats array size matches capacity
	if len(in.Seats) != in.Capacity {
		return nil, errors.New("seats array size must match event capacity")
	}

	e := &events.Event{
		Name:                     in.Name,
		Venue:                    in.Venue,
		Category:                 in.Category,
		StartTime:                in.StartTime,
		EndTime:                  in.EndTime,
		Capacity:                 in.Capacity,
		Metadata:                 in.Metadata,
		Status:                   "upcoming",
		TicketPrice:              in.TicketPrice,
		CancellationFee:          in.CancellationFee,
		MaximumTicketsPerBooking: in.MaximumTicketsPerBooking,
	}
	e, err := a.events.Create(ctx, e)
	if err != nil {
		return nil, err
	}

	// Create seats in the seats table
	err = a.seats.CreateSeats(ctx, e.ID, in.Seats)
	if err != nil {
		a.log.Error("Failed to create seats", zap.Error(err), zap.String("event_id", e.ID))
		// Note: We don't return error here as the event is already created
		// In production, you might want to rollback the event creation
	}

	_ = a.tokens.InitTokens(ctx, e.ID, e.Capacity)
	return e, nil
}

func (a *AdminService) GetSummary(ctx context.Context, from, to time.Time) (*admin.AnalyticsSummary, error) {
	return a.admin.GetSummary(ctx, from, to)
}

func (a *AdminService) CancelEvent(ctx context.Context, eventID string) error {
	// Get event details for email notifications
	event, err := a.events.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return errors.New("event not found")
	}

	// Cancel the event
	err = a.admin.CancelEvent(ctx, eventID)
	if err != nil {
		return err
	}

	bookings, err := a.bookings.ListByEvent(ctx, eventID, 1000, 0) // Get all bookings
	if err != nil {
		return err
	}
	for _, booking := range bookings {
		if booking.PaymentStatus == "paid" {
			user, err := a.users.GetByID(ctx, booking.UserID)
			if err != nil {
				a.log.Error("User not found", zap.String("user_id", booking.UserID))
			}
			a.mailer.SendEventCancellationEmail(user.Email, event.Name, event.TicketPrice)
		}
	}
	a.log.Info("Event cancelled", zap.String("event_id", eventID), zap.String("event_name", event.Name))
	return nil
}

func (a *AdminService) UpdateEvent(ctx context.Context, eventID string, updates map[string]interface{}) error {
	return a.admin.UpdateEvent(ctx, eventID, updates)
}

func (a *AdminService) CreateAdminFromUser(ctx context.Context, userID string) error {
	return a.admin.CreateAdminFromUser(ctx, userID)
}

func (a *AdminService) RemoveAdmin(ctx context.Context, userID string) error {
	return a.admin.RemoveAdmin(ctx, userID)
}

func (a *AdminService) RemoveUser(ctx context.Context, userID string) error {
	return a.admin.RemoveUser(ctx, userID)
}

func (a *AdminService) GetUserByEmail(ctx context.Context, email string) (*users.User, error) {
	return a.users.GetByEmail(ctx, email)
}
