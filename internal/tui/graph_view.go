package tui

import (
	"fmt"
	"strings"

	"otel-tui/internal/store"

	"github.com/charmbracelet/lipgloss"
)

// RenderServiceGraphView renders the interactive service dependency topology.
func RenderServiceGraphView(graph *store.ServiceGraph, width, height int) string {
	if graph == nil || len(graph.Nodes) == 0 {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render("  No service topology captured yet. Ingest traces to view dependencies.")
	}

	var lines []string

	title := fmt.Sprintf("SERVICE TOPOLOGY GRAPH (%d services, %d dependency links)", len(graph.Nodes), len(graph.Edges))
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorAccent).Render("◆ "+title))
	lines = append(lines, "")

	// 1. Service Nodes Table
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render("SERVICES SUMMARY:"))
	nodeHeader := fmt.Sprintf("  %-24s %-14s %-14s %-12s %-10s",
		"SERVICE NAME", "INBOUND CALLS", "OUTBOUND CALLS", "ERROR COUNT", "ERROR RATE")
	lines = append(lines, TableHeaderStyle.Width(width-4).Render(nodeHeader))

	for _, n := range graph.Nodes {
		svcCol := lipgloss.NewStyle().Foreground(ServiceColor(n.Name)).Bold(true).Render(n.Name)
		errRateStr := fmt.Sprintf("%.1f%%", n.ErrorRate)
		if n.ErrorRate > 0 {
			errRateStr = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(errRateStr)
		} else {
			errRateStr = lipgloss.NewStyle().Foreground(ColorSecondary).Render(errRateStr)
		}

		row := fmt.Sprintf("  %-24s %-14d %-14d %-12d %-10s",
			svcCol, n.InboundCalls, n.OutboundCalls, n.ErrorCount, errRateStr)
		lines = append(lines, row)
	}

	lines = append(lines, "")

	// 2. Directed Edges
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render("DEPENDENCY CALL FLOWS:"))
	if len(graph.Edges) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("  <no cross-service calls observed yet>"))
	} else {
		for _, e := range graph.Edges {
			fromCol := lipgloss.NewStyle().Foreground(ServiceColor(e.FromService)).Bold(true).Render(e.FromService)
			toCol := lipgloss.NewStyle().Foreground(ServiceColor(e.ToService)).Bold(true).Render(e.ToService)

			statBadge := fmt.Sprintf(" %d calls │ avg %s │ p95 %s ", e.CallCount, FormatDuration(e.AvgDuration), FormatDuration(e.P95Duration))
			var edgeStyled string
			if e.ErrorCount > 0 {
				errText := fmt.Sprintf(" (%d ERRORS)", e.ErrorCount)
				statBadge += lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(errText)
				edgeStyled = lipgloss.NewStyle().Foreground(ColorError).Render("──[" + statBadge + "]──▶ ")
			} else {
				edgeStyled = lipgloss.NewStyle().Foreground(ColorSecondary).Render("──[" + statBadge + "]──▶ ")
			}

			lines = append(lines, fmt.Sprintf("  %s %s%s", fromCol, edgeStyled, toCol))
		}
	}

	return strings.Join(lines, "\n")
}
