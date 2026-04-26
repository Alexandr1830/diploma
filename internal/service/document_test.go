package service

import (
	"database/sql"
	"testing"

	"diploma/internal/models"
)

// newDoc собирает документ для теста: статус, владелец, опционально ревьюер.
func newDoc(status models.DocumentStatus, createdBy int64, reviewerID int64) *models.Document {
	d := &models.Document{
		ID:        1,
		CreatedBy: createdBy,
		Status:    status,
	}
	if reviewerID > 0 {
		d.ReviewerID = sql.NullInt64{Int64: reviewerID, Valid: true}
	}
	return d
}

// Черновик — приватный документ автора. Никто, даже admin, его не видит.
func TestCanRead_DraftIsPrivateToOwner(t *testing.T) {
	doc := newDoc(models.StatusDraft, 10, 20)

	cases := []struct {
		name   string
		userID int64
		role   models.UserRole
		want   bool
	}{
		{"автор видит свой черновик", 10, models.RoleWriter, true},
		{"admin НЕ видит чужой черновик", 99, models.RoleAdmin, false},
		{"назначенный ревьюер НЕ видит черновик", 20, models.RoleReviewer, false},
		{"developer не видит черновики", 30, models.RoleDeveloper, false},
		{"чужой writer не видит черновик", 11, models.RoleWriter, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canRead(doc, tc.userID, tc.role); got != tc.want {
				t.Errorf("canRead=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanRead_InReviewVisibility(t *testing.T) {
	doc := newDoc(models.StatusInReview, 10, 20)

	cases := []struct {
		name   string
		userID int64
		role   models.UserRole
		want   bool
	}{
		{"admin видит документы на ревью", 99, models.RoleAdmin, true},
		{"автор видит свой документ", 10, models.RoleWriter, true},
		{"чужой writer не видит", 11, models.RoleWriter, false},
		{"назначенный ревьюер видит", 20, models.RoleReviewer, true},
		{"чужой ревьюер не видит", 21, models.RoleReviewer, false},
		{"developer не видит документы на ревью", 30, models.RoleDeveloper, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canRead(doc, tc.userID, tc.role); got != tc.want {
				t.Errorf("canRead=%v, want %v", got, tc.want)
			}
		})
	}
}

func TestCanRead_PublishedVisibleToDeveloper(t *testing.T) {
	doc := newDoc(models.StatusPublished, 10, 20)

	if !canRead(doc, 99, models.RoleDeveloper) {
		t.Error("developer должен видеть опубликованные документы")
	}
	if canRead(doc, 11, models.RoleWriter) {
		t.Error("чужой writer не должен видеть чужой опубликованный документ напрямую")
	}
}

func TestCanWrite(t *testing.T) {
	doc := newDoc(models.StatusInReview, 10, 20)

	cases := []struct {
		name   string
		userID int64
		role   models.UserRole
		want   bool
	}{
		{"admin может писать", 99, models.RoleAdmin, true},
		{"автор может писать в свой документ", 10, models.RoleWriter, true},
		{"чужой writer не может", 11, models.RoleWriter, false},
		{"ревьюер не может писать (только править метаданные)", 20, models.RoleReviewer, false},
		{"developer не может", 30, models.RoleDeveloper, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canWrite(doc, tc.userID, tc.role); got != tc.want {
				t.Errorf("canWrite=%v, want %v", got, tc.want)
			}
		})
	}
}

// canEditMeta шире, чем canWrite: ревьюер тоже может править title/описание,
// потому что часто именно он формулирует точную формулировку при приёмке.
func TestCanEditMeta_ReviewerAllowed(t *testing.T) {
	doc := newDoc(models.StatusInReview, 10, 20)

	cases := []struct {
		name   string
		userID int64
		role   models.UserRole
		want   bool
	}{
		{"admin", 99, models.RoleAdmin, true},
		{"writer-владелец", 10, models.RoleWriter, true},
		{"чужой writer", 11, models.RoleWriter, false},
		{"назначенный ревьюер ДОЛЖЕН мочь править метаданные", 20, models.RoleReviewer, true},
		{"чужой ревьюер не может", 21, models.RoleReviewer, false},
		{"developer никогда", 30, models.RoleDeveloper, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := canEditMeta(doc, tc.userID, tc.role); got != tc.want {
				t.Errorf("canEditMeta=%v, want %v", got, tc.want)
			}
		})
	}
}
