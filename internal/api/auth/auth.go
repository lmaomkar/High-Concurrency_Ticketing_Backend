package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	authMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	authService "github.com/samirwankhede/lewly-pgpyewj/internal/service/auth"
)

type AuthHandler struct {
	log    *zap.Logger
	svc    *authService.AuthService
	secret string
}

func NewAuthHandler(log *zap.Logger, svc *authService.AuthService, secret string) *AuthHandler {
	return &AuthHandler{log: log, svc: svc, secret: secret}
}

func (h *AuthHandler) Register(r *gin.Engine) {
	auth := r.Group("/v1/auth")
	{
		auth.POST("/signup", h.signup)
		auth.POST("/login", h.login)
		auth.POST("/logout", h.logout)
		auth.POST("/password/request-otp", h.requestPasswordChangeOTP)
		auth.POST("/password/verify-otp", h.verifyPasswordChangeOTP)
	}

	// Protected routes
	protected := r.Group("/v1/auth")
	protected.Use(authMiddleware.Middleware(h.secret, false))
	{
		protected.GET("/profile", h.getProfile)
		protected.PUT("/profile", h.updateProfile)
		protected.PUT("/password", h.changePassword)
	}
}

func (h *AuthHandler) signup(c *gin.Context) {
	var req authService.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.Signup(c.Request.Context(), req)
	if err != nil {
		if err == authService.ErrUserExists {
			c.JSON(http.StatusConflict, gin.H{"error": "User already exists"})
			return
		}
		h.log.Error("Signup failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *AuthHandler) login(c *gin.Context) {
	var req authService.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		if err == authService.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
			return
		}
		if err == authService.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.log.Error("Login failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *AuthHandler) logout(c *gin.Context) {
	// In a stateless JWT system, logout is typically handled client-side
	// by removing the token. We could implement token blacklisting here if needed.
	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}

func (h *AuthHandler) getProfile(c *gin.Context) {
	userID := c.GetString("uid")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	profile, err := h.svc.GetProfile(c.Request.Context(), userID)
	if err != nil {
		if err == authService.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.log.Error("Get profile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

func (h *AuthHandler) updateProfile(c *gin.Context) {
	userID := c.GetString("uid")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.UpdateProfile(c.Request.Context(), userID, req.Name, req.Phone)
	if err != nil {
		if err == authService.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.log.Error("Update profile failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Profile updated successfully"})
}

func (h *AuthHandler) changePassword(c *gin.Context) {
	userID := c.GetString("uid")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var req authService.PasswordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.ChangePassword(c.Request.Context(), userID, req)
	if err != nil {
		if err == authService.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid current password"})
			return
		}
		if err == authService.ErrOAuthUser {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password change not allowed for OAuth users"})
			return
		}
		if err == authService.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.log.Error("Change password failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}

func (h *AuthHandler) requestPasswordChangeOTP(c *gin.Context) {
	var req authService.OTPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.RequestPasswordChangeOTP(c.Request.Context(), req)
	if err != nil {
		h.log.Error("Request password change OTP failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Always return success to prevent email enumeration
	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, an OTP has been sent"})
}

func (h *AuthHandler) verifyPasswordChangeOTP(c *gin.Context) {
	var req authService.OTPVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.svc.VerifyPasswordChangeOTP(c.Request.Context(), req)
	if err != nil {
		if err == authService.ErrInvalidOTP {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired OTP"})
			return
		}
		if err == authService.ErrOAuthUser {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password change not allowed for OAuth users"})
			return
		}
		if err == authService.ErrUserNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.log.Error("Verify password change OTP failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password changed successfully"})
}
