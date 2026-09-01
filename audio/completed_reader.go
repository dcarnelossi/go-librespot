package audio

import "io"

// completedReaderAt is an immutable view over fully downloaded chunk data.
// It remains readable after HttpChunkedReader is closed, allowing best-effort
// completion callbacks (cache/export) to finish independently of playback.
type completedReaderAt struct {
	chunks [][]byte
	size   int64
}

func newCompletedReaderAt(chunks []*chunkItem, size int64) *completedReaderAt {
	data := make([][]byte, len(chunks))
	for i, chunk := range chunks {
		chunk.L.Lock()
		data[i] = chunk.data
		chunk.L.Unlock()
	}
	return &completedReaderAt{chunks: data, size: size}
}

func (r *completedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, io.EOF
	}
	if off >= r.size {
		return 0, io.EOF
	}

	n := 0
	for len(p) > 0 && off < r.size {
		idx := int(off / DefaultChunkSize)
		chunkOff := int(off % DefaultChunkSize)
		if idx >= len(r.chunks) || r.chunks[idx] == nil || chunkOff >= len(r.chunks[idx]) {
			break
		}
		copied := copy(p, r.chunks[idx][chunkOff:])
		n += copied
		off += int64(copied)
		p = p[copied:]
	}
	if len(p) > 0 {
		return n, io.EOF
	}
	return n, nil
}
