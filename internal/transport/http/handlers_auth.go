package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/auth"
	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/google/uuid"
)

type authCredentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	User  domain.User `json:"user"`
	Token string      `json:"token"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req authCredentialsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	validationErrs := map[string]any{}
	if email == "" {
		validationErrs["email"] = "email is required"
	} else if _, err := mail.ParseAddress(email); err != nil || len(email) > 254 {
		validationErrs["email"] = "must be a valid email address"
	}
	if len(req.Password) < 8 {
		validationErrs["password"] = "password must be at least 8 characters"
	} else if len(req.Password) > 128 {
		validationErrs["password"] = "password must not exceed 128 characters"
	}
	if len(validationErrs) > 0 {
		RespondValidationError(w, r, validationErrs)
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("password hash failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to create account")
		return
	}

	now := time.Now().UTC()
	created, err := s.store.CreateUser(r.Context(), domain.User{
		ID:           uuid.NewString(),
		Email:        email,
		PasswordHash: hash,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			RespondError(w, r, http.StatusConflict, ErrCodeConflict, "An account with this email already exists")
			return
		}
		s.logger.Error("create user failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to create account")
		return
	}

	token, err := auth.IssueToken(created.ID, created.Email, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		s.logger.Error("issue token failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to issue session token")
		return
	}

	RespondJSON(w, http.StatusCreated, authResponse{
		User:  created,
		Token: token,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req authCredentialsRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" || req.Password == "" {
		RespondValidationError(w, r, map[string]any{
			"credentials": "email and password are required",
		})
		return
	}

	user, err := s.store.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Invalid email or password")
			return
		}
		s.logger.Error("get user by email failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Authentication failed")
		return
	}

	ok, err := auth.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil || !ok {
		RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Invalid email or password")
		return
	}

	token, err := auth.IssueToken(user.ID, user.Email, s.jwtSecret, s.jwtExpiry)
	if err != nil {
		s.logger.Error("issue token failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to issue session token")
		return
	}

	RespondJSON(w, http.StatusOK, authResponse{
		User:  user,
		Token: token,
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	user, err := s.store.GetUserByID(r.Context(), userID)
	if err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusUnauthorized, ErrCodeUnauthorized, "Account not found")
			return
		}
		s.logger.Error("get current user failed", "error", err, "request_id", RequestIDFromContext(r.Context()))
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load user profile")
		return
	}

	RespondJSON(w, http.StatusOK, user)
}

func decodeJSONBody(r *http.Request, dst any) error {
	if r.Body == nil {
		return errors.New("request body is required")
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return errors.New("request payload exceeds 2 MiB size limit")
		}
		return errors.New("invalid JSON request body")
	}
	return nil
}
