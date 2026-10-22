package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"otel-tui/internal/demo"
	"otel-tui/internal/store"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewMode int

const (
	ViewTraceList ViewMode = iota
	ViewTraceDetail
	ViewTraceDiff
	ViewServiceGraph
)

type FocusArea int

const (
	FocusWaterfall FocusArea = iota
	FocusInspector
)

type traceMsg *store.Trace

type AppModel struct {
	store         *store.Store
	grpcPort      int
	httpPort      int
	width         int
	height        int
	viewMode      ViewMode
	focusArea     FocusArea
	paused        bool
	statusMsg     string
	statusExpiry  time.Time
	subChan       chan *store.Trace

	// List View State
	traces          []*store.Trace
	selectedListIdx int
	filterInput     textinput.Model
	isFiltering     bool
	markedTraceID   string // Baseline trace for diffing

	// Detail View State
	activeTrace     *store.Trace
	selectedSpanIdx int
	inspectorScroll int
	useFlamegraph   bool // Toggle between waterfall and flamegraph

	// Diff & Graph State
	activeDiff  *store.TraceDiff
	activeGraph *store.ServiceGraph

	// Help popup toggle
	showHelp bool
}

// NewAppModel constructs a new interactive Bubbletea model.
func NewAppModel(s *store.Store, grpcPort, httpPort int) *AppModel {
	ti := textinput.New()
	ti.Placeholder = "Filter by service, error:true, >200ms..."
	ti.CharLimit = 120
	ti.Width = 40

	sub := s.Subscribe(200)

	m := &AppModel{
		store:        s,
		grpcPort:     grpcPort,
		httpPort:     httpPort,
		viewMode:     ViewTraceList,
		subChan:      sub,
		filterInput:  ti,
		traces:       s.ListTraces(""),
		statusExpiry: time.Now(),
	}

	return m
}

// waitForTrace returns a Bubbletea command that blocks until a trace update arrives.
func (m *AppModel) waitForTrace() tea.Cmd {
	return func() tea.Msg {
		t, ok := <-m.subChan
		if !ok {
			return nil
		}
		return traceMsg(t)
	}
}

func (m *AppModel) Init() tea.Cmd {
	return tea.Batch(m.waitForTrace(), textinput.Blink)
}

