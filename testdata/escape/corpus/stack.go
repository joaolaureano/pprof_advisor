package corpus

// localStruct demonstrates a struct allocated locally, does not escape.
func localStruct() int {
	type S struct {
		a int
		b int
	}
	s := S{a: 1, b: 2}
	return s.a
}

// localSlice demonstrates a slice used only locally, does not escape.
func localSlice() int {
	s := make([]int, 10)
	s[0] = 42
	return s[0]
}

// localArray demonstrates a small array that stays on stack.
func localArray() int {
	arr := [5]int{1, 2, 3, 4, 5}
	return arr[0]
}
