package covergen

import (
	"regexp"
)

type Profile struct {
	Candidate struct {
		FullName string `yaml:"full_name"`
		Email    string `yaml:"email"`
		Phone    string `yaml:"phone"`
		Location string `yaml:"location"`
		Linkedin string `yaml:"linkedin"`
		Github   string `yaml:"github"`
	} `yaml:"candidate"`
	TargetRoles struct {
		Primary []string `yaml:"primary"`
	} `yaml:"target_roles"`
}

type Payload struct {
	Candidate  map[string]interface{} `json:"candidate"`
	Letter     map[string]interface{} `json:"letter"`
	OutputPath string                 `json:"output_path,omitempty"`
}

type GeneratedLetter struct {
	Letter        map[string]interface{} `json:"letter"`
	DjinniMessage string                 `json:"djinni_message"`
}

type CVContent struct {
	SummaryText        string `json:"summary_text"`
	CompetenciesHTML   string `json:"competencies_html"`
	ExperienceHTML     string `json:"experience_html"`
	ProjectsHTML       string `json:"projects_html"`
	EducationHTML      string `json:"education_html"`
	CertificationsHTML string `json:"certifications_html"`
	SkillsHTML         string `json:"skills_html"`
}

var placeholderPattern = regexp.MustCompile(`\{\{([A-Z][A-Z_]+)\}\}`)
