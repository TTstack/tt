package commandline

import (
	"flag"
	"io"
	"testing"
)

func TestLoneDashRemainsAStringOptionValue(t *testing.T) {
	fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	value := fs.String("data-dir", "", "")
	help := fs.Bool("help", false, "")
	args := []string{"--data-dir", "-", "--help"}
	if err := CheckFlags(fs, args); err != nil {
		t.Fatal(err)
	}
	if err := fs.Parse(args); err != nil {
		t.Fatal(err)
	}
	if *value != "-" || !*help {
		t.Fatal(*value, *help)
	}
}

func TestResourceAndPortFlagsUseDecimalRustIntegers(t *testing.T) {
	for _, tc := range []struct {
		text  string
		bits  int
		want  uint
		valid bool
	}{{"010", 32, 10, true}, {"08", 32, 8, true}, {"+12", 32, 12, true}, {"0x10", 32, 0, false}, {"0b10", 32, 0, false}, {"1_000", 32, 0, false}, {"4294967295", 32, 4294967295, true}, {"4294967296", 32, 0, false}, {"65535", 16, 65535, true}, {"65536", 16, 0, false}} {
		fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		var value uint
		UintVar(fs, &value, "number", 0, tc.bits, "")
		err := fs.Parse([]string{"--number", tc.text})
		if (err == nil) != tc.valid || (tc.valid && value != tc.want) {
			t.Fatal(tc, value, err)
		}
	}
}

func TestSignedIntegerFlagsAllowNegativeValuesWithoutAcceptingOptionsAsValues(t *testing.T) {
	for _, tc := range []struct {
		args  []string
		valid bool
	}{
		{[]string{"--grace", "-1"}, true},
		{[]string{"--grace=-1"}, true},
		{[]string{"--grace", "--help"}, false},
		{[]string{"--grace", "-invalid"}, false},
		{[]string{"--data-dir", "-1"}, false},
		{[]string{"--cpu", "-1"}, false},
	} {
		fs := flag.NewFlagSet("fixture", flag.ContinueOnError)
		fs.Int("grace", -1, "")
		fs.String("data-dir", "", "")
		var cpu uint
		UintVar(fs, &cpu, "cpu", 0, 32, "")
		if err := CheckFlags(fs, tc.args); (err == nil) != tc.valid {
			t.Fatal(tc.args, err)
		}
	}
}
