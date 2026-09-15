package agent

import (
	"encoding/json"
	"iter"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/session"
)

// NewCoordinatorAgent creates a custom agent that orchestrates the workflow.
func NewCoordinatorAgent(scanner, filter, evaluator, coverLetter, notifier agent.Agent) (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        "CoordinatorAgent",
		Description: "Orchestrates the job application pipeline",
		SubAgents:   []agent.Agent{scanner, filter, evaluator, coverLetter, notifier},
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				// 1. Run Scanner
				for ev, err := range scanner.Run(ctx) {
					if !yield(ev, err) {
						return
					}
				}

				// 2. Run Filter
				var filterResultStr string
				for ev, err := range filter.Run(ctx) {
					if ev != nil && ev.Actions.StateDelta != nil {
						if val, ok := ev.Actions.StateDelta["filter_result"]; ok {
							if strVal, isStr := val.(string); isStr {
								filterResultStr = strVal
							}
						}
					}
					if !yield(ev, err) {
						return
					}
				}

				// Check filter result
				if filterResultStr != "" {
					var result struct {
						Proceed bool `json:"proceed"`
					}
					if err := json.Unmarshal([]byte(filterResultStr), &result); err == nil {
						if !result.Proceed {
							// Pipeline stops early
							return
						}
					} else {
                        // try to find "proceed": true or false in string if json is malformed?
                        // let's assume valid JSON for now.
                    }
				}

				// 3. Run Evaluator
				for ev, err := range evaluator.Run(ctx) {
					if !yield(ev, err) {
						return
					}
				}

				// 4. Run CoverLetter
				for ev, err := range coverLetter.Run(ctx) {
					if !yield(ev, err) {
						return
					}
				}

				// 5. Run Notifier
				for ev, err := range notifier.Run(ctx) {
					if !yield(ev, err) {
						return
					}
				}
			}
		},
	})
}

// NewScannerNode creates a mock scanner node for the pipeline.
func NewScannerNode() (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        "ScannerNode",
		Description: "Scans for job postings",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				// Mock implementation
				yield(session.NewEvent(ctx, ctx.InvocationID()), nil)
			}
		},
	})
}

// NewNotifierNode creates a mock notifier node (e.g., Telegram).
func NewNotifierNode() (agent.Agent, error) {
	return agent.New(agent.Config{
		Name:        "NotifierNode",
		Description: "Sends notifications",
		Run: func(ctx agent.InvocationContext) iter.Seq2[*session.Event, error] {
			return func(yield func(*session.Event, error) bool) {
				// Mock implementation
				yield(session.NewEvent(ctx, ctx.InvocationID()), nil)
			}
		},
	})
}
