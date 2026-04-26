package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"diploma/internal/models"
)

type userRepo struct{ db *sqlx.DB }

func NewUserRepository(db *sqlx.DB) UserRepository { return &userRepo{db} }

func (r *userRepo) Create(ctx context.Context, u *models.User) error {
	q := `INSERT INTO users (name, email, password_hash, role, is_active, must_change_password, created_at, updated_at)
	      VALUES (:name, :email, :password_hash, :role, :is_active, :must_change_password, NOW(), NOW())
	      RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, u)
	if err != nil {
		return fmt.Errorf("userRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(u)
	}
	return nil
}

func (r *userRepo) GetByID(ctx context.Context, id int64) (*models.User, error) {
	u := &models.User{}
	err := r.db.GetContext(ctx, u, `SELECT * FROM users WHERE id=$1`, id)
	return u, err
}

func (r *userRepo) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	err := r.db.GetContext(ctx, u, `SELECT * FROM users WHERE email=$1`, email)
	return u, err
}

func (r *userRepo) List(ctx context.Context) ([]models.User, error) {
	users := []models.User{}
	err := r.db.SelectContext(ctx, &users, `SELECT * FROM users ORDER BY id`)
	return users, err
}

func (r *userRepo) ListByRole(ctx context.Context, role models.UserRole) ([]models.User, error) {
	users := []models.User{}
	err := r.db.SelectContext(ctx, &users,
		`SELECT * FROM users WHERE role=$1 AND is_active=true ORDER BY name`, role)
	return users, err
}

func (r *userRepo) Update(ctx context.Context, u *models.User) error {
	u.UpdatedAt = time.Now()
	_, err := r.db.NamedExecContext(ctx,
		`UPDATE users SET name=:name, email=:email, role=:role, updated_at=:updated_at WHERE id=:id`, u)
	return err
}

func (r *userRepo) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE users SET is_active=$1, updated_at=NOW() WHERE id=$2`, active, id)
	return err
}

func (r *userRepo) UpdatePassword(ctx context.Context, id int64, passwordHash string, mustChange bool) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET password_hash=$1, must_change_password=$2, updated_at=NOW() WHERE id=$3`,
		passwordHash, mustChange, id)
	return err
}

type projectRepo struct{ db *sqlx.DB }

func NewProjectRepository(db *sqlx.DB) ProjectRepository { return &projectRepo{db} }

func (r *projectRepo) Create(ctx context.Context, p *models.Project) error {
	q := `INSERT INTO projects (name, description, created_at) VALUES (:name, :description, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, p)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(p)
	}
	return nil
}

func (r *projectRepo) GetByID(ctx context.Context, id int64) (*models.Project, error) {
	p := &models.Project{}
	return p, r.db.GetContext(ctx, p, `SELECT * FROM projects WHERE id=$1`, id)
}

func (r *projectRepo) List(ctx context.Context) ([]models.Project, error) {
	list := []models.Project{}
	return list, r.db.SelectContext(ctx, &list, `SELECT * FROM projects ORDER BY id`)
}

type categoryRepo struct{ db *sqlx.DB }

func NewCategoryRepository(db *sqlx.DB) CategoryRepository { return &categoryRepo{db} }

func (r *categoryRepo) Create(ctx context.Context, c *models.Category) error {
	q := `INSERT INTO categories (name, description, created_at) VALUES (:name, :description, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(c)
	}
	return nil
}

func (r *categoryRepo) GetByID(ctx context.Context, id int64) (*models.Category, error) {
	c := &models.Category{}
	return c, r.db.GetContext(ctx, c, `SELECT * FROM categories WHERE id=$1`, id)
}

func (r *categoryRepo) List(ctx context.Context) ([]models.Category, error) {
	list := []models.Category{}
	return list, r.db.SelectContext(ctx, &list, `SELECT * FROM categories ORDER BY id`)
}

type documentRepo struct{ db *sqlx.DB }

func NewDocumentRepository(db *sqlx.DB) DocumentRepository { return &documentRepo{db} }

func (r *documentRepo) Create(ctx context.Context, d *models.Document) error {
	q := `INSERT INTO documents
		(title, description, project_id, category_id, created_by, reviewer_id, status, created_at, updated_at)
		VALUES (:title, :description, :project_id, :category_id, :created_by, :reviewer_id, :status, NOW(), NOW())
		RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, d)
	if err != nil {
		return fmt.Errorf("documentRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(d)
	}
	return nil
}

