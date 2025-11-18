/*
Copyright 2025 The KEDA Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/kedacore/http-add-on/pkg/util"
)

func TestLeaderElectionEnvVarsIntegration(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		expectedLease *time.Duration
		expectedRenew *time.Duration
		expectedRetry *time.Duration
		expectError   bool
	}{
		{
			name: "all environment variables set with valid values",
			envVars: map[string]string{
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION": "30s",
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE": "20s",
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD":   "5s",
			},
			expectedLease: durationPtr(30 * time.Second),
			expectedRenew: durationPtr(20 * time.Second),
			expectedRetry: durationPtr(5 * time.Second),
			expectError:   false,
		},
		{
			name:          "no environment variables set - should return nil for defaults",
			envVars:       map[string]string{},
			expectedLease: nil,
			expectedRenew: nil,
			expectedRetry: nil,
			expectError:   false,
		},
		{
			name: "invalid lease duration",
			envVars: map[string]string{
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION": "invalid",
			},
			expectError: true,
		},
		{
			name: "invalid renew deadline",
			envVars: map[string]string{
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE": "not-a-duration",
			},
			expectError: true,
		},
		{
			name: "invalid retry period",
			envVars: map[string]string{
				"KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD": "xyz",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key, value := range tt.envVars {
				t.Setenv(key, value)
			}

			leaseDuration, leaseErr := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION")
			renewDeadline, renewErr := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE")
			retryPeriod, retryErr := util.ResolveOsEnvDuration("KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD")

			if tt.expectError {
				// At least one of the errors should be non-nil
				hasError := false
				if _, ok := tt.envVars["KEDA_HTTP_OPERATOR_LEADER_ELECTION_LEASE_DURATION"]; ok && leaseErr != nil {
					hasError = true
				}
				if _, ok := tt.envVars["KEDA_HTTP_OPERATOR_LEADER_ELECTION_RENEW_DEADLINE"]; ok && renewErr != nil {
					hasError = true
				}
				if _, ok := tt.envVars["KEDA_HTTP_OPERATOR_LEADER_ELECTION_RETRY_PERIOD"]; ok && retryErr != nil {
					hasError = true
				}
				if !hasError {
					t.Errorf("expected error but got none")
				}
			} else {
				// No errors expected
				if leaseErr != nil {
					t.Errorf("unexpected error for lease duration: %v", leaseErr)
				}
				if renewErr != nil {
					t.Errorf("unexpected error for renew deadline: %v", renewErr)
				}
				if retryErr != nil {
					t.Errorf("unexpected error for retry period: %v", retryErr)
				}

				// Verify the parsed values match expectations
				assert.Equal(t, tt.expectedLease, leaseDuration)
				assert.Equal(t, tt.expectedRenew, renewDeadline)
				assert.Equal(t, tt.expectedRetry, retryPeriod)
			}
		})
	}
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}
