package utils

import "strconv"

// TryParseInt tries to parse a string as an integer
// Returns the integer value and whether parsing was successful
func TryParseInt(s string) (int, bool) {
	num, err := strconv.Atoi(s)
	return num, err == nil
}

func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
