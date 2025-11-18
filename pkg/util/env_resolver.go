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

// Controller-runtime default values for leader election
// Reference: https://github.com/kubernetes-sigs/controller-runtime/blob/main/pkg/manager/manager.go
const (
	defaultLeaseDuration = 15 * time.Second
	defaultRenewDeadline = 10 * time.Second
	defaultRetryPeriod   = 2 * time.Second
)

// ValidateLeaderElectionConfig ensures LeaseDuration > RenewDeadline > RetryPeriod
// to prevent multiple active leaders and unnecessary leadership churn.
// This validation checks against the actual runtime values (user-provided or defaults)
// to catch invalid partial configurations.
func ValidateLeaderElectionConfig(leaseDuration, renewDeadline, retryPeriod *time.Duration) error {
	// Resolve actual values that will be used at runtime (user-provided or defaults)
	lease := defaultLeaseDuration
	if leaseDuration != nil {
		lease = *leaseDuration
	}

	renew := defaultRenewDeadline
	if renewDeadline != nil {
		renew = *renewDeadline
	}

	retry := defaultRetryPeriod
	if retryPeriod != nil {
		retry = *retryPeriod
	}

	// Validate all values are positive
	if lease <= 0 {
		return fmt.Errorf("lease duration must be greater than 0, got %v", lease)
	}
	if renew <= 0 {
		return fmt.Errorf("renew deadline must be greater than 0, got %v", renew)
	}
	if retry <= 0 {
		return fmt.Errorf("retry period must be greater than 0, got %v", retry)
	}

	// Validate relationships: LeaseDuration > RenewDeadline > RetryPeriod
	if lease <= renew {
		return fmt.Errorf("lease duration (%v) must be greater than renew deadline (%v)", lease, renew)
	}
	if renew <= retry {
		return fmt.Errorf("renew deadline (%v) must be greater than retry period (%v)", renew, retry)
	}
	if lease <= retry {
		return fmt.Errorf("lease duration (%v) must be greater than retry period (%v)", lease, retry)
	}

	return nil
}
