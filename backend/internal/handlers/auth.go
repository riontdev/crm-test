package handlers

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/riont/crm/backend/internal/auth"
)

const sessionCookieName = "crm_session"

func containsNotFound(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "no rows") || strings.Contains(msg, "no encontrado")
}

type AuthHandler struct {
	users *auth.UserService
}

func NewAuthHandler(users *auth.UserService) *AuthHandler {
	return &AuthHandler{users: users}
}

func (h *AuthHandler) ensureService(c echo.Context) bool {
	if h.users == nil {
		c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "database not connected"})
		return false
	}
	return true
}

// Login authenticates a user and sets the session cookie.
// POST /api/auth/login
func (h *AuthHandler) Login(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil || req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "email y contraseña son obligatorios"})
	}

	user, err := h.users.Authenticate(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "email o contraseña incorrectos"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error de autenticación"})
	}

	token, err := auth.GenerateToken(*user)
	if err != nil {
		if errors.Is(err, auth.ErrSecretNotConfigured) {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "login deshabilitado: AUTH_JWT_SECRET no configurada"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "error generando sesión"})
	}

	secure := os.Getenv("RAILWAY_ENVIRONMENT") != ""
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(auth.SessionDuration.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	})

	expiresAt := time.Now().Add(auth.SessionDuration).UTC().Format(time.RFC3339)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"user":               user,
		"session_expires_at": expiresAt,
	})
}

// Me returns the authenticated user.
// GET /api/auth/me
func (h *AuthHandler) Me(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}

	userID, _ := c.Get("user_id").(string)
	user, err := h.users.GetByID(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "usuario no encontrado"})
	}

	response := map[string]interface{}{"user": user}
	if claims, ok := c.Get("claims").(*auth.Claims); ok && claims.ExpiresAt != nil {
		response["session_expires_at"] = claims.ExpiresAt.Time.UTC().Format(time.RFC3339)
	}
	return c.JSON(http.StatusOK, response)
}

// Logout clears the session cookie.
// POST /api/auth/logout
func (h *AuthHandler) Logout(c echo.Context) error {
	c.SetCookie(&http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   os.Getenv("RAILWAY_ENVIRONMENT") != "",
	})
	return c.JSON(http.StatusOK, map[string]interface{}{"ok": true})
}

// ListUsers returns all users. Admin only.
// GET /api/users
func (h *AuthHandler) ListUsers(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}
	users, err := h.users.List(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "no se pudieron listar los usuarios"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"data": users, "count": len(users)})
}

// CreateUser creates a new user. Admin only.
// POST /api/users
func (h *AuthHandler) CreateUser(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}

	var req struct {
		Email    string `json:"email"`
		Name     string `json:"name"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}
	if req.Role == "" {
		req.Role = "agente"
	}

	user, err := h.users.Create(c.Request().Context(), req.Email, req.Name, req.Password, req.Role)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusCreated, map[string]interface{}{"user": user})
}

// UpdateUser modifies name/role/password. Admin only.
// PUT /api/users/:id
func (h *AuthHandler) UpdateUser(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}

	var req struct {
		Name     *string `json:"name"`
		Role     *string `json:"role"`
		Password *string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "cuerpo inválido"})
	}

	user, err := h.users.Update(c.Request().Context(), c.Param("id"), req.Name, req.Role, req.Password)
	if err != nil {
		if err.Error() == "id inválido" || containsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "usuario no encontrado"})
		}
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"user": user})
}

// DeleteUser removes a user. Admin only.
// DELETE /api/users/:id
func (h *AuthHandler) DeleteUser(c echo.Context) error {
	if !h.ensureService(c) {
		return nil
	}

	selfID, _ := c.Get("user_id").(string)
	id := c.Param("id")
	if id == selfID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "no podés eliminar tu propio usuario"})
	}

	if err := h.users.Delete(c.Request().Context(), id); err != nil {
		if containsNotFound(err) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "usuario no encontrado"})
		}
		if err.Error() == "no se puede eliminar el último administrador" {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "no se pudo eliminar el usuario"})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"ok": true})
}
