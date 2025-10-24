package service

import (
	"context"
	"database/sql"
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/noah-isme/sma-adp-api/internal/models"
)

type mockUserRepo struct {
	users          map[string]*models.User
	listUsers      []models.User
	listCount      int
	listErr        error
	findByIDErr    error
	findByEmailErr error
	auditLogs      []*models.AuditLog
}

func (m *mockUserRepo) List(ctx context.Context, filter models.UserFilter) ([]models.User, int, error) {
	if m.listErr != nil {
		return nil, 0, m.listErr
	}
	if m.listUsers != nil {
		return m.listUsers, m.listCount, nil
	}
	var users []models.User
	for _, u := range m.users {
		users = append(users, *u)
	}
	return users, len(users), nil
}

func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*models.User, error) {
	if m.findByIDErr != nil {
		return nil, m.findByIDErr
	}
	if user, ok := m.users[id]; ok {
		copy := *user
		return &copy, nil
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	if m.findByEmailErr != nil {
		return nil, m.findByEmailErr
	}
	for _, u := range m.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (m *mockUserRepo) Create(ctx context.Context, user *models.User) error {
	if m.users == nil {
		m.users = make(map[string]*models.User)
	}
	copy := *user
	m.users[user.ID] = &copy
	return nil
}

func (m *mockUserRepo) Update(ctx context.Context, user *models.User) error {
	if m.users == nil {
		m.users = make(map[string]*models.User)
	}
	copy := *user
	m.users[user.ID] = &copy
	return nil
}

func (m *mockUserRepo) Delete(ctx context.Context, id string) error {
	if user, ok := m.users[id]; ok {
		user.Active = false
		m.users[id] = user
		return nil
	}
	return sql.ErrNoRows
}

func (m *mockUserRepo) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	m.auditLogs = append(m.auditLogs, log)
	return nil
}

func TestUserServiceList(t *testing.T) {
	repo := &mockUserRepo{listUsers: []models.User{{ID: "1", Email: "a@example.com"}}, listCount: 1}
	svc := NewUserService(repo, validator.New(), zap.NewNop())
	users, pagination, err := svc.List(context.Background(), models.UserFilter{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Len(t, users, 1)
	assert.Equal(t, 1, pagination.TotalCount)
}

func TestUserServiceCreate(t *testing.T) {
	repo := &mockUserRepo{users: make(map[string]*models.User)}
	repo.findByEmailErr = sql.ErrNoRows
	svc := NewUserService(repo, validator.New(), zap.NewNop())
	user, err := svc.Create(context.Background(), CreateUserRequest{Email: "USER@EXAMPLE.COM", FullName: "User", Password: "secret1", Role: models.RoleAdmin, Active: true}, "actor", models.LoginRequest{})
	require.NoError(t, err)
	assert.Equal(t, "user@example.com", user.Email)
	assert.NotEmpty(t, repo.auditLogs)
}

func TestUserServiceUpdate(t *testing.T) {
	repo := &mockUserRepo{users: map[string]*models.User{"1": {ID: "1", Email: "a@example.com", FullName: "Old", Role: models.RoleTeacher, Active: true}}}
	svc := NewUserService(repo, validator.New(), zap.NewNop())
	active := false
	user, err := svc.Update(context.Background(), "1", UpdateUserRequest{FullName: "New", Role: models.RoleAdmin, Active: &active}, "actor", models.LoginRequest{})
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, user.Role)
	assert.False(t, user.Active)
	assert.NotEmpty(t, repo.auditLogs)
}

func TestUserServiceDelete(t *testing.T) {
	repo := &mockUserRepo{users: map[string]*models.User{"1": {ID: "1", Email: "a@example.com", FullName: "Old", Role: models.RoleTeacher, Active: true}}}
	svc := NewUserService(repo, validator.New(), zap.NewNop())
	err := svc.Delete(context.Background(), "1", "actor", models.LoginRequest{})
	require.NoError(t, err)
	assert.False(t, repo.users["1"].Active)
	assert.NotEmpty(t, repo.auditLogs)
}
