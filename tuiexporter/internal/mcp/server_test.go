package mcp

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/telemetry"
)

func TestServerStartShutdown(t *testing.T) {
	store := telemetry.NewStore()
	srv := New(":0", store)
	err := srv.Start()
	assert.NoError(t, err)

	resp, err := http.Get("http://" + srv.Address() + "/healthz")
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()

	err = srv.Shutdown(context.Background())
	assert.NoError(t, err)
}
