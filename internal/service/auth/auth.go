package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"

	jwtMiddleware "github.com/samirwankhede/lewly-pgpyewj/internal/middleware"
	redisx "github.com/samirwankhede/lewly-pgpyewj/internal/redis"
	"github.com/samirwankhede/lewly-pgpyewj/internal/service/mailer"
	"github.com/samirwankhede/lewly-pgpyewj/internal/store/users"
)

type AuthService struct {
	log    *zap.Logger
	users  *users.UsersRepository
	redis  *redisx.TokenBucket
	secret string
	mailer *mailer.MailerService
}

type SignupRequest struct {
	Name     string `json:"name" binding:"required"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	Phone    string `json:"phone"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	Token   string    `json:"token"`
	User    UserInfo  `json:"user"`
	Expires time.Time `json:"expires"`
}

type UserInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone"`
	Role  string `json:"role"`
}

type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

type OTPRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type OTPVerifyRequest struct {
	Email       string `json:"email" binding:"required,email"`
	OTP         string `json:"otp" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidOTP         = errors.New("invalid or expired OTP")
	ErrOAuthUser          = errors.New("password change not allowed for OAuth users")
)

func NewAuthService(log *zap.Logger, users *users.UsersRepository, redis *redisx.TokenBucket, secret string, mailer *mailer.MailerService) *AuthService {
	return &AuthService{
		log:    log,
		users:  users,
		redis:  redis,
		secret: secret,
		mailer: mailer,
	}
}

func (s *AuthService) Signup(ctx context.Context, req SignupRequest) (*LoginResponse, error) {
	// Check if user already exists
	existing, err := s.users.GetByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, ErrUserExists
	}

	// s.log.Info("Signup check", zap.String("email", req.Email), zap.Any("existing", existing), zap.Error(err))

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &users.User{
		Name:         req.Name,
		Email:        req.Email,
		Phone:        req.Phone,
		PasswordHash: string(hashedPassword),
		Role:         "user",
	}

	user, err = s.users.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate token
	token, expires, err := s.generateToken(user.ID, user.Role == "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		Token:   token,
		User:    s.userToInfo(user),
		Expires: expires,
	}, nil
}

func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	// Check if user has password (not OAuth user)
	if user.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	// Generate token
	token, expires, err := s.generateToken(user.ID, user.Role == "admin")
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	return &LoginResponse{
		Token:   token,
		User:    s.userToInfo(user),
		Expires: expires,
	}, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID string, req PasswordChangeRequest) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	// Check if user has password (not OAuth user)
	if user.PasswordHash == "" {
		return ErrOAuthUser
	}

	// Verify current password
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword))
	if err != nil {
		return ErrInvalidCredentials
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	return s.users.UpdatePassword(ctx, userID, string(hashedPassword))
}

func (s *AuthService) RequestPasswordChangeOTP(ctx context.Context, req OTPRequest) error {
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		// Don't reveal if user exists or not
		return nil
	}

	// Check if user has password (not OAuth user)
	if user.PasswordHash == "" {
		// Don't reveal if user exists or not
		return nil
	}

	// Generate OTP
	otp := s.generateOTP()

	// Store OTP in Redis with 15 minute TTL
	key := fmt.Sprintf("password_change_otp:%s", req.Email)
	err = s.redis.GetClient().Set(ctx, key, otp, 15*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to store OTP: %w", err)
	}

	// Send OTP via email
	err = s.mailer.SendPasswordChangeOTPEmail(req.Email, otp)
	if err != nil {
		s.log.Error("Failed to send OTP email", zap.Error(err))
		// Don't return error to prevent email enumeration
	}

	return nil
}

func (s *AuthService) VerifyPasswordChangeOTP(ctx context.Context, req OTPVerifyRequest) error {
	// Verify OTP
	key := fmt.Sprintf("password_change_otp:%s", req.Email)
	storedOTP, err := s.redis.GetClient().Get(ctx, key).Result()
	if err != nil {
		return ErrInvalidOTP
	}

	if storedOTP != req.OTP {
		return ErrInvalidOTP
	}

	// Get user
	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		return ErrUserNotFound
	}

	// Check if user has password (not OAuth user)
	if user.PasswordHash == "" {
		return ErrOAuthUser
	}

	// Hash new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password
	err = s.users.UpdatePassword(ctx, user.ID, string(hashedPassword))
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Delete OTP
	s.redis.GetClient().Del(ctx, key)

	return nil
}

func (s *AuthService) GetProfile(ctx context.Context, userID string) (*UserInfo, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	if user == nil {
		return nil, ErrUserNotFound
	}

	return &UserInfo{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Phone: user.Phone,
		Role:  user.Role,
	}, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, userID string, name, phone string) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return ErrUserNotFound
	}

	if user == nil {
		return ErrUserNotFound
	}
	return s.users.UpdateProfile(ctx, userID, name, phone)
}

func (s *AuthService) generateToken(userID string, isAdmin bool) (string, time.Time, error) {
	expires := time.Now().Add(24 * time.Hour)
	token, err := jwtMiddleware.Issue(s.secret, userID, isAdmin, 24*time.Hour)
	if err != nil {
		return "", time.Time{}, err
	}
	return token, expires, nil
}

func (s *AuthService) generateOTP() string {
	bytes := make([]byte, 3)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (s *AuthService) userToInfo(user *users.User) UserInfo {
	return UserInfo{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Phone: user.Phone,
		Role:  user.Role,
	}
}
