package main

import (
	"bytes"
	"flag"
	"reflect"
	"strings"
	"testing"
)

func TestParseInterspersed(t *testing.T) {
	cases := []struct {
		args   []string
		wantP  []string
		wantSk string
	}{
		{[]string{"demo", "--socket", "s"}, []string{"demo"}, "s"},
		{[]string{"--socket", "s", "demo"}, []string{"demo"}, "s"},
		{[]string{"a", "--socket=s", "b"}, []string{"a", "b"}, "s"},
		{nil, nil, ""},
	}
	for _, c := range cases {
		fs := flag.NewFlagSet("t", flag.ContinueOnError)
		sk := fs.String("socket", "", "")
		pos, err := parseInterspersed(fs, c.args)
		if err != nil {
			t.Fatalf("%v: %v", c.args, err)
		}
		if !reflect.DeepEqual(pos, c.wantP) || *sk != c.wantSk {
			t.Errorf("%v: pos=%v socket=%q, want %v %q", c.args, pos, *sk, c.wantP, c.wantSk)
		}
	}
}

func TestRunDispatch(t *testing.T) {
	var out, errOut bytes.Buffer

	if code := run(nil, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "usage:") {
		t.Errorf("no args: code=%d err=%q", code, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	if code := run([]string{"help"}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "restore") {
		t.Errorf("help: code=%d out=%q", code, out.String())
	}

	out.Reset()
	errOut.Reset()
	if code := run([]string{"bogus"}, &out, &errOut); code != 2 || !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("bogus: code=%d err=%q", code, errOut.String())
	}

	out.Reset()
	errOut.Reset()
	if code := run([]string{"restore"}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "exactly one") {
		t.Errorf("restore without arg: code=%d err=%q", code, errOut.String())
	}
}

func TestEveryCommandIsComplete(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range commands {
		if c.name == "" || c.usage == "" || c.summary == "" || c.run == nil {
			t.Errorf("incomplete command: %+v", c)
		}
		if seen[c.name] {
			t.Errorf("duplicate command %q", c.name)
		}
		seen[c.name] = true
	}
}
