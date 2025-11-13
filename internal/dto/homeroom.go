package dto

// HomeroomItem represents a homeroom assignment entry for a class and term.
type HomeroomItem struct {
	ClassID             string  `db:"class_id" json:"classId"`
	ClassName           string  `db:"class_name" json:"className"`
	TermID              string  `db:"term_id" json:"termId"`
	TermName            string  `db:"term_name" json:"termName"`
	HomeroomTeacherID   *string `db:"homeroom_teacher_id" json:"homeroomTeacherId,omitempty"`
	HomeroomTeacherName *string `db:"homeroom_teacher_name" json:"homeroomTeacherName,omitempty"`
}

// HomeroomFilter filters list queries.
type HomeroomFilter struct {
	TermID  string
	ClassID string
}

// SetHomeroomRequest defines payload for creating/updating a homeroom.
type SetHomeroomRequest struct {
	ClassID   string `json:"classId" validate:"required"`
	TermID    string `json:"termId" validate:"required"`
	TeacherID string `json:"teacherId" validate:"required"`
}
