package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"otel-tui/internal/demo"
	"otel-tui/internal/receiver"
	"otel-tui/internal/store"
	"otel-tui/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func printUsage() {
	fmt.Printf(`otel-tui — Zero-Setup Terminal OpenTelemetry Viewer

USAGE:
  otel-tui [flags]
  otel-tui demo [flags]

COMMANDS:
  demo                      Send synthetic multi-service sample traces to test the UI

FLAGS:
  --grpc-port <int>         gRPC ingestion port (default: 4317)
  --http-port <int>         HTTP ingestion port (default: 4318)
  --max-traces <int>        Max traces held in memory ring buffer (default: 1000)
  --headless                Disable interactive TUI and stream trace logs to stdout
  --json                    In headless mode, output structured JSON lines instead of colorized text
  --replay <path.json>      Load an exported JSON trace file for offline inspection
  --forward-to <url>        Proxy/relay incoming traces to an upstream collector (e.g. http://localhost:4319)
  --auto-fallback           Try fallback ports if default ports are busy (default: true)
  -h, --help                Show help information

EXAMPLES:
  # Start local collector & launch interactive TUI
  otel-tui

  # Send sample demo traces to localhost:4318
  otel-tui demo

  # Forward traces upstream while inspecting locally
  otel-tui --forward-to http://staging-collector:4318

  # Stream incoming traces to stdout (ideal for CI or logging)
  otel-tui --headless

  # Inspect a previously exported trace offline
  otel-tui --replay trace_3a4b5c6d.json
`)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		runDemo(os.Args[2:])
		return
	}

	fs := flag.NewFlagSet("otel-tui", flag.ExitOnError)
	grpcPort := fs.Int("grpc-port", 4317, "gRPC intake port")
	httpPort := fs.Int("http-port", 4318, "HTTP intake port")
	maxTraces := fs.Int("max-traces", 1000, "Maximum number of traces in bounded buffer")
	headless := fs.Bool("headless", false, "Disable TUI, stream formatted trace summaries to stdout")
	jsonOutput := fs.Bool("json", false, "In headless mode, output structured JSON lines instead of colorized text")
	replayFile := fs.String("replay", "", "Load exported OTLP trace file into the TUI for inspection")
	forwardTo := fs.String("forward-to", "", "Proxy/forward incoming traces & logs to an upstream collector (e.g. http://localhost:4319)")
	autoFallback := fs.Bool("auto-fallback", true, "Automatically try alternative ports if default ports are busy")
	help := fs.Bool("help", false, "Show help")
	fs.BoolVar(help, "h", false, "Show help")

	_ = fs.Parse(os.Args[1:])

	if *help {
		printUsage()
		return
	}

	st := store.NewStore(*maxTraces)

	// If replay file is provided, load it
	if *replayFile != "" {
		data, err := os.ReadFile(*replayFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading replay file %s: %v\n", *replayFile, err)
			os.Exit(1)
		}
		if err := st.ImportTraceJSON(data); err != nil {
			fmt.Fprintf(os.Stderr, "Error parsing trace JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Loaded %d traces from %s\n", st.Count(), *replayFile)
	}

	// Start OTLP Receiver
	rec := receiver.NewReceiver(st, receiver.Config{
		GRPCPort:         *grpcPort,
		HTTPPort:         *httpPort,
		ForwardTo:        *forwardTo,
		AutoPortFallback: *autoFallback,
	})

	if err := rec.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error starting receiver: %v\n", err)
		os.Exit(1)
	}
	defer rec.Stop()

	if *headless {
		runHeadless(st, rec.GRPCPort(), rec.HTTPPort(), *jsonOutput)
		return
	}

	// Launch interactive Bubbletea TUI
	app := tui.NewAppModel(st, rec.GRPCPort(), rec.HTTPPort())
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func runHeadless(st *store.Store, grpcPort, httpPort int, jsonOutput bool) {
	if !jsonOutput {
		fmt.Printf("[otel-tui] Headless mode started.\n")
		fmt.Printf("[otel-tui] Listening on gRPC :%d and HTTP :%d\n", grpcPort, httpPort)
		fmt.Printf("[otel-tui] Streaming trace summaries to stdout (Ctrl+C to stop)...\n\n")
	}

	sub := st.Subscribe(500)
	defer st.Unsubscribe(sub)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	pending := make(map[string]*store.Trace)
	lastSeen := make(map[string]time.Time)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()

	flushTrace := func(tr *store.Trace) {
		if jsonOutput {
			data, err := json.Marshal(tr)
			if err == nil {
				fmt.Println(string(data))
			}
		} else {
			printHeadlessTrace(tr)
		}
	}

	for {
		select {
		case <-sigChan:
			for id, tr := range pending {
				flushTrace(tr)
				delete(pending, id)
				delete(lastSeen, id)
			}
			if !jsonOutput {
				fmt.Println("\n[otel-tui] Shutting down headless stream.")
			}
			return
		case tr, ok := <-sub:
			if !ok {
				return
			}
			pending[tr.TraceID] = tr
			lastSeen[tr.TraceID] = time.Now()
		case <-ticker.C:
			now := time.Now()
			for id, tr := range pending {
				if now.Sub(lastSeen[id]) >= 100*time.Millisecond {
					flushTrace(tr)
					delete(pending, id)
					delete(lastSeen, id)
				}
			}
		}
	}
}

func printHeadlessTrace(tr *store.Trace) {
	status := lipgloss.NewStyle().Foreground(tui.ColorSecondary).Bold(true).Render(" OK  ")
	if tr.HasError {
		status = lipgloss.NewStyle().Foreground(tui.ColorError).Bold(true).Render(" ERR ")
	}

	timeStr := tr.StartTime.Format("15:04:05.000")
	durStr := tui.FormatDuration(tr.Duration)
	svc := lipgloss.NewStyle().Foreground(tui.ServiceColor(tr.RootServiceName)).Bold(true).Render(tr.RootServiceName)

	fmt.Printf("[%s] [%s] %-20s │ %-30s │ %8s │ %2d spans │ trace=%s\n",
		timeStr, status, svc, tr.RootOperationName, durStr, tr.SpanCount, tr.TraceID)

	if tr.HasError {
		for _, s := range tr.SpanTree {
			if s.HasError || s.StatusCode == "ERROR" {
				errDesc := s.StatusMessage
				if errDesc == "" && len(s.Events) > 0 {
					for _, ev := range s.Events {
						if ev.Name == "exception" {
							errDesc = ev.Attributes["exception.message"]
						}
					}
				}
				fmt.Printf("   ↳ [FAIL] %s (%s): %s\n", s.ServiceName, s.Name, errDesc)
			}
		}
	}
}

func runDemo(args []string) {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	target := fs.String("target", "http://localhost:4318", "Target HTTP OTLP base URL")
	count := fs.Int("count", 1, "Number of sample traces to generate")
	interval := fs.Duration("interval", 500*time.Millisecond, "Interval between traces if count > 1")
	_ = fs.Parse(args)

	fmt.Printf("Generating %d sample distributed trace(s) -> %s\n", *count, *target)

	for i := 1; i <= *count; i++ {
		traceID, err := demo.SendDemoTraces(*target)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] Failed to send demo trace: %v\n", i, *count, err)
			os.Exit(1)
		}
		fmt.Printf("[%d/%d] Sent trace %s (api-gateway -> auth-service -> payment-service -> postgres-db, stripe-client)\n",
			i, *count, traceID)

		if i < *count && *interval > 0 {
			time.Sleep(*interval)
		}
	}
	fmt.Println("Demo trace generation complete.")
}