func (r *documentRepo) GetByID(ctx context.Context, id int64) (*models.Document, error) {
	d := &models.Document{}
	return d, r.db.GetContext(ctx, d, `SELECT * FROM documents WHERE id=$1`, id)
}

// GetByTitle ищет документ по названию без учёта регистра. Возвращает
// sql.ErrNoRows если не найден — service использует это для проверки
// уникальности при создании/переименовании.
func (r *documentRepo) GetByTitle(ctx context.Context, title string) (*models.Document, error) {
	d := &models.Document{}
	return d, r.db.GetContext(ctx, d,
		`SELECT * FROM documents WHERE LOWER(title) = LOWER($1) LIMIT 1`, title)
}

func (r *documentRepo) List(ctx context.Context, f DocumentFilters) ([]models.Document, error) {
	q := `SELECT * FROM documents WHERE 1=1`
	args := []interface{}{}
	i := 1
	if f.ProjectID != nil {
		q += fmt.Sprintf(" AND project_id=$%d", i)
		args = append(args, *f.ProjectID)
		i++
	}
	if f.CategoryID != nil {
		q += fmt.Sprintf(" AND category_id=$%d", i)
		args = append(args, *f.CategoryID)
		i++
	}
	if f.Status != nil {
		q += fmt.Sprintf(" AND status=$%d", i)
		args = append(args, *f.Status)
		i++
	}
	if f.CreatedBy != nil {
		q += fmt.Sprintf(" AND created_by=$%d", i)
		args = append(args, *f.CreatedBy)
		i++
	}
	if f.ReviewerID != nil {
		q += fmt.Sprintf(" AND reviewer_id=$%d", i)
		args = append(args, *f.ReviewerID)
	}
	q += " ORDER BY created_at DESC"
	list := []models.Document{}
	return list, r.db.SelectContext(ctx, &list, q, args...)
}

func (r *documentRepo) Update(ctx context.Context, d *models.Document) error {
	d.UpdatedAt = time.Now()
	_, err := r.db.NamedExecContext(ctx,
		`UPDATE documents SET title=:title, description=:description, reviewer_id=:reviewer_id,
		 category_id=:category_id, project_id=:project_id, updated_at=:updated_at WHERE id=:id`, d)
	return err
}

func (r *documentRepo) UpdateStatus(ctx context.Context, id int64, status models.DocumentStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE documents SET status=$1, updated_at=NOW() WHERE id=$2`, status, id)
	return err
}

func (r *documentRepo) SetCurrentVersion(ctx context.Context, docID, versionID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET current_version_id=$1, updated_at=NOW() WHERE id=$2`, versionID, docID)
	return err
}

func (r *documentRepo) SetPublishedVersion(ctx context.Context, docID, versionID int64, publishedBy int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE documents SET published_version_id=$1, published_by=$2, published_at=NOW(),
		 status='published', updated_at=NOW() WHERE id=$3`, versionID, publishedBy, docID)
	return err
}

func (r *documentRepo) ListPublished(ctx context.Context) ([]models.Document, error) {
	list := []models.Document{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM documents WHERE status='published' ORDER BY published_at DESC`)
}

func (r *documentRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM documents WHERE id=$1`, id)
	return err
}

type documentVersionRepo struct{ db *sqlx.DB }

func NewDocumentVersionRepository(db *sqlx.DB) DocumentVersionRepository {
	return &documentVersionRepo{db}
}

func (r *documentVersionRepo) Create(ctx context.Context, v *models.DocumentVersion) error {
	q := `INSERT INTO document_versions
		(document_id, version_number, file_name, file_path, file_type, parsed_text, change_summary, has_images, uploaded_by, is_current, created_at)
		VALUES (:document_id, :version_number, :file_name, :file_path, :file_type, :parsed_text, :change_summary, :has_images, :uploaded_by, :is_current, NOW())
		RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, v)
	if err != nil {
		return fmt.Errorf("documentVersionRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(v)
	}
	return nil
}

func (r *documentVersionRepo) GetByID(ctx context.Context, id int64) (*models.DocumentVersion, error) {
	v := &models.DocumentVersion{}
	return v, r.db.GetContext(ctx, v, `SELECT * FROM document_versions WHERE id=$1`, id)
}

