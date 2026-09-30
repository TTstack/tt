package engine

import (
	"reflect"
	"testing"
)

func TestFreeBSDEngineRustGoldenCommandsAndConfiguration(t *testing.T) {
	fixture := readGolden(t)
	vm := testVM()
	if got := bhyveCommand(vm, "/retained/disk", "tap42").Args; !reflect.DeepEqual(got, fixture.Bhyve) {
		t.Fatal(got)
	}
	got, err := jailConfiguration(vm, "/retained/root")
	if err != nil || got != fixture.Jail {
		t.Fatal(got, err)
	}
}
