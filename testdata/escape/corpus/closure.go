package corpus

// closureByRef demonstrates a closure capturing a variable by reference and escaping.
func closureByRef() func() int {
	n := 42
	return func() int {
		n++
		return n
	}
}
