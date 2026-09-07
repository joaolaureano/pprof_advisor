package corpus

// allocHeap demonstrates a pointer escaping to heap via return.
func allocHeap() *int {
	x := 42
	return &x
}

// sliceHeap demonstrates a slice with non-constant cap escaping to heap.
func sliceHeap(n int) []int {
	return make([]int, n)
}
