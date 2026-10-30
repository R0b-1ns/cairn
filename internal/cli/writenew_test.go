package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// A path that passes the regular-file check but is swapped for a symlink before
// the open must not be followed.
func TestWriteNewDoesNotFollowSwappedSymlink(t *testing.T) {
	dir := realTemp(t)
	victim := filepath.Join(dir, "victim.txt")
	os.WriteFile(victim, []byte("precious"), 0o600)
	decoy := filepath.Join(dir, "decoy.txt")
	os.WriteFile(decoy, []byte("x"), 0o600)
	target := filepath.Join(dir, "out.json")
	if err := os.Symlink(victim, target); err != nil {
		t.Fatal(err)
	}
	old := lstat
	t.Cleanup(func() { lstat = old })
	lstat = func(string) (os.FileInfo, error) { return os.Lstat(decoy) } // "looks like a regular file" at check time
	if err := writeNew(target, "replaced", true); err == nil {
		t.Fatal("writing through a symlink must fail")
	}
	if b, _ := os.ReadFile(victim); string(b) != "precious" {
		t.Fatalf("symlink target was overwritten: %q", b)
	}
}

func TestWriteNewForcedReplaceKeepsOwnerOnlyMode(t *testing.T) {
	path := filepath.Join(realTemp(t), "out.json")
	os.WriteFile(path, []byte("old"), 0o644)
	if err := writeNew(path, "new", true); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if b, _ := os.ReadFile(path); string(b) != "new" || fi.Mode().Perm() != 0o600 {
		t.Fatalf("content %q mode %v", b, fi.Mode().Perm())
	}
}

// Replacing a file must never truncate an inode other than the one that was checked.
func TestWriteNewRefusesHardlinkedFile(t *testing.T) {
	dir := realTemp(t)
	victim := filepath.Join(dir, "victim.txt")
	os.WriteFile(victim, []byte("precious"), 0o600)
	target := filepath.Join(dir, "out.json")
	if err := os.Link(victim, target); err != nil {
		t.Fatal(err)
	}
	if err := writeNew(target, "replaced", true); err == nil {
		t.Fatal("a file with several hard links must not be replaced")
	}
	if b, _ := os.ReadFile(victim); string(b) != "precious" {
		t.Fatalf("the other link was overwritten: %q", b)
	}
}

func TestWriteNewRefusesAFileSwappedAfterTheCheck(t *testing.T) {
	dir := realTemp(t)
	checked := filepath.Join(dir, "checked.txt")
	os.WriteFile(checked, []byte("x"), 0o600)
	swapped := filepath.Join(dir, "out.json")
	os.WriteFile(swapped, []byte("precious"), 0o600)
	old := lstat
	t.Cleanup(func() { lstat = old })
	lstat = func(string) (os.FileInfo, error) { return os.Lstat(checked) } // the check saw a different file
	if err := writeNew(swapped, "replaced", true); err == nil {
		t.Fatal("the opened file is not the one that was checked")
	}
	if b, _ := os.ReadFile(swapped); string(b) != "precious" {
		t.Fatalf("swapped file was truncated: %q", b)
	}
}