func (m *AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.filterInput.Width = msg.Width - 25
		if m.filterInput.Width < 20 {
			m.filterInput.Width = 20
		}

	case traceMsg:
		if !m.paused {
			m.refreshTraces()
		}
		if m.viewMode == ViewTraceDetail && m.activeTrace != nil && msg != nil && m.activeTrace.TraceID == msg.TraceID {
			if updated, ok := m.store.GetTrace(msg.TraceID); ok {
				m.activeTrace = updated
				if m.selectedSpanIdx >= len(m.activeTrace.SpanTree) {
					m.selectedSpanIdx = len(m.activeTrace.SpanTree) - 1
				}
			}
		}
		cmds = append(cmds, m.waitForTrace())

	case tea.KeyMsg:
		// When filter input is focused
		if m.isFiltering {
			switch msg.Type {
			case tea.KeyEnter:
				m.isFiltering = false
				m.filterInput.Blur()
				m.refreshTraces()
				m.selectedListIdx = 0
				return m, nil
			case tea.KeyEsc:
				m.isFiltering = false
				m.filterInput.Blur()
				m.filterInput.SetValue("")
				m.refreshTraces()
				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.refreshTraces()
				return m, cmd
			}
		}

		// Global Keybindings
		switch msg.String() {
		case "ctrl+c":
			m.store.Unsubscribe(m.subChan)
			return m, tea.Quit
		case "q":
			if m.viewMode != ViewTraceList {
				m.viewMode = ViewTraceList
				return m, nil
			}
			m.store.Unsubscribe(m.subChan)
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		}

		// View-specific keybindings
		switch m.viewMode {
		case ViewTraceList:
			m.handleListKeys(msg)
		case ViewTraceDetail:
			m.handleDetailKeys(msg)
		case ViewTraceDiff:
			m.handleDiffKeys(msg)
		case ViewServiceGraph:
			m.handleGraphKeys(msg)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *AppModel) refreshTraces() {
	query := m.filterInput.Value()
	m.traces = m.store.ListTraces(query)
	if m.selectedListIdx >= len(m.traces) {
		m.selectedListIdx = len(m.traces) - 1
	}
	if m.selectedListIdx < 0 {
		m.selectedListIdx = 0
	}
}

func (m *AppModel) handleListKeys(msg tea.KeyMsg) {
	switch msg.String() {
	case "up", "k":
		if m.selectedListIdx > 0 {
			m.selectedListIdx--
		}
	case "down", "j":
		if m.selectedListIdx < len(m.traces)-1 {
			m.selectedListIdx++
		}
	case "home", "g":
		m.selectedListIdx = 0
	case "end", "G":
		if len(m.traces) > 0 {
			m.selectedListIdx = len(m.traces) - 1
		}
	case "pageup":
		m.selectedListIdx -= 10
		if m.selectedListIdx < 0 {
			m.selectedListIdx = 0
		}
	case "pagedown":
		m.selectedListIdx += 10
		if m.selectedListIdx >= len(m.traces) {
			m.selectedListIdx = len(m.traces) - 1
		}
		if m.selectedListIdx < 0 {
			m.selectedListIdx = 0
		}
	case "enter":
		if len(m.traces) > 0 && m.selectedListIdx < len(m.traces) {
			m.activeTrace = m.traces[m.selectedListIdx]
			m.selectedSpanIdx = 0
			m.inspectorScroll = 0
			m.focusArea = FocusWaterfall
			m.viewMode = ViewTraceDetail
		}
	case "/":
		m.isFiltering = true
		m.filterInput.Focus()
	case "p":
		m.paused = !m.paused
		if !m.paused {
			m.refreshTraces()
		}
	case "c":
		m.store.Clear()
		m.traces = nil
		m.selectedListIdx = 0
		m.setStatus("Cleared trace buffer", 2*time.Second)
	case "x":
		if len(m.traces) > 0 && m.selectedListIdx < len(m.traces) {
			m.exportTrace(m.traces[m.selectedListIdx].TraceID)
		}
	case "m":
		if len(m.traces) > 0 && m.selectedListIdx < len(m.traces) {
			tid := m.traces[m.selectedListIdx].TraceID
			if m.markedTraceID == tid {
				m.markedTraceID = ""
				m.setStatus("Unmarked baseline trace", 2*time.Second)
			} else {
				m.markedTraceID = tid
				m.setStatus(fmt.Sprintf("Marked baseline trace %s (select another & press 'd')", tid[:8]), 3*time.Second)
			}
		}
	case "d":
		if len(m.traces) > 0 {
			curr := m.traces[m.selectedListIdx]
			var base *store.Trace
			if m.markedTraceID != "" && m.markedTraceID != curr.TraceID {
				base, _ = m.store.GetTrace(m.markedTraceID)
			} else if m.selectedListIdx < len(m.traces)-1 {
				base = m.traces[m.selectedListIdx+1]
			} else if m.selectedListIdx > 0 {
				base = m.traces[m.selectedListIdx-1]
			}
			if base != nil && base.TraceID != curr.TraceID {
				m.activeDiff = store.DiffTraces(base, curr)
				m.viewMode = ViewTraceDiff
			} else {
				m.setStatus("Need two distinct traces to diff. Press 'm' to mark a baseline trace.", 3*time.Second)
			}
		}
	case "S":
		m.activeGraph = m.store.BuildServiceGraph()
		m.viewMode = ViewServiceGraph
	case "D":
		sc := demo.RandomScenario()
		tid := demo.InjectSyntheticTrace(m.store, sc)
		m.refreshTraces()
		m.setStatus(fmt.Sprintf("Injected %s trace (%s)", sc, tid[:8]), 3*time.Second)
	case "y":
		if len(m.traces) > 0 && m.selectedListIdx < len(m.traces) {
			tid := m.traces[m.selectedListIdx].TraceID
			_ = CopyToClipboard(tid)
			m.setStatus("Copied Trace ID to clipboard: "+tid[:8]+"...", 2*time.Second)
		}
	case "Y":
		if len(m.traces) > 0 && m.selectedListIdx < len(m.traces) {
			data, _ := m.store.ExportTraceJSON(m.traces[m.selectedListIdx].TraceID)
			_ = CopyToClipboard(string(data))
			m.setStatus("Copied full Trace JSON to clipboard", 2*time.Second)
		}
	}
}

func (m *AppModel) handleDetailKeys(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "backspace":
		m.viewMode = ViewTraceList
		m.refreshTraces()
	case "tab":
		if m.focusArea == FocusWaterfall {
			m.focusArea = FocusInspector
		} else {
			m.focusArea = FocusWaterfall
		}
	case "f":
		m.useFlamegraph = !m.useFlamegraph
	case "up", "k":
		if m.focusArea == FocusWaterfall {
			if m.selectedSpanIdx > 0 {
				m.selectedSpanIdx--
				m.inspectorScroll = 0
			}
		} else {
			if m.inspectorScroll > 0 {
				m.inspectorScroll--
			}
		}
	case "down", "j":
		if m.focusArea == FocusWaterfall {
			if m.activeTrace != nil && m.selectedSpanIdx < len(m.activeTrace.SpanTree)-1 {
				m.selectedSpanIdx++
				m.inspectorScroll = 0
			}
		} else {
			m.inspectorScroll++
		}
	case "pageup", "K", "[":
		m.inspectorScroll -= 5
		if m.inspectorScroll < 0 {
			m.inspectorScroll = 0
		}
	case "pagedown", "J", "]":
		m.inspectorScroll += 5
	case "x":
		if m.activeTrace != nil {
			m.exportTrace(m.activeTrace.TraceID)
		}
	case "y":
		if m.activeTrace != nil && len(m.activeTrace.SpanTree) > 0 && m.selectedSpanIdx < len(m.activeTrace.SpanTree) {
			s := m.activeTrace.SpanTree[m.selectedSpanIdx]
			_ = CopyToClipboard(s.SpanID)
			m.setStatus("Copied Span ID to clipboard: "+s.SpanID, 2*time.Second)
		}
	case "Y":
		if m.activeTrace != nil {
			data, _ := m.store.ExportTraceJSON(m.activeTrace.TraceID)
			_ = CopyToClipboard(string(data))
			m.setStatus("Copied full Trace JSON to clipboard", 2*time.Second)
		}
	}
}

