package auth

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// RequireAuth validates the session cookie and stores user_id/role in context.
func RequireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(cookieName)
			if err != nil || cookie.Value == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "no autenticado"})
			}

			claims, err := ParseToken(cookie.Value)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "sesión inválida o expirada"})
			}

			c.Set("user_id", claims.UserID)
			c.Set("role", claims.Role)
			c.Set("email", claims.Email)
			return next(c)
		}
	}
}

// RequireRole rejects requests whose role claim is not in the allowed list.
func RequireRole(roles ...string) echo.MiddlewareFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role, _ := c.Get("role").(string)
			if !allowed[role] {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "sin permisos"})
			}
			return next(c)
		}
	}
}
