package repository

import (
	"context"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

// PortalLookup composes repositories to implement PortalUserLookup interface.
type PortalLookup struct {
	studentRepo           *StudentRepository
	parentStudentRepo     *ParentStudentRepository
	portalPreferencesRepo *PortalPreferencesRepository
	deviceTokenRepo       *DeviceTokenRepository
}

// NewPortalLookup creates a new PortalLookup.
func NewPortalLookup(
	studentRepo *StudentRepository,
	parentStudentRepo *ParentStudentRepository,
	portalPreferencesRepo *PortalPreferencesRepository,
	deviceTokenRepo *DeviceTokenRepository,
) *PortalLookup {
	return &PortalLookup{
		studentRepo:           studentRepo,
		parentStudentRepo:     parentStudentRepo,
		portalPreferencesRepo: portalPreferencesRepo,
		deviceTokenRepo:       deviceTokenRepo,
	}
}

// FindStudentByUserID fetches a student by the linked user ID.
func (p *PortalLookup) FindStudentByUserID(ctx context.Context, userID string) (*models.StudentDetail, error) {
	return p.studentRepo.FindByUserID(ctx, userID)
}

// FindStudentByID fetches a student by ID.
func (p *PortalLookup) FindStudentByID(ctx context.Context, studentID string) (*models.StudentDetail, error) {
	return p.studentRepo.FindByID(ctx, studentID)
}

// FindParentLinksByParentID returns all links for a parent.
func (p *PortalLookup) FindParentLinksByParentID(ctx context.Context, parentID string) ([]*models.ParentStudentLink, error) {
	return p.parentStudentRepo.FindByParentID(ctx, parentID)
}

// FindParentStudentLinkByParentAndStudent returns the relationship used to
// authorize a parent request for one specific student.
func (p *PortalLookup) FindParentStudentLinkByParentAndStudent(ctx context.Context, parentID, studentID string) (*models.ParentStudentLink, error) {
	return p.parentStudentRepo.FindByParentAndStudent(ctx, parentID, studentID)
}

// FindPortalPreferences returns preferences for a user.
func (p *PortalLookup) FindPortalPreferences(ctx context.Context, userID string) (*models.PortalPreferences, error) {
	return p.portalPreferencesRepo.FindByUserID(ctx, userID)
}

// FindDeviceTokensByUserID returns all device tokens for a user.
func (p *PortalLookup) FindDeviceTokensByUserID(ctx context.Context, userID string) ([]*models.DeviceToken, error) {
	return p.deviceTokenRepo.FindByUserID(ctx, userID)
}

// FindParentStudentLinkByID returns a parent-student link by ID.
func (p *PortalLookup) FindParentStudentLinkByID(ctx context.Context, id string) (*models.ParentStudentLink, error) {
	return p.parentStudentRepo.FindByID(ctx, id)
}

// CreateParentStudentLink creates a new parent-student link.
func (p *PortalLookup) CreateParentStudentLink(ctx context.Context, link *models.ParentStudentLink) error {
	return p.parentStudentRepo.Create(ctx, link)
}

// UpdateParentStudentLink updates a parent-student link.
func (p *PortalLookup) UpdateParentStudentLink(ctx context.Context, link *models.ParentStudentLink) error {
	return p.parentStudentRepo.Update(ctx, link)
}

// DeleteParentStudentLink deletes a parent-student link.
func (p *PortalLookup) DeleteParentStudentLink(ctx context.Context, id string) error {
	return p.parentStudentRepo.Delete(ctx, id)
}
