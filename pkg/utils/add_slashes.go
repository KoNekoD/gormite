package utils

import "bytes"

func AddSlashes(s string) string {
	if ln := len(s); ln == 0 {
		return ""
	}

	var buf bytes.Buffer
	for _, char := range s {
		switch char {
		case '\'', '"', '\\':
			buf.WriteRune('\\')
		}
		buf.WriteRune(char)
	}

	return buf.String()
}
