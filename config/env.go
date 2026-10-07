package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

func LoadEnv() error {
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		return nil
	}
	return godotenv.Load()
}

func GetEnv(key string) string {
	return os.Getenv(key)
}

func GetEnvInt(key string, fallback int) int {
	v, err := strconv.Atoi(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return v
}