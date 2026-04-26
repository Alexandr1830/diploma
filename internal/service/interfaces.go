package service

import (
	"context"

	"diploma/internal/models"
)

// AuthService — логин и смена собственного пароля. Заведение пользователей
// делает админ через AdminService.
type AuthService interface {
	Login(ctx context.Context, req models.LoginRequest) (string, error)
	Me(ctx context.Context, userID int64) (*models.User, error)
	ChangePassword(ctx context.Context, userID int64, req models.ChangePasswordRequest) error
}

// DocumentService — CRUD по документам и переходы между статусами.
type DocumentService interface {
	Create(ctx context.Context, req models.CreateDocumentRequest, userID int64, role string) (*models.Document, error)
	List(ctx context.Context, userID int64, role string, q models.DocumentQuery) ([]models.Document, error)
	GetByID(ctx context.Context, id, userID int64, role string) (*models.Document, error)
	Update(ctx context.Context, id int64, req models.UpdateDocumentRequest, userID int64, role string) (*models.Document, error)
	Delete(ctx context.Context, id, userID int64, role string) error
	Publish(ctx context.Context, id, userID int64, role string) (*models.Document, error)
	Unpublish(ctx context.Context, id, userID int64, role string) (*models.Document, error)
	Archive(ctx context.Context, id, userID int64, role string) (*models.Document, error)
}

// VersionService — загрузка версий документа и работа с ними.
type VersionService interface {
	Upload(ctx context.Context, docID int64, req models.UploadVersionRequest, userID int64, role string) (*models.DocumentVersion, error)
	List(ctx context.Context, docID int64, userID int64, role string) ([]models.DocumentVersion, error)
	GetByID(ctx context.Context, docID, versionID int64, userID int64, role string) (*models.DocumentVersion, error)
	Restore(ctx context.Context, docID, versionID int64, userID int64, role string) (*models.DocumentVersion, error)
	Diff(ctx context.Context, docID, v1ID, v2ID, userID int64, role string) (*models.DiffResponse, error)
}

// UserListService — выборка пользователей для селекторов в UI.
type UserListService interface {
	ListByRole(ctx context.Context, role models.UserRole) ([]models.UserShort, error)
}

// ReviewService — переходы по ревью (submit/approve/revision), комментарии
// и история действий ревьюера.
type ReviewService interface {
	Submit(ctx context.Context, docID, userID int64, role string) (*models.Document, error)
	Approve(ctx context.Context, docID, userID int64, role string) (*models.ReviewActionRecord, error)
	RequestRevision(ctx context.Context, docID int64, note string, userID int64, role string) (*models.ReviewActionRecord, error)
	AddComment(ctx context.Context, docID, versionID int64, text string, userID int64, role string) (*models.ReviewComment, error)
	ListComments(ctx context.Context, docID, versionID int64, userID int64, role string) ([]models.ReviewComment, error)
	ListActions(ctx context.Context, docID, userID int64, role string) ([]models.ReviewActionRecord, error)
}

// DiscussionService — треды и сообщения.
// Внутренние треды (ревью): только writer/reviewer/admin, привязаны к версии.
// Публичные треды библиотеки: любой залогиненный, привязаны к опубликованному документу.
type DiscussionService interface {
	CreateThread(ctx context.Context, docID, versionID int64, req models.CreateThreadRequest, userID int64, role string) (*models.DiscussionThread, error)
	ListThreads(ctx context.Context, docID, versionID int64, userID int64, role string) ([]models.DiscussionThread, error)
	CreateMessage(ctx context.Context, threadID int64, text string, userID int64, role string) (*models.DiscussionMessage, error)
	ListMessages(ctx context.Context, threadID int64, userID int64, role string) ([]models.DiscussionMessage, error)
	ResolveThread(ctx context.Context, threadID int64, userID int64, role string) (*models.DiscussionThread, error)
	GetDiscussionView(ctx context.Context, docID, userID int64, role string) (*models.DiscussionViewResponse, error)
	CreateLibraryThread(ctx context.Context, docID int64, req models.CreateThreadRequest, userID int64) (*models.DiscussionThread, error)
	ListLibraryThreads(ctx context.Context, docID int64) ([]models.ThreadWithMessages, error)
}

// AICheckService — заглушка под будущие AI/GOST-проверки.
type AICheckService interface{}

// LibraryService — публичная библиотека опубликованных документов.
type LibraryService interface {
	List(ctx context.Context) ([]models.Document, error)
	GetByID(ctx context.Context, id, userID int64, role string) (*models.LibraryDocumentResponse, error)
}

// AdminService — управление пользователями (только для admin).
type AdminService interface {
	CreateUser(ctx context.Context, req models.CreateUserRequest) (*models.User, error)
	UpdateUser(ctx context.Context, id int64, req models.UpdateUserAdminRequest) (*models.User, error)
	ListUsers(ctx context.Context) ([]models.User, error)
	SetUserActive(ctx context.Context, id int64, active bool) error
	ResetPassword(ctx context.Context, id int64, newPassword string) error
}

// RuleSetService — админский CRUD по наборам правил compliance и их правилам.
type RuleSetService interface {
	CreateSet(ctx context.Context, req models.CreateRuleSetRequest, createdBy int64) (*models.RuleSet, error)
	GetSet(ctx context.Context, id int64) (*models.RuleSetWithRules, error)
	ListSets(ctx context.Context, activeOnly bool) ([]models.RuleSet, error)
	UpdateSet(ctx context.Context, id int64, req models.UpdateRuleSetRequest) (*models.RuleSet, error)
	DeleteSet(ctx context.Context, id int64) error

	CreateRule(ctx context.Context, ruleSetID int64, req models.CreateRuleRequest) (*models.Rule, error)
	UpdateRule(ctx context.Context, ruleID int64, req models.UpdateRuleRequest) (*models.Rule, error)
	DeleteRule(ctx context.Context, ruleID int64) error
}

// ComplianceService прогоняет набор правил по parsed_text версии и сохраняет
// результат в compliance_checks.
type ComplianceService interface {
	Run(ctx context.Context, docID, versionID, ruleSetID, userID int64, role string) (*models.ComplianceCheck, error)
	ListByVersion(ctx context.Context, docID, versionID, userID int64, role string) ([]models.ComplianceCheck, error)
}
