package measurement

import "testing"

func TestObjectives(t *testing.T) {
	for _, tc := range []struct {
		kind                                  Kind
		unit, sample, sampleUnit, attribution string
	}{
		{CPU, "ns/op", "cpu", "nanoseconds", "self"},
		{Memory, "B/op", "alloc_space", "bytes", "first_focus_frame"},
		{Memory, "allocs/op", "alloc_objects", "count", "first_focus_frame"},
	} {
		t.Run(tc.unit, func(t *testing.T) {
			c, err := Resolve(tc.kind, tc.unit)
			if err != nil {
				t.Fatal(err)
			}
			if c.SampleType != tc.sample || c.SampleUnit != tc.sampleUnit || c.Attribution != tc.attribution {
				t.Fatalf("config = %+v", c)
			}
			if err := c.Validate(); err != nil {
				t.Fatal(err)
			}
			if got, err := Resolve("", tc.unit); err != nil || got != c {
				t.Fatalf("inference = %+v, %v", got, err)
			}
			metrics := c.Metrics()
			if metrics[0].Role != Objective || metrics[0].Unit != tc.unit {
				t.Fatalf("metrics = %+v", metrics)
			}
			if tc.kind == Memory && (metrics[1].Role != Guard || metrics[1].Unit != "ns/op") {
				t.Fatalf("missing CPU protection: %+v", metrics)
			}
			c.SampleUnit = "wrong"
			if c.Validate() == nil {
				t.Fatal("inconsistent unit accepted")
			}
		})
	}
}

func TestDefaultsAndInvalidCombinations(t *testing.T) {
	cpu, _ := Resolve("", "")
	mem, _ := Resolve(Memory, "")
	if cpu.Unit != "ns/op" || mem.Unit != "B/op" {
		t.Fatalf("defaults: %+v %+v", cpu, mem)
	}
	for _, tc := range []struct {
		kind Kind
		unit string
	}{{CPU, "B/op"}, {Memory, "ns/op"}, {Memory, "inuse_space"}, {"heap", ""}} {
		if _, err := Resolve(tc.kind, tc.unit); err == nil {
			t.Fatalf("accepted %+v", tc)
		}
	}
	if (Config{}).Validate() == nil {
		t.Fatal("accepted incomplete configuration")
	}
}
