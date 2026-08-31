package player

import (
	"encoding/hex"
	"io"

	librespot "github.com/devgianlu/go-librespot"
	"github.com/devgianlu/go-librespot/audioexport"
)

var oggExporter *audioexport.Exporter

func ConfigureOggExporter(log librespot.Logger, enabled bool, directory string, overwrite bool) {
	if !enabled {
		oggExporter = nil
		return
	}
	oggExporter = audioexport.New(log, directory, overwrite)
}

func oggExportEnabled() bool { return oggExporter != nil }

func (p *Player) exportOgg(fileID []byte, encrypted io.ReaderAt, size int64, key []byte) {
	if oggExporter == nil {
		return
	}
	file := hex.EncodeToString(fileID)
	path, err := oggExporter.Export(fileID, encrypted, size, key)
	if err != nil {
		p.log.WithError(err).WithField("file", file).Warnf("audio export failed")
		return
	}
	p.log.WithField("file", file).WithField("path", path).Infof("audio export completed")
}
