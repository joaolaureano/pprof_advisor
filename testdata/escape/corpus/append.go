package corpus

// appendGrows demonstrates an append operation that grows past capacity.
func appendGrows(s []int) []int {
	return append(s, 42)
}
