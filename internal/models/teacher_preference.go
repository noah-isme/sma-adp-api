package models

import (
	"time"

	"github.com/jmoiron/sqlx/types"
)

// TeacherUnavailableSlot describes a blocked teaching window.
type TeacherUnavailableSlot struct {
	DayOfWeek string `json:"day_of_week"`
	TimeRange string `json:"time_range"`
}

// TeacherPreference stores capacity and availability rules for a teacher.
type TeacherPreference struct {
	ID             string         `db:"id" json:"id"`
	TeacherID      string         `db:"teacher_id" json:"teacher_id"`
	MaxLoadPerDay  int            `db:"max_load_per_day" json:"max_load_per_day"`
	MaxLoadPerWeek int            `db:"max_load_per_week" json:"max_load_per_week"`
	Unavailable    types.JSONText `db:"unavailable" json:"unavailable"`
	CreatedAt      time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at" json:"updated_at"`
}
