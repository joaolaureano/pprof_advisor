package matcher

import (
	"fmt"
	"testing"
)

func table() *Table {
	patterns := make([]string, 0, 64)
	for i := 0; i < 50; i++ {
		patterns = append(patterns, fmt.Sprintf("/assets/%d/static/file", i))
	}
	patterns = append(patterns,
		"/api/v1/users/:user/posts/:post",
		"/api/v1/health",
	)
	return New(patterns)
}

func BenchmarkStatic(b *testing.B) {
	t := table()
	b.ReportAllocs()
	for b.Loop() {
		if !t.Match("/api/v1/health") {
			b.Fatal("no match")
		}
	}
}

func BenchmarkParam(b *testing.B) {
	t := table()
	b.ReportAllocs()
	for b.Loop() {
		if t.Params("/api/v1/users/42/posts/7") == nil {
			b.Fatal("no match")
		}
	}
}

func BenchmarkFanout50(b *testing.B) {
	t := table()
	b.ReportAllocs()
	for b.Loop() {
		if !t.Match("/assets/49/static/file") {
			b.Fatal("no match")
		}
	}
}

func TestMatchAndParams(t *testing.T) {
	tab := table()
	if !tab.Match("/api/v1/health") {
		t.Error("static pattern did not match")
	}
	if tab.Match("/api/v1/nope") {
		t.Error("unknown path matched")
	}
	got := tab.Params("/api/v1/users/42/posts/7")
	if got["user"] != "42" || got["post"] != "7" {
		t.Errorf("Params = %v", got)
	}
}
