package audioexport

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/audio"
	"github.com/devgianlu/go-librespot/vorbis"
)

// Exporter persists Spotify Ogg/Vorbis files after decryption and removal of
// Spotify's private metadata page. It never owns or closes the encrypted input.
type Exporter struct {
	log       librespot.Logger
	directory string
	overwrite bool
}

type ExportStatus uint8

const (
	ExportCreated ExportStatus = iota + 1
	ExportAlreadyExists
)

type ExportResult struct {
	Path   string
	Status ExportStatus
}

func New(log librespot.Logger, directory string, overwrite bool) *Exporter {
	return &Exporter{log: log, directory: directory, overwrite: overwrite}
}

// ExportMetadata writes a JSON sidecar next to the exported audio using the
// same Spotify file ID. Metadata export is atomic and best-effort at the caller.
func (e *Exporter) ExportMetadata(fileID []byte, data []byte) (string, error) {
	path, err := writeMetadata(e.directory, e.overwrite, fileID, data)
	if err != nil {
		return "", err
	}
	if err := os.Chmod(path, 0o664); err != nil {
		return "", fmt.Errorf("audio export metadata: chmod JSON: %w", err)
	}
	return path, nil
}

// Export writes one complete encrypted Spotify audio file as an independent
// clean Ogg/Vorbis file. The supplied reader must expose the complete encrypted
// file and remain valid for the duration of this call.
func (e *Exporter) Export(fileID []byte, encrypted io.ReaderAt, size int64, key []byte) (ExportResult, error) {
	if len(fileID) == 0 {
		return ExportResult{}, fmt.Errorf("audio export: empty file id")
	}
	if size <= 0 {
		return ExportResult{}, fmt.Errorf("audio export: invalid encrypted size %d", size)
	}
	if e.directory == "" {
		return ExportResult{}, fmt.Errorf("audio export: empty directory")
	}

	if err := os.MkdirAll(e.directory, 0o755); err != nil {
		return ExportResult{}, fmt.Errorf("audio export: create directory: %w", err)
	}

	name := hex.EncodeToString(fileID) + ".ogg"
	finalPath := filepath.Join(e.directory, name)
	if !e.overwrite {
		if info, err := os.Stat(finalPath); err == nil {
			if !info.Mode().IsRegular() {
				return ExportResult{}, fmt.Errorf("audio export: existing target is not a regular file: %s", finalPath)
			}
			if err := os.Chmod(finalPath, 0o664); err != nil {
				return ExportResult{}, fmt.Errorf("audio export: chmod existing Ogg: %w", err)
			}
			return ExportResult{Path: finalPath, Status: ExportAlreadyExists}, nil
		} else if !os.IsNotExist(err) {
			return ExportResult{}, fmt.Errorf("audio export: stat target: %w", err)
		}
	}

	decryptor, err := audio.NewAesAudioDecryptor(encrypted, key)
	if err != nil {
		return ExportResult{}, fmt.Errorf("audio export: initialize decryptor: %w", err)
	}

	cleanOgg, _, err := vorbis.ExtractMetadataPage(e.log, decryptor, size)
	if err != nil {
		return ExportResult{}, fmt.Errorf("audio export: extract metadata page: %w", err)
	}

	status, err := e.writeOgg(name, finalPath, cleanOgg)
	if err != nil {
		return ExportResult{}, err
	}
	if err := os.Chmod(finalPath, 0o664); err != nil {
		return ExportResult{}, fmt.Errorf("audio export: chmod Ogg: %w", err)
	}
	return ExportResult{Path: finalPath, Status: status}, nil
}

func (e *Exporter) writeOgg(name, finalPath string, cleanOgg librespot.SizedReadAtSeeker) (ExportStatus, error) {
	tmp, err := os.CreateTemp(e.directory, "."+name+".*.part")
	if err != nil {
		return 0, fmt.Errorf("audio export: create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmp, cleanOgg)
	if err != nil {
		return 0, fmt.Errorf("audio export: write Ogg: %w", err)
	}
	if expected := cleanOgg.Size(); written != expected {
		return 0, fmt.Errorf("audio export: incomplete encrypted source: wrote %d of %d clean Ogg bytes", written, expected)
	}
	if err := tmp.Sync(); err != nil {
		return 0, fmt.Errorf("audio export: sync Ogg: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("audio export: close Ogg: %w", err)
	}

	if !e.overwrite {
		// Re-check after writing to reduce the chance that concurrent exports of
		// the same Spotify file replace an already completed target.
		if info, err := os.Stat(finalPath); err == nil {
			if !info.Mode().IsRegular() {
				return 0, fmt.Errorf("audio export: existing target is not a regular file: %s", finalPath)
			}
			return ExportAlreadyExists, nil
		} else if !os.IsNotExist(err) {
			return 0, fmt.Errorf("audio export: stat target before rename: %w", err)
		}
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return 0, fmt.Errorf("audio export: finalize Ogg: %w", err)
	}
	committed = true
	return ExportCreated, nil
}
