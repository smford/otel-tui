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
	nodeHeader := fmt.Sprintf("  %s %s %s %s %s",
		PadRight("SERVICE NAME", 24),
		PadRight("INBOUND CALLS", 15),
		PadRight("OUTBOUND CALLS", 15),
		PadRight("ERROR COUNT", 13),
		PadRight("ERROR RATE", 12))
	lines = append(lines, TableHeaderStyle.Width(width-4).Render(nodeHeader))

	for _, n := range graph.Nodes {
		svcName := n.Name
		if lipgloss.Width(svcName) > 24 {
			svcName = svcName[:21] + "..."
		}
		svcCol := lipgloss.NewStyle().Foreground(ServiceColor(n.Name)).Bold(true).Render(svcName)
		svcCell := PadRight(svcCol, 24)

		inboundCell := PadRight(fmt.Sprintf("%d", n.InboundCalls), 15)
		outboundCell := PadRight(fmt.Sprintf("%d", n.OutboundCalls), 15)
		errCountCell := PadRight(fmt.Sprintf("%d", n.ErrorCount), 13)

		errRatePlain := fmt.Sprintf("%.1f%%", n.ErrorRate)
		var errRateCell string
		if n.ErrorRate > 0 {
			errRateCell = lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(errRatePlain)
		} else {
			errRateCell = lipgloss.NewStyle().Foreground(ColorSecondary).Render(errRatePlain)
		}
		errRateCell = PadRight(errRateCell, 12)

		row := fmt.Sprintf("  %s %s %s %s %s",
			svcCell, inboundCell, outboundCell, errCountCell, errRateCell)
		lines = append(lines, row)
	}

	lines = append(lines, "")

	// 2. Directed Edges
	lines = append(lines, lipgloss.NewStyle().Bold(true).Foreground(ColorWhite).Render("DEPENDENCY CALL FLOWS:"))
	if len(graph.Edges) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(ColorMuted).Render("  <no cross-service calls observed yet>"))
	} else {
		for _, e := range graph.Edges {
			fromName := e.FromService
			if lipgloss.Width(fromName) > 22 {
				fromName = fromName[:19] + "..."
			}
			fromCol := lipgloss.NewStyle().Foreground(ServiceColor(e.FromService)).Bold(true).Render(fromName)
			fromCell := PadRight(fromCol, 22)

			toName := e.ToService
			if lipgloss.Width(toName) > 22 {
				toName = toName[:19] + "..."
			}
			toCol := lipgloss.NewStyle().Foreground(ServiceColor(e.ToService)).Bold(true).Render(toName)

			statBadge := fmt.Sprintf(" %d calls │ avg %s │ p95 %s ", e.CallCount, FormatDuration(e.AvgDuration), FormatDuration(e.P95Duration))
			var edgeStyled string
			if e.ErrorCount > 0 {
				errText := fmt.Sprintf(" (%d ERRORS)", e.ErrorCount)
				statBadge += lipgloss.NewStyle().Foreground(ColorError).Bold(true).Render(errText)
				edgeStyled = lipgloss.NewStyle().Foreground(ColorError).Render("──[" + statBadge + "]──▶ ")
			} else {
				edgeStyled = lipgloss.NewStyle().Foreground(ColorSecondary).Render("──[" + statBadge + "]──▶ ")
			}

			lines = append(lines, fmt.Sprintf("  %s %s%s", fromCell, edgeStyled, toCol))
		}
	}

	return strings.Join(lines, "\n")
}
