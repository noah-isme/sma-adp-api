package config

// FeatureFlags is the runtime feature snapshot exposed to clients so the admin
// panel can hide navigation for modules this deployment did not enable, instead
// of rendering a page that answers 404.
//
// Keys are stable, dash-free identifiers; the admin panel keys its navigation
// gating off them. Add a field here whenever a new gated module is introduced so
// the contract stays derived from config rather than hand-maintained.
type FeatureFlags struct {
	Analytics        bool `json:"analytics"`
	Dashboard        bool `json:"dashboard"`
	Scheduler        bool `json:"scheduler"`
	Reports          bool `json:"reports"`
	Mutations        bool `json:"mutations"`
	Archives         bool `json:"archives"`
	Documents        bool `json:"documents"`
	Homerooms        bool `json:"homerooms"`
	Configuration    bool `json:"configuration"`
	CalendarAlias    bool `json:"calendarAlias"`
	AttendanceAlias  bool `json:"attendanceAlias"`
	Audit            bool `json:"audit"`
	LessonAttendance bool `json:"lessonAttendance"`
}

// FeatureResponse wraps the flag set with the resolved API prefix so a client
// can discover both what exists and where to call it from one request.
type FeatureResponse struct {
	APIPrefix string       `json:"apiPrefix"`
	Env       string       `json:"env"`
	Features  FeatureFlags `json:"features"`
}

// Features derives the feature snapshot from the loaded configuration.
func (c *Config) Features() FeatureFlags {
	if c == nil {
		return FeatureFlags{}
	}
	return FeatureFlags{
		Analytics: c.Analytics.Enabled,
		Dashboard: c.Dashboard.Enabled,
		Scheduler: c.Scheduler.Enabled,
		Reports:   c.Reports.Enabled,
		Mutations: c.Mutations.Enabled,
		Archives:  c.Archives.Enabled,
		// /documents is an alias over the archive store, so it is reachable
		// exactly when archives are mounted.
		Documents:       c.Archives.Enabled,
		Homerooms:       c.Homerooms.Enabled,
		Configuration:   c.Configuration.Enabled,
		CalendarAlias:   c.Aliases.CalendarEnabled,
		AttendanceAlias: c.Aliases.AttendanceEnabled,
		// Audit reads are always mounted: the trail is written unconditionally,
		// so gating the viewer would only hide data that already exists.
		Audit: true,
		// Lesson attendance rides on the attendance module, so it can only be
		// reachable when that module is mounted.
		LessonAttendance: c.Aliases.AttendanceEnabled,
	}
}

// FeatureResponse builds the full discovery payload.
func (c *Config) FeatureResponse() FeatureResponse {
	if c == nil {
		return FeatureResponse{Features: FeatureFlags{}}
	}
	return FeatureResponse{
		APIPrefix: c.APIPrefix,
		Env:       c.Env,
		Features:  c.Features(),
	}
}
