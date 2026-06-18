package config

import (
	"os"

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