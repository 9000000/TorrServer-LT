package torr

import (
	"context"
	"errors"
	"fmt"
	"io"

	"server/torr/state"
	"server/torr/storage/torrstor"
)

// ReadFile reads a small file of a torrent in full (external subtitles, for
// one). Unlike Stream it does not mark the file as viewed and writes no HTTP
// headers. A file larger than maxSize is refused, and reading stops when ctx
// is done.
func (t *Torrent) ReadFile(ctx context.Context, fileID int, maxSize int64) ([]byte, error) {
	if t == nil {
		return nil, errors.New("torr.ReadFile: no torrent")
	}
	if !t.GotInfo() {
		return nil, errors.New("torr.ReadFile: torrent has no metadata yet")
	}

	// Resolve the file the way Stream does: by Id in the status, then by path
	// among the torrent's files, so the ids match what the API hands out.
	fileStats := t.Status().FileStats
	var stFile *state.TorrentFileStat
	for _, fileStat := range fileStats {
		if fileStat.Id == fileID {
			stFile = fileStat
			break
		}
	}
	if stFile == nil && len(fileStats) == 1 && fileID == 1 {
		stFile = fileStats[0]
	}
	if stFile == nil {
		return nil, fmt.Errorf("torr.ReadFile: file %d not found", fileID)
	}
	var file *File
	for _, f := range t.Files() {
		if f.Path == stFile.Path {
			file = f
			break
		}
	}
	if file == nil {
		return nil, fmt.Errorf("torr.ReadFile: file %d not found", fileID)
	}
	if file.Length < 0 {
		return nil, fmt.Errorf("torr.ReadFile: invalid file length: %d", file.Length)
	}
	if file.Length > maxSize {
		return nil, fmt.Errorf("torr.ReadFile: file too large: %d bytes (max %d)", file.Length, maxSize)
	}
	if file.Length == 0 {
		return []byte{}, nil
	}

	// Use AuxReaderGroup so auxiliary subtitle/file reads never masquerade as
	// an active video stream (never abort preload or clobber burst peer caps),
	// and disable optimistic unverified reads (upstream #924) so a corrupt
	// peer block cannot corrupt parsed subtitle text.
	reader := t.NewReaderGroup(file, torrstor.AuxReaderGroup)
	if reader == nil {
		return nil, errors.New("torr.ReadFile: no reader (cache not yet open)")
	}
	defer t.CloseReader(reader)
	reader.SetResponsive(false)
	// Unblock a read parked on a missing piece as soon as the caller gives up.
	reader.SetContext(ctx)

	type result struct {
		data []byte
		err  error
	}
	// Buffered: the goroutine never blocks once we have left on ctx.
	done := make(chan result, 1)
	go func() {
		// Read exactly the file's length: the reader does not stop at the end
		// of the file and would carry on into the next one. (Stream is safe
		// because ServeContent copies exactly that many bytes.)
		buf := make([]byte, file.Length)
		_, err := io.ReadFull(reader, buf)
		done <- result{buf, err}
	}()

	select {
	case r := <-done:
		if r.err != nil {
			return nil, r.err
		}
		return r.data, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
