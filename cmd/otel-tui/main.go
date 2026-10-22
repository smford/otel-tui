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
  --demo                    Run built-in synthetic trace generator in background (no emitter needed!)
  --demo-interval <dur>     Interval between background demo traces (default: 2s)
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
  # Start with continuous synthetic traffic (Zero emitter needed!):
  otel-tui --demo

  # Inside the TUI, press 'D' anytime to inject sample traces on demand!

  # Stream realistic multi-service synthetic traces continuously:
  otel-tui demo --stream --interval 1s

  # Start local collector & wait for your own application:
  otel-tui

  # Forward traces upstream while inspecting locally:
  otel-tui --forward-to http://staging-collector:4318

  # Inspect a previously exported trace offline:
  otel-tui --replay trace_3a4b5c6d.json
`)
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "demo" {
		runDemo(os.Args[2:])
		return
	}

	fs := flag.NewFlagSet("otel-tui", flag.ExitOnError)
	demoMode := fs.Bool("demo", false, "Continuously generate synthetic distributed traces in the background")
	demoInterval := fs.Duration("demo-interval", 2*time.Second, "Interval between synthetic background traces when --demo is active")
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

	// If --demo is requested, run continuous synthetic trace generator in background
	if *demoMode {
		go func() {
			time.Sleep(300 * time.Millisecond)
			ticker := time.NewTicker(*demoInterval)
			defer ticker.Stop()

			// Immediately inject one trace
			demo.InjectSyntheticTrace(st, demo.ScenarioCheckout)

			for range ticker.C {
				sc := demo.RandomScenario()
				demo.InjectSyntheticTrace(st, sc)
			}
		}()
	}

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
	scenario := fs.String("scenario", "all", "Scenario to emit: all, checkout, search, auth-failure, background-job")
	stream := fs.Bool("stream", false, "Continuously stream synthetic traces until interrupted")
	count := fs.Int("count", 1, "Number of sample traces to generate (ignored if --stream)")
	interval := fs.Duration("interval", 1*time.Second, "Interval between emitted traces")
	_ = fs.Parse(args)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	selectScenario := func(idx int) demo.Scenario {
		switch *scenario {
		case "checkout":
			return demo.ScenarioCheckout
		case "search":
			return demo.ScenarioProductSearch
		case "auth-failure", "auth":
			return demo.ScenarioAuthFailure
		case "background-job", "job", "worker":
			return demo.ScenarioBackgroundJob
		default:
			// Cycle through all scenarios
			return demo.AllScenarios[idx%len(demo.AllScenarios)]
		}
	}

	if *stream {
		fmt.Printf("Streaming synthetic OpenTelemetry events to %s (interval: %v, Ctrl+C to stop)...\n", *target, *interval)
		idx := 0
		ticker := time.NewTicker(*interval)
		defer ticker.Stop()

		for {
			sc := selectScenario(idx)
			tid, err := demo.SendDemoScenario(*target, sc)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to emit %s trace: %v\n", sc, err)
			} else {
				fmt.Printf("[%s] Emitted scenario '%s' (trace_id: %s)\n", time.Now().Format("15:04:05"), sc, tid)
			}
			idx++

			select {
			case <-sigChan:
				fmt.Println("\nStopped synthetic trace stream.")
				return
			case <-ticker.C:
			}
		}
	}

	fmt.Printf("Emitting %d synthetic distributed trace(s) -> %s\n", *count, *target)
	for i := 1; i <= *count; i++ {
		sc := selectScenario(i - 1)
		tid, err := demo.SendDemoScenario(*target, sc)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[%d/%d] Failed to emit trace: %v\n", i, *count, err)
			os.Exit(1)
		}
		fmt.Printf("[%d/%d] Emitted scenario '%s' (trace_id: %s)\n", i, *count, sc, tid)

		if i < *count && *interval > 0 {
			time.Sleep(*interval)
		}
	}
	fmt.Println("Synthetic trace emission complete.")
}
