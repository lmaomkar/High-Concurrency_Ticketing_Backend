package payment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
)

type PaymentService struct {
	log      *zap.Logger
	bookings *bookings.BookingsRepository
	events   *events.EventsRepository
}

type PaymentRequest struct {
	BookingID string  `json:"booking_id"`
	Amount    float64 `json:"amount"`
	PaymentID string  `json:"payment_id"` // From payment provider (e.g., Stripe)
}

type PaymentResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	BookingID string `json:"booking_id,omitempty"`
}

var (
	ErrBookingNotFound = errors.New("booking not found")
	ErrInvalidAmount   = errors.New("invalid amount")
	ErrPaymentFailed   = errors.New("payment failed")
	ErrBookingExpired  = errors.New("booking expired")
	ErrAlreadyPaid     = errors.New("booking already paid")
)

func NewPaymentService(log *zap.Logger, bookings *bookings.BookingsRepository, events *events.EventsRepository) *PaymentService {
	return &PaymentService{
		log:      log,
		bookings: bookings,
		events:   events,
	}
}

func (s *PaymentService) ProcessBookingPayment(ctx context.Context, req PaymentRequest) (*PaymentResponse, error) {
	// Get booking
	booking, err := s.bookings.GetByID(ctx, req.BookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, ErrBookingNotFound
	}

	// Check if booking is still pending
	if booking.Status != "pending" {
		if booking.Status == "booked" {
			return nil, ErrAlreadyPaid
		}
		return nil, fmt.Errorf("booking is in %s status", booking.Status)
	}

	// Get event details
	event, err := s.events.Get(ctx, booking.EventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, errors.New("event not found")
	}

	// Parse seats from JSON to get actual count
	var seats []string
	if len(booking.Seats) > 0 {
		if err := json.Unmarshal(booking.Seats, &seats); err != nil {
			s.log.Error("Failed to parse seats JSON", zap.Error(err))
			seats = []string{"seat1"} // fallback
		}
	}
	if len(seats) == 0 {
		seats = []string{"seat1"} // fallback
	}

	// Validate amount based on actual seat count
	expectedAmount := event.TicketPrice * float64(len(seats))
	if req.Amount < expectedAmount {
		return nil, ErrInvalidAmount
	}

	// Simulate payment processing (in real implementation, integrate with Stripe/PayPal)
	success := s.simulatePaymentProcessing(req.PaymentID, req.Amount)
	if !success {
		return &PaymentResponse{
			Success: false,
			Message: "Payment processing failed",
		}, nil
	}

	// Update booking status to paid
	err = s.bookings.UpdatePaymentStatus(ctx, req.BookingID, "paid", req.Amount)
	if err != nil {
		s.log.Error("Failed to update payment status", zap.Error(err))
		return nil, err
	}

	// Finalize booking (mark as booked and update event reserved count)
	seatsBytes, _ := json.Marshal(seats)
	err = s.bookings.FinalizeBooking(ctx, req.BookingID, seatsBytes, req.Amount)
	if err != nil {
		s.log.Error("Failed to finalize booking", zap.Error(err))
		return nil, err
	}

	return &PaymentResponse{
		Success:   true,
		Message:   "Payment processed successfully",
		BookingID: req.BookingID,
	}, nil
}

func (s *PaymentService) ProcessCancellationRefund(ctx context.Context, BookingID string) (*PaymentResponse, error) {
	// Get booking
	booking, err := s.bookings.GetByID(ctx, BookingID)
	if err != nil {
		return nil, err
	}
	if booking == nil {
		return nil, ErrBookingNotFound
	}

	// Check if booking was actually paid
	if booking.PaymentStatus != "paid" {
		return nil, errors.New("booking was not paid")
	}

	// Get event details for cancellation fee calculation
	event, err := s.events.Get(ctx, booking.EventID)
	if err != nil {
		return nil, err
	}
	if event == nil {
		return nil, errors.New("event not found")
	}

	// Calculate refund amount (subtract cancellation fee)
	cancellationFee := event.CancellationFee
	refundAmount := booking.AmountPaid - cancellationFee
	if refundAmount < 0 {
		refundAmount = 0
	}

	// Simulate refund processing
	success := s.simulateRefundProcessing(booking.ID, refundAmount)
	if !success {
		return &PaymentResponse{
			Success: false,
			Message: "Refund processing failed",
		}, nil
	}

	// Update booking payment status
	err = s.bookings.UpdatePaymentStatus(ctx, BookingID, "refunded", refundAmount)
	if err != nil {
		s.log.Error("Failed to update refund status", zap.Error(err))
		return nil, err
	}

	return &PaymentResponse{
		Success:   true,
		Message:   fmt.Sprintf("Refund processed successfully. Amount: %.2f, Cancellation fee: %.2f", refundAmount, cancellationFee),
		BookingID: BookingID,
	}, nil
}

func (s *PaymentService) ProcessEventCancellationRefund(ctx context.Context, eventID string) error {
	// Get all paid bookings for the event
	bookings, err := s.bookings.ListByEvent(ctx, eventID, 1000, 0) // Get all bookings
	if err != nil {
		return err
	}

	// Get event details
	event, err := s.events.Get(ctx, eventID)
	if err != nil {
		return err
	}
	if event == nil {
		return errors.New("event not found")
	}

	// Process refunds for all paid bookings
	for _, booking := range bookings {
		if booking.PaymentStatus == "paid" {
			// Full refund for event cancellation
			success := s.simulateRefundProcessing(booking.ID, booking.AmountPaid)
			if success {
				err = s.bookings.UpdatePaymentStatus(ctx, booking.ID, "refunded", booking.AmountPaid)
				if err != nil {
					s.log.Error("Failed to update refund status", zap.Error(err), zap.String("booking_id", booking.ID))
				}
			} else {
				s.log.Error("Refund processing failed", zap.String("booking_id", booking.ID))
			}
		}
	}

	return nil
}

// Simulate payment processing (replace with real payment provider integration)
func (s *PaymentService) simulatePaymentProcessing(paymentID string, amount float64) bool {
	// In real implementation, this would call Stripe/PayPal API
	s.log.Info("Processing payment", zap.String("payment_id", paymentID), zap.Float64("amount", amount))

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	// Simulate 95% success rate
	return true // Always succeed for demo
}

// Simulate refund processing (replace with real payment provider integration)
func (s *PaymentService) simulateRefundProcessing(bookingID string, amount float64) bool {
	// In real implementation, this would call Stripe/PayPal API
	s.log.Info("Processing refund", zap.String("booking_id", bookingID), zap.Float64("amount", amount))

	// Simulate some processing time
	time.Sleep(100 * time.Millisecond)

	// Simulate 95% success rate
	return true // Always succeed for demo
}
