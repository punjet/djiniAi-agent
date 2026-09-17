package covergen

import "strings"

// NormalizeForATS replaces Unicode characters that ATS (Applicant Tracking System)
// parsers cannot reliably handle with their plain ASCII equivalents and removes
// invisible / zero-width characters. This runs on the HTML source before Folio
// renders it so that the text layer in the resulting PDF is ATS-clean.
func NormalizeForATS(text string) string {
	replacer := strings.NewReplacer(
		// Em-dashes and dash variants → hyphen-minus
		"\u2014", "-", // — EM DASH
		"\u2013", "-", // – EN DASH
		"\u2012", "-", // ‒ FIGURE DASH
		"\u2015", "-", // ― HORIZONTAL BAR

		// Smart / curly single quotes → apostrophe
		"\u2018", "'", // ' LEFT SINGLE QUOTATION MARK
		"\u2019", "'", // ' RIGHT SINGLE QUOTATION MARK
		"\u201A", "'", // ‚ SINGLE LOW-9 QUOTATION MARK
		"\u2039", "'", // ‹ SINGLE LEFT-POINTING ANGLE QUOTATION MARK
		"\u203A", "'", // › SINGLE RIGHT-POINTING ANGLE QUOTATION MARK

		// Smart / curly double quotes → quotation mark
		"\u201C", "\"", // " LEFT DOUBLE QUOTATION MARK
		"\u201D", "\"", // " RIGHT DOUBLE QUOTATION MARK
		"\u201E", "\"", // „ DOUBLE LOW-9 QUOTATION MARK
		"\u00AB", "\"", // « LEFT-POINTING DOUBLE ANGLE QUOTATION MARK
		"\u00BB", "\"", // » RIGHT-POINTING DOUBLE ANGLE QUOTATION MARK

		// Non-breaking and special spaces → regular space
		"\u00A0", " ", // NBSP
		"\u202F", " ", // NARROW NO-BREAK SPACE
		"\u2009", " ", // THIN SPACE
		"\u2003", " ", // EM SPACE
		"\u2002", " ", // EN SPACE
		"\u2007", " ", // FIGURE SPACE
		"\u2008", " ", // PUNCTUATION SPACE

		// Ellipsis → three dots
		"\u2026", "...", // … HORIZONTAL ELLIPSIS

		// Common bullet / list variants → hyphen-minus
		"\u2022", "-", // • BULLET
		"\u2023", "-", // ‣ TRIANGULAR BULLET
		"\u25E6", "-", // ◦ WHITE BULLET
		"\u2043", "-", // ⁃ HYPHEN BULLET
		"\u204C", "-", // ⁌ BLACK LEFTWARDS BULLET
		"\u204D", "-", // ⁍ BLACK RIGHTWARDS BULLET

		// Remove zero-width and invisible characters
		"\u200B", "", // ZERO WIDTH SPACE
		"\u200C", "", // ZERO WIDTH NON-JOINER
		"\u200D", "", // ZERO WIDTH JOINER
		"\uFEFF", "", // BOM / ZERO WIDTH NO-BREAK SPACE
		"\u00AD", "", // SOFT HYPHEN (invisible, confuses ATS)
	)
	return replacer.Replace(text)
}
