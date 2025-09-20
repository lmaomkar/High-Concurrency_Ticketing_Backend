package bookings

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	jwtMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	"github.com/samirwankhede/lewly-pgpyewj/internal/service/bookings"
)

type BookingsHandler struct {
	svc    *bookings.BookingsService
	secret string
}

func NewBookingsHandler(svc *bookings.BookingsService, secret string) *BookingsHandler {
	return &BookingsHandler{svc: svc, secret: secret}
}

func (h *BookingsHandler) Register(r *gin.Engine) {
	// Protected routes
	protected := r.Group("/v1/bookings")
	protected.Use(jwtMiddleware.Middleware(h.secret, false))
	{
		protected.POST("/:id/book", h.book)
		protected.GET("/:id/status", h.getStatus)
		protected.POST("/:id/cancel", h.cancel)
		protected.GET("/user-bookings", h.listUserBookings)
	}
}

func (h *BookingsHandler) book(c *gin.Context) {
	eventID := c.Param("id")
	userID := c.GetString("uid")
	IdempotencyKey := uuid.NewString() //This Part should be handled by another service - currently we're just creating a new uuid
	type Seats struct {
		Seats []string `json:"seats" binding:"required"`
	}
	var seats Seats
	if err := c.ShouldBindJSON(&seats); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing user id"})
		return
	}
	if eventID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing event id"})
		return
	}
	resp, code, err := h.svc.Create(c, eventID, userID, &IdempotencyKey, seats.Seats)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, resp)
}

func (h *BookingsHandler) getStatus(c *gin.Context) {
	id := c.Param("id")
	status, err := h.svc.GetBookingStatus(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if status == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "Booking not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
}

func (h *BookingsHandler) listUserBookings(c *gin.Context) {
	userID := c.GetString("uid")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	bookings, err := h.svc.ListUserBookings(c.Request.Context(), userID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"bookings": bookings, "limit": limit, "offset": offset})
}

func (h *BookingsHandler) cancel(c *gin.Context) {
	id := c.Param("id")
	resp, code, err := h.svc.Cancel(c.Request.Context(), id)
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	c.JSON(code, resp)
}
