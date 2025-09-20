package waitlist

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	jwtMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/waitlist"
)

type WaitlistHandler struct {
	repo   *waitlist.WaitlistRepository
	secret string
}

func NewWaitlistHandler(repo *waitlist.WaitlistRepository, secret string) *WaitlistHandler {
	return &WaitlistHandler{repo: repo, secret: secret}
}

func (h *WaitlistHandler) Register(r *gin.Engine) {
	r.GET("/v1/waitlist/:event_id/count", h.getCount)
	r.GET("/v1/waitlist/:event_id", h.list)
	// These routes should be kept only for upcoming as default adds to waitlist in booking if capacity full
	protected := r.Group("/v1/waitlist")
	protected.Use(jwtMiddleware.Middleware(h.secret, false))
	{
		protected.POST("/:event_id/join", h.join)
		protected.POST("/:event_id/optout", h.optout)
	}

}

func (h *WaitlistHandler) join(c *gin.Context) {
	eventID := c.Param("event_id")
	userID := c.GetString("uid")
	pos, err := h.repo.Add(c.Request.Context(), eventID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"position": pos})
}

func (h *WaitlistHandler) optout(c *gin.Context) {
	eventID := c.Param("event_id")
	userID := c.GetString("uid")
	if err := h.repo.OptOut(c.Request.Context(), eventID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"opted_out": true})
}

func (h *WaitlistHandler) getCount(c *gin.Context) {
	eventID := c.Param("event_id")
	count, err := h.repo.Count(c.Request.Context(), eventID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

func (h *WaitlistHandler) list(c *gin.Context) {
	eventID := c.Param("event_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	entries, err := h.repo.ListByEvent(c.Request.Context(), eventID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"waitlist": entries, "limit": limit, "offset": offset})
}
