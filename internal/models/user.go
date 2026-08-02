package models

import "time"

// UserRole represents the available roles for the RBAC system.
type UserRole string

const (
	RoleSuperAdmin    UserRole = "SUPERADMIN"
	RoleAdminTU       UserRole = "ADMIN_TU"
	RoleWaliKelas     UserRole = "WALI_KELAS"
	RoleGuruMapel     UserRole = "GURU_MAPEL"
	RoleKepalaSekolah UserRole = "KEPALA_SEKOLAH"
	RoleSiswa         UserRole = "SISWA"
	RoleOrtu          UserRole = "ORTU"
	RoleAdmin         UserRole = RoleAdminTU
	RoleTeacher       UserRole = RoleGuruMapel
	RoleStudent       UserRole = RoleSiswa
)

// User represents an application user stored in the users table.
type User struct {
	ID           string     `db:"id" json:"id"`
	Email        string     `db:"email" json:"email"`
	PasswordHash string     `db:"password_hash" json:"-"`
	FullName     string     `db:"full_name" json:"full_name"`
	Role         UserRole   `db:"role" json:"role"`
	TeacherID    *string    `db:"teacher_id" json:"teacher_id,omitempty"`
	StudentID    *string    `db:"student_id" json:"student_id,omitempty"`
	ClassID      *string    `db:"class_id" json:"class_id,omitempty"`
	Active       bool       `db:"active" json:"active"`
	LastLogin    *time.Time `db:"last_login" json:"last_login,omitempty"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

// UserFilter captures filtering criteria for listing users.
type UserFilter struct {
	Role      *UserRole
	Active    *bool
	Search    string
	Page      int
	PageSize  int
	SortBy    string
	SortOrder string
}

// Pagination contains pagination metadata returned in list responses.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	TotalCount int `json:"total_count"`
}
