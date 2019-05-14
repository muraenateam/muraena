package crawler

import "strings"

func Contains(slice *[]string, find string) bool {
	for _, a := range *slice {
		if a == find {
			return true
		}
	}
	return false
}

func IsSubdomain(ref string, toCheck string) bool {
	if strings.HasSuffix(toCheck, ref) {
		return true
	}
	return false
}