func (m *AppModel) handleDiffKeys(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "backspace", "q":
		m.viewMode = ViewTraceList
	}
}

func (m *AppModel) handleGraphKeys(msg tea.KeyMsg) {
	switch msg.String() {
	case "esc", "backspace", "q":
		m.viewMode = ViewTraceList
	}
}

func (m *AppModel) exportTrace(traceID string) {
	data, err := m.store.ExportTraceJSON(traceID)
	if err != nil {
		m.setStatus(fmt.Sprintf("Export error: %v", err), 3*time.Second)
		return
	}
	filename := fmt.Sprintf("trace_%s.json", traceID[:8])
	if err := os.WriteFile(filename, data, 0644); err != nil {
		m.setStatus(fmt.Sprintf("Failed to write file: %v", err), 3*time.Second)
		return
	}
	m.setStatus(fmt.Sprintf("Saved trace to %s", filename), 3*time.Second)
}

func (m *AppModel) setStatus(msg string, duration time.Duration) {
	m.statusMsg = msg
	m.statusExpiry = time.Now().Add(duration)
}

func (m *AppModel) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing terminal..."
	}

	var content string
	if m.showHelp {
		content = m.renderHelpView()
	} else {
		switch m.viewMode {
		case ViewTraceList:
			content = m.renderListView()
		case ViewTraceDetail:
			content = m.renderDetailView()
		case ViewTraceDiff:
			content = m.renderDiffView()
		case ViewServiceGraph:
			content = m.renderGraphView()
		}
	}

	return content
}

