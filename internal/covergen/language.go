package covergen

import "github.com/abadojack/whatlanggo"

func DetectJDLanguage(jdText string) string {
	info := whatlanggo.Detect(jdText)
	if info.Lang == whatlanggo.Eng {
		return "English"
	}
	return "Ukrainian"
}
