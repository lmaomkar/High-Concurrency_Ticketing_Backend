package bookings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	kafkax "github.com/samirwankhede/lewly-pgpyewj/internal/kafka"
	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	mailer "github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/waitlist"
)

type BookingsService struct {
	log        *zap.Logger
	repo       *bookings.BookingsRepository
	events     *events.EventsRepository
	users      *users.UsersRepository
	tokens     *redisx.TokenBucket
	prod       *kafkax.Producer
	wait       *waitlist.WaitlistRepository
	mailer     *mailer.MailerService
	paymentURL string
}

type BookingRequest struct {
	UserID         string   `json:"user_id"`
	Seats          []string `json:"seats"`
	IdempotencyKey *string  `json:"idempotency_key"`
}

type BookingResponse struct {
	BookingID string `json:"booking_id"`
	Status    string `json:"status"`
	Position  int    `json:"position,omitempty"`
}

func NewBookingsService(log *zap.Logger, repo *bookings.BookingsRepository, events *events.EventsRepository, users *users.UsersRepository, tokens *redisx.TokenBucket, prod *kafkax.Producer, wait *waitlist.WaitlistRepository, mailer *mailer.MailerService, paymentURL string) *BookingsService {
	return &BookingsService{log: log, repo: repo, events: events, users: users, tokens: tokens, prod: prod, wait: wait, mailer: mailer, paymentURL: paymentURL}
}

func (s *BookingsService) Create(ctx context.Context, eventID string, userID string, IdempotencyKey *string, seats []string) (*BookingResponse, int, error) {
	// Check if event exists and is not expired
	event, err := s.events.Get(ctx, eventID)
	if err != nil {
		return nil, 500, err
	}
	if event == nil {
		return nil, 404, errors.New("event not found")
	}

	// Check if event is expired
	if event.EndTime.Before(time.Now()) {
		// Update event status to expired
		s.events.UpdateStatus(ctx, eventID, "expired")
		return nil, 400, errors.New("event is expired")
	}

	// Check if user is trying to book more than maximum allowed
	if len(seats) > event.MaximumTicketsPerBooking {
		return nil, 400, fmt.Errorf("cannot book more than %d tickets", event.MaximumTicketsPerBooking)
	}

	// Idempotency check
	if IdempotencyKey != nil && *IdempotencyKey != "" {
		if b, err := s.repo.GetByIdempotency(ctx, *IdempotencyKey); err == nil && b != nil {
			return &BookingResponse{BookingID: b.ID, Status: b.Status}, 200, nil
		}
	}

	// Reserve tokens for the number of seats requested
	ok, err := s.tokens.Reserve(ctx, eventID, len(seats))
	if err != nil {
		return nil, 500, err
	}

	if ok {
		// Store seats in booking
		seatsJSON, _ := json.Marshal(seats)
		b, err := s.repo.CreatePending(ctx, userID, eventID, IdempotencyKey, seatsJSON)
		if err != nil {
			return nil, 500, err
		}

		payload := map[string]any{
			"type":            "finalize_booking",
			"booking_id":      b.ID,
			"event_id":        eventID,
			"user_id":         userID,
			"seats":           seats,
			"idempotency_key": IdempotencyKey,
		}
		by, _ := json.Marshal(payload)
		if err := s.prod.Publish(ctx, []byte(eventID), by); err != nil {
			s.log.Error("kafka publish error", zap.Error(err))
		}
		return &BookingResponse{BookingID: b.ID, Status: "pending"}, 202, nil
	}

	// Fallback: Auto waitlist
	position, err := s.wait.Add(ctx, eventID, userID)
	if err != nil {
		return nil, 500, err
	}

	return &BookingResponse{Status: "waitlisted", Position: position}, 200, nil
}

var ErrValidation = errors.New("validation error")

func (s *BookingsService) Cancel(ctx context.Context, bookingID string) (map[string]any, int, error) {
	b, wasBooked, err := s.repo.CancelBookingTx(ctx, bookingID)
	if err != nil {
		return nil, 409, err
	}

	// release tokens when a booked reservation is cancelled
	if wasBooked {
		// Get the number of seats from the booking
		var seats []string
		if len(b.Seats) > 0 {
			json.Unmarshal(b.Seats, &seats)
		}
		seatCount := len(seats)
		if seatCount == 0 {
			seatCount = 1 // fallback
		}

		_ = s.tokens.Release(ctx, b.EventID, seatCount)

		event, err := s.events.Get(ctx, b.EventID)
		if err != nil {
			return nil, 409, err
		}

		// Send cancellation email with fee and payment link
		if s.mailer != nil {
			user, err := s.users.GetByID(ctx, b.UserID)
			if err != nil {
				return nil, 409, err
			}
			paymentLink := fmt.Sprintf("%s/v1/payment/refund?booking_id=%s", s.paymentURL, bookingID)
			s.mailer.SendCancellationEmail(user.Email, event.CancellationFee, paymentLink)
		}

		// Promote next person from waitlist
		if s.wait != nil {
			if id, userID, _, err := s.wait.NextActive(ctx, b.EventID); err == nil && userID != "" {
				// Get seats from the cancelled booking
				var seats []string
				if len(b.Seats) > 0 {
					json.Unmarshal(b.Seats, &seats)
				}
				seatsJSON, _ := json.Marshal(seats)
				if pb, cerr := s.repo.CreatePending(ctx, userID, b.EventID, nil, seatsJSON); cerr == nil {
					payload := map[string]any{
						"type":            "finalize_booking",
						"booking_id":      pb.ID,
						"event_id":        b.EventID,
						"user_id":         userID,
						"seats":           seats,
						"idempotency_key": pb.IdempotencyKey,
					}
					by, _ := json.Marshal(payload)
					_ = s.prod.Publish(ctx, []byte(b.EventID), by)
					_ = s.wait.Remove(ctx, id)

					// Send waitlist promotion email
					if s.mailer != nil {
						user, err := s.users.GetByID(ctx, userID)
						if err != nil {
							return nil, 409, err
						}
						s.mailer.SendWaitlistPromotionEmail(user.Email, event.Name)
					}
				}
			}
		}
	}
	return map[string]any{"booking_id": b.ID, "status": b.Status}, 200, nil
}

func (s *BookingsService) GetBookingStatus(ctx context.Context, bookingID string) (string, error) {
	return s.repo.GetBookingStatus(ctx, bookingID)
}

func (s *BookingsService) GetAvailableSeats(ctx context.Context, eventID string) ([]string, error) {
	return s.events.GetAvailableSeats(ctx, eventID)
}

func (s *BookingsService) ListUserBookings(ctx context.Context, userID string, limit, offset int) ([]*bookings.Booking, error) {
	return s.repo.ListByUser(ctx, userID, limit, offset)
}

func (s *BookingsService) FinalizeBooking(ctx context.Context, bookingID string, seats []string, amountPaid float64) error {
	seatsJSON, _ := json.Marshal(seats)
	return s.repo.FinalizeBooking(ctx, bookingID, seatsJSON, amountPaid)
}
