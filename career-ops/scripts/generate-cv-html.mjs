import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const PROJECT_ROOT = path.resolve(__dirname, '..');

const llmBaseUrl = process.env.LLM_BASE_URL || 'http://localhost:3001/v1';
const llmApiKey = process.env.LLM_API_KEY || '';
let llmModel = process.env.LLM_MODEL || 'gpt-4o'; // Or fallback from env

// Resolve LLM model availability
try {
  const modelsUrl = llmBaseUrl.endsWith('/v1') ? `${llmBaseUrl}/models` : `${llmBaseUrl}/v1/models`;
  const headers = llmApiKey ? { 'Authorization': `Bearer ${llmApiKey}` } : {};
  const modelsRes = await fetch(modelsUrl, { headers }).then(r => r.json());
  if (modelsRes && Array.isArray(modelsRes.data)) {
    const availableIds = modelsRes.data.map(m => m.id);
    if (!availableIds.includes(llmModel)) {
      console.log(`⚠️ Configured LLM_MODEL "${llmModel}" is not in the catalog.`);
      const fallback = availableIds.find(id => id.includes('llama-3.3-70b') || id.includes('llama-3.3') || id.includes('llama3.3') || id.includes('llama3') || id.includes('qwen3') || id.includes('free'));
      if (fallback) {
        console.log(`🔄 Falling back to available model: "${fallback}"`);
        llmModel = fallback;
      } else if (availableIds.length > 0) {
        llmModel = availableIds[0];
        console.log(`🔄 Falling back to first available model: "${llmModel}"`);
      }
    }
  }
} catch (err) {
  console.warn('⚠️ Warning: Could not verify model list from LLM endpoint. Using default.', err.message);
}

async function runLLM(systemPrompt, userPrompt) {
  const endpoint = llmBaseUrl.endsWith('/v1') ? `${llmBaseUrl}/chat/completions` : `${llmBaseUrl}/v1/chat/completions`;
  const headers = { 'Content-Type': 'application/json' };
  if (llmApiKey) headers['Authorization'] = `Bearer ${llmApiKey}`;

  const payload = {
    model: llmModel,
    response_format: { type: 'json_object' },
    messages: [
      { role: 'system', content: systemPrompt },
      { role: 'user', content: userPrompt }
    ]
  };

  if (llmBaseUrl.includes('api.openai.com')) {
    payload.max_tokens = 4096;
  } else {
    payload.max_tokens = 4096;
    payload.max_completion_tokens = 4096;
  }

  const body = JSON.stringify(payload);

  const res = await fetch(endpoint, { method: 'POST', headers, body });
  if (!res.ok) {
    const errorText = await res.text();
    throw new Error(`LLM Error: ${res.status} ${errorText}`);
  }
  const data = await res.json();
  return data.choices?.[0]?.message?.content?.trim();
}

