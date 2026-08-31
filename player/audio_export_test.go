package player

import (
	"bytes"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	librespot "github.com/devgianlu/go-librespot"
)

func TestFailedOggExportDoesNotWriteMetadata(t *testing.T) {
	dir := t.TempDir()
	fileID := []byte{0x01, 0x02, 0x03}
	ConfigureOggExporter(&librespot.NullLogger{}, true, dir, false)
	t.Cleanup(func() { ConfigureOggExporter(nil, false, "", false) })

	p := &Player{log: &librespot.NullLogger{}}
	p.exportOgg(fileID, bytes.NewReader([]byte("short")), 100, make([]byte, 16), []byte("{\"schemaVersion\":1}\n"))

	base := filepath.Join(dir, hex.EncodeToString(fileID))
	if _, err := os.Stat(base + ".ogg"); !os.IsNotExist(err) {
		t.Fatalf("final Ogg must not exist, stat error: %v", err)
	}
	if _, err := os.Stat(base + ".json"); !os.IsNotExist(err) {
		t.Fatalf("JSON must not exist after failed Ogg export, stat error: %v", err)
	}
}

func TestExistingOggCanReceiveMissingMetadata(t *testing.T) {
	dir := t.TempDir()
	fileID := []byte{0x01, 0x02, 0x03}
	base := filepath.Join(dir, hex.EncodeToString(fileID))
	if err := os.WriteFile(base+".ogg", []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}

	ConfigureOggExporter(&librespot.NullLogger{}, true, dir, false)
	t.Cleanup(func() { ConfigureOggExporter(nil, false, "", false) })
	p := &Player{log: &librespot.NullLogger{}}
	metadata := []byte("{\"schemaVersion\":1}\n")
	p.exportOgg(fileID, nil, 100, nil, metadata)

	got, err := os.ReadFile(base + ".json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, metadata) {
		t.Fatalf("metadata = %q, want %q", got, metadata)
	}
}
