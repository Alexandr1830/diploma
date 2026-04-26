package models

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

// JSONB — обёртка над []byte с реализацией sql.Scanner, driver.Valuer и
// json.Marshaler/Unmarshaler. Позволяет колонке типа JSONB пройти через
// структуру без промежуточного парсинга. Используется в Rule.Params и
// ComplianceCheck.Results.
type JSONB []byte

func (j *JSONB) Scan(src any) error {
	switch v := src.(type) {
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = append((*j)[:0], v...)
	case nil:
		*j = nil
	default:
		return fmt.Errorf("models.JSONB: unsupported scan type %T", src)
	}
	return nil
}

func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return []byte(j), nil
}

func (j JSONB) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("null"), nil
	}
	return []byte(j), nil
}

func (j *JSONB) UnmarshalJSON(data []byte) error {
	*j = append((*j)[:0], data...)
	return nil
}

type UserRole string

const (
	RoleWriter    UserRole = "writer"
	RoleReviewer  UserRole = "reviewer"
	RoleDeveloper UserRole = "developer"
	RoleAdmin     UserRole = "admin"
)

type DocumentStatus string

const (
	StatusDraft         DocumentStatus = "draft"
	StatusInReview      DocumentStatus = "in_review"
	StatusNeedsRevision DocumentStatus = "needs_revision"
	StatusApproved      DocumentStatus = "approved"
	StatusPublished     DocumentStatus = "published"
	StatusArchived      DocumentStatus = "archived"
)

type FileType string

const (
	FileTypeDocx FileType = "docx"
	FileTypeTXT  FileType = "txt"
	FileTypeMD   FileType = "md"
	FileTypeYAML FileType = "yaml"
)

// AllowedFileTypes — что разрешено заливать как новую версию.
var AllowedFileTypes = map[FileType]struct{}{
	FileTypeDocx: {},
	FileTypeTXT:  {},
	FileTypeMD:   {},
	FileTypeYAML: {},
}

type CheckType string

const (
	CheckTypeAI   CheckType = "AI"
	CheckTypeGOST CheckType = "GOST"
)

type CheckStatus string

const (
	CheckStatusOK      CheckStatus = "ok"
	CheckStatusWarning CheckStatus = "warning"
	CheckStatusError   CheckStatus = "error"
)

type ThreadType string

const (
	ThreadTypeGeneral  ThreadType = "general"
	ThreadTypeAnchored ThreadType = "anchored"
)

type ThreadStatus string

const (
	ThreadStatusOpen     ThreadStatus = "open"
	ThreadStatusResolved ThreadStatus = "resolved"
)

type ReviewAction string

const (
	ReviewActionApprove         ReviewAction = "approve"
	ReviewActionRequestRevision ReviewAction = "request_revision"
)

