package notify

import (
	"regexp"
	"strings"
)

func ParseMarkdownToRichMessage(md string) InputRichMessage {
	var msg InputRichMessage

	lines := strings.Split(md, "\n")

	var currentDetails *InputRichBlockDetails

	blockHeaderRegex := regexp.MustCompile(`^###\s+((?i)Block\s+[A-G]|Блок\s+[A-GА-Я]|[A-G]\)).*`)
	headingRegex := regexp.MustCompile(`^#+\s+(.*)`)
	boldKeyRegex := regexp.MustCompile(`^\*\*([^\*]+)\*\*(.*)`)
	listRegex := regexp.MustCompile(`^[\-\*•]\s+(.*)`)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if blockHeaderRegex.MatchString(line) {
			if currentDetails != nil {
				msg.Blocks = append(msg.Blocks, *currentDetails)
			}
			summaryText := strings.TrimSpace(strings.TrimLeft(line, "# "))
			currentDetails = &InputRichBlockDetails{
				Type:    "details",
				Summary: summaryText,
				Blocks:  []interface{}{},
				IsOpen:  false,
			}
			continue
		}

		var para InputRichBlockParagraph
		para.Type = "paragraph"

		if match := headingRegex.FindStringSubmatch(line); match != nil {
			para.Text = []interface{}{
				RichTextBold{
					Type: "bold",
					Text: match[1],
				},
			}
		} else if match := boldKeyRegex.FindStringSubmatch(line); match != nil {
			para.Text = []interface{}{
				RichTextBold{
					Type: "bold",
					Text: match[1],
				},
				match[2],
			}
		} else if match := listRegex.FindStringSubmatch(line); match != nil {
			para.Text = []interface{}{
				"• " + match[1],
			}
		} else {
			para.Text = []interface{}{line}
		}

		if currentDetails != nil {
			currentDetails.Blocks = append(currentDetails.Blocks, para)
		} else {
			msg.Blocks = append(msg.Blocks, para)
		}
	}

	if currentDetails != nil {
		msg.Blocks = append(msg.Blocks, *currentDetails)
	}

	return msg
}
