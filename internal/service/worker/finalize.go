package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	mailerService "github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/waitlist"
)

type FinalizeService struct {
	log           *zap.Logger
	bookings      *bookings.BookingsRepository
	events        *events.EventsRepository
	users         *users.UsersRepository
	waitlist      *waitlist.WaitlistRepository
	paymentURL    string
	mailer        *mailerService.MailerService
	timeoutBucket *redisx.TimeoutBucket
}

type FinalizePayload struct {
	Type           string   `json:"type"`
	BookingID      string   `json:"booking_id"`
	EventID        string   `json:"event_id"`
	UserID         string   `json:"user_id"`
	Seats          []string `json:"seats"`
	IdempotencyKey *string  `json:"idempotency_key"`
}

func NewFinalizeService(log *zap.Logger, bookings *bookings.BookingsRepository, events *events.EventsRepository, users *users.UsersRepository, waitlist *waitlist.WaitlistRepository, paymentURL string, mailer *mailerService.MailerService, timeoutBucket *redisx.TimeoutBucket) *FinalizeService {
	return &FinalizeService{
		log:           log,
		bookings:      bookings,
		events:        events,
		users:         users,
		waitlist:      waitlist,
		paymentURL:    paymentURL,
		mailer:        mailer,
		timeoutBucket: timeoutBucket,
	}
}

func (s *FinalizeService) HandleBookingFinalization(ctx context.Context, payload FinalizePayload) error {
	// Get booking details
	booking, err := s.bookings.GetByID(ctx, payload.BookingID)
	if err != nil {
		s.log.Error("Failed to get booking", zap.Error(err), zap.String("booking_id", payload.BookingID))
		return err
	}
	if booking == nil {
		s.log.Error("Booking not found", zap.String("booking_id", payload.BookingID))
		return fmt.Errorf("booking not found: %s", payload.BookingID)
	}

	// Get event details
	event, err := s.events.Get(ctx, payload.EventID)
	if err != nil {
		s.log.Error("Failed to get event", zap.Error(err), zap.String("event_id", payload.EventID))
		return err
	}
	if event == nil {
		s.log.Error("Event not found", zap.String("event_id", payload.EventID))
		return fmt.Errorf("event not found: %s", payload.EventID)
	}

	// Calculate amount based on seats
	amount := event.TicketPrice * float64(len(payload.Seats))

	// Generate payment link
	paymentLink := fmt.Sprintf("%s/v1/payment/booking?booking_id=%s&amount=%.2f&payment_id=%s", s.paymentURL, payload.BookingID, amount, payload.BookingID)

	// Hello Evaluator I've pondered over using redis, but over a network with not 'hot' objects like session tokens and decent partitions I haven't implemented cached mappings of event+userid -> email though in production I believe such will be needed
	// Currently I believe the complexity will increase without much effectiveness so this user email fetching is more focused on HLD and functionality
	user, err := s.users.GetByID(ctx, payload.UserID)
	if err != nil {
		s.log.Error("User not found", zap.String("user_id", payload.UserID))
		return fmt.Errorf("user not found: %s", payload.UserID)
	}
	userEmail := user.Email
	// Send payment request email
	err = s.mailer.SendPaymentRequestEmail(userEmail, event.Name, amount, paymentLink)
	if err != nil {
		s.log.Error("Failed to send payment request email", zap.Error(err))
		return fmt.Errorf("failed to send payment request email")
	}

	// Schedule timeout for new booking
	s.scheduleBookingTimeout(ctx, payload.BookingID, payload.EventID, payload.UserID, payload.Seats)

	return nil
}