async function generateCVHtml(jobUrl, company, role, reportPath) {
    console.log(`\n📄 Generating tailored CV HTML for ${company}...`);
    
    const cvPath = path.join(PROJECT_ROOT, 'cv.md');
    const profilePath = path.join(PROJECT_ROOT, 'config', 'profile.yml');
    const templatePath = path.join(PROJECT_ROOT, 'templates', 'cv-template.html');
    const modesPath = path.join(PROJECT_ROOT, 'modes', 'pdf.md');

    if (!fs.existsSync(cvPath) || !fs.existsSync(templatePath)) {
        throw new Error("Missing cv.md or cv-template.html");
    }

    const cvData = fs.readFileSync(cvPath, 'utf8');
    let profileData = "";
    if (fs.existsSync(profilePath)) {
        profileData = fs.readFileSync(profilePath, 'utf8');
    }
    const templateHtml = fs.readFileSync(templatePath, 'utf8');
    const modesData = fs.readFileSync(modesPath, 'utf8');
    
    let reportData = "";
    if (fs.existsSync(reportPath)) {
        reportData = fs.readFileSync(reportPath, 'utf8');
    }

    const isLightModel = false;
    let processedCvData = cvData;
    let processedProfileData = profileData;

    let additionalInstructions = "";
    if (isLightModel) {
        additionalInstructions = `
IMPORTANT: You are running on a model with output limits. Be concise but complete.
- Include up to 4 most recent/relevant roles in EXPERIENCE.
- Include up to 3 most relevant projects in PROJECTS.
- Keep bullet points concise (max 4-5 per role/project, 1-2 sentences each).
- Include EDUCATION and CERTIFICATIONS briefly.
Do NOT pad with unnecessary text - be precise and information-dense.
`;
    }

    const prompt = `
You are an expert ATS-optimized CV writer.
I need you to fill the placeholders in an HTML CV template based on my cv.md and the job evaluation report.
Follow the rules from modes/pdf.md.
Extract keywords from the JD report, rewrite the Professional Summary, and reorder the experience bullets to match the job requirements.

Here is the profile.yml:
${processedProfileData}

Here is the cv.md:
${processedCvData}

Here is the JD Evaluation Report:
${reportData}

Job: ${role} at ${company}
URL: ${jobUrl}

Output ONLY a raw JSON object (no markdown formatting, no \`\`\`json) with the keys exactly matching the placeholders in cv-template.html (without the curly braces).
Required keys:
- LANG (e.g. "en")
- PAGE_WIDTH ("a4" or "letter")
- NAME
- EMAIL
- LINKEDIN_URL
- LINKEDIN_DISPLAY
- PORTFOLIO_URL
- PORTFOLIO_DISPLAY
- LOCATION
- SECTION_SUMMARY (e.g. "Professional Summary")
- SUMMARY_TEXT
- SECTION_COMPETENCIES (e.g. "Core Competencies")
- COMPETENCIES (HTML string of <span class="competency-tag">...</span>)
- SECTION_EXPERIENCE (e.g. "Work Experience")
- EXPERIENCE (HTML structure matching the template for each job)
- SECTION_PROJECTS (e.g. "Projects")
- PROJECTS (HTML structure for top relevant projects)
- SECTION_EDUCATION (e.g. "Education")
- EDUCATION (HTML structure)
- SECTION_CERTIFICATIONS (e.g. "Certifications")
- CERTIFICATIONS (HTML structure)
- SECTION_SKILLS (e.g. "Skills")
- SKILLS (HTML structure)
${additionalInstructions}
CRITICAL LANGUAGE RULE: Generate ALL CV text in the SAME language as the Job Description. 
- If the JD is in Ukrainian → write CV in Ukrainian
- If the JD is in English → write CV in English
- NEVER write CV content in Russian (the evaluation report may be in Russian, but the CV must match the JD language)
- Set the LANG field accordingly: "uk" for Ukrainian, "en" for English

Do NOT include any extra text outside the JSON. Ensure the JSON is valid.
`;

    function validateJsonBraces(str) {
        let depth = 0;
        for (const ch of str) {
            if (ch === '{') depth++;
            if (ch === '}') depth--;
        }
        return depth;
    }

    function repairTruncatedJson(str) {
        // Track whether we're inside a JSON string, handling escaped quotes
        let inString = false;
        let escapeNext = false;
        let result = '';

        for (let i = 0; i < str.length; i++) {
            const ch = str[i];
            result += ch;

            if (escapeNext) {
                escapeNext = false;
                continue;
            }

            if (ch === '\\' && inString) {
                escapeNext = true;
                continue;
            }

            if (ch === '"') {
                inString = !inString;
            }
        }

        // If we ended inside a string value, close it
        if (inString) {
            result += '"';
        }

        // Balance braces
        let unbalanced = validateJsonBraces(result);
        while (unbalanced > 0) {
            result += '}';
            unbalanced--;
        }

        return result;
    }

    const cvSystemPrompt = `You are a JSON-only output machine. You NEVER explain, analyze, or write prose. You output ONLY a raw JSON object. No markdown. No code fences. No reasoning. No "1." or "**". Just: { ... }`;
    const cvUserPrompt = prompt;

    let rawResult;
    try {
        for (let attempt = 0; attempt < 3; attempt++) {
            rawResult = await runLLM(cvSystemPrompt, cvUserPrompt);
            if (rawResult && rawResult.trim().startsWith('{')) break;
            if (attempt < 2) {
                console.warn(`⚠️ LLM returned non-JSON (attempt ${attempt + 1}), retrying...`);
            }
        }
        if (!rawResult) {
            throw new Error("Empty LLM response after retries");
        }

        // Extract JSON from possible markdown code fences or surrounding text
        let jsonStr = rawResult;

        // 1. If response contains a markdown code block, extract content from inside it
        //    Supports ```json\n...\n```, ```\n...\n```, ```json{...}``` (same line)
        const codeBlockMatch = jsonStr.match(/```(?:json)?\s*([\s\S]*?)```/);
        if (codeBlockMatch) {
            jsonStr = codeBlockMatch[1].trim();
        }

        // 2. Isolate the JSON object by finding first { and last } if they exist
        jsonStr = jsonStr.trim();
        const firstBrace = jsonStr.indexOf('{');
        const lastBrace = jsonStr.lastIndexOf('}');
        if (firstBrace !== -1 && lastBrace !== -1 && lastBrace > firstBrace) {
            jsonStr = jsonStr.slice(firstBrace, lastBrace + 1);
        } else if (firstBrace !== -1) {
            // No closing brace — try extracting from firstBrace and repairing
            jsonStr = jsonStr.slice(firstBrace);
        }

        // 3. Strip control characters that break JSON.parse (e.g. \x00, \x07)
        jsonStr = jsonStr.replace(/[\x00-\x08\x0B\x0C\x0E-\x1F]/g, '');

        // 4. Try to parse — if it fails, attempt truncation repair
        let placeholders;
        try {
            placeholders = JSON.parse(jsonStr);
        } catch (parseErr) {
            // Attempt repair: close unclosed strings and balance braces
            const repaired = repairTruncatedJson(jsonStr);
            try {
                placeholders = JSON.parse(repaired);
                console.warn("⚠️ JSON was truncated — repair succeeded (added closing braces/quotes)");
            } catch (repairErr) {
                // If repair also fails, rethrow the original parse error for clarity
                throw parseErr;
            }
        }
        
        let finalHtml = templateHtml;
        for (const [key, value] of Object.entries(placeholders)) {
            const regex = new RegExp(`{{${key}}}`, 'g');
            finalHtml = finalHtml.replace(regex, value || '');
        }
        
        return finalHtml;
    } catch (err) {
        console.error("❌ Failed to generate CV HTML.");
        if (rawResult) {
            console.error("--- Raw LLM response (first 1000 chars) ---");
            console.error(rawResult.slice(0, 1000));
            console.error("--- End raw excerpt ---");
        }
        console.error("Error:", err.message);
        throw err;
    }
}

export { generateCVHtml };

// Allow running standalone for testing
if (process.argv[1] === fileURLToPath(import.meta.url)) {
    const jobUrl = process.argv[2] || "https://djinni.co/jobs/123-test";
    const company = process.argv[3] || "TestCompany";
    const role = process.argv[4] || "TestRole";
    const reportPath = process.argv[5] || path.join(PROJECT_ROOT, 'reports', 'test.md');
    
    generateCVHtml(jobUrl, company, role, reportPath).then(html => {
        fs.writeFileSync(path.join(PROJECT_ROOT, `output/test-cv-${company}.html`), html);
        console.log(`Saved test HTML to output/test-cv-${company}.html`);
    }).catch(err => {
        console.error(err);
        process.exit(1);
    });
}
