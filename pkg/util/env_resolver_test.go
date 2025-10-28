package util

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestResolveMissingOsEnvBool(t *testing.T) {
	actual, err := ResolveOsEnvBool("missing_bool", true)
	assert.True(t, actual)
	assert.Nil(t, err)

	t.Setenv("empty_bool", "")
	actual, err = ResolveOsEnvBool("empty_bool", true)
	assert.True(t, actual)
	assert.Nil(t, err)
}

func TestResolveInvalidOsEnvBool(t *testing.T) {
	t.Setenv("blank_bool", "    ")
	actual, err := ResolveOsEnvBool("blank_bool", true)
	assert.False(t, actual)
	assert.NotNil(t, err)

	t.Setenv("invalid_bool", "deux heures")
	actual, err = ResolveOsEnvBool("invalid_bool", true)
	assert.False(t, actual)
	assert.NotNil(t, err)
}

func TestResolveValidOsEnvBool(t *testing.T) {
	t.Setenv("valid_bool", "true")
	actual, err := ResolveOsEnvBool("valid_bool", false)
	assert.True(t, actual)
	assert.Nil(t, err)

	t.Setenv("valid_bool", "false")
	actual, err = ResolveOsEnvBool("valid_bool", true)
	assert.False(t, actual)
	assert.Nil(t, err)
}

func TestResolveMissingOsEnvInt(t *testing.T) {
	actual, err := ResolveOsEnvInt("missing_int", 1)
	assert.Equal(t, 1, actual)
	assert.Nil(t, err)

	t.Setenv("empty_int", "")
	actual, err = ResolveOsEnvInt("empty_int", 1)
	assert.Equal(t, 1, actual)
	assert.Nil(t, err)
}

func TestResolveInvalidOsEnvInt(t *testing.T) {
	t.Setenv("blank_int", "    ")
	actual, err := ResolveOsEnvInt("blank_int", 1)
	assert.Equal(t, 0, actual)
	assert.NotNil(t, err)

	t.Setenv("invalid_int", "deux heures")
	actual, err = ResolveOsEnvInt("invalid_int", 1)
	assert.Equal(t, 0, actual)
	assert.NotNil(t, err)
}

func TestResolveValidOsEnvInt(t *testing.T) {
	t.Setenv("valid_int", "2")
	actual, err := ResolveOsEnvInt("valid_int", 1)
	assert.Equal(t, 2, actual)
	assert.Nil(t, err)
}

func TestResolveMissingOsEnvDuration(t *testing.T) {
	actual, err := ResolveOsEnvDuration("missing_duration")
	assert.Nil(t, actual)
	assert.Nil(t, err)

	t.Setenv("empty_duration", "")
	actual, err = ResolveOsEnvDuration("empty_duration")
	assert.Nil(t, actual)
	assert.Nil(t, err)
}

func TestResolveInvalidOsEnvDuration(t *testing.T) {
	t.Setenv("blank_duration", "    ")
	actual, err := ResolveOsEnvDuration("blank_duration")
	assert.Equal(t, time.Duration(0), *actual)
	assert.NotNil(t, err)

	t.Setenv("invalid_duration", "deux heures")
	actual, err = ResolveOsEnvDuration("invalid_duration")
	assert.Equal(t, time.Duration(0), *actual)
	assert.NotNil(t, err)
}

func TestResolveValidOsEnvDuration(t *testing.T) {
	t.Setenv("valid_duration_seconds", "8s")
	actual, err := ResolveOsEnvDuration("valid_duration_seconds")
	assert.Equal(t, time.Duration(8)*time.Second, *actual)
	assert.Nil(t, err)

	t.Setenv("valid_duration_minutes", "30m")
	actual, err = ResolveOsEnvDuration("valid_duration_minutes")
	assert.Equal(t, time.Duration(30)*time.Minute, *actual)
	assert.Nil(t, err)
}

func TestValidateLeaderElectionDurations(t *testing.T) {
	tests := []struct {
		name          string
		leaseDuration *time.Duration
		renewDeadline *time.Duration
		retryPeriod   *time.Duration
		wantErr       bool
		errContains   string
	}{
		{
			name:          "all nil is valid (uses defaults)",
			leaseDuration: nil,
			renewDeadline: nil,
			retryPeriod:   nil,
			wantErr:       false,
		},
		{
			name:          "valid configuration",
			leaseDuration: ptr(15 * time.Second),
			renewDeadline: ptr(10 * time.Second),
			retryPeriod:   ptr(2 * time.Second),
			wantErr:       false,
		},
		{
			name:          "lease duration <= 0",
			leaseDuration: ptr(0 * time.Second),
			renewDeadline: ptr(10 * time.Second),
			retryPeriod:   ptr(2 * time.Second),
			wantErr:       true,
			errContains:   "lease duration must be greater than 0",
		},
		{
			name:          "renew deadline <= 0",
			leaseDuration: ptr(15 * time.Second),
			renewDeadline: ptr(0 * time.Second),
			retryPeriod:   ptr(2 * time.Second),
			wantErr:       true,
			errContains:   "renew deadline must be greater than 0",
		},
		{
			name:          "retry period <= 0",
			leaseDuration: ptr(15 * time.Second),
			renewDeadline: ptr(10 * time.Second),
			retryPeriod:   ptr(0 * time.Second),
			wantErr:       true,
			errContains:   "retry period must be greater than 0",
		},
		{
			name:          "lease duration <= renew deadline",
			leaseDuration: ptr(10 * time.Second),
			renewDeadline: ptr(10 * time.Second),
			retryPeriod:   ptr(2 * time.Second),
			wantErr:       true,
			errContains:   "lease duration",
		},
		{
			name:          "renew deadline <= retry period",
			leaseDuration: ptr(15 * time.Second),
			renewDeadline: ptr(2 * time.Second),
			retryPeriod:   ptr(2 * time.Second),
			wantErr:       true,
			errContains:   "renew deadline",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLeaderElectionConfig(tt.leaseDuration, tt.renewDeadline, tt.retryPeriod)
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func ptr(d time.Duration) *time.Duration {
	return &d
}
