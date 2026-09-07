package benchgen

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDecodeSeed(t *testing.T) {
	for _, tc := range []struct {
		body, typ string
		want      []byte
		bad       bool
	}{
		{`string("hello")`, "string", []byte("hello"), false},
		{`[]byte("\x00\xff\n")`, "[]byte", []byte{0, 255, 10}, false},
		{`string("")`, "string", []byte{}, false},
		{`[]byte("x")`, "string", nil, true},
		{`"x"`, "[]byte", nil, true},
		{`[]byte{1}`, "[]byte", nil, true},
		{`[]byte(call())`, "[]byte", nil, true},
		{"\"x\"\n\"y\"", "string", nil, true},
		{`"x" + "y"`, "string", nil, true},
		{`"x"`, "string", nil, true},
	} {
		t.Run(tc.body+tc.typ, func(t *testing.T) {
			got, err := decodeSeed("go test fuzz v1\n"+tc.body+"\n", tc.typ)
			if (err != nil) != tc.bad {
				t.Fatalf("error %v", err)
			}
			if !tc.bad && !bytes.Equal(got, tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
	if _, err := decodeSeed("go test fuzz v2\n\"x\"", "string"); err == nil {
		t.Fatal("accepted unknown header")
	}
}

func TestCorpusStableDeduplicated(t *testing.T) {
	dir := t.TempDir()
	if _, err := loadCorpus(dir, "string"); err == nil {
		t.Fatal("accepted empty corpus")
	}
	for name, value := range map[string]string{"z": "foo", "a": "bar", "b": "foo"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("go test fuzz v1\nstring(\""+value+"\")\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	seeds, err := loadCorpus(dir, "string")
	if err != nil {
		t.Fatal(err)
	}
	if len(seeds) != 2 || seeds[0].Hash >= seeds[1].Hash {
		t.Fatalf("bad seed order: %+v", seeds)
	}
	for _, seed := range seeds {
		if string(seed.Data) == "foo" && len(seed.Origins) != 2 {
			t.Fatal("duplicate origins lost")
		}
	}
	again, err := loadCorpus(dir, "string")
	if err != nil || !reflect.DeepEqual(seeds, again) {
		t.Fatal("unstable corpus")
	}
}

func TestDecodeScalarSeedCanonicalizesAndRejectsMismatches(t *testing.T) {
	for _, tc := range []struct {
		input, body, want string
		bad               bool
	}{
		{"bool", "bool(true)", "bool(true)", false},
		{"int8", "int8(-7)", "int8(-7)", false},
		{"uint16", "uint16(0xff)", "uint16(255)", false},
		{"int32", "int32(65)", "int32(65)", false},
		{"float32", "float32(1.5)", "float32(1.5)", false},
		{"float64", "float64(-2e3)", "float64(-2000)", false},
		{"uint8", "int(1)", "", true},
		{"int8", "int8(128)", "", true},
		{"uint", "uint(-1)", "", true},
		{"bool", "bool(1)", "", true},
		{"float64", "float64(NaN)", "", true},
	} {
		t.Run(tc.input+tc.body, func(t *testing.T) {
			data, literal, err := decodeCorpusSeed("go test fuzz v1\n"+tc.body+"\n", tc.input)
			if (err != nil) != tc.bad {
				t.Fatalf("err=%v", err)
			}
			if !tc.bad && (string(data) != tc.want || literal != tc.want) {
				t.Fatalf("data=%q literal=%q want=%q", data, literal, tc.want)
			}
		})
	}
}
