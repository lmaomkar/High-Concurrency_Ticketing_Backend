package events

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	jwtMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	"github.com/samirwankhede/lewly-pgpyewj/internal/service/events"
)

type EventsHandler struct {
	log    *zap.Logger
	svc    *events.EventsService
	secret string
}

func NewEventsHandler(log *zap.Logger, svc *events.EventsService, secret string) *EventsHandler {
	return &EventsHandler{log: log, svc: svc, secret: secret}
}

func (h *EventsHandler) Register(r *gin.Engine) {
	r.GET("/v1/events", h.list)
	r.GET("/v1/events/all", h.listAll)
	r.GET("/v1/events/upcoming", h.listUpcoming)
	r.GET("/v1/events/popular", h.listPopular)
	r.GET("/v1/events/:id", h.get)
	r.GET("/v1/events/:id/seats", h.getAvailableSeats)

	// Protected routes for liking events
	protected := r.Group("/v1/events")
	protected.Use(jwtMiddleware.Middleware(h.secret, false))
	{
		protected.POST("/:id/like", h.likeEvent)
		protected.DELETE("/:id/like", h.unlikeEvent)
	}
}

func (h *EventsHandler) list(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	q := c.Query("q")
	var fromPtr, toPtr *time.Time
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			fromPtr = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			toPtr = &t
		}
	}
	items, err := h.svc.List(c.Request.Context(), limit, offset, q, fromPtr, toPtr)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items, "limit": limit, "offset": offset})
}

func (h *EventsHandler) listAll(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, err := h.svc.ListAll(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items, "limit": limit, "offset": offset})
}

func (h *EventsHandler) listUpcoming(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, err := h.svc.ListUpcoming(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items, "limit": limit, "offset": offset})
}

func (h *EventsHandler) listPopular(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	items, err := h.svc.ListPopular(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"events": items, "limit": limit, "offset": offset})
}

func (h *EventsHandler) get(c *gin.Context) {
	id := c.Param("id")
	e, rem, err := h.svc.Get(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"event": e, "tokens_remaining": rem})
}

func (h *EventsHandler) getAvailableSeats(c *gin.Context) {
	id := c.Param("id")
	seats, err := h.svc.GetAvailableSeats(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"seats": seats})
}

func (h *EventsHandler) likeEvent(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("uid")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.svc.LikeEvent(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Event liked successfully"})
}

func (h *EventsHandler) unlikeEvent(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetString("uid")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err := h.svc.UnlikeEvent(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Event unliked successfully"})
}
