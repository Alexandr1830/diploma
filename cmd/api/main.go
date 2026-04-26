package main

import (
	"context"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/joho/godotenv"

	"diploma/internal/handler"
	"diploma/internal/middleware"
	"diploma/internal/observability"
	"diploma/internal/parser"
	"diploma/internal/repository"
	"diploma/internal/service"
	"diploma/pkg/database"
)

func main() {
	// .env подхватывается только локально, в проде переменные приходят из окружения
	_ = godotenv.Load()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Fatal("JWT_SECRET environment variable is required")
	}

	// OpenTelemetry: трейсы, метрики и логи уезжают в OTLP-коллектор
	// (grafana/otel-lgtm). Если коллектор недоступен — Setup всё равно
	// вернёт работающий no-op.
	shutdownObs := observability.MustSetup("diploma-api")
	defer func() { _ = shutdownObs(context.Background()) }()
	middleware.InitMetrics()

	dbCfg := database.NewConfig()
	db, err := database.Connect(dbCfg)
	if err != nil {
		log.Fatalf("cannot connect to database: %v", err)
	}
	defer db.Close()

	// Логи через slog уезжают и в stdout, и в OTLP (Loki).
	slog.Info("connected to database",
		"user", dbCfg.User,
		"host", dbCfg.Host,
		"port", dbCfg.Port,
		"db", dbCfg.DBName,
	)

	userRepo := repository.NewUserRepository(db)
	docRepo := repository.NewDocumentRepository(db)
	verRepo := repository.NewDocumentVersionRepository(db)
	commentRepo := repository.NewReviewCommentRepository(db)
	actionRepo := repository.NewReviewActionRepository(db)
	threadRepo := repository.NewDiscussionThreadRepository(db)
	msgRepo := repository.NewDiscussionMessageRepository(db)
	checkRepo := repository.NewAICheckRepository(db)
	_ = checkRepo
	auditRepo := repository.NewAuditLogRepository(db)
	_ = auditRepo
	errRepo := repository.NewSystemErrorRepository(db)
	_ = errRepo
	ruleSetRepo := repository.NewRuleSetRepository(db)
	complianceRepo := repository.NewComplianceCheckRepository(db)

	authSvc := service.NewAuthService(userRepo, jwtSecret, 24*time.Hour)
	docSvc := service.NewDocumentService(docRepo)
	verSvc := service.NewVersionService(docRepo, verRepo, parser.New())
	reviewSvc := service.NewReviewService(docRepo, verRepo, commentRepo, actionRepo, threadRepo, msgRepo)
	discussionSvc := service.NewDiscussionService(docRepo, verRepo, threadRepo, msgRepo)
	libSvc := service.NewLibraryService(docRepo, verRepo)
	userListSvc := service.NewUserListService(userRepo)
	adminSvc := service.NewAdminService(userRepo)
	ruleSetSvc := service.NewRuleSetService(ruleSetRepo)
	complianceSvc := service.NewComplianceService(docRepo, verRepo, ruleSetRepo, complianceRepo)

	r := handler.NewRouter(
		jwtSecret,
		authSvc,
		docSvc,
		verSvc,
		reviewSvc,
		discussionSvc,
		nil, // AI-проверка пока не реализована
		libSvc,
		adminSvc,
		userListSvc,
		ruleSetSvc,
		complianceSvc,
	)

	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	slog.Info("starting server", "port", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
