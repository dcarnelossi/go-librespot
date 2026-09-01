from pathlib import Path


def replace(path, old, new):
    p = Path(path)
    s = p.read_text()
    if old not in s:
        raise SystemExit(f"expected block not found in {path}: {old[:80]!r}")
    p.write_text(s.replace(old, new, 1))

# Make the exporter configuration callable from daemon and queryable by NewStream.
replace("player/audio_export.go",
'''func configureOggExporter(log librespot.Logger, enabled bool, directory string, overwrite bool) {
	if !enabled {
		oggExporter = nil
		return
	}
	oggExporter = audioexport.New(log, directory, overwrite)
}
''',
'''func ConfigureOggExporter(log librespot.Logger, enabled bool, directory string, overwrite bool) {
	if !enabled {
		oggExporter = nil
		return
	}
	oggExporter = audioexport.New(log, directory, overwrite)
}

func oggExportEnabled() bool { return oggExporter != nil }
''')

# Resolve format before cache/CDN handling so export policy is known there.
replace("player/player.go",
'''\tlog.Debugf("selected format %s (%x)", file.Format.String(), file.FileId)\n\n\taudioKey, err := p.retrieveAudioKey(ctx, spotId, file.FileId)''',
'''\tlog.Debugf("selected format %s (%x)", file.Format.String(), file.FileId)\n\n\taudioFormat := GetAudioFileFormatAudioFormat(*file.Format)\n\texportOgg := oggExportEnabled() && audioFormat == AudioFormatOGGVorbis\n\tif oggExportEnabled() && !exportOgg {\n\t\tlog.Debugf("audio export skipped for unsupported format %s", file.Format.String())\n\t}\n\n\taudioKey, err := p.retrieveAudioKey(ctx, spotId, file.FileId)''')

# Cache hit: use an independent file descriptor so exporter cannot consume/close playback's reader.
replace("player/player.go",
'''\tif p.cache != nil {\n\t\tif cached, ok := p.cache.File(file.FileId); ok {\n\t\t\tlog.Debugf("using cached audio file (%d bytes)", cached.Size())\n\t\t\trawStream = cached\n\t\t}\n\t}\n''',
'''\tif p.cache != nil {\n\t\tif cached, ok := p.cache.File(file.FileId); ok {\n\t\t\tlog.Debugf("using cached audio file (%d bytes)", cached.Size())\n\t\t\trawStream = cached\n\t\t\tif exportOgg {\n\t\t\t\tfileID := append([]byte(nil), file.FileId...)\n\t\t\t\tkey := append([]byte(nil), audioKey...)\n\t\t\t\tif exportReader, ok := p.cache.File(fileID); ok {\n\t\t\t\t\tgo func() {\n\t\t\t\t\t\tif closer, ok := exportReader.(io.Closer); ok {\n\t\t\t\t\t\t\tdefer closer.Close()\n\t\t\t\t\t\t}\n\t\t\t\t\t\tp.exportOgg(fileID, exportReader, exportReader.Size(), key)\n\t\t\t\t\t}()\n\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n''')

# CDN completion: compose cache persistence and export in the one OnComplete callback.
replace("player/player.go",
'''\t\t// Persist the encrypted file to the cache once it has been fully\n\t\t// downloaded. This is best-effort: caching failures never affect\n\t\t// playback.\n\t\tif p.cache != nil {\n\t\t\tfileId := file.FileId\n\t\t\thttpStream.OnComplete(func(r io.ReaderAt, size int64) {\n\t\t\t\tif err := p.cache.SaveFile(fileId, io.NewSectionReader(r, 0, size)); err != nil {\n\t\t\t\t\tlog.WithError(err).Warnf("failed caching audio file")\n\t\t\t\t}\n\t\t\t})\n\t\t}\n''',
'''\t\t// Persist/cache and export only after every encrypted chunk is present.\n\t\t// Both operations are best-effort and never affect playback.\n\t\tif p.cache != nil || exportOgg {\n\t\t\tfileID := append([]byte(nil), file.FileId...)\n\t\t\tkey := append([]byte(nil), audioKey...)\n\t\t\thttpStream.OnComplete(func(r io.ReaderAt, size int64) {\n\t\t\t\tif p.cache != nil {\n\t\t\t\t\tif err := p.cache.SaveFile(fileID, io.NewSectionReader(r, 0, size)); err != nil {\n\t\t\t\t\t\tlog.WithError(err).Warnf("failed caching audio file")\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tif exportOgg {\n\t\t\t\t\tp.exportOgg(fileID, r, size, key)\n\t\t\t\t}\n\t\t\t})\n\t\t}\n''')
replace("player/player.go", '''\taudioFormat := GetAudioFileFormatAudioFormat(*file.Format)\n\tif audioFormat == AudioFormatOGGVorbis {''', '''\tif audioFormat == AudioFormatOGGVorbis {''')

