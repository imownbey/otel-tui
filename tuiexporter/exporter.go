package tuiexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/mcp"
	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/telemetry"
	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/tui"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tuiExporter struct {
	app    *tui.TUIApp
	server *mcp.Server
}

func newTuiExporter(config *Config) *tuiExporter {
	var initialInterval time.Duration
	if config.FromJSONFile {
		// FIXME: When reading telemetry from a JSON file on startup, the UI will break
		//        if it runs at the same time as the UI drawing. As a workaround, wait for a second.
		initialInterval = 1 * time.Second
	}
	store := telemetry.NewStore()
	srv := mcp.New(fmt.Sprintf(":%d", config.MCPPort), store)
	return &tuiExporter{
		app:    tui.NewTUIApp(store, initialInterval),
		server: srv,
	}
}

func (e *tuiExporter) pushTraces(_ context.Context, traces ptrace.Traces) error {
	e.app.Store().AddSpan(&traces)

	return nil
}

func (e *tuiExporter) pushMetrics(_ context.Context, metrics pmetric.Metrics) error {
	e.app.Store().AddMetric(&metrics)

	return nil
}

func (e *tuiExporter) pushLogs(_ context.Context, logs plog.Logs) error {
	e.app.Store().AddLog(&logs)

	return nil
}

// Start runs the TUI exporter
func (e *tuiExporter) Start(ctx context.Context, _ component.Host) error {
	if err := e.server.Start(); err != nil {
		return err
	}
	go func() {
		err := e.app.Run()
		if err != nil {
			fmt.Printf("error running tui app: %s\n", err)
		}
	}()
	return nil
}

// Shutdown stops the TUI exporter
func (e *tuiExporter) Shutdown(ctx context.Context) error {
	_ = e.server.Shutdown(ctx)
	return e.app.Stop()
}
