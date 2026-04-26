package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"diploma/pkg/token"
)

const (
	ContextUserID   = "user_id"
	ContextUserRole = "user_role"
)

// JWTAuth — middleware, который снимает Bearer-токен из заголовка Authorization,
// валидирует его и кладёт user_id и role в gin-контекст. Дальше обработчики
// читают эти значения через c.GetInt64 / c.GetString.
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization header"})
			return
		}

		claims, err := token.Parse(parts[1], secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		c.Set(ContextUserID, claims.UserID)
		c.Set(ContextUserRole, claims.Role)
		c.Next()
	}
}
