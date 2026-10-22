package store

import (
	"sort"
	"time"
)

// ServiceEdge represents a directed call dependency between two services.
type ServiceEdge struct {
	FromService string        `json:"from_service"`
	ToService   string        `json:"to_service"`
	CallCount   int           `json:"call_count"`
	ErrorCount  int           `json:"error_count"`
	TotalTime   time.Duration `json:"total_time"`
	AvgDuration time.Duration `json:"avg_duration"`
	P95Duration time.Duration `json:"p95_duration"`

	durations []time.Duration
}

// ServiceNode represents a service within the distributed architecture.
type ServiceNode struct {
	Name         string  `json:"name"`
	InboundCalls int     `json:"inbound_calls"`
	OutboundCalls int    `json:"outbound_calls"`
	ErrorCount   int     `json:"error_count"`
	ErrorRate    float64 `json:"error_rate"`
}

// ServiceGraph represents the aggregate service dependency topology.
type ServiceGraph struct {
	Nodes []*ServiceNode `json:"nodes"`
	Edges []*ServiceEdge `json:"edges"`
}

// BuildServiceGraph aggregates service-to-service calls across all stored traces.
func (s *Store) BuildServiceGraph() *ServiceGraph {
	s.mu.RLock()
	defer s.mu.RUnlock()

	type edgeKey struct {
		from, to string
	}

	edgeMap := make(map[edgeKey]*ServiceEdge)
	nodeMap := make(map[string]*ServiceNode)

	ensureNode := func(name string) *ServiceNode {
		if n, ok := nodeMap[name]; ok {
			return n
		}
		n := &ServiceNode{Name: name}
		nodeMap[name] = n
		return n
	}

	for _, t := range s.traces {
		if t == nil {
			continue
		}

		for _, span := range t.Spans {
			fromSvc := span.ServiceName
			node := ensureNode(fromSvc)
			if span.HasError || span.StatusCode == "ERROR" {
				node.ErrorCount++
			}

			if span.ParentSpanID != "" {
				parent := t.SpansByID[span.ParentSpanID]
				if parent != nil && parent.ServiceName != fromSvc {
					parentSvc := parent.ServiceName
					ensureNode(parentSvc)

					nodeMap[parentSvc].OutboundCalls++
					nodeMap[fromSvc].InboundCalls++

					ek := edgeKey{from: parentSvc, to: fromSvc}
					edge, ok := edgeMap[ek]
					if !ok {
						edge = &ServiceEdge{
							FromService: parentSvc,
							ToService:   fromSvc,
						}
						edgeMap[ek] = edge
					}

					edge.CallCount++
					if span.HasError || span.StatusCode == "ERROR" {
						edge.ErrorCount++
					}
					edge.TotalTime += span.Duration
					edge.durations = append(edge.durations, span.Duration)
				}
			}
		}
	}

	// Calculate Edge statistics
	edges := make([]*ServiceEdge, 0, len(edgeMap))
	for _, edge := range edgeMap {
		if edge.CallCount > 0 {
			edge.AvgDuration = edge.TotalTime / time.Duration(edge.CallCount)
			sort.Slice(edge.durations, func(i, j int) bool {
				return edge.durations[i] < edge.durations[j]
			})
			p95Idx := int(float64(len(edge.durations)) * 0.95)
			if p95Idx >= len(edge.durations) {
				p95Idx = len(edge.durations) - 1
			}
			edge.P95Duration = edge.durations[p95Idx]
		}
		edges = append(edges, edge)
	}

	// Sort edges by CallCount desc
	sort.Slice(edges, func(i, j int) bool {
		return edges[i].CallCount > edges[j].CallCount
	})

	// Nodes list
	nodes := make([]*ServiceNode, 0, len(nodeMap))
	for _, node := range nodeMap {
		totalCalls := node.InboundCalls + node.OutboundCalls
		if totalCalls > 0 {
			node.ErrorRate = float64(node.ErrorCount) / float64(totalCalls) * 100.0
		}
		nodes = append(nodes, node)
	}

	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].Name < nodes[j].Name
	})

	return &ServiceGraph{
		Nodes: nodes,
		Edges: edges,
	}
}
