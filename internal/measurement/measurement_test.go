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

func TestContentionObjectives(t *testing.T) {
	for _, kind := range []Kind{Block, Mutex} {
		t.Run(string(kind), func(t *testing.T) {
			// Test Resolve(kind, "ns/op") succeeds with correct fields
			c, err := Resolve(kind, "ns/op")
			if err != nil {
				t.Fatalf("Resolve(%q, %q): %v", kind, "ns/op", err)
			}
			if c.SampleType != "delay" {
				t.Fatalf("Resolve(%q, %q).SampleType = %q, want %q", kind, "ns/op", c.SampleType, "delay")
			}
			if c.SampleUnit != "nanoseconds" {
				t.Fatalf("Resolve(%q, %q).SampleUnit = %q, want %q", kind, "ns/op", c.SampleUnit, "nanoseconds")
			}
			if c.Attribution != "first_focus_frame" {
				t.Fatalf("Resolve(%q, %q).Attribution = %q, want %q", kind, "ns/op", c.Attribution, "first_focus_frame")
			}
			if c.Unit != "ns/op" {
				t.Fatalf("Resolve(%q, %q).Unit = %q, want %q", kind, "ns/op", c.Unit, "ns/op")
			}

			// Test Resolve(kind, "") yields the same config (default unit is ns/op)
			cDefault, err := Resolve(kind, "")
			if err != nil {
				t.Fatalf("Resolve(%q, %q): %v", kind, "", err)
			}
			if cDefault != c {
				t.Fatalf("Resolve(%q, %q) = %+v, want %+v", kind, "", cDefault, c)
			}

			// Test Validate() returns nil
			if err := c.Validate(); err != nil {
				t.Fatalf("Resolve(%q, %q).Validate(): %v", kind, "ns/op", err)
			}

			// Test Metrics()
			metrics := c.Metrics()
			if len(metrics) == 0 {
				t.Fatalf("Resolve(%q, %q).Metrics() returned empty slice", kind, "ns/op")
			}
			if metrics[0].Role != Objective {
				t.Fatalf("Resolve(%q, %q).Metrics()[0].Role = %q, want %q", kind, "ns/op", metrics[0].Role, Objective)
			}
			if metrics[0].Unit != "ns/op" {
				t.Fatalf("Resolve(%q, %q).Metrics()[0].Unit = %q, want %q", kind, "ns/op", metrics[0].Unit, "ns/op")
			}

			// Test Resolve(kind, "B/op") returns an error
			if _, err := Resolve(kind, "B/op"); err == nil {
				t.Fatalf("Resolve(%q, %q) should return error", kind, "B/op")
			}

			// Test Resolve(kind, "allocs/op") returns an error
			if _, err := Resolve(kind, "allocs/op"); err == nil {
				t.Fatalf("Resolve(%q, %q) should return error", kind, "allocs/op")
			}

			// Test that modifying SampleUnit makes Validate() return error
			c.SampleUnit = "wrong"
			if c.Validate() == nil {
				t.Fatalf("After c.SampleUnit = \"wrong\", Validate() should return error, got nil")
			}

			// Test Resolve("", "ns/op") returns CPU, not the contention kind
			cpuConfig, err := Resolve("", "ns/op")
			if err != nil {
				t.Fatalf("Resolve(%q, %q): %v", "", "ns/op", err)
			}
			if cpuConfig.Profile != CPU {
				t.Fatalf("Resolve(%q, %q).Profile = %q, want %q", "", "ns/op", cpuConfig.Profile, CPU)
			}
		})
	}
}