func (m *AppModel) renderHeader() string {
	title := TitleStyle.Render(" otel-tui ")
	ports := fmt.Sprintf("gRPC :%d │ HTTP :%d", m.grpcPort, m.httpPort)
	portsBadge := lipgloss.NewStyle().Foreground(ColorMuted).Render(ports)

	var streamBadge string
	if m.paused {
		streamBadge = BadgePaused.Render()
	} else {
		streamBadge = BadgeLive.Render()
	}

	totalTraces := fmt.Sprintf("%d traces", m.store.Count())
	countBadge := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render(totalTraces)

	statusText := ""
	if time.Now().Before(m.statusExpiry) && m.statusMsg != "" {
		statusText = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render(" ★ " + m.statusMsg)
	}

	left := lipgloss.JoinHorizontal(lipgloss.Center, title, " ", streamBadge, " ", countBadge, " ", statusText)
	right := lipgloss.JoinHorizontal(lipgloss.Center, portsBadge, " ")

	spaceWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if spaceWidth < 1 {
		spaceWidth = 1
	}
	spacer := strings.Repeat(" ", spaceWidth)

	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, right)
}

func (m *AppModel) renderListView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	// Filter bar
	filterPrompt := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("Filter [/]: ")
	filterView := m.filterInput.View()
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, filterPrompt, filterView))
	b.WriteString("\n\n")

	// Table Header
	headerCols := fmt.Sprintf("  %s %s %s %s %s %s %s %s",
		PadRight("STAT", 6),
		PadRight("TIME", 10),
		PadRight("SERVICE", 20),
		PadRight("OPERATION", 28),
		PadRight("DURATION", 10),
		PadRight("SPANS", 6),
		PadRight("TRACE ID", 16),
		"FLAGS")
	b.WriteString(TableHeaderStyle.Width(m.width).Render(headerCols))
	b.WriteString("\n")

	// Table rows
	contentHeight := m.height - 7
	if contentHeight < 4 {
		contentHeight = 4
	}

	if len(m.traces) == 0 {
		emptyMsg := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).
			Render("  No traces captured yet. Waiting for OTLP on ports " + fmt.Sprintf("%d/%d...", m.grpcPort, m.httpPort))
		b.WriteString("\n" + emptyMsg + "\n")
	} else {
		startIdx := 0
		if m.selectedListIdx >= contentHeight {
			startIdx = m.selectedListIdx - contentHeight + 1
		}
		endIdx := startIdx + contentHeight
		if endIdx > len(m.traces) {
			endIdx = len(m.traces)
		}

		for i := startIdx; i < endIdx; i++ {
			t := m.traces[i]
			isSelected := i == m.selectedListIdx

			statusBadge := " OK   "
			if t.HasError {
				statusBadge = " ERR  "
			}

			timeStr := t.StartTime.Format("15:04:05")
			durStr := FormatDuration(t.Duration)
			svcName := t.RootServiceName
			if len(svcName) > 18 {
				svcName = svcName[:16] + ".."
			}
			opName := t.RootOperationName
			if len(opName) > 26 {
				opName = opName[:24] + ".."
			}
			traceIDShort := t.TraceID
			if len(traceIDShort) > 16 {
				traceIDShort = traceIDShort[:16]
			}

			// Flags
			var flags []string
			if t.TraceID == m.markedTraceID {
				flags = append(flags, lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true).Render("[BASE]"))
			}
			if t.HasNPlusOne {
				flags = append(flags, lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("[N+1]"))
			}
			flagStr := strings.Join(flags, " ")

			cursor := "  "
			if isSelected {
				cursor = "❯ "
			}

			cursorCell := cursor
			statCell := PadRight(statusBadge, 6)
			timeCell := PadRight(timeStr, 10)
			svcCell := PadRight(svcName, 20)
			opCell := PadRight(opName, 28)
			durCell := PadRight(durStr, 10)
			spansCell := PadRight(fmt.Sprintf("%d", t.SpanCount), 6)
			idCell := PadRight(traceIDShort, 16)

			rowStr := fmt.Sprintf("%s%s %s %s %s %s %s %s %s",
				cursorCell, statCell, timeCell, svcCell, opCell, durCell, spansCell, idCell, flagStr)

			if isSelected {
				b.WriteString(RowSelectedStyle.Width(m.width).Render(rowStr))
			} else {
				if t.HasError {
					b.WriteString(lipgloss.NewStyle().Foreground(ColorError).Render(rowStr))
				} else {
					b.WriteString(RowNormalStyle.Render(rowStr))
				}
			}
			b.WriteString("\n")
		}
	}

	// Footer help bar
	footer := m.renderFooterHelp([]keyHelp{
		{"Enter", "Waterfall"},
		{"D", "Inject Synthetic Trace"},
		{"d", "Diff"},
		{"m", "Mark Base"},
		{"S", "Topology"},
		{"y", "Copy ID"},
		{"/", "Filter"},
		{"p", "Pause"},
		{"c", "Clear"},
		{"x", "Export"},
		{"?", "Help"},
		{"q", "Quit"},
	})
	b.WriteString("\n" + footer)

	return b.String()
}

