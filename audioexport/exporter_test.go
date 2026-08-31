package audioexport

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	librespot "github.com/devgianlu/go-librespot"
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
	if got.Path != path || got.Status != ExportAlreadyExists {
		t.Fatalf("got %#v", got)
	}
}

func TestExportRejectsDirectoryAtExistingPath(t *testing.T) {
	dir := t.TempDir()
	id := []byte{1, 2, 3}
	path := filepath.Join(dir, hex.EncodeToString(id)+".ogg")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}

	e := New(nil, dir, false)
	if _, err := e.Export(id, nil, 1, nil); err == nil {
		t.Fatal("expected non-regular existing target to be rejected")
	}
}

func TestShortSourceCannotFinalizeOgg(t *testing.T) {
	dir := t.TempDir()
	name := "010203.ogg"
	finalPath := filepath.Join(dir, name)
	e := New(&librespot.NullLogger{}, dir, false)
	cleanOgg := &reportedSizeReader{Reader: bytes.NewReader([]byte("short")), size: 10}

	if _, err := e.writeOgg(name, finalPath, cleanOgg); err == nil {
		t.Fatal("expected incomplete source error")
	}
	if _, err := os.Stat(finalPath); !os.IsNotExist(err) {
		t.Fatalf("final Ogg must not exist, stat error: %v", err)
	}
	parts, err := filepath.Glob(filepath.Join(dir, ".010203.ogg.*.part"))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 0 {
		t.Fatalf("temporary files left behind: %v", parts)
	}
}

type reportedSizeReader struct {
	*bytes.Reader
	size int64
}

func (r *reportedSizeReader) Size() int64 { return r.size }
