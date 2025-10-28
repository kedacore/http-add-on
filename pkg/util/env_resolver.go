/*
Copyright 2023 The KEDA Authors

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

package util

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

func ResolveOsEnvBool(envName string, defaultValue bool) (bool, error) {
	valueStr, found := os.LookupEnv(envName)

	if found && valueStr != "" {
		return strconv.ParseBool(valueStr)
	}

	return defaultValue, nil
}

func ResolveOsEnvInt(envName string, defaultValue int) (int, error) {
	valueStr, found := os.LookupEnv(envName)

	if found && valueStr != "" {
		return strconv.Atoi(valueStr)
	}

	return defaultValue, nil
}

func ResolveOsEnvDuration(envName string) (*time.Duration, error) {
	valueStr, found := os.LookupEnv(envName)

	if found && valueStr != "" {
		value, err := time.ParseDuration(valueStr)
		return &value, err
	}

	return nil, nil
}

// ValidateLeaderElectionDurations ensures LeaseDuration > RenewDeadline > RetryPeriod
// to prevent multiple active leaders and unnecessary leadership churn.
func ValidateLeaderElectionConfig(leaseDuration, renewDeadline, retryPeriod *time.Duration) error {
	if leaseDuration == nil && renewDeadline == nil && retryPeriod == nil {
		return nil
	}

	// If any are set, validate relationships
	if leaseDuration != nil && *leaseDuration <= 0 {
		return fmt.Errorf("lease duration must be greater than 0, got %v", *leaseDuration)
	}
	if renewDeadline != nil && *renewDeadline <= 0 {
		return fmt.Errorf("renew deadline must be greater than 0, got %v", *renewDeadline)
	}
	if retryPeriod != nil && *retryPeriod <= 0 {
		return fmt.Errorf("retry period must be greater than 0, got %v", *retryPeriod)
	}

	// Validate relationships when multiple values are set
	if leaseDuration != nil && renewDeadline != nil && *leaseDuration <= *renewDeadline {
		return fmt.Errorf("lease duration (%v) must be greater than renew deadline (%v)",
			*leaseDuration, *renewDeadline)
	}
	if renewDeadline != nil && retryPeriod != nil && *renewDeadline <= *retryPeriod {
		return fmt.Errorf("renew deadline (%v) must be greater than retry period (%v)",
			*renewDeadline, *retryPeriod)
	}

	return nil
}