func (m *AppModel) renderDetailView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	if m.activeTrace == nil {
		return "Trace details unavailable."
	}

	statusBadge := BadgeOk.Render()
	if m.activeTrace.HasError {
		statusBadge = BadgeError.Render()
	}

	n1Indicator := ""
	if m.activeTrace.HasNPlusOne {
		n1Indicator = " " + lipgloss.NewStyle().Foreground(ColorWarning).Bold(true).Render("[N+1 DETECTED]")
	}

	traceInfo := fmt.Sprintf("%s %s %s [%s] %s | %d spans | %s%s",
		statusBadge,
		lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render(m.activeTrace.RootOperationName),
		lipgloss.NewStyle().Foreground(ServiceColor(m.activeTrace.RootServiceName)).Render(m.activeTrace.RootServiceName),
		lipgloss.NewStyle().Foreground(ColorCyan).Render(m.activeTrace.TraceID),
		lipgloss.NewStyle().Bold(true).Foreground(ColorHighlight).Render(FormatDuration(m.activeTrace.Duration)),
		m.activeTrace.SpanCount,
		m.activeTrace.StartTime.Format("15:04:05.000"),
		n1Indicator,
	)
	b.WriteString(HeaderBoxStyle.Width(m.width).Render(traceInfo))
	b.WriteString("\n")

	availableHeight := m.height - 7
	if availableHeight < 10 {
		availableHeight = 10
	}
	waterfallHeight := availableHeight / 2
	inspectorHeight := availableHeight - waterfallHeight

	var selectedSpan *store.Span
	if len(m.activeTrace.SpanTree) > 0 && m.selectedSpanIdx < len(m.activeTrace.SpanTree) {
		selectedSpan = m.activeTrace.SpanTree[m.selectedSpanIdx]
	}

	var topPaneContent string
	var topPaneTitle string
	if m.useFlamegraph {
		topPaneContent = RenderFlamegraph(m.activeTrace, m.selectedSpanIdx, m.width-4, waterfallHeight-2)
		topPaneTitle = "Stacked Flamegraph View [f: switch to Waterfall]"
	} else {
		topPaneContent = RenderWaterfall(m.activeTrace, m.selectedSpanIdx, m.width-4, waterfallHeight-2)
		topPaneTitle = "Distributed Trace Waterfall [f: switch to Flamegraph, Tab: Focus]"
	}

	inspectorContent := RenderSpanInspector(selectedSpan, m.width-4, inspectorHeight-2, m.inspectorScroll)

	waterfallPanelStyle := PanelStyle
	inspectorPanelStyle := PanelStyle
	if m.focusArea == FocusWaterfall {
		waterfallPanelStyle = PanelActiveStyle
	} else {
		inspectorPanelStyle = PanelActiveStyle
	}

	inspectorTitle := "Span Inspector Drawer [Tab: Focus, J/K: Scroll, y: Copy Span ID]"

	b.WriteString(waterfallPanelStyle.Width(m.width - 2).Height(waterfallHeight).
		Render(lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(topPaneTitle) + "\n" + topPaneContent))
	b.WriteString("\n")
	b.WriteString(inspectorPanelStyle.Width(m.width - 2).Height(inspectorHeight).
		Render(lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render(inspectorTitle) + "\n" + inspectorContent))

	// Detail Footer Help Bar
	footer := m.renderFooterHelp([]keyHelp{
		{"Esc/Bksp", "Back to List"},
		{"f", "Toggle Flamegraph"},
		{"Tab", "Toggle Focus"},
		{"↑/↓", "Select Span"},
		{"J/K", "Scroll Inspector"},
		{"y", "Copy Span ID"},
		{"x", "Export JSON"},
		{"q", "Back"},
	})
	b.WriteString("\n" + footer)

	return b.String()
}