func (r *documentVersionRepo) ListByDocument(ctx context.Context, documentID int64) ([]models.DocumentVersion, error) {
	list := []models.DocumentVersion{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM document_versions WHERE document_id=$1 ORDER BY created_at DESC`, documentID)
}

func (r *documentVersionRepo) GetCurrent(ctx context.Context, documentID int64) (*models.DocumentVersion, error) {
	v := &models.DocumentVersion{}
	return v, r.db.GetContext(ctx, v,
		`SELECT * FROM document_versions WHERE document_id=$1 AND is_current=true LIMIT 1`, documentID)
}

func (r *documentVersionRepo) UnsetCurrentForDocument(ctx context.Context, documentID int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE document_versions SET is_current=false WHERE document_id=$1`, documentID)
	return err
}

func (r *documentVersionRepo) SetCurrent(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE document_versions SET is_current=true WHERE id=$1`, id)
	return err
}

func (r *documentVersionRepo) UpdateParsedText(ctx context.Context, id int64, text string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE document_versions SET parsed_text=$1 WHERE id=$2`, text, id)
	return err
}

type reviewCommentRepo struct{ db *sqlx.DB }

func NewReviewCommentRepository(db *sqlx.DB) ReviewCommentRepository { return &reviewCommentRepo{db} }

func (r *reviewCommentRepo) Create(ctx context.Context, c *models.ReviewComment) error {
	q := `INSERT INTO review_comments (document_id, version_id, author_id, comment_text, created_at)
		  VALUES (:document_id, :version_id, :author_id, :comment_text, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(c)
	}
	return nil
}

func (r *reviewCommentRepo) ListByVersion(ctx context.Context, versionID int64) ([]models.ReviewComment, error) {
	list := []models.ReviewComment{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM review_comments WHERE version_id=$1 ORDER BY created_at`, versionID)
}

func (r *reviewCommentRepo) ListByDocument(ctx context.Context, documentID int64) ([]models.ReviewComment, error) {
	list := []models.ReviewComment{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM review_comments WHERE document_id=$1 ORDER BY created_at`, documentID)
}

type reviewActionRepo struct{ db *sqlx.DB }

func NewReviewActionRepository(db *sqlx.DB) ReviewActionRepository { return &reviewActionRepo{db} }

func (r *reviewActionRepo) Create(ctx context.Context, a *models.ReviewActionRecord) error {
	q := `INSERT INTO review_actions (document_id, version_id, reviewer_id, action, note, created_at)
		  VALUES (:document_id, :version_id, :reviewer_id, :action, :note, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, a)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(a)
	}
	return nil
}

func (r *reviewActionRepo) ListByDocument(ctx context.Context, documentID int64) ([]models.ReviewActionRecord, error) {
	list := []models.ReviewActionRecord{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM review_actions WHERE document_id=$1 ORDER BY created_at`, documentID)
}

type discussionThreadRepo struct{ db *sqlx.DB }

func NewDiscussionThreadRepository(db *sqlx.DB) DiscussionThreadRepository {
	return &discussionThreadRepo{db}
}

func (r *discussionThreadRepo) Create(ctx context.Context, t *models.DiscussionThread) error {
	q := `INSERT INTO discussion_threads
		(document_id, version_id, created_by, thread_type, page_number, anchor_text, status, is_public, created_at)
		VALUES (:document_id, :version_id, :created_by, :thread_type, :page_number, :anchor_text, :status, :is_public, NOW())
		RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, t)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(t)
	}
	return nil
}

func (r *discussionThreadRepo) GetByID(ctx context.Context, id int64) (*models.DiscussionThread, error) {
	t := &models.DiscussionThread{}
	return t, r.db.GetContext(ctx, t, `SELECT * FROM discussion_threads WHERE id=$1`, id)
}

func (r *discussionThreadRepo) ListByVersion(ctx context.Context, versionID int64) ([]models.DiscussionThread, error) {
	list := []models.DiscussionThread{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM discussion_threads WHERE version_id=$1 ORDER BY created_at`, versionID)
}

// ListByDocument возвращает треды по всем версиям документа. Если
// includePublic=false, публичные треды библиотеки исключаются — на странице
// ревью они не нужны.
func (r *discussionThreadRepo) ListByDocument(ctx context.Context, documentID int64, includePublic bool) ([]models.DiscussionThread, error) {
	list := []models.DiscussionThread{}
	q := `SELECT * FROM discussion_threads WHERE document_id=$1`
	if !includePublic {
		q += ` AND is_public=false`
	}
	q += ` ORDER BY created_at`
	return list, r.db.SelectContext(ctx, &list, q, documentID)
}

func (r *discussionThreadRepo) ListPublicByDocument(ctx context.Context, documentID int64) ([]models.DiscussionThread, error) {
	list := []models.DiscussionThread{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM discussion_threads WHERE document_id=$1 AND is_public=true ORDER BY created_at`, documentID)
}

