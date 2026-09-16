package covergen

import (
	"context"
	"testing"
)

func TestRenderHTMLToPDFWithFolioFallback(t *testing.T) {
	ctx := context.Background()
	htmlContent := "<h1>Test</h1>"

	pdfBytes, err := renderHTMLToPDF(ctx, htmlContent)
	if err != nil {
		t.Fatalf("renderHTMLToPDF failed: %v", err)
	}

	if len(pdfBytes) < 5 {
		t.Fatalf("PDF bytes too short")
	}

	signature := string(pdfBytes[:5])
	if signature != "%PDF-" {
		t.Errorf("Expected %%PDF- signature, got %s", signature)
	}
}
