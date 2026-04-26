package repository

import (
	"context"

	"diploma/internal/models"
)

type UserRepository interface {
	Create(ctx context.Context, u *models.User) error
	GetByID(ctx context.Context, id int64) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	List(ctx context.Context) ([]models.User, error)
	ListByRole(ctx context.Context, role models.UserRole) ([]models.User, error)
	Update(ctx context.Context, u *models.User) error
	SetActive(ctx context.Context, id int64, active bool) error
	UpdatePassword(ctx context.Context, id int64, passwordHash string, mustChange bool) error
}

type ProjectRepository interface {
	Create(ctx context.Context, p *models.Project) error
	GetByID(ctx context.Context, id int64) (*models.Project, error)
	List(ctx context.Context) ([]models.Project, error)
}

type CategoryRepository interface {
	Create(ctx context.Context, c *models.Category) error
	GetByID(ctx context.Context, id int64) (*models.Category, error)
	List(ctx context.Context) ([]models.Category, error)
}

type DocumentRepository interface {
	Create(ctx context.Context, d *models.Document) error
	GetByID(ctx context.Context, id int64) (*models.Document, error)
	GetByTitle(ctx context.Context, title string) (*models.Document, error)
	List(ctx context.Context, filters DocumentFilters) ([]models.Document, error)
	Update(ctx context.Context, d *models.Document) error
	UpdateStatus(ctx context.Context, id int64, status models.DocumentStatus) error
	SetCurrentVersion(ctx context.Context, docID, versionID int64) error
	SetPublishedVersion(ctx context.Context, docID, versionID int64, publishedBy int64) error
	ListPublished(ctx context.Context) ([]models.Document, error)
	Delete(ctx context.Context, id int64) error
}

type DocumentFilters struct {
	ProjectID  *int64
	CategoryID *int64
	Status     *models.DocumentStatus
	CreatedBy  *int64
	ReviewerID *int64
}

type DocumentVersionRepository interface {
	Create(ctx context.Context, v *models.DocumentVersion) error
	GetByID(ctx context.Context, id int64) (*models.DocumentVersion, error)
	ListByDocument(ctx context.Context, documentID int64) ([]models.DocumentVersion, error)
	GetCurrent(ctx context.Context, documentID int64) (*models.DocumentVersion, error)
	UnsetCurrentForDocument(ctx context.Context, documentID int64) error
	SetCurrent(ctx context.Context, id int64) error
	UpdateParsedText(ctx context.Context, id int64, parsedText string) error
}

type ReviewCommentRepository interface {
	Create(ctx context.Context, c *models.ReviewComment) error
	ListByVersion(ctx context.Context, versionID int64) ([]models.ReviewComment, error)
	ListByDocument(ctx context.Context, documentID int64) ([]models.ReviewComment, error)
}

type ReviewActionRepository interface {
	Create(ctx context.Context, a *models.ReviewActionRecord) error
	ListByDocument(ctx context.Context, documentID int64) ([]models.ReviewActionRecord, error)
}

type DiscussionThreadRepository interface {
	Create(ctx context.Context, t *models.DiscussionThread) error
	GetByID(ctx context.Context, id int64) (*models.DiscussionThread, error)
	ListByVersion(ctx context.Context, versionID int64) ([]models.DiscussionThread, error)
	ListByDocument(ctx context.Context, documentID int64, includePublic bool) ([]models.DiscussionThread, error)
	ListPublicByDocument(ctx context.Context, documentID int64) ([]models.DiscussionThread, error)
	UpdateStatus(ctx context.Context, id int64, status models.ThreadStatus) error
}

type DiscussionMessageRepository interface {
	Create(ctx context.Context, m *models.DiscussionMessage) error
	ListByThread(ctx context.Context, threadID int64) ([]models.DiscussionMessage, error)
}

type AICheckRepository interface {
	Create(ctx context.Context, c *models.AICheck) error
	GetByID(ctx context.Context, id int64) (*models.AICheck, error)
	ListByVersion(ctx context.Context, versionID int64) ([]models.AICheck, error)
	ListByDocument(ctx context.Context, documentID int64) ([]models.AICheck, error)
}

type AuditLogRepository interface {
	Create(ctx context.Context, l *models.AuditLog) error
	List(ctx context.Context, limit, offset int) ([]models.AuditLog, error)
	ListByUser(ctx context.Context, userID int64) ([]models.AuditLog, error)
}

type SystemErrorRepository interface {
	Create(ctx context.Context, e *models.SystemError) error
	List(ctx context.Context, limit, offset int) ([]models.SystemError, error)
}

// RuleSetRepository — наборы правил для compliance, заводятся админом.
type RuleSetRepository interface {
	CreateSet(ctx context.Context, rs *models.RuleSet) error
	GetSetByID(ctx context.Context, id int64) (*models.RuleSet, error)
	ListSets(ctx context.Context, activeOnly bool) ([]models.RuleSet, error)
	UpdateSet(ctx context.Context, rs *models.RuleSet) error
	DeleteSet(ctx context.Context, id int64) error

	CreateRule(ctx context.Context, r *models.Rule) error
	GetRuleByID(ctx context.Context, id int64) (*models.Rule, error)
	ListRulesBySet(ctx context.Context, ruleSetID int64) ([]models.Rule, error)
	UpdateRule(ctx context.Context, r *models.Rule) error
	DeleteRule(ctx context.Context, id int64) error
}

// ComplianceCheckRepository — записанные прогоны набора правил по версии.
type ComplianceCheckRepository interface {
	Create(ctx context.Context, c *models.ComplianceCheck) error
	GetByID(ctx context.Context, id int64) (*models.ComplianceCheck, error)
	ListByVersion(ctx context.Context, versionID int64) ([]models.ComplianceCheck, error)
}
