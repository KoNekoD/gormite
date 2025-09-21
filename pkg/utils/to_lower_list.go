package utils

import "strings"

func ToLowerList(stringList []string) []string {
	out := make([]string, len(stringList))
	for i, s := range stringList {
		out[i] = strings.ToLower(s)
	}
	return out
}
