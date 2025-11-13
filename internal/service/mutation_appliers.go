package service

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
	appErrors "github.com/noah-isme/sma-adp-api/pkg/errors"
)

type studentMutationRepository interface {
	FindByID(ctx context.Context, id string) (*models.StudentDetail, error)
	ExistsByNIS(ctx context.Context, nis string, excludeID string) (bool, error)
	Update(ctx context.Context, student *models.Student) error
}

// StudentMutationApplier mutates student entities based on mutation payloads.
type StudentMutationApplier struct {
	repo   studentMutationRepository
	logger *zap.Logger
}

// NewStudentMutationApplier constructs an applier backed by the student repository.
func NewStudentMutationApplier(repo studentMutationRepository, logger *zap.Logger) *StudentMutationApplier {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &StudentMutationApplier{repo: repo, logger: logger}
}

// Apply updates student fields and returns the refreshed snapshot.
func (a *StudentMutationApplier) Apply(ctx context.Context, mutation *models.Mutation) ([]byte, error) {
	if a.repo == nil {
		return nil, appErrors.Clone(appErrors.ErrInternal, "student repository not configured")
	}
	detail, err := a.repo.FindByID(ctx, mutation.EntityID)
	if err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to load student")
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(mutation.RequestedChanges, &payload); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "invalid student mutation payload")
	}
	student := detail.Student
	changes := 0

	if str, ok, err := readString(payload, "full_name", "fullName"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "fullName must be a string")
	} else if ok {
		student.FullName = *str
		changes++
	}
	if str, ok, err := readString(payload, "address"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "address must be a string")
	} else if ok {
		student.Address = *str
		changes++
	}
	if str, ok, err := readString(payload, "phone"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "phone must be a string")
	} else if ok {
		student.Phone = *str
		changes++
	}
	if str, ok, err := readString(payload, "gender"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "gender must be a string")
	} else if ok {
		student.Gender = strings.ToUpper(*str)
		changes++
	}
	if str, ok, err := readString(payload, "nis"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "nis must be a string")
	} else if ok {
		if *str != student.NIS {
			exists, err := a.repo.ExistsByNIS(ctx, *str, student.ID)
			if err != nil {
				return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to validate NIS")
			}
			if exists {
				return nil, appErrors.Clone(appErrors.ErrConflict, "nis already used")
			}
			student.NIS = *str
		}
		changes++
	}
	if str, ok, err := readString(payload, "birth_date", "birthDate"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "birthDate must be a string")
	} else if ok {
		ts, err := time.Parse("2006-01-02", *str)
		if err != nil {
			return nil, appErrors.Clone(appErrors.ErrValidation, "birthDate must be YYYY-MM-DD")
		}
		student.BirthDate = ts
		changes++
	}
	if val, ok, err := readBool(payload, "active"); err != nil {
		return nil, appErrors.Clone(appErrors.ErrValidation, "active must be a boolean")
	} else if ok {
		student.Active = val
		changes++
	}

	if changes == 0 {
		return nil, appErrors.Clone(appErrors.ErrValidation, "no supported student fields provided")
	}
	if err := a.repo.Update(ctx, &student); err != nil {
		return nil, appErrors.Wrap(err, appErrors.ErrInternal.Code, appErrors.ErrInternal.Status, "failed to update student")
	}
	snapshot, err := json.Marshal(student)
	if err != nil {
		a.logger.Warn("failed to marshal student snapshot", zap.Error(err))
		return []byte("{}"), nil
	}
	return snapshot, nil
}

func readString(payload map[string]json.RawMessage, keys ...string) (*string, bool, error) {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			var val string
			if err := json.Unmarshal(raw, &val); err != nil {
				return nil, false, err
			}
			val = strings.TrimSpace(val)
			return &val, true, nil
		}
	}
	return nil, false, nil
}

func readBool(payload map[string]json.RawMessage, keys ...string) (bool, bool, error) {
	for _, key := range keys {
		if raw, ok := payload[key]; ok {
			var val bool
			if err := json.Unmarshal(raw, &val); err != nil {
				return false, false, err
			}
			return val, true, nil
		}
	}
	return false, false, nil
}
