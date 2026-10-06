package recovery

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestBoundedDumpOutputDoesNotWritePastItsBudget(t *testing.T) {
	var output bytes.Buffer
	writer := boundedOutput{writer: &output, remaining: 3}
	if n, err := writer.Write([]byte("ab")); n != 2 || err != nil {
		t.Fatal(n, err)
	}
	if n, err := writer.Write([]byte("cd")); n != 0 || err == nil {
		t.Fatal("accepted an over-budget dump chunk", n, err)
	}
	if output.String() != "ab" {
		t.Fatal("dump exceeded its byte budget")
	}
}

func TestSnapshotBasisReusesBytesWithoutLinkingMutableSources(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	if err := os.Mkdir(source, 0700); err != nil {
		t.Fatal(err)
	}
	original := filepath.Join(source, "original")
	if err := os.WriteFile(original, []byte("preserved one"), 0600); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, "first")
	second := filepath.Join(root, "second")
	third := filepath.Join(root, "third")
	if err := copyTree(source, first); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(source, filepath.Join(root, "writable-basis"), first); err == nil {
		t.Fatal("writable snapshot basis accepted")
	}
	if err := os.Chmod(filepath.Join(first, "original"), 0400); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(source, second, first); err != nil {
		t.Fatal(err)
	}
	one, _ := os.Stat(filepath.Join(first, "original"))
	two, _ := os.Stat(filepath.Join(second, "original"))
	live, _ := os.Stat(original)
	if !os.SameFile(one, two) || os.SameFile(one, live) {
		t.Fatal("unchanged snapshots must share bytes without sharing the mutable source inode")
	}
	// Same size, same mtime: a checksum must still detect the source mutation.
	if err := os.WriteFile(original, []byte("preserved two"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(original, live.ModTime(), live.ModTime()); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(source, third, second); err != nil {
		t.Fatal(err)
	}
	three, _ := os.Stat(filepath.Join(third, "original"))
	if os.SameFile(two, three) {
		t.Fatal("changed bytes reused an old inode")
	}
	for _, old := range []string{first, second} {
		bytes, err := os.ReadFile(filepath.Join(old, "original"))
		if err != nil || string(bytes) != "preserved one" {
			t.Fatal("source mutation altered an earlier snapshot")
		}
	}
	bytes, _ := os.ReadFile(filepath.Join(third, "original"))
	if string(bytes) != "preserved two" {
		t.Fatal("new snapshot did not retain the changed bytes")
	}
}

func TestSnapshotBasisRejectsLinksAndIncompleteEvidence(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source")
	basis := filepath.Join(root, "basis")
	for _, dir := range []string{source, basis} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(source, "original"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(source, "original"), filepath.Join(basis, "original")); err != nil {
		t.Fatal(err)
	}
	if err := copyTree(source, filepath.Join(root, "new"), basis); err == nil {
		t.Fatal("symlink basis accepted")
	}
	if err := verifyBasis(basis); err == nil {
		t.Fatal("incomplete basis accepted")
	}
}
