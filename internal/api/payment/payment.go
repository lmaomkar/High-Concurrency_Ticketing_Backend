package payment

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	jwtMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	"github.com/samirwankhede/lewly-pgpyewj/internal/service/payment"
)

type PaymentHandler struct {
	log    *zap.Logger
	svc    *payment.PaymentService
	secret string
}

func NewPaymentHandler(log *zap.Logger, svc *payment.PaymentService, secret string) *PaymentHandler {
	return &PaymentHandler{log: log, svc: svc, secret: secret}
}

func (h *PaymentHandler) Register(r *gin.Engine) {
	payments := r.Group("/v1/payment")
	payments.GET("/booking", h.processBookingPayment)
	payments.GET("/refund", h.processRefund)
	payments.Use(jwtMiddleware.Middleware(h.secret, true))
	{
		payments.POST("/events/:id/refund", h.processEventCancellationRefund)
	}
}

func (h *PaymentHandler) processBookingPayment(c *gin.Context) {
	booking_id := c.Query("booking_id")
	amt, err := strconv.ParseFloat(c.DefaultQuery("amount", "-1"), 64)
	payment_id := c.Query("payment_id")
	req := payment.PaymentRequest{
		BookingID: booking_id,
		Amount:    amt,
		PaymentID: payment_id,
	}
	if amt == float64(-1) || err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error with amount parameter"})
		return
	}

	resp, err := h.svc.ProcessBookingPayment(c.Request.Context(), req)
	if err != nil {
		if err == payment.ErrBookingNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		if err == payment.ErrInvalidAmount {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid amount"})
			return
		}
		if err == payment.ErrAlreadyPaid {
			c.JSON(http.StatusConflict, gin.H{"error": "Booking already paid"})
			return
		}
		h.log.Error("Payment processing failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if resp.Success {
		c.JSON(http.StatusOK, resp)
	} else {
		c.JSON(http.StatusPaymentRequired, resp)
	}
}

func (h *PaymentHandler) processRefund(c *gin.Context) {
	BookingID := c.Query("booking_id")
	if BookingID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Booking not found"})
	}

	resp, err := h.svc.ProcessCancellationRefund(c.Request.Context(), BookingID)
	if err != nil {
		if err == payment.ErrBookingNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
			return
		}
		h.log.Error("Refund processing failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if resp.Success {
		c.JSON(http.StatusOK, resp)
	} else {
		c.JSON(http.StatusPaymentRequired, resp)
	}
}

func (h *PaymentHandler) processEventCancellationRefund(c *gin.Context) {
	eventID := c.Param("id")

	err := h.svc.ProcessEventCancellationRefund(c.Request.Context(), eventID)
	if err != nil {
		h.log.Error("Event cancellation refund failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Event cancellation refunds processed successfully"})
}
