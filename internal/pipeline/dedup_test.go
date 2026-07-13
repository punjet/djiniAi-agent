package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDedup(t *testing.T) {
	tmp := t.TempDir()

	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}

	historyContent := "url\tfirst_seen\tportal\ttitle\tcompany\tstatus\n" +
		"https://djinni.co/jobs/111-go-dev/\t2026-04-13\tDjinni\tGo Developer\tAcme\tadded\n" +
		"https://djinni.co/jobs/333-unicode/?source=search\t2026-04-13\tDjinni\tGo Developer\tТОВ Рога\tadded\n"

	appsContent := "# Applications Tracker\n\n" +
		"| # | Fecha | Empresa | Rol | Score | Estado | PDF | Report | Rejection Reason |\n" +
		"|---|-------|---------|-----|-------|--------|-----|--------|------------------|\n" +
		"| 27 | 2026-04-13 | Globex | Senior AI Solutions Architect | 3.6/5 | Skipped | ❌ | [006](reports/006-globex.md) | — |\n" +
		"| 28 | 2026-04-13 | ТОВ Рога | Golang розробник | 4.0/5 | Skipped | ❌ | [007](reports/007.md) | — |\n" +
		"| 29 | 2026-04-13 | ShortCorp | Dev | 4.0/5 | Skipped | ❌ | [008](reports/008.md) | — |\n"

	if err := os.WriteFile(filepath.Join(dataDir, "scan-history.tsv"), []byte(historyContent), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataDir, "applications.md"), []byte(appsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	d, err := LoadDedup(tmp)
	if err != nil {
		t.Fatalf("LoadDedup failed: %v", err)
	}

	// Test URL seen in scan-history
	if d.IsNew("https://djinni.co/jobs/111-go-dev/", "Acme", "Go Developer") {
		t.Error("expected seen URL to be blocked")
	}

	// Test URL seen in applications.md link
	if d.IsNew("reports/006-globex.md", "Globex", "Solutions Architect") {
		t.Error("expected report URL link to be blocked")
	}

	// Test exact company-role pair from applications.md
	if d.IsNew("https://djinni.co/jobs/new-job/", "Globex", "Senior AI Solutions Architect") {
		t.Error("expected exact company/role match to be blocked")
	}

	// Test fuzzy role match under same company
	if d.IsNew("https://djinni.co/jobs/new-job/", "Globex", "AI Solutions Architect") {
		t.Error("expected fuzzy role overlap (AI Solutions Architect) to be blocked")
	}

	// Test new company-role
	if !d.IsNew("https://djinni.co/jobs/new-job/", "Globex", "Golang Engineer") {
		t.Error("expected new role (Golang Engineer) to be allowed")
	}
	if !d.IsNew("https://djinni.co/jobs/new-job/", "OtherCorp", "Senior AI Solutions Architect") {
		t.Error("expected different company (OtherCorp) with same role to be allowed")
	}

	// Unicode company/roles check
	if d.IsNew("https://djinni.co/jobs/new-job-unicode/", "ТОВ Рога", "Golang розробник") {
		t.Error("expected exact Unicode company/role to be blocked")
	}

	// Same URL, different params
	if d.IsNew("https://djinni.co/jobs/111-go-dev/?utm_source=test", "Acme", "Go Developer") {
		t.Error("expected URL with query parameters to be blocked if base URL is seen")
	}

	// Short roles check (words <= 3 chars usually filtered in fuzzy matching, but exact should still match)
	if d.IsNew("https://djinni.co/jobs/new-job-short/", "ShortCorp", "Dev") {
		t.Error("expected exact short company/role match to be blocked")
	}
}
