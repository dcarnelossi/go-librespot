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

func New(log librespot.Logger, directory string, overwrite bool) *Exporter {
	return &Exporter{log: log, directory: directory, overwrite: overwrite}
}

// Export writes one complete encrypted Spotify audio file as an independent
// clean Ogg/Vorbis file. The supplied reader must expose the complete encrypted
// file and remain valid for the duration of this call.
func (e *Exporter) Export(fileID []byte, encrypted io.ReaderAt, size int64, key []byte) (string, error) {
	if len(fileID) == 0 {
		return "", fmt.Errorf("audio export: empty file id")
	}
	if size <= 0 {
		return "", fmt.Errorf("audio export: invalid encrypted size %d", size)
	}
	if e.directory == "" {
		return "", fmt.Errorf("audio export: empty directory")
	}

	if err := os.MkdirAll(e.directory, 0o755); err != nil {
		return "", fmt.Errorf("audio export: create directory: %w", err)
	}

	name := hex.EncodeToString(fileID) + ".ogg"
	finalPath := filepath.Join(e.directory, name)
	if !e.overwrite {
		if _, err := os.Stat(finalPath); err == nil {
			return finalPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("audio export: stat target: %w", err)
		}
	}

	decryptor, err := audio.NewAesAudioDecryptor(encrypted, key)
	if err != nil {
		return "", fmt.Errorf("audio export: initialize decryptor: %w", err)
	}

	cleanOgg, _, err := vorbis.ExtractMetadataPage(e.log, decryptor, size)
	if err != nil {
		return "", fmt.Errorf("audio export: extract metadata page: %w", err)
	}

	tmp, err := os.CreateTemp(e.directory, "."+name+".*.part")
	if err != nil {
		return "", fmt.Errorf("audio export: create temporary file: %w", err)
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		_ = tmp.Close()
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(tmp, cleanOgg); err != nil {
		return "", fmt.Errorf("audio export: write Ogg: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return "", fmt.Errorf("audio export: sync Ogg: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("audio export: close Ogg: %w", err)
	}

	if !e.overwrite {
		// Re-check after writing to reduce the chance that concurrent exports of
		// the same Spotify file replace an already completed target.
		if _, err := os.Stat(finalPath); err == nil {
			return finalPath, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("audio export: stat target before rename: %w", err)
		}
	}

	if err := os.Rename(tmpPath, finalPath); err != nil {
		return "", fmt.Errorf("audio export: finalize Ogg: %w", err)
	}
	committed = true
	return finalPath, nil
}
