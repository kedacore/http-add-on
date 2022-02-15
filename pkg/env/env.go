package env

import (
	"fmt"
	"os"
)

// Get gets the value of the environment variable called envName. If that variable
// is not set, returns a non-nil error
func Get(envName string) (string, error) {
	fromEnv := os.Getenv(envName)
	if fromEnv == "" {
		return "", fmt.Errorf(
			"environnment variable %s not found",
			envName,
		)
	}
	return fromEnv, nil
}
