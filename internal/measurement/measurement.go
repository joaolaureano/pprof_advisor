// Package measurement defines the supported optimization objectives. It has no
// knowledge of processes, profiles on disk, CLI flags, or model providers.
package measurement

import "fmt"

type Kind string

const (
	CPU           Kind = "cpu"
	Memory        Kind = "memory"
	Block         Kind = "block"
	Mutex         Kind = "mutex"
	Objective          = "objective"
	Guard              = "guard"
	Informational      = "informational"
)

// Config is resolved once for a run and shared by all of its stages.
type Config struct {
	Profile     Kind   `json:"profile"`
	Unit        string `json:"unit"`
	SampleType  string `json:"sample_type"`
	SampleUnit  string `json:"sample_unit"`
	Attribution string `json:"attribution"`
}

type Metric struct {
	Unit          string `json:"unit"`
	Role          string `json:"role"`
	LowerIsBetter bool   `json:"lower_is_better"`
}

// Resolve accepts the benchmark unit rather than a pprof sample name. An omitted
// profile is inferred from an explicit memory unit, otherwise it defaults to CPU.
func Resolve(kind Kind, unit string) (Config, error) {
	if kind == "" {
		kind = CPU
		if unit == "B/op" || unit == "allocs/op" {
			kind = Memory
		}
	}
	c := Config{Profile: kind, Unit: unit}
	switch kind {
	case CPU:
		if unit == "" {
			c.Unit = "ns/op"
		}
		if c.Unit != "ns/op" {
			return Config{}, fmt.Errorf("cpu profile requires ns/op, got %q", unit)
		}
		c.SampleType, c.SampleUnit, c.Attribution = "cpu", "nanoseconds", "self"
	case Memory:
		if unit == "" {
			c.Unit = "B/op"
		}
		c.Attribution = "first_focus_frame"
		switch c.Unit {
		case "B/op":
			c.SampleType, c.SampleUnit = "alloc_space", "bytes"
		case "allocs/op":
			c.SampleType, c.SampleUnit = "alloc_objects", "count"
		default:
			return Config{}, fmt.Errorf("memory profile requires B/op or allocs/op, got %q", unit)
		}
	case Block, Mutex:
		if unit == "" {
			c.Unit = "ns/op"
		}
		if c.Unit != "ns/op" {
			return Config{}, fmt.Errorf("%s profile requires ns/op, got %q", kind, unit)
		}
		// The leaf of a contention sample is runtime.chanrecv or
		// sync.(*Mutex).Lock, never the code that caused the wait, so cost is
		// charged the same way an allocation is: to the innermost frame inside
		// the code under test.
		c.SampleType, c.SampleUnit, c.Attribution = "delay", "nanoseconds", "first_focus_frame"
	default:
		return Config{}, fmt.Errorf("unsupported profile %q (want cpu, memory, block, or mutex)", kind)
	}
	return c, nil
}

func (c Config) Metrics() []Metric {
	metrics := []Metric{{Unit: c.Unit, Role: Objective, LowerIsBetter: true}}
	if c.Profile == Memory {
		other := "allocs/op"
		if c.Unit == other {
			other = "B/op"
		}
		metrics = append(metrics, Metric{Unit: "ns/op", Role: Guard, LowerIsBetter: true}, Metric{Unit: other, Role: Informational, LowerIsBetter: true})
	} else {
		metrics = append(metrics, Metric{Unit: "B/op", Role: Informational, LowerIsBetter: true}, Metric{Unit: "allocs/op", Role: Informational, LowerIsBetter: true})
	}
	return metrics
}

// Validate rejects inconsistent metadata instead of silently interpreting costs
// in a different unit or applying a different acceptance policy.
func (c Config) Validate() error {
	want, err := Resolve(c.Profile, c.Unit)
	if err != nil {
		return err
	}
	if c != want {
		return fmt.Errorf("incomplete or inconsistent measurement configuration")
	}
	return nil
}