func (m *AppModel) renderDiffView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	diffContent := RenderTraceDiffView(m.activeDiff, m.width, m.height-6)
	b.WriteString(diffContent)

	footer := m.renderFooterHelp([]keyHelp{
		{"Esc/Bksp/q", "Back to List"},
		{"y", "Copy Diff"},
	})
	b.WriteString("\n\n" + footer)
	return b.String()
}

func (m *AppModel) renderGraphView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n")

	graphContent := RenderServiceGraphView(m.activeGraph, m.width, m.height-6)
	b.WriteString(graphContent)

	footer := m.renderFooterHelp([]keyHelp{
		{"Esc/Bksp/q", "Back to List"},
	})
	b.WriteString("\n\n" + footer)
	return b.String()
}

type keyHelp struct {
	key  string
	desc string
}

func (m *AppModel) renderFooterHelp(items []keyHelp) string {
	var parts []string
	for _, it := range items {
		k := HelpKeyStyle.Render(it.key)
		d := HelpDescStyle.Render(it.desc)
		parts = append(parts, fmt.Sprintf("%s:%s", k, d))
	}
	return " " + strings.Join(parts, "  │  ")
}

func (m *AppModel) renderHelpView() string {
	var b strings.Builder
	b.WriteString(m.renderHeader())
	b.WriteString("\n\n")

	helpText := `
  otel-tui — Terminal OpenTelemetry Viewer

  NAVIGATION:
    ↑ / k             Move cursor up
    ↓ / j             Move cursor down
    PageUp / PageDown Jump 10 rows
    Home / End        Jump to first / last item
    Enter             Open trace waterfall / flamegraph
    Esc / Backspace   Return to trace list view
    Tab               Toggle focus between Waterfall and Inspector drawer
    J / K             Scroll Span Inspector drawer up / down

  NEW FEATURES & ACTIONS:
    D                 Inject a realistic synthetic trace immediately (no emitter needed!)
    f                 Toggle between Waterfall and stacked Flamegraph view
    d                 Diff selected trace against marked baseline or adjacent trace
    m                 Mark / unmark baseline trace for comparison
    S                 View live Service Dependency Topology Architecture map
    y                 Copy selected TraceID or SpanID to clipboard (OSC 52 / pbcopy)
    Y                 Copy entire Trace JSON to clipboard
    /                 Filter traces (e.g. "service:auth", "error:true", ">200ms", "login")
    p                 Pause / Resume auto-scrolling live intake
    c                 Clear memory buffer
    x                 Export selected trace to trace_<id>.json
    ?                 Toggle this help dialog
    q / Ctrl+C        Quit

  INGESTION ENDPOINTS:
    OTLP / gRPC:      localhost:4317
    OTLP / HTTP:      localhost:4318 (POST /v1/traces, POST /v1/logs)
                      Supports application/x-protobuf & application/json

  Press '?' or 'Esc' to close this help screen.
`
	b.WriteString(PanelActiveStyle.Width(m.width - 4).Render(helpText))
	return b.String()
}
