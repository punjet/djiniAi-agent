package covergen

import (
	"fmt"
	"strings"
)

func ValidateCVHTML(htmlContent string) error {
	if strings.Contains(htmlContent, "{{") || strings.Contains(htmlContent, "}}") {
		return fmt.Errorf("generated CV contains unresolved template placeholders (found '{{' or '}}')")
	}
	return nil
}
