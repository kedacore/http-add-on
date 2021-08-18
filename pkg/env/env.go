package env

import (
	"fmt"
	"os"
	"strconv"
)

// GetOr gets the value of the environment variable called envName. If that variable
// is not set, returns otherwise
func GetOr(envName, otherwise string) string {
	fromEnv, err := Get(envName)
	if err != nil {
		return otherwise
	}
	return fromEnv
}

// Get gets the value of the environment variable called envName. If that variable
// is not set, returns a non-nil error
func Get(envName string) (string, error) {
	fromEnv := os.Getenv(envName)
	if fromEnv == "" {
		return "", fmt.Errorf("Environnment variable %s not found", envName)
	}
	return fromEnv, nil
}

// GetInt32Or returns the int32 value of the environment variable called envName.
// If the environment variable is missing or it's not a valid int32, returns otherwise
func GetInt32Or(envName string, otherwise int32) int32 {
	strVal, err := Get(envName)
	if err != nil {
		return otherwise
	}
	val, err := strconv.ParseInt(strVal, 10, 32)
	if err != nil {
		return otherwise
	}
	return int32(val)
}

func GetIntOr(envName string, otherwise int) int {
	strVal, err := Get(envName)
	if err != nil {
		return otherwise
	}
	val, err := strconv.Atoi(strVal)
	if err != nil {
		return otherwise
	}
	return val
}
