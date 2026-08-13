package service

import (
	"context"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

// analyticsAssignmentAccess is the smallest assignment repository contract
// required to enforce teacher object scope for analytics.
type analyticsAssignmentAccess interface {
	HasClassAccess(ctx context.Context, teacherID, classID, termID string) (bool, error)
	ListByTeacher(ctx context.Context, teacherID string) ([]models.TeacherAssignmentDetail, error)
}

// analyticsEnrollmentAccess is the read-only contract used to resolve a
// student's active class before checking the teacher assignment scope.
type analyticsEnrollmentAccess interface {
	ListByStudentAndTerm(ctx context.Context, studentID, termID string) ([]models.EnrollmentDetail, error)
}

type analyticsSubjectTupleAccess interface {
	Exists(ctx context.Context, teacherID, classID, subjectID, termID string) (bool, error)
}

// DatabaseAnalyticsAuthorizer applies object-level analytics access using the
// same teacher assignments and enrollments used by the rest of the API.
// Administrators retain tenant-wide access; teachers are limited to assigned
// classes/subjects and students may only read their own record.
type DatabaseAnalyticsAuthorizer struct {
	assignments analyticsAssignmentAccess
	enrollments analyticsEnrollmentAccess
}

// NewDatabaseAnalyticsAuthorizer constructs an object authorizer. A nil
// dependency fails closed for teacher-scoped requests rather than widening
// access accidentally.
func NewDatabaseAnalyticsAuthorizer(assignments analyticsAssignmentAccess, enrollments analyticsEnrollmentAccess) *DatabaseAnalyticsAuthorizer {
	return &DatabaseAnalyticsAuthorizer{assignments: assignments, enrollments: enrollments}
}

// AuthorizeAnalytics enforces resource-level access after route RBAC has
// established that the caller has an analytics-capable role.
func (a *DatabaseAnalyticsAuthorizer) AuthorizeAnalytics(ctx context.Context, claims *models.JWTClaims, resourceType, resourceID, termID string) error {
	if claims == nil {
		return appErrors.ErrUnauthorized
	}

	switch claims.Role {
	case models.RoleSuperAdmin, models.RoleAdmin:
		return nil
	case models.RoleStudent:
		if resourceType == "student" && claims.StudentID != "" && claims.StudentID == resourceID {
			return nil
		}
		return appErrors.ErrForbidden
	case models.RoleTeacher:
		if claims.TeacherID == "" || a.assignments == nil {
			return appErrors.ErrForbidden
		}
		switch resourceType {
		case "class":
			allowed, err := a.assignments.HasClassAccess(ctx, claims.TeacherID, resourceID, termID)
			if err != nil {
				return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify teacher class access")
			}
			if allowed {
				return nil
			}
			return appErrors.ErrForbidden
		case "subject":
			assignments, err := a.assignments.ListByTeacher(ctx, claims.TeacherID)
			if err != nil {
				return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify teacher subject access")
			}
			for _, assignment := range assignments {
				if assignment.SubjectID == resourceID && assignment.TermID == termID {
					return nil
				}
			}
			return appErrors.ErrForbidden
		case "student":
			if a.enrollments == nil {
				return appErrors.ErrForbidden
			}
			enrollments, err := a.enrollments.ListByStudentAndTerm(ctx, resourceID, termID)
			if err != nil {
				return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify student class access")
			}
			for _, enrollment := range enrollments {
				if enrollment.TermID != termID || enrollment.ClassID == "" {
					continue
				}
				allowed, err := a.assignments.HasClassAccess(ctx, claims.TeacherID, enrollment.ClassID, termID)
				if err != nil {
					return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify student class access")
				}
				if allowed {
					return nil
				}
			}
			return appErrors.ErrForbidden
		default:
			return appErrors.ErrForbidden
		}
	default:
		return appErrors.ErrForbidden
	}
}

// AuthorizeAnalyticsSubject checks the complete subject scope. A teacher who
// teaches a subject in one class must not gain access to that subject's data
// in a different class merely because both records share a term.
func (a *DatabaseAnalyticsAuthorizer) AuthorizeAnalyticsSubject(ctx context.Context, claims *models.JWTClaims, subjectID, classID, termID string) error {
	if claims == nil {
		return appErrors.ErrUnauthorized
	}
	if claims.Role == models.RoleSuperAdmin || claims.Role == models.RoleAdmin {
		return nil
	}
	if claims.Role != models.RoleTeacher || claims.TeacherID == "" || a.assignments == nil {
		return appErrors.ErrForbidden
	}
	if classID == "" {
		return a.AuthorizeAnalytics(ctx, claims, "subject", subjectID, termID)
	}
	if tuple, ok := a.assignments.(analyticsSubjectTupleAccess); ok {
		allowed, err := tuple.Exists(ctx, claims.TeacherID, classID, subjectID, termID)
		if err != nil {
			return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify teacher subject access")
		}
		if allowed {
			return nil
		}
		return appErrors.ErrForbidden
	}
	assignments, err := a.assignments.ListByTeacher(ctx, claims.TeacherID)
	if err != nil {
		return appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to verify teacher subject access")
	}
	for _, assignment := range assignments {
		if assignment.ClassID == classID && assignment.SubjectID == subjectID && assignment.TermID == termID {
			return nil
		}
	}
	return appErrors.ErrForbidden
}
