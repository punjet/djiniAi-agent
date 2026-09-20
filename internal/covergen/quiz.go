package covergen

import (
	"context"
	"djinni-bot-go/internal/config"
	"djinni-bot-go/internal/extractor"
	"djinni-bot-go/internal/llm"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func AnswerQuizQuestions(ctx context.Context, cfg *config.Config, engine llm.Engine, contextDir string, questions []extractor.QuizQuestion, jdText, company, role string) ([]extractor.QuizQuestion, error) {
	// 1. Load profile
	profilePath := filepath.Join(contextDir, "config", "profile.yml")
	profileData, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile.yml: %w", err)
	}

	var prof Profile
	if err := yaml.Unmarshal(profileData, &prof); err != nil {
		return nil, fmt.Errorf("failed to parse profile.yml: %w", err)
	}

	// 2. Load CV
	cvPath := filepath.Join(contextDir, "cv.md")
	cvData, err := os.ReadFile(cvPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cv.md: %w", err)
	}

	// 3. Setup LLM provider
	provider, err := llm.NewProvider(cfg, engine, "resume")
	if err != nil {
		return nil, err
	}

	// 4. Build question list text
	var questionLines []string
	for i, q := range questions {
		questionLines = append(questionLines, fmt.Sprintf("  %d. [name=%s] %s", i+1, q.Name, q.Text))
	}
	questionsText := strings.Join(questionLines, "\n")

	systemPrompt := `You are Kyrylo Kirov, a Senior AI Automation Expert applying for a job. You have been sent a short quiz by the recruiter.

Your task: Answer each quiz question professionally, concisely, and truthfully based on your CV and profile context below.

Instructions:
1. Answer each question as if you are the candidate (Kyrylo Kirov). Use "I" / "me" where appropriate.
2. Be concise — 1-3 sentences per answer is usually enough.
3. Write each answer in the SAME LANGUAGE as the question itself (e.g. if the question is in English, answer in English; if in Ukrainian, answer in Ukrainian; if in Russian, answer in Russian).
4. Base your answers on the provided CV and profile. If the CV doesn't cover the topic, give a general but relevant answer.

You MUST respond with a single JSON object (no markdown wrappers like ` + "`" + `json or comments) matching this schema exactly:
{
  "answers": [
    {
      "name": "field_name_like_answer_12345",
      "answer": "your answer text here"
    }
  ]
}`

	userPrompt := fmt.Sprintf(`Candidate Profile:
Name: %s
Email: %s
Location: %s
LinkedIn: %s
GitHub: %s
Target Roles: %v

Resume/CV:
%s

Job Details:
Company: %s
Role: %s
Job Description:
%s

Recruiter Quiz Questions:
%s

Please answer the above questions and return them as JSON.`, prof.Candidate.FullName, prof.Candidate.Email, prof.Candidate.Location, prof.Candidate.Linkedin, prof.Candidate.Github, prof.TargetRoles.Primary, string(cvData), company, role, jdText, questionsText)

	response, err := provider.GenerateText(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM quiz answering failed: %w", err)
	}

	cleanJSON := llm.CleanJSON(response)

	var result struct {
		Answers []struct {
			Name   string `json:"name"`
			Answer string `json:"answer"`
		} `json:"answers"`
	}
	if err := json.Unmarshal([]byte(cleanJSON), &result); err != nil {
		return nil, fmt.Errorf("failed to parse LLM response JSON: %w (raw: %q)", err, response)
	}

	// 6. Map answers back to questions by name
	answerMap := make(map[string]string, len(result.Answers))
	for _, a := range result.Answers {
		answerMap[a.Name] = a.Answer
	}

	for i := range questions {
		if ans, ok := answerMap[questions[i].Name]; ok {
			questions[i].Answer = ans
		}
	}

	return questions, nil
}
