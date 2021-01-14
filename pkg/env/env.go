package env

import (
	"fmt"
	"os"
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
