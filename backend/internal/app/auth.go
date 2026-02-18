package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"github.com/willianpsouza/StressTestPlatform/internal/domain"
	"github.com/willianpsouza/StressTestPlatform/internal/pkg/config"
)

type AuthService struct {
	jwtConfig   config.JWTConfig
	userRepo    domain.UserRepository
	sessionRepo domain.SessionRepository
}

func NewAuthService(
	jwtConfig config.JWTConfig,
	userRepo domain.UserRepository,
	sessionRepo domain.SessionRepository,
) *AuthService {
	return &AuthService{
		jwtConfig:   jwtConfig,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
	}
}

func (s *AuthService) Register(input domain.RegisterInput) (*domain.LoginResponse, error) {
	if input.Email == "" || input.Password == "" || input.Name == "" {
		return nil, domain.NewValidationError(map[string]string{
			"email":    "Email is required",
			"password": "Password is required",
			"name":     "Name is required",
		})
	}

	if len(input.Password) < 8 {
		return nil, domain.NewValidationError(map[string]string{
			"password": "Password must be at least 8 characters",
		})
	}

	if input.Password != input.ConfirmPassword {
		return nil, domain.NewValidationError(map[string]string{
			"confirm_password": "Passwords do not match",
		})
	}

	existing, _ := s.userRepo.GetByEmail(input.Email)
	if existing != nil {
		return nil, domain.NewConflictError("Email already registered")
	}

	passwordHash, err := HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		Email:        input.Email,
		PasswordHash: passwordHash,
		Name:         input.Name,
		Role:         domain.UserRoleUser,
		Status:       domain.UserStatusActive,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return s.generateLoginResponse(user, "", "")
}

func (s *AuthService) Login(input domain.LoginInput, ip, userAgent string) (*domain.LoginResponse, error) {
	user, err := s.userRepo.GetByEmail(input.Email)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.NewUnauthorizedError("Invalid credentials")
		}
		return nil, err
	}

	if !VerifyPassword(input.Password, user.PasswordHash) {
		return nil, domain.NewUnauthorizedError("Invalid credentials")
	}

	if user.Status != domain.UserStatusActive {
		return nil, domain.NewUnauthorizedError("Account is not active")
	}

	now := time.Now()
	user.LastLoginAt = &now
	_ = s.userRepo.Update(user)

	return s.generateLoginResponse(user, ip, userAgent)
}

func (s *AuthService) Logout(token string) error {
	if strings.TrimSpace(token) == "" {
		return nil
	}
	hash := hashToken(token)
	session, err := s.sessionRepo.GetByTokenHash(hash)
	if err != nil {
		return nil
	}
	return s.sessionRepo.Revoke(session.ID)
}

func (s *AuthService) RefreshToken(refreshToken string) (*domain.LoginResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, domain.NewUnauthorizedError("Invalid refresh token")
	}

	hash := hashToken(refreshToken)
	session, err := s.sessionRepo.GetByTokenHash(hash)
	if err != nil || !session.IsValid() {
		return nil, domain.NewUnauthorizedError("Invalid refresh token")
	}

	user, err := s.userRepo.GetByID(session.UserID)
	if err != nil {
		return nil, domain.NewUnauthorizedError("Invalid refresh token")
	}

	if user.Status != domain.UserStatusActive {
		return nil, domain.NewUnauthorizedError("Account is not active")
	}

	if err := s.sessionRepo.Revoke(session.ID); err != nil {
		return nil, err
	}

	ip := ""
	if session.IPAddress != nil {
		ip = *session.IPAddress
	}
	userAgent := ""
	if session.UserAgent != nil {
		userAgent = *session.UserAgent
	}

	return s.generateLoginResponse(user, ip, userAgent)
}

func (s *AuthService) ValidateToken(tokenString string) (*domain.TokenClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, domain.ErrTokenInvalid
		}
		return []byte(s.jwtConfig.Secret), nil
	})
	if err != nil {
		return nil, domain.ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return nil, domain.ErrTokenInvalid
	}

	exp, ok := claims["exp"].(float64)
	if !ok || time.Unix(int64(exp), 0).Before(time.Now()) {
		return nil, domain.ErrTokenExpired
	}

	userID, err := getUUIDClaim(claims, "user_id")
	if err != nil {
		return nil, err
	}

	email, err := getStringClaim(claims, "email")
	if err != nil {
		return nil, err
	}

	role, err := getStringClaim(claims, "role")
	if err != nil {
		return nil, err
	}

	return &domain.TokenClaims{
		UserID: userID,
		Email:  email,
		Role:   domain.UserRole(role),
	}, nil
}