func (r *discussionThreadRepo) UpdateStatus(ctx context.Context, id int64, status models.ThreadStatus) error {
	resolvedAt := "NULL"
	if status == models.ThreadStatusResolved {
		resolvedAt = "NOW()"
	}
	_, err := r.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE discussion_threads SET status=$1, resolved_at=%s WHERE id=$2`, resolvedAt),
		status, id)
	return err
}

type discussionMessageRepo struct{ db *sqlx.DB }

func NewDiscussionMessageRepository(db *sqlx.DB) DiscussionMessageRepository {
	return &discussionMessageRepo{db}
}

func (r *discussionMessageRepo) Create(ctx context.Context, m *models.DiscussionMessage) error {
	q := `INSERT INTO discussion_messages (thread_id, author_id, message_text, created_at)
		  VALUES (:thread_id, :author_id, :message_text, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, m)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(m)
	}
	return nil
}

func (r *discussionMessageRepo) ListByThread(ctx context.Context, threadID int64) ([]models.DiscussionMessage, error) {
	list := []models.DiscussionMessage{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM discussion_messages WHERE thread_id=$1 ORDER BY created_at`, threadID)
}

type aiCheckRepo struct{ db *sqlx.DB }

func NewAICheckRepository(db *sqlx.DB) AICheckRepository { return &aiCheckRepo{db} }

func (r *aiCheckRepo) Create(ctx context.Context, c *models.AICheck) error {
	q := `INSERT INTO ai_checks (document_id, version_id, check_type, score, status, result_json, created_at)
		  VALUES (:document_id, :version_id, :check_type, :score, :status, :result_json, NOW())
		  RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(c)
	}
	return nil
}

func (r *aiCheckRepo) GetByID(ctx context.Context, id int64) (*models.AICheck, error) {
	c := &models.AICheck{}
	return c, r.db.GetContext(ctx, c, `SELECT * FROM ai_checks WHERE id=$1`, id)
}

func (r *aiCheckRepo) ListByVersion(ctx context.Context, versionID int64) ([]models.AICheck, error) {
	list := []models.AICheck{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM ai_checks WHERE version_id=$1 ORDER BY created_at DESC`, versionID)
}

func (r *aiCheckRepo) ListByDocument(ctx context.Context, documentID int64) ([]models.AICheck, error) {
	list := []models.AICheck{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM ai_checks WHERE document_id=$1 ORDER BY created_at DESC`, documentID)
}

type auditLogRepo struct{ db *sqlx.DB }

func NewAuditLogRepository(db *sqlx.DB) AuditLogRepository { return &auditLogRepo{db} }

func (r *auditLogRepo) Create(ctx context.Context, l *models.AuditLog) error {
	q := `INSERT INTO audit_logs (user_id, action, entity_type, entity_id, details, created_at)
		  VALUES (:user_id, :action, :entity_type, :entity_id, :details, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, l)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(l)
	}
	return nil
}

func (r *auditLogRepo) List(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	list := []models.AuditLog{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
}

func (r *auditLogRepo) ListByUser(ctx context.Context, userID int64) ([]models.AuditLog, error) {
	list := []models.AuditLog{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM audit_logs WHERE user_id=$1 ORDER BY created_at DESC`, userID)
}

type systemErrorRepo struct{ db *sqlx.DB }

func NewSystemErrorRepository(db *sqlx.DB) SystemErrorRepository { return &systemErrorRepo{db} }

func (r *systemErrorRepo) Create(ctx context.Context, e *models.SystemError) error {
	q := `INSERT INTO system_errors (service_name, error_message, error_context, created_at)
		  VALUES (:service_name, :error_message, :error_context, NOW()) RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, e)
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(e)
	}
	return nil
}

func (r *systemErrorRepo) List(ctx context.Context, limit, offset int) ([]models.SystemError, error) {
	list := []models.SystemError{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM system_errors ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
}

type ruleSetRepo struct{ db *sqlx.DB }

func NewRuleSetRepository(db *sqlx.DB) RuleSetRepository { return &ruleSetRepo{db} }

func (r *ruleSetRepo) CreateSet(ctx context.Context, rs *models.RuleSet) error {
	q := `INSERT INTO rule_sets (name, description, is_active, created_by, created_at, updated_at)
	      VALUES (:name, :description, :is_active, :created_by, NOW(), NOW())
	      RETURNING id, created_at, updated_at`
	rows, err := r.db.NamedQueryContext(ctx, q, rs)
	if err != nil {
		return fmt.Errorf("ruleSetRepo.CreateSet: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(rs)
	}
	return nil
}

func (r *ruleSetRepo) GetSetByID(ctx context.Context, id int64) (*models.RuleSet, error) {
	var rs models.RuleSet
	if err := r.db.GetContext(ctx, &rs, `SELECT * FROM rule_sets WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &rs, nil
}

