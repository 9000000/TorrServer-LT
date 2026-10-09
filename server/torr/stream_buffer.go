package torr

import (
	"bufio"
	"io"
)

// streamBufferSize is how much of the file the HTTP serve path reads at once.
// Measured on a 2 GB file from the disk cache (CPU of the whole process):
// 32 KB (no buffer) 0.75 s/GB at 1450 MB/s, 256 KB 0.54 s/GB at 2080 MB/s,
// 512 KB 0.48 s/GB at 2340 MB/s, 1 MB 0.47-0.49 s/GB at 2270-2420 MB/s. At a
// realistic 20 MB/s the buffered sizes are within noise of each other, so the
// difference only shows where reads run at full speed: the GStreamer pipeline,
// several clients at once, slow CPUs.
const streamBufferSize = 1 << 20

type bufferedStreamReader struct {
	source io.ReadSeeker
	buffer *bufio.Reader
}

func newBufferedStreamReader(source io.ReadSeeker, size int) *bufferedStreamReader {
	return &bufferedStreamReader{source: source, buffer: bufio.NewReaderSize(source, size)}
}

func (r *bufferedStreamReader) Read(p []byte) (int, error) { return r.buffer.Read(p) }
func (r *bufferedStreamReader) Seek(offset int64, whence int) (int64, error) {
	if whence == io.SeekCurrent {
		// The source has advanced past unread buffered bytes.
		offset -= int64(r.buffer.Buffered())
	}
	pos, err := r.source.Seek(offset, whence)
	if err == nil {
		r.buffer.Reset(r.source)
	}
	return pos, err
}
