package corpus

var globalPtr *int

// leakToResult returns the param, so it leaks to result.
func leakToResult(p *int) *int {
	return p
}

// leakToHeap stores param in global, so it leaks to heap.
func leakToHeap(p *int) {
	globalPtr = p
}

// readOnly receives a param but only reads it, does not escape.
func readOnly(p *int) int {
	return *p
}

// leakContent takes a pointer to slice, content leaks when appended to global.
var globalSlice []int

func leakContent(p *[]int) {
	globalSlice = append(globalSlice, (*p)...)
}

// leakContentDirect stores a reference to the content of a parameter.
var globalInt *int

func leakContentDirect(p *[]*int) {
	if len(*p) > 0 {
		globalInt = (*p)[0]
	}
}

// unused receives a param that is never read or modified.
func unused(p *int) {
	// p is not used
}

// inert receives a param that is never read, written, or called.
func inert(p interface{}) {
	// p is truly inert
}

// funcParamInert receives a function pointer that is never called.
func funcParamInert(fn func()) {
	// fn is never called
}

// ptrParamInert receives a pointer that is never dereferenced.
func ptrParamInert(p *struct{}) {
	_ = p
}
