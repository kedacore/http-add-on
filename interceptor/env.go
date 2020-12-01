package main

import (
	"fmt"
	"os"
)

func envOr(envName, otherwise string) string {
	fromEnv, err := env(envName)
	if err != nil {
		return otherwise
	}
	return fromEnv
}

func env(envName string) (string, error) {
	fromEnv := os.Getenv(envName)
	if fromEnv == "" {
		return "", fmt.Errorf("Environnment variable %s not found", envName)
	}
	return fromEnv, nil
}
