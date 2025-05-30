package tuiexporter

import (
	"context"
	"fmt"
	"time"

	"github.com/ymtdzzz/otel-tui/internal/mcpserver"
	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/telemetry"
	"github.com/ymtdzzz/otel-tui/tuiexporter/internal/tui"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type tuiExporter struct {
	app *tui.TUIApp
	mcp *mcpserver.Server
}

func newTuiExporter(config *Config) *tuiExporter {
	var initialInterval time.Duration
	if config.FromJSONFile {
		// FIXME: When reading telemetry from a JSON file on startup, the UI will break
		//        if it runs at the same time as the UI drawing. As a workaround, wait for a second.
		initialInterval = 1 * time.Second
	}
	app := tui.NewTUIApp(telemetry.NewStore(), initialInterval)
	var srv *mcpserver.Server
	if config.MCPAddr != "" {
		srv = mcpserver.NewServer(config.MCPAddr, app.Store())
	}
	return &tuiExporter{
		app: app,
		mcp: srv,
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
func (e *tuiExporter) Start(_ context.Context, _ component.Host) error {
	go func() {
		if e.mcp != nil {
			_ = e.mcp.Start()
		}
		err := e.app.Run()
		if err != nil {
			fmt.Printf("error running tui app: %s\n", err)
		}
	}()
	return nil
}

// Shutdown stops the TUI exporter
func (e *tuiExporter) Shutdown(_ context.Context) error {
	if e.mcp != nil {
		_ = e.mcp.Stop(context.Background())
	}
	return e.app.Stop()
}
