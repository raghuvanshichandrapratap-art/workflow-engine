package services

import (
	"net/http"
	"testing"
	"time"

	"github.com/openai/openai-go/v3"
	"github.com/stretchr/testify/assert"
)

func TestRateLimitBackoff_HonorsRetryAfterHeader(t *testing.T) {
	apiErr := &openai.Error{
		Response: &http.Response{
			Header: http.Header{"Retry-After": []string{"3"}},
		},
	}

	got := rateLimitBackoff(apiErr, 0)

	assert.Equal(t, 3*time.Second, got)
}

func TestRateLimitBackoff_FallsBackToExponential(t *testing.T) {
	apiErr := &openai.Error{
		Response: &http.Response{Header: http.Header{}},
	}

	assert.Equal(t, 1*time.Second, rateLimitBackoff(apiErr, 0))
	assert.Equal(t, 2*time.Second, rateLimitBackoff(apiErr, 1))
	assert.Equal(t, 4*time.Second, rateLimitBackoff(apiErr, 2))
}

func TestRateLimitBackoff_NoResponse(t *testing.T) {
	apiErr := &openai.Error{}

	assert.Equal(t, 1*time.Second, rateLimitBackoff(apiErr, 0))
}
