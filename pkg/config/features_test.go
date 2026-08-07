package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeaturesReflectsDisabledDeployment(t *testing.T) {
	cfg := &Config{}

	features := cfg.Features()

	assert.False(t, features.Analytics)
	assert.False(t, features.Dashboard)
	assert.False(t, features.Scheduler)
	assert.False(t, features.Reports)
	assert.False(t, features.Mutations)
	assert.False(t, features.Archives)
	assert.False(t, features.Documents)
	assert.False(t, features.Homerooms)
	assert.False(t, features.Configuration)
	assert.False(t, features.CalendarAlias)
	assert.False(t, features.AttendanceAlias)
	assert.False(t, features.LessonAttendance)
	// Audit rows are written unconditionally, so the viewer is always mounted.
	assert.True(t, features.Audit)
}

func TestFeaturesReflectsFullyEnabledDeployment(t *testing.T) {
	cfg := &Config{}
	cfg.Analytics.Enabled = true
	cfg.Dashboard.Enabled = true
	cfg.Scheduler.Enabled = true
	cfg.Reports.Enabled = true
	cfg.Mutations.Enabled = true
	cfg.Archives.Enabled = true
	cfg.Homerooms.Enabled = true
	cfg.Configuration.Enabled = true
	cfg.Aliases.CalendarEnabled = true
	cfg.Aliases.AttendanceEnabled = true

	features := cfg.Features()

	assert.True(t, features.Analytics)
	assert.True(t, features.Dashboard)
	assert.True(t, features.Scheduler)
	assert.True(t, features.Reports)
	assert.True(t, features.Mutations)
	assert.True(t, features.Archives)
	assert.True(t, features.Homerooms)
	assert.True(t, features.Configuration)
	assert.True(t, features.CalendarAlias)
	assert.True(t, features.AttendanceAlias)
	assert.True(t, features.Audit)
}

// /documents is an alias over the archive store, so it must never report
// available when archives are off.
func TestDocumentsTracksArchives(t *testing.T) {
	cfg := &Config{}
	assert.False(t, cfg.Features().Documents)

	cfg.Archives.Enabled = true
	assert.True(t, cfg.Features().Documents)
}

// Lesson attendance is served by the attendance module, so it must track that
// flag rather than being independently advertised.
func TestLessonAttendanceTracksAttendanceAlias(t *testing.T) {
	cfg := &Config{}
	assert.False(t, cfg.Features().LessonAttendance)

	cfg.Aliases.AttendanceEnabled = true
	assert.True(t, cfg.Features().LessonAttendance)
}

func TestFeatureResponseIncludesPrefixAndEnv(t *testing.T) {
	cfg := &Config{Env: EnvProduction, APIPrefix: "/api/v1"}
	cfg.Reports.Enabled = true

	payload := cfg.FeatureResponse()

	assert.Equal(t, "/api/v1", payload.APIPrefix)
	assert.Equal(t, EnvProduction, payload.Env)
	assert.True(t, payload.Features.Reports)
}

func TestFeaturesOnNilConfigIsSafe(t *testing.T) {
	var cfg *Config

	require.NotPanics(t, func() {
		features := cfg.Features()
		assert.False(t, features.Audit)

		payload := cfg.FeatureResponse()
		assert.Empty(t, payload.APIPrefix)
	})
}

func TestFeaturesReflectsAllOnMode(t *testing.T) {
	cfg := &Config{}
	// Simulate ENABLE_ALL_FEATURES=true by enabling all flags
	// Since ENABLE_ALL_FEATURES is handled in Load(), we test the equivalent
	// by manually setting all flags to true
	cfg.Analytics.Enabled = true
	cfg.Dashboard.Enabled = true
	cfg.Scheduler.Enabled = true
	cfg.Reports.Enabled = true
	cfg.Mutations.Enabled = true
	cfg.Archives.Enabled = true
	cfg.Homerooms.Enabled = true
	cfg.Configuration.Enabled = true
	cfg.Aliases.CalendarEnabled = true
	cfg.Aliases.AttendanceEnabled = true

	features := cfg.Features()

	assert.True(t, features.Analytics)
	assert.True(t, features.Dashboard)
	assert.True(t, features.Scheduler)
	assert.True(t, features.Reports)
	assert.True(t, features.Mutations)
	assert.True(t, features.Archives)
	assert.True(t, features.Documents)
	assert.True(t, features.Homerooms)
	assert.True(t, features.Configuration)
	assert.True(t, features.CalendarAlias)
	assert.True(t, features.AttendanceAlias)
	assert.True(t, features.LessonAttendance)
	assert.True(t, features.Audit)
}
