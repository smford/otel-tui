package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"otel-tui/internal/demo"
	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestGenerateScreenshots(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	st := store.NewStore(50)

	id1 := demo.InjectSyntheticTrace(st, demo.ScenarioCheckout)
	time.Sleep(2 * time.Millisecond)
	id2 := demo.InjectSyntheticTrace(st, demo.ScenarioAuthFailure)
	time.Sleep(2 * time.Millisecond)
	id3 := demo.InjectSyntheticTrace(st, demo.ScenarioProductSearch)
	time.Sleep(2 * time.Millisecond)
	id4 := demo.InjectSyntheticTrace(st, demo.ScenarioBackgroundJob)
	time.Sleep(2 * time.Millisecond)
	id5 := demo.InjectSyntheticTrace(st, demo.ScenarioCheckout)
	time.Sleep(2 * time.Millisecond)
	id6 := demo.InjectSyntheticTrace(st, demo.ScenarioProductSearch)
	time.Sleep(2 * time.Millisecond)
	id7 := demo.InjectSyntheticTrace(st, demo.ScenarioAuthFailure)
	time.Sleep(2 * time.Millisecond)
	id8 := demo.InjectSyntheticTrace(st, demo.ScenarioBackgroundJob)

	_ = id2
	_ = id3
	_ = id5
	_ = id6
	_ = id7
	_ = id8

	outDir := "/tmp/otel-screens"
	os.MkdirAll(outDir, 0755)

	width := 102
	height := 22

	// 1. Trace List
	app := NewAppModel(st, 4317, 4318)
	app.width = width
	app.height = height
	app.filterInput.Width = width - 25
	app.refreshTraces()
	app.markedTraceID = id5
	app.selectedListIdx = 1
	os.WriteFile(filepath.Join(outDir, "trace-list.ansi"), []byte(app.View()), 0644)

	// 2. Trace Waterfall View
	tr1, _ := st.GetTrace(id1)
	app.viewMode = ViewTraceDetail
	app.activeTrace = tr1
	app.selectedSpanIdx = 2
	app.useFlamegraph = false
	os.WriteFile(filepath.Join(outDir, "waterfall.ansi"), []byte(app.View()), 0644)

	// 3. Flamegraph View
	app.useFlamegraph = true
	os.WriteFile(filepath.Join(outDir, "flamegraph.ansi"), []byte(app.View()), 0644)

	// 4. Service Topology View
	app.viewMode = ViewServiceGraph
	app.activeGraph = st.BuildServiceGraph()
	os.WriteFile(filepath.Join(outDir, "topology.ansi"), []byte(app.View()), 0644)

	// 5. Trace Diff View
	app.viewMode = ViewTraceDiff
	tr4, _ := st.GetTrace(id4)
	app.activeDiff = store.DiffTraces(tr1, tr4)
	os.WriteFile(filepath.Join(outDir, "trace-diff.ansi"), []byte(app.View()), 0644)

	// 6. Span Inspector Drawer (focus inspector on payment span)
	app.viewMode = ViewTraceDetail
	app.useFlamegraph = false
	app.focusArea = FocusInspector
	app.selectedSpanIdx = 2
	os.WriteFile(filepath.Join(outDir, "inspector.ansi"), []byte(app.View()), 0644)

	t.Logf("Generated 6 screen captures in %s (IDs: %s, %s, %s, %s)", outDir, id1[:8], id2[:8], id3[:8], id4[:8])
}
