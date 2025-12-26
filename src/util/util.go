package util

import "strings"

func PadStringsF[T any](padVal string, arr []T, get func(*T) string, set func(val *T, newValue string)) {
	maxLen := 0
	for i := range arr {
		val := get(&arr[i])
		maxLen = max(maxLen, len(val))
	}

	for i := range arr {
		val := get(&arr[i])
		if len(val) < maxLen {
			set(&arr[i], val+strings.Repeat(padVal, maxLen-len(val)))
		}
	}
}