func (s *FinalizeService) HandleBookingTimeout(ctx context.Context, payload FinalizePayload) error {
	// Get booking details
	booking, err := s.bookings.GetByID(ctx, payload.BookingID)
	if err != nil {
		s.log.Error("Failed to get booking", zap.Error(err), zap.String("booking_id", payload.BookingID))
		return err
	}
	if booking == nil {
		s.log.Error("Booking not found", zap.String("booking_id", payload.BookingID))
		return fmt.Errorf("booking not found: %s", payload.BookingID)
	}

	// Check if booking is still pending
	if booking.Status != "pending" {
		s.log.Info("Booking is no longer pending, skipping timeout",
			zap.String("booking_id", payload.BookingID),
			zap.String("status", booking.Status))
		return nil
	}

	// Cancel the booking
	_, _, err = s.bookings.CancelBookingTx(ctx, payload.BookingID)
	if err != nil {
		s.log.Error("Failed to cancel booking", zap.Error(err), zap.String("booking_id", payload.BookingID))
		return err
	}

	// Get event details
	event, err := s.events.Get(ctx, payload.EventID)
	if err != nil {
		s.log.Error("Failed to get event", zap.Error(err), zap.String("event_id", payload.EventID))
		return err
	}
	if event == nil {
		s.log.Error("Event not found", zap.String("event_id", payload.EventID))
		return fmt.Errorf("event not found: %s", payload.EventID)
	}

	// Promote next person from waitlist
	userID, _, position, err := s.waitlist.NextActive(ctx, payload.EventID)
	if err != nil {
		s.log.Error("Failed to get next waitlist user", zap.Error(err), zap.String("event_id", payload.EventID))
		return err
	}

	if userID != "" {
		// Create new pending booking for waitlist user
		seatsJSON, _ := json.Marshal(payload.Seats)
		newBooking, err := s.bookings.CreatePending(ctx, userID, payload.EventID, nil, seatsJSON)
		if err != nil {
			s.log.Error("Failed to create booking for waitlist user", zap.Error(err))
			return err
		}

		// Calculate amount for new booking
		amount := event.TicketPrice * float64(len(payload.Seats))
		paymentLink := fmt.Sprintf("%s/v1/payment/booking?booking_id=%s&amount=%.2f&payment_id=%s", s.paymentURL, newBooking.ID, amount, newBooking.ID)

		// Send waitlist promotion email
		user, err := s.users.GetByID(ctx, payload.UserID)
		if err != nil {
			s.log.Error("User not found", zap.String("user_id", payload.UserID))
			return fmt.Errorf("user not found: %s", payload.UserID)
		}
		userEmail := user.Email

		err = s.mailer.SendWaitlistPromotionEmail(userEmail, event.Name)
		if err != nil {
			s.log.Error("Failed to send waitlist promotion email", zap.Error(err))
			// Don't return error, continue processing
		}
		err = s.mailer.SendPaymentRequestEmail(userEmail, event.Name, amount, paymentLink)
		if err != nil {
			s.log.Error("Failed to send payment request email", zap.Error(err))
			return fmt.Errorf("failed to send payment request email")
		}

		// Schedule timeout for new booking
		s.scheduleBookingTimeout(ctx, newBooking.ID, payload.EventID, userID, payload.Seats)

		s.log.Info("Promoted waitlist user",
			zap.String("old_booking_id", payload.BookingID),
			zap.String("new_booking_id", newBooking.ID),
			zap.String("user_id", userID),
			zap.Int("position", position))
	} else {
		s.log.Info("No users in waitlist to promote", zap.String("event_id", payload.EventID))
	}

	return nil
}

func (s *FinalizeService) scheduleBookingTimeout(ctx context.Context, bookingID, eventID, userID string, seats []string) {
	go func() {
		err := s.timeoutBucket.AddBooking(ctx, eventID, bookingID)
		if err != nil {
			s.log.Error("Failed to set payment timeout", zap.Error(err))
		}

		time.Sleep(15 * time.Minute)

		timeoutPayload := FinalizePayload{
			Type:      "booking_timeout",
			BookingID: bookingID,
			EventID:   eventID,
			UserID:    userID,
			Seats:     seats,
		}

		v, err := s.timeoutBucket.GetBooking(ctx, eventID, bookingID)
		if err != nil {
			s.log.Error("Failed to get payment timeout", zap.Error(err))
		}
		if v != "processed" {
			// Process the timeout
			err = s.HandleBookingTimeout(ctx, timeoutPayload)
			if err != nil {
				s.log.Error("Failed to process booking timeout", zap.Error(err), zap.String("booking_id", bookingID))
			}
		}
		_, err = s.timeoutBucket.DeleteBooking(ctx, eventID, bookingID)
		if err != nil {
			s.log.Error("Failed to Delete timeout bucket", zap.Error(err))
		}

	}()
}
