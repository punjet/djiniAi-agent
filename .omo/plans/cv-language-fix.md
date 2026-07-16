# cv-language-fix - Work Plan

## TL;DR
**What you'll get:** Two-step cover letter generation process to guarantee the language matches the Job Description while explicitly blocking Russian.
**Why this approach:** LLMs occasionally ignore negative constraints (like "do not use Russian") in large prompts or default to the CV's native language. Separating the language detection from the generation steps guarantees that the LLM receives an explicit positive constraint ("Generate strictly in Ukrainian/English") tailored to the JD.

## Scope
### Must have
1. A new LLM classification step in `internal/covergen/generator.go` (before generating the cover letter) that determines if the `jdText` requires an `English` or `Ukrainian` response.
2. Update the main system prompt in `GenerateCoverLetter` to dynamically interpolate the detected language.
3. Remove the existing hardcoded language rule ("Write the Cover Letter and Djinni message in UKRAINIAN if...") and replace it with: `Write the Cover Letter and Djinni message STRICTLY in {DETECTED_LANGUAGE}. NEVER write in Russian.`
4. Use structured JSON output for the detection step to ensure reliable parsing.

### Must NOT have
- No new packages or dependencies.
- No changes to the existing cover letter JSON schema.
- No structural changes to the PDF generation via chromedp.

## Execution strategy
### Parallel execution waves
**Wave 1:**
- T1: Add `DetectJDLanguage(ctx context.Context, provider llm.Provider, jdText string) (string, error)` helper to `internal/covergen/generator.go` which uses the LLM provider to classify the JD language as either `Ukrainian` or `English`.
- T2: Modify `GenerateCoverLetter` to call `DetectJDLanguage` first, and interpolate the resulting language string into the system prompt.

## Todos
- [x] 1. Add `DetectJDLanguage` helper in `internal/covergen/generator.go`
  What to do: Create a new function that takes the `jdText` and queries the LLM. The system prompt should instruct the LLM to classify the job description's language. If it is Ukrainian or Russian, return `Ukrainian`. If it is English or any other language, return `English`. Require the LLM to respond with a simple JSON object: `{"language": "Ukrainian"}` or `{"language": "English"}`. Return the parsed string.
  Acceptance criteria: `go build ./internal/covergen/...` exits 0.

- [x] 2. Integrate detected language into `GenerateCoverLetter`
  What to do: Call `DetectJDLanguage` at the start of `GenerateCoverLetter` (after loading files, before the main generation call). Modify the `systemPrompt` text to replace the existing "Language Rule" (Rule 8) with: `8. Language Rule: Write the Cover Letter and Djinni message STRICTLY in %s. NEVER write in Russian under any circumstances.`, using `fmt.Sprintf` to inject the detected language.
  Acceptance criteria: `go build ./...` exits 0.

## Final verification wave
- [x] F1. `go build ./... && go vet ./...` exits 0.
- [x] F2. `go test ./... -race -count=1` exits 0.
