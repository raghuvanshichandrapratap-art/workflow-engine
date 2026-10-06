package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEnv_NoFilePresent(t *testing.T) {
	t.Chdir(t.TempDir())

	err := LoadEnv()

	assert.NoError(t, err, "LoadEnv must not fail when .env is absent, e.g. inside Docker")
}

func TestLoadEnv_FilePresent(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte("TEST_LOAD_ENV_VAR=hello\n"), 0644))

	err := LoadEnv()

	require.NoError(t, err)
	assert.Equal(t, "hello", GetEnv("TEST_LOAD_ENV_VAR"))
}
