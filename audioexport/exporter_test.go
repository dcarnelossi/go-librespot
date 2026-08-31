package audioexport

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestExportRejectsInvalidInput(t *testing.T) {
	e := New(nil, t.TempDir(), false)
	if _, err := e.Export(nil, nil, 1, nil); err == nil {
		t.Fatal("expected error")
	}
	if _, err := e.Export([]byte{1}, nil, 0, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestExportSkipsExistingFile(t *testing.T) {
	dir := t.TempDir()
	id := []byte{1, 2, 3}
	path := filepath.Join(dir, hex.EncodeToString(id)+".ogg")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(nil, dir, false)
	got, err := e.Export(id, nil, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("got %q", got)
	}
}
