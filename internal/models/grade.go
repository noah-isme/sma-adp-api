package models

import "time"

// GradeCalculationScheme represents how the final grade is computed.
type GradeCalculationScheme string

const (
	// GradeSchemeWeighted applies component weights to grade values.
	GradeSchemeWeighted GradeCalculationScheme = "WEIGHTED"
	// GradeSchemeAverage computes the average of all grade values.
	GradeSchemeAverage GradeCalculationScheme = "AVERAGE"
)

// GradeComponent describes a reusable grading component.
type GradeComponent struct {
	ID          string    `db:"id" json:"id"`
	Code        string    `db:"code" json:"code"`
	Name        string    `db:"name" json:"name"`
	Description *string   `db:"description" json:"description,omitempty"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// GradeConfig defines calculation configuration for a class+subject+term.
type GradeConfig struct {
	ID                string                 `db:"id" json:"id"`
	ClassID           string                 `db:"class_id" json:"class_id"`
	SubjectID         string                 `db:"subject_id" json:"subject_id"`
	TermID            string                 `db:"term_id" json:"term_id"`
	CalculationScheme GradeCalculationScheme `db:"calculation_scheme" json:"calculation_scheme"`
	Finalized         bool                   `db:"finalized" json:"finalized"`
	CreatedAt         time.Time              `db:"created_at" json:"created_at"`
	UpdatedAt         time.Time              `db:"updated_at" json:"updated_at"`
	Components        []GradeConfigComponent `json:"components,omitempty"`
}

// GradeConfigComponent maps grade components to configurations.
type GradeConfigComponent struct {
	ID            string    `db:"id" json:"id"`
	GradeConfigID string    `db:"grade_config_id" json:"grade_config_id"`
	ComponentID   string    `db:"component_id" json:"component_id"`
	Weight        float64   `db:"weight" json:"weight"`
	ComponentCode string    `db:"component_code" json:"component_code"`
	ComponentName string    `db:"component_name" json:"component_name"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// Grade represents a single grade entry for a component.
type Grade struct {
	ID            string    `db:"id" json:"id"`
	EnrollmentID  string    `db:"enrollment_id" json:"enrollment_id"`
	SubjectID     string    `db:"subject_id" json:"subject_id"`
	ComponentID   string    `db:"component_id" json:"component_id"`
	GradeValue    float64   `db:"grade_value" json:"grade_value"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
	UpdatedAt     time.Time `db:"updated_at" json:"updated_at"`
	ComponentCode string    `db:"component_code" json:"component_code"`
}

// GradeFinal stores the computed final grade for an enrollment + subject.
type GradeFinal struct {
	ID              string    `db:"id" json:"id"`
	EnrollmentID    string    `db:"enrollment_id" json:"enrollment_id"`
	SubjectID       string    `db:"subject_id" json:"subject_id"`
	FinalGrade      float64   `db:"final_grade" json:"final_grade"`
	Finalized       bool      `db:"finalized" json:"finalized"`
	CalculatedAt    time.Time `db:"calculated_at" json:"calculated_at"`
	CalculationNote string    `db:"calculation_note" json:"calculation_note"`
}

// GradeFilter allows querying of grade entries.
type GradeFilter struct {
	EnrollmentID string
	SubjectID    string
	ComponentID  string
}

// FinalGradeFilter scopes recalculation/finalisation queries.
type FinalGradeFilter struct {
	ClassID   string
	SubjectID string
	TermID    string
}

// GradeReportSubject summarises student performance per subject.
type GradeReportSubject struct {
	SubjectID   string   `db:"subject_id" json:"subject_id"`
	SubjectName string   `db:"subject_name" json:"subject_name"`
	FinalGrade  *float64 `db:"final_grade" json:"final_grade,omitempty"`
}

// StudentReportCard contains per-subject grades for a student.
type StudentReportCard struct {
	StudentID string               `json:"student_id"`
	TermID    string               `json:"term_id"`
	Subjects  []GradeReportSubject `json:"subjects"`
}

// ClassGradeDistribution summarises final grade metrics for a class.
type ClassGradeDistribution struct {
	SubjectID string   `db:"subject_id" json:"subject_id"`
	TermID    string   `db:"term_id" json:"term_id"`
	Min       *float64 `db:"min" json:"min,omitempty"`
	Max       *float64 `db:"max" json:"max,omitempty"`
	Average   *float64 `db:"average" json:"average,omitempty"`
}

// ClassGradeReport aggregates class performance.
type ClassGradeReport struct {
	ClassID      string                  `json:"class_id"`
	TermID       string                  `json:"term_id"`
	SubjectID    string                  `json:"subject_id"`
	Distribution *ClassGradeDistribution `json:"distribution,omitempty"`
	Students     []GradeFinalReportRow   `json:"students"`
}

// GradeFinalReportRow represents a student's final grade row for reporting.
type GradeFinalReportRow struct {
	StudentID   string   `db:"student_id" json:"student_id"`
	StudentName string   `db:"student_name" json:"student_name"`
	FinalGrade  *float64 `db:"final_grade" json:"final_grade,omitempty"`
	Rank        *int     `db:"rank" json:"rank,omitempty"`
}