func (s *AuthService) GetCurrentUser(userID uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(userID)
}

func (s *AuthService) UpdateProfile(userID uuid.UUID, input domain.UpdateProfileInput) (*domain.User, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		user.Name = *input.Name
	}
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) ChangePassword(userID uuid.UUID, input domain.ChangePasswordInput) error {
	if input.NewPassword != input.ConfirmPassword {
		return domain.NewValidationError(map[string]string{
			"confirm_password": "Passwords do not match",
		})
	}
	if len(input.NewPassword) < 8 {
		return domain.NewValidationError(map[string]string{
			"new_password": "Password must be at least 8 characters",
		})
	}

	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}

	if !VerifyPassword(input.CurrentPassword, user.PasswordHash) {
		return domain.NewValidationError(map[string]string{
			"current_password": "Current password is incorrect",
		})
	}

	passwordHash, err := HashPassword(input.NewPassword)
	if err != nil {
		return err
	}

	user.PasswordHash = passwordHash
	return s.userRepo.Update(user)
}

// Admin user management
func (s *AuthService) ListUsers(filter domain.UserFilter) ([]domain.User, int64, error) {
	return s.userRepo.List(filter)
}

func (s *AuthService) GetUser(id uuid.UUID) (*domain.User, error) {
	return s.userRepo.GetByID(id)
}

func (s *AuthService) UpdateUser(id uuid.UUID, input domain.UpdateUserInput) (*domain.User, error) {
	user, err := s.userRepo.GetByID(id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		user.Name = *input.Name
	}
	if input.Role != nil {
		user.Role = *input.Role
	}
	if input.Status != nil {
		user.Status = *input.Status
	}
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) DeleteUser(id uuid.UUID) error {
	return s.userRepo.Delete(id)
}

// Internal helpers

func (s *AuthService) generateLoginResponse(user *domain.User, ip, userAgent string) (*domain.LoginResponse, error) {
	expiresAt := time.Now().Add(s.jwtConfig.AccessTokenDuration)
	accessToken, err := s.generateAccessToken(user, expiresAt)
	if err != nil {
		return nil, err
	}

	refreshToken, err := generateRandomToken()
	if err != nil {
		return nil, err
	}

	session := &domain.Session{
		UserID:    user.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().Add(s.jwtConfig.RefreshTokenDuration),
	}
	if userAgent != "" {
		session.UserAgent = &userAgent
	}
	if ip != "" {
		session.IPAddress = &ip
	}

	if err := s.sessionRepo.Create(session); err != nil {
		return nil, err
	}

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
		User:         *user,
	}, nil
}

func (s *AuthService) generateAccessToken(user *domain.User, expiresAt time.Time) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"role":    string(user.Role),
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	})
	return token.SignedString([]byte(s.jwtConfig.Secret))
}

func HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	b64Salt := base64.StdEncoding.EncodeToString(salt)
	b64Hash := base64.StdEncoding.EncodeToString(hash)
	return b64Salt + "$" + b64Hash, nil
}

func VerifyPassword(password, hash string) bool {
	parts := strings.SplitN(hash, "$", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return false
	}
	salt, err := base64.StdEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	storedHash, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	computedHash := argon2.IDKey([]byte(password), salt, 3, 64*1024, 2, 32)
	return subtle.ConstantTimeCompare(computedHash, storedHash) == 1
}

func generateRandomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func getStringClaim(claims jwt.MapClaims, key string) (string, error) {
	raw, ok := claims[key]
	if !ok {
		return "", domain.ErrTokenInvalid
	}
	value, ok := raw.(string)
	if !ok || value == "" {
		return "", domain.ErrTokenInvalid
	}
	return value, nil
}

func getUUIDClaim(claims jwt.MapClaims, key string) (uuid.UUID, error) {
	value, err := getStringClaim(claims, key)
	if err != nil {
		return uuid.Nil, err
	}
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil, domain.ErrTokenInvalid
	}
	return parsed, nil
}