type User struct {
	ID                 int64     `db:"id"                   json:"id"`
	Name               string    `db:"name"                 json:"name"`
	Email              string    `db:"email"                json:"email"`
	PasswordHash       string    `db:"password_hash"        json:"-"`
	Role               UserRole  `db:"role"                 json:"role"`
	IsActive           bool      `db:"is_active"            json:"is_active"`
	MustChangePassword bool      `db:"must_change_password" json:"must_change_password"`
	CreatedAt          time.Time `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time `db:"updated_at"           json:"updated_at"`
}

type Project struct {
	ID          int64     `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

type Category struct {
	ID          int64     `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
}

type Document struct {
	ID                 int64          `db:"id"                   json:"id"`
	Title              string         `db:"title"                json:"title"`
	Description        string         `db:"description"          json:"description"`
	ProjectID          int64          `db:"project_id"           json:"project_id"`
	CategoryID         int64          `db:"category_id"          json:"category_id"`
	CreatedBy          int64          `db:"created_by"           json:"created_by"`
	ReviewerID         sql.NullInt64  `db:"reviewer_id"          json:"reviewer_id"`
	Status             DocumentStatus `db:"status"               json:"status"`
	CurrentVersionID   sql.NullInt64  `db:"current_version_id"   json:"current_version_id"`
	PublishedVersionID sql.NullInt64  `db:"published_version_id" json:"published_version_id"`
	PublishedAt        sql.NullTime   `db:"published_at"         json:"published_at"`
	PublishedBy        sql.NullInt64  `db:"published_by"         json:"published_by"`
	CreatedAt          time.Time      `db:"created_at"           json:"created_at"`
	UpdatedAt          time.Time      `db:"updated_at"           json:"updated_at"`
}

type DocumentVersion struct {
	ID            int64          `db:"id"             json:"id"`
	DocumentID    int64          `db:"document_id"    json:"document_id"`
	VersionNumber string         `db:"version_number" json:"version_number"`
	FileName      string         `db:"file_name"      json:"file_name"`
	FilePath      string         `db:"file_path"      json:"file_path"`
	FileType      FileType       `db:"file_type"      json:"file_type"`
	ParsedText    sql.NullString `db:"parsed_text"    json:"-"`
	ChangeSummary sql.NullString `db:"change_summary" json:"change_summary"`
	HasImages     bool           `db:"has_images"     json:"has_images"`
	UploadedBy    int64          `db:"uploaded_by"    json:"uploaded_by"`
	IsCurrent     bool           `db:"is_current"     json:"is_current"`
	CreatedAt     time.Time      `db:"created_at"     json:"created_at"`
}

type ReviewComment struct {
	ID          int64     `db:"id"           json:"id"`
	DocumentID  int64     `db:"document_id"  json:"document_id"`
	VersionID   int64     `db:"version_id"   json:"version_id"`
	AuthorID    int64     `db:"author_id"    json:"author_id"`
	CommentText string    `db:"comment_text" json:"comment_text"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}

type ReviewActionRecord struct {
	ID         int64        `db:"id"          json:"id"`
	DocumentID int64        `db:"document_id" json:"document_id"`
	VersionID  int64        `db:"version_id"  json:"version_id"`
	ReviewerID int64        `db:"reviewer_id" json:"reviewer_id"`
	Action     ReviewAction `db:"action"      json:"action"`
	Note       string       `db:"note"        json:"note"`
	CreatedAt  time.Time    `db:"created_at"  json:"created_at"`
}

type DiscussionThread struct {
	ID         int64          `db:"id"          json:"id"`
	DocumentID int64          `db:"document_id" json:"document_id"`
	VersionID  int64          `db:"version_id"  json:"version_id"`
	CreatedBy  int64          `db:"created_by"  json:"created_by"`
	ThreadType ThreadType     `db:"thread_type" json:"thread_type"`
	PageNumber sql.NullInt32  `db:"page_number" json:"page_number"`
	AnchorText sql.NullString `db:"anchor_text" json:"anchor_text"`
	Status     ThreadStatus   `db:"status"      json:"status"`
	IsPublic   bool           `db:"is_public"   json:"is_public"`
	CreatedAt  time.Time      `db:"created_at"  json:"created_at"`
	ResolvedAt sql.NullTime   `db:"resolved_at" json:"resolved_at"`
}

type DiscussionMessage struct {
	ID          int64     `db:"id"           json:"id"`
	ThreadID    int64     `db:"thread_id"    json:"thread_id"`
	AuthorID    int64     `db:"author_id"    json:"author_id"`
	MessageText string    `db:"message_text" json:"message_text"`
	CreatedAt   time.Time `db:"created_at"   json:"created_at"`
}

type AICheck struct {
	ID         int64           `db:"id"          json:"id"`
	DocumentID int64           `db:"document_id" json:"document_id"`
	VersionID  int64           `db:"version_id"  json:"version_id"`
	CheckType  CheckType       `db:"check_type"  json:"check_type"`
	Score      sql.NullFloat64 `db:"score"       json:"score"`
	Status     CheckStatus     `db:"status"      json:"status"`
	ResultJSON sql.NullString  `db:"result_json" json:"result_json"`
	CreatedAt  time.Time       `db:"created_at"  json:"created_at"`
}

type AuditLog struct {
	ID         int64          `db:"id"          json:"id"`
	UserID     int64          `db:"user_id"     json:"user_id"`
	Action     string         `db:"action"      json:"action"`
	EntityType string         `db:"entity_type" json:"entity_type"`
	EntityID   int64          `db:"entity_id"   json:"entity_id"`
	Details    sql.NullString `db:"details"     json:"details"`
	CreatedAt  time.Time      `db:"created_at"  json:"created_at"`
}

type SystemError struct {
	ID           int64     `db:"id"            json:"id"`
	ServiceName  string    `db:"service_name"  json:"service_name"`
	ErrorMessage string    `db:"error_message" json:"error_message"`
	ErrorContext string    `db:"error_context" json:"error_context"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

// LoginRequest — тело запроса POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// TokenResponse возвращается после успешного логина.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

// CreateUserRequest — тело запроса POST /api/v1/admin/users.
type CreateUserRequest struct {
	Name     string   `json:"name"     binding:"required"`
	Email    string   `json:"email"    binding:"required,email"`
	Password string   `json:"password" binding:"required,min=8"`
	Role     UserRole `json:"role"     binding:"required"`
}

// UpdateUserAdminRequest — тело запроса PUT /api/v1/admin/users/:id.
// Смена пароля идёт через /reset-password, активность — через /active,
// поэтому здесь только базовые поля профиля.
type UpdateUserAdminRequest struct {
	Name  string   `json:"name"  binding:"required"`
	Email string   `json:"email" binding:"required,email"`
	Role  UserRole `json:"role"  binding:"required"`
}

// ChangePasswordRequest — тело запроса POST /api/v1/auth/change-password.
// OldPassword необязателен: если у пользователя must_change_password=true
// (форсированная первая смена), сервис не проверяет старый пароль.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// ResetPasswordRequest — тело запроса POST /api/v1/admin/users/:id/reset-password.
type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

// SetActiveRequest — тело запроса PUT /api/v1/admin/users/:id/active.
type SetActiveRequest struct {
	IsActive bool `json:"is_active"`
}

// CreateDocumentRequest — тело запроса POST /api/v1/documents.
// WriterID учитывается ТОЛЬКО для admin/reviewer — они могут создать документ
// от имени конкретного writer'а. Для остальных ролей backend это поле
// игнорирует и проставляет created_by = caller.
type CreateDocumentRequest struct {
	Title       string `json:"title"       binding:"required"`
	Description string `json:"description"`
	ProjectID   int64  `json:"project_id"  binding:"required"`
	CategoryID  int64  `json:"category_id" binding:"required"`
	WriterID    *int64 `json:"writer_id"`
	ReviewerID  *int64 `json:"reviewer_id"`
}

// UpdateDocumentRequest — тело запроса PUT /api/v1/documents/:id.
// reviewer_id=null снимает ревьюера.
type UpdateDocumentRequest struct {
	Title       string `json:"title"       binding:"required"`
	Description string `json:"description"`
	CategoryID  int64  `json:"category_id" binding:"required"`
	ReviewerID  *int64 `json:"reviewer_id"`
}

// DocumentQuery — query-параметры для GET /api/v1/documents.
type DocumentQuery struct {
	ProjectID  *int64
	CategoryID *int64
	Status     *DocumentStatus
}

// UploadVersionRequest — тело запроса POST /api/v1/documents/:id/versions.
type UploadVersionRequest struct {
	FileName      string   `json:"file_name"      binding:"required"`
	FilePath      string   `json:"file_path"      binding:"required"`
	FileType      FileType `json:"file_type"      binding:"required"`
	ParsedText    string   `json:"parsed_text"`
	ChangeSummary string   `json:"change_summary"`
}

// AddCommentRequest — тело запроса POST /api/v1/documents/:id/versions/:vid/comments.
type AddCommentRequest struct {
	CommentText string `json:"comment_text" binding:"required"`
}

// RequestRevisionRequest — тело запроса POST /api/v1/documents/:id/revision.
type RequestRevisionRequest struct {
	Note string `json:"note" binding:"required"`
}

// CreateThreadRequest — тело запроса POST /api/v1/documents/:id/versions/:vid/threads.
type CreateThreadRequest struct {
	Type   ThreadType `json:"type"   binding:"required"`
	Anchor *string    `json:"anchor"`
}

// CreateMessageRequest — тело запроса POST /api/v1/threads/:tid/messages.
type CreateMessageRequest struct {
	Message string `json:"message" binding:"required"`
}

// ThreadWithMessages — тред + статистика и сообщения, возвращается агрегатным
// эндпоинтом discussion-view, чтобы фронту хватило одного запроса.
type ThreadWithMessages struct {
	DiscussionThread
	MessagesCount      int                 `json:"messages_count"`
	LastMessageAt      *time.Time          `json:"last_message_at"`
	LastMessagePreview string              `json:"last_message_preview"`
	Messages           []DiscussionMessage `json:"messages"`
}

// DiscussionViewResponse — тело ответа GET /api/v1/documents/:id/discussion-view.
type DiscussionViewResponse struct {
	Document       *Document            `json:"document"`
	CurrentVersion *DocumentVersion     `json:"current_version"`
	Threads        []ThreadWithMessages `json:"threads"`
}

// LibraryDocumentResponse — тело ответа GET /api/v1/library/:id. Сразу
// включает опубликованную версию, чтобы фронт мог нарисовать файл inline
// без дополнительного запроса.
type LibraryDocumentResponse struct {
	Document         *Document        `json:"document"`
	PublishedVersion *DocumentVersion `json:"published_version"`
}

// UserShort — компактный вид пользователя для селекторов и ссылок.
type UserShort struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// DiffChunk — один блок diff'а: добавленный, удалённый или общий кусок.
type DiffChunk struct {
	Type string `json:"type"` // "added", "removed", "equal"
	Text string `json:"text"`
}

// DiffResponse — тело ответа GET /api/v1/documents/:id/versions/:v1/diff/:v2.
type DiffResponse struct {
	TextDiff      []DiffChunk `json:"text_diff"`
	ImagesChanged bool        `json:"images_changed"`
}

// RuleType — тип проверки. Каждый тип задаёт свою форму JSONB params
// (схема описана в migrations/006_rule_sets.sql).
type RuleType string

const (
	RuleMustContainPhrase    RuleType = "must_contain_phrase"
	RuleMustNotContainPhrase RuleType = "must_not_contain_phrase"
	RuleSectionOrder         RuleType = "section_order"
	RuleRegexMatch           RuleType = "regex_match"
	RuleMinWordCount         RuleType = "min_word_count"
)

// AllowedRuleTypes — белый список для валидации входа в админке.
var AllowedRuleTypes = map[RuleType]struct{}{
	RuleMustContainPhrase:    {},
	RuleMustNotContainPhrase: {},
	RuleSectionOrder:         {},
	RuleRegexMatch:           {},
	RuleMinWordCount:         {},
}

type RuleSeverity string

const (
	SeverityError   RuleSeverity = "error"
	SeverityWarning RuleSeverity = "warning"
)

// RuleSet — группа правил под одним стандартом (например, «ГОСТ 7.32-2017»).
type RuleSet struct {
	ID          int64     `db:"id"          json:"id"`
	Name        string    `db:"name"        json:"name"`
	Description string    `db:"description" json:"description"`
	IsActive    bool      `db:"is_active"   json:"is_active"`
	CreatedBy   int64     `db:"created_by"  json:"created_by"`
	CreatedAt   time.Time `db:"created_at"  json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"  json:"updated_at"`
}

// Rule — одно правило внутри RuleSet. Params — сырой JSON, форма зависит от
// Type. Валидацию params делает пакет engine.
type Rule struct {
	ID        int64        `db:"id"          json:"id"`
	RuleSetID int64        `db:"rule_set_id" json:"rule_set_id"`
	Name      string       `db:"name"        json:"name"`
	Type      RuleType     `db:"rule_type"   json:"rule_type"`
	Params    JSONB        `db:"params"      json:"params"`
	Severity  RuleSeverity `db:"severity"    json:"severity"`
	Position  int          `db:"position"    json:"position"`
	CreatedAt time.Time    `db:"created_at"  json:"created_at"`
}

// RuleSetWithRules — джойн на стороне сервиса, удобно отдавать в админку и
// в момент запуска проверки.
type RuleSetWithRules struct {
	RuleSet
	Rules []Rule `json:"rules"`
}

// ComplianceCheck — одно выполнение RuleSet поверх версии. Results — JSONB
// массив RuleResult.
type ComplianceCheck struct {
	ID           int64     `db:"id"            json:"id"`
	VersionID    int64     `db:"version_id"    json:"version_id"`
	RuleSetID    int64     `db:"rule_set_id"   json:"rule_set_id"`
	TotalRules   int       `db:"total_rules"   json:"total_rules"`
	PassedRules  int       `db:"passed_rules"  json:"passed_rules"`
	FailedRules  int       `db:"failed_rules"  json:"failed_rules"`
	Results      JSONB     `db:"results"       json:"results"`
	CreatedBy    int64     `db:"created_by"    json:"created_by"`
	CreatedAt    time.Time `db:"created_at"    json:"created_at"`
}

// RuleResult — один элемент в массиве compliance_checks.results.
type RuleResult struct {
	RuleID   int64        `json:"rule_id"`
	Name     string       `json:"name"`
	Type     RuleType     `json:"rule_type"`
	Severity RuleSeverity `json:"severity"`
	Passed   bool         `json:"passed"`
	Message  string       `json:"message"`
	Location string       `json:"location,omitempty"`
}

// CreateRuleSetRequest — тело запроса POST /admin/rule-sets.
type CreateRuleSetRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	IsActive    *bool  `json:"is_active"`
}

