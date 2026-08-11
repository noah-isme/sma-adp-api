package dto

// schedule_dto.go provides alias definitions for scheduler DTOs.

// GenerateScheduleRequest instructs the generator to build a proposal for the class/term.
// ClassIDs and class_ids support multi-class schedule generation.
type GenerateScheduleRequestAlias struct {
	TermID          string               `json:"termId" validate:"required"`
	ClassID         string               `json:"classId,omitempty"`
	ClassIDs        []string             `json:"classIds,omitempty"` // class_ids
	TimeSlotsPerDay int                  `json:"timeSlotsPerDay" validate:"required,min=1,max=16"`
	Days            []int                `json:"days" validate:"required,min=1,dive,min=1,max=7"`
	SubjectLoads    []SubjectLoadRequest `json:"subjectLoads" validate:"required,min=1,dive"`
	HardConstraints []string             `json:"hardConstraints"`
	SoftConstraints []string             `json:"softConstraints"`
	Meta            map[string]any       `json:"meta"`
}

// GenerateScheduleResponse returns the built timetable proposal.
type GenerateScheduleResponseAlias struct {
	ProposalID string                 `json:"proposalId"`
	Score      float64                `json:"score"`
	Slots      []ScheduleSlotProposal `json:"slots"`
	Conflicts  []ProposalConflict     `json:"conflicts"`
	Stats      ScheduleImprovementStats `json:"stats"`
}
