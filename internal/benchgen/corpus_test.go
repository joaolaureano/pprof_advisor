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
