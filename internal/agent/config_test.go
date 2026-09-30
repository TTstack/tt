package agent

import (
	"bytes"
	"context"
	"io"
	"testing"
)

func TestParseMeminfo(t *testing.T) {
	for _, tc := range []struct {
		input string
		value uint32
		ok    bool
	}{{"MemTotal:       131696312 kB\nMemFree: 1234567 kB", 131696312 / 1024, true}, {"nothing here", 0, false}, {"MemTotal: notanumber kB", 0, false}} {
		value, ok := parseMemTotal(tc.input)
		if value != tc.value || ok != tc.ok {
			t.Fatal(value, ok)
		}
	}
}
func TestAgentConfigurationDefaultsExplicitCapacityAndValidation(t *testing.T) {
	// Empty environment values are still explicit inputs to clap. Use known valid
	// values here so any developer credentials are neither inspected nor logged.
	t.Setenv("TT_API_KEY", "fixture-key")
	t.Setenv("TT_CONTAINER_RUNTIME", "podman")
	cfg, proceed, code := parseConfig(nil, io.Discard, io.Discard)
	if !proceed || code != 0 || cfg.Storage != "file" || cfg.PortStart != 20000 || cfg.PortEnd != 65535 {
		t.Fatal(cfg, proceed, code)
	}
	if cfg.EffectiveCPU() == 0 || cfg.EffectiveMem(context.Background()) == 0 {
		t.Fatal("missing automatic capacity")
	}
	cfg, proceed, code = parseConfig([]string{"--storage", "zvol", "--cpu-total", "16", "--mem-total", "4096"}, io.Discard, io.Discard)
	if !proceed || code != 0 || cfg.EffectiveCPU() != 16 || cfg.EffectiveMem(context.Background()) != 4096 {
		t.Fatal(cfg, proceed, code)
	}
	for _, args := range [][]string{{"--cpu-total", "1", "--cpu-total", "2"}, {"--enable-disk-backup=false"}, {"-listen", "127.0.0.1:0"}, {"--storage", "foo"}, {"--port-end", "65536"}, {"--cpu-total", "4294967296"}, {"--ssh-public-address", "::1"}, {"--ssh-ingress-target", "192.0.2.2"}, {"--container-runtime", "unknown"}} {
		_, proceed, code := parseConfig(args, io.Discard, io.Discard)
		if proceed || code != 2 {
			t.Fatal(args, proceed, code)
		}
	}
	var output bytes.Buffer
	if code := Run([]string{"--help"}, &output, io.Discard); code != 0 || output.String() != help {
		t.Fatal(code, output.String())
	}
}
func TestUpgradeCommandUsesOfflineConverter(t *testing.T) {
	source := v5Fixture(t, "5", false)
	output := source + ".converted"
	var stdout, stderr bytes.Buffer
	code := RunUpgrade([]string{"--source", source, "--output", output}, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 || stdout.String() != "Converted 1 VM records to a new v6 database; source and runtime disks were not changed\n" {
		t.Fatal(code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := RunUpgrade(nil, &stdout, &stderr); code != 2 {
		t.Fatal(code)
	}
}
