package handler

import (
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"diploma/internal/middleware"
	"diploma/internal/models"
	"diploma/internal/preview"
	"diploma/internal/service"
	"diploma/pkg/token"
)

// NewRouter подключает все хендлеры к gin-engine. jwtSecret нужен и тут (для
// инициализации middleware), и внутри VersionHandler.ServeFile, чтобы
// валидировать токен из query-параметра.
func NewRouter(
	jwtSecret string,
	authSvc service.AuthService,
	docSvc service.DocumentService,
	verSvc service.VersionService,
	revSvc service.ReviewService,
	disSvc service.DiscussionService,
	aiSvc service.AICheckService,
	libSvc service.LibraryService,
	admSvc service.AdminService,
	userListSvc service.UserListService,
	ruleSetSvc service.RuleSetService,
	complianceSvc service.ComplianceService,
) *gin.Engine {
	r := gin.Default()

	// Observability: трейсинг каждого запроса через OpenTelemetry и
	// сбор Prometheus-метрик. Подключаем рано в цепочке, чтобы поймать
	// все запросы независимо от auth-результата.
	r.Use(otelgin.Middleware("diploma-api"))
	r.Use(middleware.Metrics())

	// CORS открыт настежь — фронт-дев запускается с другого порта (vite на
	// 5173, api на 8080). В проде, где они под одним доменом за реверс-прокси,
	// это можно убрать.
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	r.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// /instance — отдаёт идентификацию текущего инстанса. Удобно при запуске
	// нескольких реплик (docker-compose с двумя api или k8s deployment с
	// replicas > 1) — каждый ответит своими значениями. INSTANCE_ID берётся
	// из env, hostname = имя контейнера / pod-а.
	r.GET("/instance", func(c *gin.Context) {
		hostname, _ := os.Hostname()
		c.JSON(http.StatusOK, gin.H{
			"instance_id": os.Getenv("INSTANCE_ID"),
			"hostname":    hostname,
			"port":        os.Getenv("APP_PORT"),
			"db_host":     os.Getenv("DB_HOST"),
			"time":        time.Now().Format(time.RFC3339),
		})
	})

	api := r.Group("/api/v1")

	// Публичные роуты — без JWT.
	auth := &AuthHandler{svc: authSvc}
	api.POST("/auth/login", auth.Login)

	// Всё остальное — за JWT.
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtSecret))

	protected.GET("/auth/me", auth.Me)
	protected.POST("/auth/change-password", auth.ChangePassword)

	ul := &UserListHandler{svc: userListSvc}
	protected.GET("/users", ul.ListByRole)

	docs := &DocumentHandler{svc: docSvc}
	protected.POST("/documents", docs.Create)
	protected.GET("/documents", docs.List)
	protected.GET("/documents/:id", docs.Get)
	protected.PUT("/documents/:id", docs.Update)
	protected.DELETE("/documents/:id", docs.Delete)
	protected.POST("/documents/:id/publish", docs.Publish)
	protected.POST("/documents/:id/unpublish", docs.Unpublish)
	protected.POST("/documents/:id/archive", docs.Archive)

	ver := &VersionHandler{svc: verSvc, jwtSecret: jwtSecret}
	protected.POST("/documents/:id/versions", ver.Upload)
	protected.GET("/documents/:id/versions", ver.List)
	protected.GET("/documents/:id/versions/:vid", ver.Get)
	protected.POST("/documents/:id/versions/:vid/restore", ver.Restore)
	// Раздача файлов вынесена из protected: iframe не умеет ставить
	// Authorization-заголовок, поэтому токен берём из ?token=.
	api.GET("/documents/:id/versions/:vid/file", ver.ServeFile)
	protected.GET("/documents/:id/diff", ver.Diff)

	rev := &ReviewHandler{svc: revSvc}
	protected.POST("/documents/:id/submit", rev.Submit)
	protected.POST("/documents/:id/approve", rev.Approve)
	protected.POST("/documents/:id/revision", rev.RequestRevision)
	protected.POST("/documents/:id/versions/:vid/comments", rev.AddComment)
	protected.GET("/documents/:id/versions/:vid/comments", rev.ListComments)
	protected.GET("/documents/:id/review-actions", rev.ListActions)

	dis := &DiscussionHandler{svc: disSvc}
	protected.GET("/documents/:id/discussion-view", dis.DiscussionView)
	protected.POST("/documents/:id/versions/:vid/threads", dis.CreateThread)
	protected.GET("/documents/:id/versions/:vid/threads", dis.ListThreads)
	protected.POST("/threads/:tid/messages", dis.Reply)
	protected.GET("/threads/:tid/messages", dis.ListMessages)
	protected.POST("/threads/:tid/resolve", dis.Resolve)

	// AI-проверка — заглушка, оба роута пока возвращают 501.
	ai := &AICheckHandler{svc: aiSvc}
	protected.POST("/documents/:id/versions/:vid/checks", ai.RunCheck)
	protected.GET("/documents/:id/versions/:vid/checks", ai.ListChecks)

	lib := &LibraryHandler{svc: libSvc}
	protected.GET("/library", lib.List)
	protected.GET("/library/:id", lib.GetPublished)
	// Публичные треды по опубликованным документам — открыты любому
	// залогиненному пользователю, в том числе developer'у.
	libDis := &LibraryDiscussionHandler{svc: disSvc}
	protected.GET("/library/:id/threads", libDis.ListThreads)
	protected.POST("/library/:id/threads", libDis.CreateThread)

	// Админский раздел закрыт RequireRole(admin).
	adm := &AdminHandler{svc: admSvc}
	adminGroup := protected.Group("/admin")
	adminGroup.Use(middleware.RequireRole(string(models.RoleAdmin)))
	adminGroup.GET("/users", adm.ListUsers)
	adminGroup.POST("/users", adm.CreateUser)
	adminGroup.PUT("/users/:id", adm.UpdateUser)
	adminGroup.PUT("/users/:id/active", adm.SetActive)
	adminGroup.POST("/users/:id/reset-password", adm.ResetPassword)

	rsh := &RuleSetHandler{svc: ruleSetSvc}
	adminGroup.GET("/rule-sets", rsh.List)
	adminGroup.POST("/rule-sets", rsh.Create)
	adminGroup.GET("/rule-sets/:id", rsh.Get)
	adminGroup.PUT("/rule-sets/:id", rsh.Update)
	adminGroup.DELETE("/rule-sets/:id", rsh.Delete)
	adminGroup.POST("/rule-sets/:id/rules", rsh.CreateRule)
	adminGroup.PUT("/rule-sets/:id/rules/:rid", rsh.UpdateRule)
	adminGroup.DELETE("/rule-sets/:id/rules/:rid", rsh.DeleteRule)

	// Прогон проверки доступен всем, у кого есть доступ к документу
	// (writer-владелец, ревьюер или admin). Список активных наборов отдаём
	// всем — пользователю надо из чего-то выбрать; права проверяются дальше
	// внутри ComplianceService.
	protected.GET("/rule-sets/active", rsh.ListActive)
	cmp := &ComplianceHandler{svc: complianceSvc}
	protected.POST("/documents/:id/versions/:vid/compliance", cmp.Run)
	protected.GET("/documents/:id/versions/:vid/compliance", cmp.List)

	return r
}

