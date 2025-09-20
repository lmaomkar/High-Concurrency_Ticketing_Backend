package api

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/samirwankhede/lewly-pgpyewj/internal/api/admin"
	"github.com/samirwankhede/lewly-pgpyewj/internal/api/auth"
	"github.com/samirwankhede/lewly-pgpyewj/internal/api/bookings"
	"github.com/samirwankhede/lewly-pgpyewj/internal/api/events"
	"github.com/samirwankhede/lewly-pgpyewj/internal/api/payment"
	"github.com/samirwankhede/lewly-pgpyewj/internal/api/waitlist"
	"github.com/samirwankhede/lewly-pgpyewj/internal/config"
	kafkax "github.com/samirwankhede/lewly-pgpyewj/internal/kafka"
	"github.com/samirwankhede/lewly-pgpyewj/internal/mailer"
	"github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	adminService "github.com/samirwankhede/lewly-pgpyewj/internal/service/admin"
	authService "github.com/samirwankhede/lewly-pgpyewj/internal/service/auth"
	bookingsService "github.com/samirwankhede/lewly-pgpyewj/internal/service/bookings"
	eventsService "github.com/samirwankhede/lewly-pgpyewj/internal/service/events"
	mailerService "github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	paymentService "github.com/samirwankhede/lewly-pgpyewj/internal/service/payment"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store"
	storeAdmin "github.com/samirwankhede/lewly-pgpyewj/internal/store/admin"
	storeBookings "github.com/samirwankhede/lewly-pgpyewj/internal/store/bookings"
	storeEvents "github.com/samirwankhede/lewly-pgpyewj/internal/store/events"
	storeSeats "github.com/samirwankhede/lewly-pgpyewj/internal/store/seats"
	storeUsers "github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
	storeWaitlist "github.com/samirwankhede/lewly-pgpyewj/internal/store/waitlist"
)

// RegisterRoutes wires all HTTP routes.
func RegisterRoutes(r *gin.Engine, log *zap.Logger) {
	r.Use(middleware.MetricsMiddleware())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"name":        "Evently",
			"description": "A scalable event booking platform with concurrency-safe ticketing, waitlists, and admin analytics.",
			"version":     "1.0.0",
			"docs":        "/docs",
			"endpoints":   []string{"/v1/health", "/v1/events", "/v1/bookings", "/v1/waitlist", "/admin"},
		})
	})
	r.GET("/v1/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	RegisterDocs(r)
	cfg := config.Load()
	// global rate limit (demo)
	r.Use(middleware.HybridRateLimit(redisx.NewTokenBucket(cfg.RedisAddr).GetClient(), 50, 100))

	// DI wiring for all services
	db, err := store.NewDB(context.Background(), cfg.PostgresURL, int32(cfg.MaxDBConnections))
	if err == nil {
		// When DB is unavailable, endpoints will still serve 500 gracefully.

		// Create repositories
		eventsRepo := storeEvents.NewEventsRepository(db, log)
		bookingsRepo := storeBookings.NewBookingsRepository(db, log)
		usersRepo := storeUsers.NewUsersRepository(db, log)
		waitlistRepo := storeWaitlist.NewWaitlistRepository(db, log)
		adminRepo := storeAdmin.NewAdminRepository(db, log)
		seatsRepo := storeSeats.NewSeatsRepository(db, log)

		// Create Redis client and mailer
		tokens := redisx.NewTokenBucket(cfg.RedisAddr)
		mailerSender := &mailer.SMTPSender{
			Host: cfg.SMTPHost,
			Port: cfg.SMTPPort,
			User: cfg.SMTPUser,
			Pass: cfg.SMTPPass,
			From: cfg.SMTPFrom,
		}
		mailerSvc := mailerService.NewMailerService(log, mailerSender)

		// Create services
		eventsSvc := eventsService.NewEventsService(log, eventsRepo, tokens)
		authSvc := authService.NewAuthService(log, usersRepo, tokens, cfg.JWTSigningSecret, mailerSvc)
		producer := kafkax.NewProducer([]string{cfg.KafkaBrokers}, "bookings")
		bookingsSvc := bookingsService.NewBookingsService(log, bookingsRepo, eventsRepo, usersRepo, tokens, producer, waitlistRepo, mailerSvc, cfg.PaymentURL)
		paymentSvc := paymentService.NewPaymentService(log, bookingsRepo, eventsRepo)
		adminSvc := adminService.NewAdminService(log, eventsRepo, usersRepo, bookingsRepo, adminRepo, seatsRepo, tokens, mailerSvc)

		// Register handlers
		events.NewEventsHandler(log, eventsSvc, cfg.JWTSigningSecret).Register(r)
		auth.NewAuthHandler(log, authSvc, cfg.JWTSigningSecret).Register(r)
		bookings.NewBookingsHandler(bookingsSvc, cfg.JWTSigningSecret).Register(r)
		waitlist.NewWaitlistHandler(waitlistRepo, cfg.JWTSigningSecret).Register(r)
		payment.NewPaymentHandler(log, paymentSvc, cfg.JWTSigningSecret).Register(r)
		admin.NewAdminHandler(adminSvc, cfg.JWTSigningSecret).Register(r)

	} else {
		log.Warn("db init failed", zap.Error(err))
	}
}
