package agent

import (
	"context"
	"iter"
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// mockAgent creates an agent that yields a specific state delta.
func mockAgent(name string, outputKey, outputValue string) agent.Agent {
	a, _ := agent.New(agent.Config{
		Name:        name,
		Description: "Mock agent",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				ev := session.NewEvent(ctx, "test-invocation")
				if outputKey != "" {
					ev.Actions.StateDelta = map[string]any{
						outputKey: outputValue,
					}
				}
				ev.Author = name
				yield(ev, nil)
			}
		},
	})
	return a
}

type mockSession struct {
    session.Session
}
func (m *mockSession) ID() string { return "test-session" }

type mockInvocationContext struct {
	context.Context
	agent agent.Agent
}

func (m *mockInvocationContext) Agent() agent.Agent             { return m.agent }
func (m *mockInvocationContext) Artifacts() agent.Artifacts     { return nil }
func (m *mockInvocationContext) Memory() agent.Memory           { return nil }
func (m *mockInvocationContext) Session() session.Session       { return &mockSession{} }
func (m *mockInvocationContext) InvocationID() string           { return "test-invocation" }
func (m *mockInvocationContext) Branch() string                 { return "test" }
func (m *mockInvocationContext) IsolationScope() string         { return "" }
func (m *mockInvocationContext) UserContent() *genai.Content    { return nil }
func (m *mockInvocationContext) RunConfig() *agent.RunConfig    { return nil }
func (m *mockInvocationContext) EndInvocation()                 {}
func (m *mockInvocationContext) Ended() bool                    { return false }
func (m *mockInvocationContext) WithContext(ctx context.Context) agent.InvocationContext {
	return &mockInvocationContext{Context: ctx, agent: m.agent}
}
func (m *mockInvocationContext) WithICDelta(d *agent.InvocationContextDelta) agent.InvocationContext {
	if d == nil {
		return m
	}
	res := *m
	if d.Agent != nil {
		res.agent = *d.Agent
	}
	return &res
}
func (m *mockInvocationContext) ResumedInput(string) (any, bool) { return nil, false }

func TestCoordinatorAgent_Proceed(t *testing.T) {
	scanner, _ := NewScannerNode()
	
	proceedResult := `{"proceed": true, "reason": "good"}`
	filter := mockAgent("FilterAgent", "filter_result", proceedResult)
	
	evaluator := mockAgent("EvaluatorAgent", "", "")
	coverLetter := mockAgent("CoverLetterAgent", "", "")
	notifier, _ := NewNotifierNode()

	coordinator, err := NewCoordinatorAgent(scanner, filter, evaluator, coverLetter, notifier)
	if err != nil {
		t.Fatalf("Failed to create CoordinatorAgent: %v", err)
	}

	ctx := &mockInvocationContext{
		Context: context.Background(),
		agent:   coordinator,
	}

	var events []string
	for ev, err := range coordinator.Run(ctx) {
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		events = append(events, ev.Author)
	}

	// We expect ScannerNode, FilterAgent, EvaluatorAgent, CoverLetterAgent, NotifierNode
	expected := []string{"ScannerNode", "FilterAgent", "EvaluatorAgent", "CoverLetterAgent", "NotifierNode"}
	if len(events) != len(expected) {
		t.Fatalf("Expected %d events, got %d. Events: %v", len(expected), len(events), events)
	}
	for i, v := range expected {
		if events[i] != v {
			t.Errorf("Expected event author %s at index %d, got %s", v, i, events[i])
		}
	}
}

func TestCoordinatorAgent_Disqualify(t *testing.T) {
	scanner, _ := NewScannerNode()
	
	proceedResult := `{"proceed": false, "reason": "bad fit"}`
	filter := mockAgent("FilterAgent", "filter_result", proceedResult)
	
	evaluator := mockAgent("EvaluatorAgent", "", "")
	coverLetter := mockAgent("CoverLetterAgent", "", "")
	notifier, _ := NewNotifierNode()

	coordinator, err := NewCoordinatorAgent(scanner, filter, evaluator, coverLetter, notifier)
	if err != nil {
		t.Fatalf("Failed to create CoordinatorAgent: %v", err)
	}

	ctx := &mockInvocationContext{
		Context: context.Background(),
		agent:   coordinator,
	}

	var events []string
	for ev, err := range coordinator.Run(ctx) {
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		events = append(events, ev.Author)
	}

	// We expect ScannerNode and FilterAgent only, pipeline should stop
	expected := []string{"ScannerNode", "FilterAgent"}
	if len(events) != len(expected) {
		t.Fatalf("Expected %d events, got %d. Events: %v", len(expected), len(events), events)
	}
	for i, v := range expected {
		if events[i] != v {
			t.Errorf("Expected event author %s at index %d, got %s", v, i, events[i])
		}
	}
}
