package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RequireRole возвращает middleware, которая отдаёт 403, если роль текущего
// пользователя не входит в список разрешённых. Подключается после JWTAuth.
func RequireRole(allowed ...string) gin.HandlerFunc {
	set := make(map[string]struct{}, len(allowed))
	for _, r := range allowed {
		set[r] = struct{}{}
	}
	return func(c *gin.Context) {
		role := c.GetString(ContextUserRole)
		if _, ok := set[role]; !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden: insufficient role"})
			return
		}
		c.Next()
	}
}
