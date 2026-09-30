package runcmd

import (
	"io"
	"os"
	"testing"
)

func TestAnonymousFileHasNoDirectoryEntryWhileItsDescriptorIsUsable(t *testing.T) {
	dir := t.TempDir()
	file, err := AnonymousFile(dir, "probe-*")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := os.Stat(file.Name()); !os.IsNotExist(err) {
		t.Fatal("scratch file remains named", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
	if _, err := file.WriteString("payload"); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(file)
	if err != nil || string(data) != "payload" {
		t.Fatal(string(data), err)
	}
}