type AuthHandler struct{ svc service.AuthService }

func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	accessToken, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, models.TokenResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserID)

	user, err := h.svc.Me(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ChangePassword — один эндпоинт и для форсированной первой смены, и для
// добровольной. Различает их authService по полю must_change_password.
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req models.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	if err := h.svc.ChangePassword(c.Request.Context(), userID, req); err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{"error": "current password is incorrect"})
		case errors.Is(err, service.ErrSamePassword):
			c.JSON(http.StatusBadRequest, gin.H{"error": "new password must differ from current"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

type UserListHandler struct{ svc service.UserListService }

func (h *UserListHandler) ListByRole(c *gin.Context) {
	roleStr := c.Query("role")
	if roleStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "role query parameter is required"})
		return
	}
	role := models.UserRole(roleStr)

	users, err := h.svc.ListByRole(c.Request.Context(), role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, users)
}

type DocumentHandler struct{ svc service.DocumentService }

func (h *DocumentHandler) Create(c *gin.Context) {
	var req models.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)
	doc, err := h.svc.Create(c.Request.Context(), req, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusCreated, doc)
}

func (h *DocumentHandler) List(c *gin.Context) {
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	var q models.DocumentQuery
	if v := c.Query("project_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project_id"})
			return
		}
		q.ProjectID = &id
	}
	if v := c.Query("category_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid category_id"})
			return
		}
		q.CategoryID = &id
	}
	if v := c.Query("status"); v != "" {
		s := models.DocumentStatus(v)
		q.Status = &s
	}

	docs, err := h.svc.List(c.Request.Context(), userID, role, q)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (h *DocumentHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.GetByID(c.Request.Context(), id, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *DocumentHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.Update(c.Request.Context(), id, req, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *DocumentHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	if err := h.svc.Delete(c.Request.Context(), id, userID, role); err != nil {
		handleDocError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *DocumentHandler) Publish(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.Publish(c.Request.Context(), id, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *DocumentHandler) Unpublish(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.Unpublish(c.Request.Context(), id, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

// Archive — POST /documents/:id/archive. Переводит документ в статус
// archived. Доступно admin'у или автору, опубликованный документ нужно
// сначала снять с публикации.
func (h *DocumentHandler) Archive(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.Archive(c.Request.Context(), id, userID, role)
	if err != nil {
		handleDocError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func handleDocError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrNotDraft):
		c.JSON(http.StatusConflict, gin.H{"error": "document must be in draft status"})
	case errors.Is(err, service.ErrNotPublished):
		c.JSON(http.StatusConflict, gin.H{"error": "document is not published"})
	case errors.Is(err, service.ErrTitleTaken):
		c.JSON(http.StatusConflict, gin.H{"error": "Документ с таким названием уже существует"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

type VersionHandler struct {
	svc       service.VersionService
	jwtSecret string
}

func (h *VersionHandler) Upload(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	var req models.UploadVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	v, err := h.svc.Upload(c.Request.Context(), docID, req, userID, role)
	if err != nil {
		handleVersionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, v)
}

func (h *VersionHandler) List(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	versions, err := h.svc.List(c.Request.Context(), docID, userID, role)
	if err != nil {
		handleVersionError(c, err)
		return
	}
	c.JSON(http.StatusOK, versions)
}

func (h *VersionHandler) Get(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	v, err := h.svc.GetByID(c.Request.Context(), docID, vid, userID, role)
	if err != nil {
		handleVersionError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

func (h *VersionHandler) Restore(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	v, err := h.svc.Restore(c.Request.Context(), docID, vid, userID, role)
	if err != nil {
		handleVersionError(c, err)
		return
	}
	c.JSON(http.StatusOK, v)
}

// ServeFile — раздача исходного файла версии. Токен принимается через
// query-параметр (?token=...), потому что iframe не умеет ставить заголовок
// Authorization. Дополнительно при ?format=pdf отдаёт сгенерированный
// preview.pdf — нужно для inline-просмотра docx без скачивания.
func (h *VersionHandler) ServeFile(c *gin.Context) {
	tokenStr := c.Query("token")
	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token required"})
		return
	}
	claims, err := token.Parse(tokenStr, h.jwtSecret)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}

	v, err := h.svc.GetByID(c.Request.Context(), docID, vid, claims.UserID, claims.Role)
	if err != nil {
		handleVersionError(c, err)
		return
	}

	if c.Query("format") == "pdf" {
		previewPath := preview.Path(v.FilePath)
		if !preview.Exists(v.FilePath) {
			c.JSON(http.StatusNotFound, gin.H{"error": "preview not available"})
			return
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", "inline")
		c.File(previewPath)
		return
	}
	c.File(v.FilePath)
}

func (h *VersionHandler) Diff(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	v1, err := strconv.ParseInt(c.Query("v1"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "v1 query parameter is required"})
		return
	}
	v2, err := strconv.ParseInt(c.Query("v2"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "v2 query parameter is required"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	resp, err := h.svc.Diff(c.Request.Context(), docID, v1, v2, userID, role)
	if err != nil {
		handleVersionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func handleVersionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, service.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrInvalidFileType):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid file type; allowed: docx, txt, md, yaml"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

type ReviewHandler struct{ svc service.ReviewService }

func (h *ReviewHandler) Submit(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.Submit(c.Request.Context(), docID, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, doc)
}

func (h *ReviewHandler) Approve(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	action, err := h.svc.Approve(c.Request.Context(), docID, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, action)
}

func (h *ReviewHandler) RequestRevision(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.RequestRevisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	action, err := h.svc.RequestRevision(c.Request.Context(), docID, req.Note, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, action)
}

func (h *ReviewHandler) AddComment(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	var req models.AddCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	comment, err := h.svc.AddComment(c.Request.Context(), docID, vid, req.CommentText, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusCreated, comment)
}

func (h *ReviewHandler) ListComments(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	comments, err := h.svc.ListComments(c.Request.Context(), docID, vid, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, comments)
}

func (h *ReviewHandler) ListActions(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	actions, err := h.svc.ListActions(c.Request.Context(), docID, userID, role)
	if err != nil {
		handleReviewError(c, err)
		return
	}
	c.JSON(http.StatusOK, actions)
}

func handleReviewError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, service.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrWrongStatus):
		c.JSON(http.StatusConflict, gin.H{"error": "operation not allowed in current document status"})
	case errors.Is(err, service.ErrNoReviewer):
		c.JSON(http.StatusConflict, gin.H{"error": "document has no reviewer assigned"})
	case errors.Is(err, service.ErrNoCurrentVersion):
		c.JSON(http.StatusConflict, gin.H{"error": "document has no current version"})
	case errors.Is(err, service.ErrNoteRequired):
		c.JSON(http.StatusBadRequest, gin.H{"error": "note is required"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

type DiscussionHandler struct{ svc service.DiscussionService }

func (h *DiscussionHandler) DiscussionView(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	resp, err := h.svc.GetDiscussionView(c.Request.Context(), docID, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *DiscussionHandler) CreateThread(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	var req models.CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	thread, err := h.svc.CreateThread(c.Request.Context(), docID, vid, req, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, thread)
}

func (h *DiscussionHandler) ListThreads(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	threads, err := h.svc.ListThreads(c.Request.Context(), docID, vid, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusOK, threads)
}

func (h *DiscussionHandler) Reply(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}
	var req models.CreateMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	msg, err := h.svc.CreateMessage(c.Request.Context(), tid, req.Message, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func (h *DiscussionHandler) ListMessages(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	msgs, err := h.svc.ListMessages(c.Request.Context(), tid, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusOK, msgs)
}

func (h *DiscussionHandler) Resolve(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	thread, err := h.svc.ResolveThread(c.Request.Context(), tid, userID, role)
	if err != nil {
		handleDiscussionError(c, err)
		return
	}
	c.JSON(http.StatusOK, thread)
}

func handleDiscussionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, service.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
	case errors.Is(err, service.ErrThreadNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "thread not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrThreadResolved):
		c.JSON(http.StatusConflict, gin.H{"error": "thread is resolved"})
	case errors.Is(err, service.ErrInvalidThreadType):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread type; allowed: general, anchored"})
	case errors.Is(err, service.ErrEmptyMessage):
		c.JSON(http.StatusBadRequest, gin.H{"error": "message must not be empty"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// AICheckHandler — заглушка: оба эндпоинта возвращают 501. Будет наполнено,
// когда подключим LLM (YandexGPT).
type AICheckHandler struct{ svc service.AICheckService }

func (h *AICheckHandler) RunCheck(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "not implemented"})
}
func (h *AICheckHandler) ListChecks(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"message": "not implemented"})
}

type LibraryHandler struct{ svc service.LibraryService }

func (h *LibraryHandler) List(c *gin.Context) {
	docs, err := h.svc.List(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, docs)
}

func (h *LibraryHandler) GetPublished(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)

	doc, err := h.svc.GetByID(c.Request.Context(), id, userID, role)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, doc)
}

// LibraryDiscussionHandler — публичные треды на /library/:id, открыты любому
// залогиненному пользователю (writer / reviewer / developer / admin).
type LibraryDiscussionHandler struct{ svc service.DiscussionService }

func (h *LibraryDiscussionHandler) ListThreads(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	threads, err := h.svc.ListLibraryThreads(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		case errors.Is(err, service.ErrDocumentNotPublished):
			c.JSON(http.StatusConflict, gin.H{"error": "document is not published"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, threads)
}

func (h *LibraryDiscussionHandler) CreateThread(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	t, err := h.svc.CreateLibraryThread(c.Request.Context(), id, req, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDocumentNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
		case errors.Is(err, service.ErrDocumentNotPublished):
			c.JSON(http.StatusConflict, gin.H{"error": "document is not published"})
		case errors.Is(err, service.ErrInvalidThreadType):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, t)
}

type AdminHandler struct{ svc service.AdminService }

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.svc.ListUsers(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, users)
}

func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req models.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.CreateUser(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, user)
}

func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.UpdateUserAdminRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := h.svc.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		case errors.Is(err, service.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, user)
}

// SetActive — деактивация пользователя. Себя самого деактивировать нельзя,
// иначе можно случайно вылететь из единственного админского аккаунта.
func (h *AdminHandler) SetActive(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.SetActiveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if id == c.GetInt64(middleware.ContextUserID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot deactivate yourself"})
		return
	}
	if err := h.svc.SetUserActive(c.Request.Context(), id, req.IsActive); err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AdminHandler) ResetPassword(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.ResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.svc.ResetPassword(c.Request.Context(), id, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.Status(http.StatusNoContent)
}

// RuleSetHandler — админский CRUD по наборам правил compliance.
type RuleSetHandler struct{ svc service.RuleSetService }

func (h *RuleSetHandler) List(c *gin.Context) {
	sets, err := h.svc.ListSets(c.Request.Context(), false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, sets)
}

// ListActive — для не-админских ролей: только активные наборы, чтобы было
// из чего выбирать в селекторе при запуске проверки.
func (h *RuleSetHandler) ListActive(c *gin.Context) {
	sets, err := h.svc.ListSets(c.Request.Context(), true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, sets)
}

func (h *RuleSetHandler) Get(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	set, err := h.svc.GetSet(c.Request.Context(), id)
	if err != nil {
		ruleSetError(c, err)
		return
	}
	c.JSON(http.StatusOK, set)
}

func (h *RuleSetHandler) Create(c *gin.Context) {
	var req models.CreateRuleSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	set, err := h.svc.CreateSet(c.Request.Context(), req, userID)
	if err != nil {
		ruleSetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, set)
}

func (h *RuleSetHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.UpdateRuleSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	set, err := h.svc.UpdateSet(c.Request.Context(), id, req)
	if err != nil {
		ruleSetError(c, err)
		return
	}
	c.JSON(http.StatusOK, set)
}

func (h *RuleSetHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.svc.DeleteSet(c.Request.Context(), id); err != nil {
		ruleSetError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *RuleSetHandler) CreateRule(c *gin.Context) {
	setID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	var req models.CreateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.svc.CreateRule(c.Request.Context(), setID, req)
	if err != nil {
		ruleSetError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (h *RuleSetHandler) UpdateRule(c *gin.Context) {
	rid, err := strconv.ParseInt(c.Param("rid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	var req models.UpdateRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule, err := h.svc.UpdateRule(c.Request.Context(), rid, req)
	if err != nil {
		ruleSetError(c, err)
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (h *RuleSetHandler) DeleteRule(c *gin.Context) {
	rid, err := strconv.ParseInt(c.Param("rid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule id"})
		return
	}
	if err := h.svc.DeleteRule(c.Request.Context(), rid); err != nil {
		ruleSetError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func ruleSetError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRuleSetNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "rule set not found"})
	case errors.Is(err, service.ErrRuleNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
	case errors.Is(err, service.ErrInvalidRuleType):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid rule type"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}

// ComplianceHandler — запуск и история прогонов набора правил по версии.
type ComplianceHandler struct{ svc service.ComplianceService }

func (h *ComplianceHandler) Run(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	var req models.RunComplianceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)
	check, err := h.svc.Run(c.Request.Context(), docID, vid, req.RuleSetID, userID, role)
	if err != nil {
		complianceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, check)
}

func (h *ComplianceHandler) List(c *gin.Context) {
	docID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid document id"})
		return
	}
	vid, err := strconv.ParseInt(c.Param("vid"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid version id"})
		return
	}
	userID := c.GetInt64(middleware.ContextUserID)
	role := c.GetString(middleware.ContextUserRole)
	checks, err := h.svc.ListByVersion(c.Request.Context(), docID, vid, userID, role)
	if err != nil {
		complianceError(c, err)
		return
	}
	c.JSON(http.StatusOK, checks)
}

func complianceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDocumentNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "document not found"})
	case errors.Is(err, service.ErrVersionNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "version not found"})
	case errors.Is(err, service.ErrRuleSetNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "rule set not found"})
	case errors.Is(err, service.ErrForbidden):
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
	case errors.Is(err, service.ErrEmptyParsedText):
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "version has no parsed text"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
