package util

import "strings"

func CenterString(str string, width int) string {
	if len(str) > width {
		return str[0:width-3] + "..."
	}
	spaces := int(float64(width-len(str)) / 2)
	return strings.Repeat(" ", spaces) + str + strings.Repeat(" ", width-(spaces+len(str)))
}

func Remove[T any](slice []T, s int) []T {
	return append(slice[:s], slice[s+1:]...)
}

func CompareSlices(slice []string, other []string) bool {
	if len(slice) == 0 && len(other) == 0 {
		return true
	}
	if len(slice) != len(other) {
		return false
	}

	for i, _ := range slice {
		if slice[i] != other[i] {
			return false
		}
	}

	return true
}
