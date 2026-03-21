package http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

func RequireAuthMiddleware(jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if strings.TrimSpace(jwtSecret) == "" {
				return (*c).JSON(http.StatusInternalServerError, map[string]string{"error": "auth secret is not configured"})
			}

			authorization := (*c).Request().Header.Get("Authorization")
			tokenString, ok := strings.CutPrefix(authorization, "Bearer ")
			if !ok || strings.TrimSpace(tokenString) == "" {
				return (*c).JSON(http.StatusUnauthorized, map[string]string{"error": "missing or invalid Authorization header"})
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				return (*c).JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token"})
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				return (*c).JSON(http.StatusUnauthorized, map[string]string{"error": "invalid token claims"})
			}

			authUserID, ok := claims["userId"].(string)
			if !ok || strings.TrimSpace(authUserID) == "" {
				return (*c).JSON(http.StatusUnauthorized, map[string]string{"error": "userId claim is missing"})
			}

			requestedUserID := strings.TrimSpace((*c).QueryParam("userId"))
			if requestedUserID != "" && requestedUserID != authUserID {
				return (*c).JSON(http.StatusForbidden, map[string]string{"error": "forbidden for requested userId"})
			}

			(*c).Set("authUserId", authUserID)
			return next(c)
		}
	}
}
