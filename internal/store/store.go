package store

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Store is a thread-safe, bounded ring buffer for OpenTelemetry traces and correlated logs.
type Store struct {
	mu          sync.RWMutex
	capacity    int
	traces      map[string]*Trace
	order       []string // TraceIDs in order of insertion (oldest at index 0)
	subscribers map[chan *Trace]struct{}
}

// NewStore initializes a trace store with the specified maximum trace capacity.
func NewStore(capacity int) *Store {
	if capacity <= 0 {
		capacity = 1000
	}
	return &Store{
		capacity:    capacity,
		traces:      make(map[string]*Trace),
		order:       make([]string, 0, capacity),
		subscribers: make(map[chan *Trace]struct{}),
	}
}

// AddSpans inserts or merges a batch of spans into their respective traces.
func (s *Store) AddSpans(spans []*Span) {
	if len(spans) == 0 {
		return
	}

	s.mu.Lock()
	updatedTraces := make(map[string]*Trace)

	for _, span := range spans {
		if span.TraceID == "" {
			continue
		}

		t, exists := s.traces[span.TraceID]
		if !exists {
			// Enforce bounded ring buffer capacity
			if len(s.traces) >= s.capacity && len(s.order) > 0 {
				oldestID := s.order[0]
				s.order = s.order[1:]
				delete(s.traces, oldestID)
			}

			t = &Trace{
				TraceID:   span.TraceID,
				SpansByID: make(map[string]*Span),
			}
			s.traces[span.TraceID] = t
			s.order = append(s.order, span.TraceID)
		}

		// Merge span: check if span already exists in this trace
		if existingSpan, hasSpan := t.SpansByID[span.SpanID]; hasSpan {
			// Merge attributes and update
			for k, v := range span.Attributes {
				if existingSpan.Attributes == nil {
					existingSpan.Attributes = make(map[string]string)
				}
				existingSpan.Attributes[k] = v
			}
			for k, v := range span.ResourceAttributes {
				if existingSpan.ResourceAttributes == nil {
					existingSpan.ResourceAttributes = make(map[string]string)
				}
				existingSpan.ResourceAttributes[k] = v
			}
			existingSpan.Events = append(existingSpan.Events, span.Events...)
			if span.StatusCode == "ERROR" {
				existingSpan.StatusCode = "ERROR"
				existingSpan.HasError = true
			}
		} else {
			t.Spans = append(t.Spans, span)
			if t.SpansByID == nil {
				t.SpansByID = make(map[string]*Span)
			}
			t.SpansByID[span.SpanID] = span
		}

		updatedTraces[span.TraceID] = t
	}

	// Rebuild all modified traces
	for _, t := range updatedTraces {
		t.RebuildTrace()
	}

	// Copy updated traces to broadcast to subscribers
	toNotify := make([]*Trace, 0, len(updatedTraces))
	for _, t := range updatedTraces {
		toNotify = append(toNotify, t)
	}

	subList := make([]chan *Trace, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subList = append(subList, ch)
	}
	s.mu.Unlock()

	// Notify subscribers without holding store lock
	for _, t := range toNotify {
		for _, ch := range subList {
			select {
			case ch <- t:
			default:
				// If channel is full, drop to prevent blocking ingestion
			}
		}
	}
}

// AddLogs attaches correlated logs to traces by TraceID.
func (s *Store) AddLogs(logs []LogRecord) {
	if len(logs) == 0 {
		return
	}

	s.mu.Lock()
	updatedTraces := make(map[string]*Trace)

	for _, log := range logs {
		if log.TraceID == "" {
			continue
		}

		t, exists := s.traces[log.TraceID]
		if !exists {
			// If trace doesn't exist yet, we still track it
			if len(s.traces) >= s.capacity && len(s.order) > 0 {
				oldestID := s.order[0]
				s.order = s.order[1:]
				delete(s.traces, oldestID)
			}

			t = &Trace{
				TraceID:   log.TraceID,
				SpansByID: make(map[string]*Span),
			}
			s.traces[log.TraceID] = t
			s.order = append(s.order, log.TraceID)
		}

		t.Logs = append(t.Logs, log)
		updatedTraces[log.TraceID] = t
	}

	for _, t := range updatedTraces {
		t.RebuildTrace()
	}

	toNotify := make([]*Trace, 0, len(updatedTraces))
	for _, t := range updatedTraces {
		toNotify = append(toNotify, t)
	}

	subList := make([]chan *Trace, 0, len(s.subscribers))
	for ch := range s.subscribers {
		subList = append(subList, ch)
	}
	s.mu.Unlock()

	for _, t := range toNotify {
		for _, ch := range subList {
			select {
			case ch <- t:
			default:
			}
		}
	}
}

// GetTrace retrieves a trace by its TraceID.
func (s *Store) GetTrace(traceID string) (*Trace, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.traces[traceID]
	return t, ok
}

// ListTraces returns traces matching the filter query, sorted newest first.
func (s *Store) ListTraces(query string) []*Trace {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Trace, 0, len(s.order))
	// Traverse in reverse order (newest first)
	for i := len(s.order) - 1; i >= 0; i-- {
		t := s.traces[s.order[i]]
		if t != nil && MatchTrace(t, query) {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the total number of traces stored.
func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.traces)
}

// Clear flushes all stored traces and logs.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.traces = make(map[string]*Trace)
	s.order = make([]string, 0, s.capacity)
}

// Subscribe registers a channel to receive notifications whenever traces are added or updated.
func (s *Store) Subscribe(bufferSize int) chan *Trace {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	ch := make(chan *Trace, bufferSize)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = struct{}{}
	return ch
}

// Unsubscribe removes a notification channel.
func (s *Store) Unsubscribe(ch chan *Trace) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, ch)
	close(ch)
}

// ExportTraceJSON serializes a single trace into formatted JSON.
func (s *Store) ExportTraceJSON(traceID string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	t, ok := s.traces[traceID]
	if !ok {
		return nil, fmt.Errorf("trace %q not found", traceID)
	}
	return json.MarshalIndent(t, "", "  ")
}

// ExportAllJSON serializes all stored traces into formatted JSON.
func (s *Store) ExportAllJSON() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	all := make([]*Trace, 0, len(s.order))
	for _, id := range s.order {
		if t, ok := s.traces[id]; ok {
			all = append(all, t)
		}
	}
	return json.MarshalIndent(all, "", "  ")
}

// ImportTraceJSON loads traces from raw JSON bytes (either a single Trace or a slice of Traces).
func (s *Store) ImportTraceJSON(data []byte) error {
	// First attempt unmarshaling as slice of traces
	var traces []*Trace
	if err := json.Unmarshal(data, &traces); err == nil && len(traces) > 0 {
		for _, t := range traces {
			s.ingestLoadedTrace(t)
		}
		return nil
	}

	// Fallback to single trace
	var single Trace
	if err := json.Unmarshal(data, &single); err != nil {
		return fmt.Errorf("invalid trace JSON format: %w", err)
	}
	s.ingestLoadedTrace(&single)
	return nil
}

func (s *Store) ingestLoadedTrace(t *Trace) {
	if t.TraceID == "" {
		return
	}
	s.AddSpans(t.Spans)
	if len(t.Logs) > 0 {
		s.AddLogs(t.Logs)
	}
}
