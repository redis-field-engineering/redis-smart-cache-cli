package util

import "strings"

func CenterString(str string, width int) string {
	if len(str) > width {
		return str[0:width-3] + "..."
	}
	spaces := int(float64(width-len(str)) / 2)
	return strings.Repeat(" ", spaces) + str + strings.Repeat(" ", width-(spaces+len(str)))
}
