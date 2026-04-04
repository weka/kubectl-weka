package utils

import "sort"

func KeysOf(m map[string]struct{}) []string {
	if len(m) == 0 {
		return nil
	}
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func KeysOfSorted(m map[string]struct{}) []string {
	out := KeysOf(m)
	sort.Strings(out)
	return out
}
