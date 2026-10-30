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