// UpdateRuleSetRequest — тело запроса PUT /admin/rule-sets/:id.
type UpdateRuleSetRequest struct {
	Name        string `json:"name"        binding:"required"`
	Description string `json:"description"`
	IsActive    bool   `json:"is_active"`
}

// CreateRuleRequest — тело запроса POST /admin/rule-sets/:id/rules.
// Params уходит в engine как есть и валидируется им же.
type CreateRuleRequest struct {
	Name     string          `json:"name"      binding:"required"`
	Type     RuleType        `json:"rule_type" binding:"required"`
	Params   map[string]any  `json:"params"    binding:"required"`
	Severity RuleSeverity    `json:"severity"`
}

// UpdateRuleRequest — то же что CreateRuleRequest плюс position для
// перетаскивания правил в нужный порядок.
type UpdateRuleRequest struct {
	Name     string          `json:"name"      binding:"required"`
	Type     RuleType        `json:"rule_type" binding:"required"`
	Params   map[string]any  `json:"params"    binding:"required"`
	Severity RuleSeverity    `json:"severity"`
	Position int             `json:"position"`
}

// RunComplianceRequest — тело запроса POST /documents/:id/versions/:vid/compliance.
type RunComplianceRequest struct {
	RuleSetID int64 `json:"rule_set_id" binding:"required"`
}
