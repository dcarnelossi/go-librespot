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

func (p *Player) exportOgg(fileID []byte, encrypted io.ReaderAt, size int64, key, metadataJSON []byte) {
	if oggExporter == nil {
		return
	}
	file := hex.EncodeToString(fileID)
	result, err := oggExporter.Export(fileID, encrypted, size, key)
	if err != nil {
		p.log.WithError(err).WithField("file", file).Warnf("audio export failed")
		return
	}
	if result.Status == audioexport.ExportAlreadyExists {
		p.log.WithField("file", file).WithField("path", result.Path).Infof("audio export already exists")
	} else {
		p.log.WithField("file", file).WithField("path", result.Path).Infof("audio export completed")
	}

	if len(metadataJSON) == 0 {
		return
	}
	metadataPath, err := oggExporter.ExportMetadata(fileID, metadataJSON)
	if err != nil {
		p.log.WithError(err).WithField("file", file).Warnf("audio export metadata failed")
		return
	}
	p.log.WithField("file", file).WithField("path", metadataPath).Infof("audio export metadata completed")
}