# OnComplete must hand callbacks a stable immutable view that remains readable after playback closes.
replace("audio/chunked-reader.go", '''\tr.onCompleteFired = true\n\tcb := r.onComplete\n\tgo cb(r, r.len)''', '''\tr.onCompleteFired = true\n\tcb := r.onComplete\n\tcompleted := newCompletedReaderAt(r.chunks, r.len)\n\tgo cb(completed, r.len)''')

# CLI YAML -> daemon config.
replace("cmd/daemon/cli_config.go",
'''\tCache struct {\n\t\tEnabled   bool   `koanf:"enabled"`\n\t\tDir       string `koanf:"dir"`\n\t\tSizeLimit string `koanf:"size_limit"`\n\t} `koanf:"cache"`\n\n\tCredentials struct {''',
'''\tCache struct {\n\t\tEnabled   bool   `koanf:"enabled"`\n\t\tDir       string `koanf:"dir"`\n\t\tSizeLimit string `koanf:"size_limit"`\n\t} `koanf:"cache"`\n\n\tAudioExport struct {\n\t\tEnabled   bool   `koanf:"enabled"`\n\t\tDirectory string `koanf:"directory"`\n\t\tOverwrite bool   `koanf:"overwrite"`\n\t} `koanf:"audio_export"`\n\n\tCredentials struct {''')
replace("cmd/daemon/cli_config.go",
'''\tdc.Cache.SizeLimit, _ = parseSize(c.Cache.SizeLimit)\n\tdc.Credentials.Type = c.Credentials.Type''',
'''\tdc.Cache.SizeLimit, _ = parseSize(c.Cache.SizeLimit)\n\tdc.AudioExport.Enabled = c.AudioExport.Enabled\n\tdc.AudioExport.Directory = c.AudioExport.Directory\n\tdc.AudioExport.Overwrite = c.AudioExport.Overwrite\n\tif dc.AudioExport.Enabled && dc.AudioExport.Directory == "" {\n\t\tdc.AudioExport.Directory = filepath.Join(c.ConfigDir, "audio-export")\n\t}\n\tdc.Credentials.Type = c.Credentials.Type''')
replace("cmd/daemon/cli_config.go", '''\t\t"cache.enabled":    false,\n\t\t"cache.size_limit": "1GB",''', '''\t\t"cache.enabled":          false,\n\t\t"cache.size_limit":       "1GB",\n\t\t"audio_export.enabled":   false,\n\t\t"audio_export.overwrite": false,''')

# Daemon configures the process-wide exporter once during startup.
replace("daemon/app.go", '''\tif app.cfg.Cache.Enabled && app.cfg.Cache.Dir != "" {''', '''\tplayer.ConfigureOggExporter(app.log, app.cfg.AudioExport.Enabled, app.cfg.AudioExport.Directory, app.cfg.AudioExport.Overwrite)\n\n\tif app.cfg.Cache.Enabled && app.cfg.Cache.Dir != "" {''')

# JSON schema for YAML validation/editors.
replace("config_schema.json", '''        "zeroconf_enabled": {''', '''        "audio_export": {\n          "type": "object",\n          "description": "Best-effort export of each complete decrypted Ogg/Vorbis track before PCM decoding",\n          "additionalProperties": false,\n          "properties": {\n            "enabled": {\n              "type": "boolean",\n              "description": "Whether per-track Ogg/Vorbis export is enabled",\n              "default": false\n            },\n            "directory": {\n              "type": "string",\n              "description": "Directory for exported .ogg files; defaults to <config_dir>/audio-export when enabled",\n              "default": ""\n            },\n            "overwrite": {\n              "type": "boolean",\n              "description": "Whether an existing completed export may be replaced",\n              "default": false\n            }\n          }\n        },\n        "zeroconf_enabled": {''')
