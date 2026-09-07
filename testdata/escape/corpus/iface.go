package corpus

import "fmt"

// ifaceEscapes demonstrates a value boxed into an interface and passed to fmt.
func ifaceEscapes(x int) string {
	return fmt.Sprintf("%v", x)
}