func (r *ruleSetRepo) ListSets(ctx context.Context, activeOnly bool) ([]models.RuleSet, error) {
	list := []models.RuleSet{}
	q := `SELECT * FROM rule_sets`
	if activeOnly {
		q += ` WHERE is_active = TRUE`
	}
	q += ` ORDER BY name`
	return list, r.db.SelectContext(ctx, &list, q)
}

func (r *ruleSetRepo) UpdateSet(ctx context.Context, rs *models.RuleSet) error {
	rs.UpdatedAt = time.Now()
	q := `UPDATE rule_sets SET name=:name, description=:description, is_active=:is_active,
	      updated_at=NOW() WHERE id=:id`
	_, err := r.db.NamedExecContext(ctx, q, rs)
	return err
}

func (r *ruleSetRepo) DeleteSet(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rule_sets WHERE id=$1`, id)
	return err
}

func (r *ruleSetRepo) CreateRule(ctx context.Context, rule *models.Rule) error {
	q := `INSERT INTO rules (rule_set_id, name, rule_type, params, severity, position, created_at)
	      VALUES (:rule_set_id, :name, :rule_type, :params, :severity, :position, NOW())
	      RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, rule)
	if err != nil {
		return fmt.Errorf("ruleSetRepo.CreateRule: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(rule)
	}
	return nil
}

func (r *ruleSetRepo) GetRuleByID(ctx context.Context, id int64) (*models.Rule, error) {
	var rule models.Rule
	if err := r.db.GetContext(ctx, &rule, `SELECT * FROM rules WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &rule, nil
}

func (r *ruleSetRepo) ListRulesBySet(ctx context.Context, ruleSetID int64) ([]models.Rule, error) {
	list := []models.Rule{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM rules WHERE rule_set_id=$1 ORDER BY position, id`, ruleSetID)
}

func (r *ruleSetRepo) UpdateRule(ctx context.Context, rule *models.Rule) error {
	q := `UPDATE rules SET name=:name, rule_type=:rule_type, params=:params,
	      severity=:severity, position=:position WHERE id=:id`
	_, err := r.db.NamedExecContext(ctx, q, rule)
	return err
}

func (r *ruleSetRepo) DeleteRule(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM rules WHERE id=$1`, id)
	return err
}

type complianceCheckRepo struct{ db *sqlx.DB }

func NewComplianceCheckRepository(db *sqlx.DB) ComplianceCheckRepository {
	return &complianceCheckRepo{db}
}

func (r *complianceCheckRepo) Create(ctx context.Context, c *models.ComplianceCheck) error {
	q := `INSERT INTO compliance_checks
	      (version_id, rule_set_id, total_rules, passed_rules, failed_rules, results, created_by, created_at)
	      VALUES (:version_id, :rule_set_id, :total_rules, :passed_rules, :failed_rules, :results, :created_by, NOW())
	      RETURNING id, created_at`
	rows, err := r.db.NamedQueryContext(ctx, q, c)
	if err != nil {
		return fmt.Errorf("complianceCheckRepo.Create: %w", err)
	}
	defer rows.Close()
	if rows.Next() {
		return rows.StructScan(c)
	}
	return nil
}

func (r *complianceCheckRepo) GetByID(ctx context.Context, id int64) (*models.ComplianceCheck, error) {
	var c models.ComplianceCheck
	if err := r.db.GetContext(ctx, &c, `SELECT * FROM compliance_checks WHERE id=$1`, id); err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *complianceCheckRepo) ListByVersion(ctx context.Context, versionID int64) ([]models.ComplianceCheck, error) {
	list := []models.ComplianceCheck{}
	return list, r.db.SelectContext(ctx, &list,
		`SELECT * FROM compliance_checks WHERE version_id=$1 ORDER BY created_at DESC`, versionID)
}
