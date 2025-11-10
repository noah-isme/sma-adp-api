package dto

// SubjectLoadRequest captures weekly demand for a subject-teacher pair.
type SubjectLoadRequest struct {
	SubjectID   string   `json:"subjectId" validate:"required"`
	TeacherID   string   `json:"teacherId" validate:"required"`
	WeeklyCount int      `json:"weeklyCount" validate:"required,min=1"`
	Difficulty  int      `json:"difficulty" validate:"omitempty,min=1,max=10"`
	Preferred   []int    `json:"preferredSlots" validate:"omitempty,dive,min=0"`
	Tags        []string `json:"tags"`
}

// GenerateScheduleRequest instructs the generator to build a proposal for the class/term.
type GenerateScheduleRequest struct {
	TermID          string               `json:"termId" validate:"required"`
	ClassID         string               `json:"classId" validate:"required"`
	TimeSlotsPerDay int                  `json:"timeSlotsPerDay" validate:"required,min=1,max=16"`
	Days            []int                `json:"days" validate:"required,min=1,dive,min=1,max=7"`
	SubjectLoads    []SubjectLoadRequest `json:"subjectLoads" validate:"required,min=1,dive"`
	HardConstraints []string             `json:"hardConstraints"`
	SoftConstraints []string             `json:"softConstraints"`
	Meta            map[string]any       `json:"meta"`
}

// ScheduleSlotProposal represents a generated slot.
type ScheduleSlotProposal struct {
	DayOfWeek int     `json:"dayOfWeek"`
	TimeSlot  int     `json:"timeSlot"`
	SubjectID string  `json:"subjectId"`
	TeacherID string  `json:"teacherId"`
	Room      *string `json:"room,omitempty"`
}

// ProposalConflict captures unmet demand or hard constraint violations.
type ProposalConflict struct {
	Type    string                `json:"type"`
	Message string                `json:"message"`
	Slot    *ScheduleSlotProposal `json:"slot,omitempty"`
	Meta    map[string]any        `json:"meta,omitempty"`
}

// ScheduleImprovementStats summarises repair iterations.
type ScheduleImprovementStats struct {
	Iterations  int     `json:"iterations"`
	GapPenalty  float64 `json:"gapPenalty"`
	LoadPenalty float64 `json:"loadPenalty"`
}

// GenerateScheduleResponse returns the built timetable proposal.
type GenerateScheduleResponse struct {
	ProposalID string                   `json:"proposalId"`
	Score      float64                  `json:"score"`
	Slots      []ScheduleSlotProposal   `json:"slots"`
	Conflicts  []ProposalConflict       `json:"conflicts"`
	Stats      ScheduleImprovementStats `json:"stats"`
}

// SaveScheduleRequest persists a proposal into semester schedules.
type SaveScheduleRequest struct {
	ProposalID    string `json:"proposalId" validate:"required"`
	CommitToDaily bool   `json:"commitToDaily"`
}

// SemesterScheduleQuery filters schedule summaries by class and term.
type SemesterScheduleQuery struct {
	TermID  string `form:"termId" json:"termId"`
	ClassID string `form:"classId" json:"classId"`
}
