package tracking

import "strings"

// InnerSubstring returns the string contained between prefix and suffix
func InnerSubstring(str string, prefix string, suffix string) string {
	var sIdx, eIdx int
	sIdx = strings.Index(str, prefix)
	if sIdx == -1 {
		sIdx = 0
		eIdx = 0
	} else if len(prefix) == 0 {
		sIdx = 0
		eIdx = strings.Index(str, suffix)
		if eIdx == -1 || len(suffix) == 0 {
			eIdx = len(str)
		}
	} else {
		sIdx += len(prefix)
		eIdx = strings.Index(str[sIdx:], suffix)
		if eIdx == -1 {
			if strings.Index(str, suffix) < sIdx {
				eIdx = sIdx
			} else {
				eIdx = len(str)
			}
		} else {
			if len(suffix) == 0 {
				eIdx = len(str)
			} else {
				eIdx += sIdx
			}
		}
	}

	return str[sIdx:eIdx]
}
