package agent

import (

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

// NewEvaluatorAgent creates a new agent for deep semantic evaluation.
func NewEvaluatorAgent(llm model.LLM, toolset tool.Toolset) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:  "EvaluatorAgent",
		Model: llm,
		Toolsets: []tool.Toolset{toolset},
		Instruction: `You are a deep semantic evaluation agent.
Compare the job details with the candidate's CV and profile.
Generate a structured match breakdown including a score out of 5, specific matching skills, missing skills, and a summary.

Write your response with clear sections:
A. SUMMARY
B. COMPANY & ROLE
C. MATCH BREAKDOWN
D. SCORE SUMMARY

SCORE_SUMMARY must include:
SCORE: [0-5]
COMPANY: [Name]
ROLE: [Title]
ARCHETYPE: [Type]
LEGITIMACY: [High/Med/Low]

JOB DESCRIPTION: {job_description}
CANDIDATE PROFILE: {candidate_profile}
CANDIDATE CV: {candidate_cv}`,
		OutputKey: "evaluation_result",
	})
}
