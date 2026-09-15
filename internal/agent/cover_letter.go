package agent

import (

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
)

// NewCoverLetterAgent creates a new agent for generating cover letters.
func NewCoverLetterAgent(llm model.LLM, toolset tool.Toolset) (agent.Agent, error) {
	return llmagent.New(llmagent.Config{
		Name:  "CoverLetterAgent",
		Model: llm,
		Toolsets: []tool.Toolset{toolset},
		Instruction: `You are an expert cover letter writer.
Using the provided job details, the candidate's CV, and the match evaluation, write a highly tailored, professional, and engaging cover letter.
Match the tone of the company and highlight the most relevant experiences.

JOB DESCRIPTION: {job_description}
CANDIDATE CV: {candidate_cv}
EVALUATION: {evaluation_result}

Output only the cover letter content.`,
		OutputKey: "cover_letter_content",
	})
}
