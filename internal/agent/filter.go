package agent

import (

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

// NewFilterAgent creates a new agent for initial filtering of job postings.
func NewFilterAgent(llm model.LLM, toolset tool.Toolset) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:  "FilterAgent",
		Model: llm,
		Toolsets: []tool.Toolset{toolset},
		Instruction: `You are an initial filter agent for job applications.
Your job is to compare job requirements against the candidate's hard constraints (e.g., visa requirements, location, mandatory tech stack).

If the candidate matches the hard constraints, output {"score": <num>, "proceed": true, "reason": "..."}
If not, output {"score": <num>, "proceed": false, "reason": "..."}

JOB DESCRIPTION: {job_description}
CANDIDATE CONSTRAINTS: {candidate_constraints}`,
		OutputKey: "filter_result",
	})
}
