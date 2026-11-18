package torrstor

import (
	"os"
	"path/filepath"
	"strconv"
)

// ScanHavePieces inspects `<savePath>/<infoHashHex>/` and returns an
// LSB-first packed bitmap of pieces present locally (size-only heuristic
// matching the "trust file-sizes" resume policy chosen in Etap 2).
//
// A piece is considered "have" if its file exists and:
//   - For non-final pieces: size == pieceLength
//   - For the final piece: size > 0 (we can't tell the exact final-piece
//     length without total_size, so accept any non-empty file)
//
// Returns nil when UseDisk is off, the save path is empty or no cache
// dir exists.
func ScanHavePieces(infoHash [20]byte, numPieces int, pieceLength int64, totalSize ...int64) []byte {
	if !useDisk() || numPieces <= 0 || pieceLength <= 0 {
		return nil
	}
	dir := filepath.Join(savePath(), hashHex(infoHash))
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) == 0 {
		return nil
	}
	var total int64
	if len(totalSize) > 0 {
		total = totalSize[0]
	}
	lastLen := int64(0)
	if total > 0 {
		lastLen = total - int64(numPieces-1)*pieceLength
	}
	bitmap := make([]byte, (numPieces+7)/8)
	hasAny := false
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		i, convErr := strconv.Atoi(entry.Name())
		if convErr != nil || i < 0 || i >= numPieces {
			continue
		}
		fi, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		sz := fi.Size()
		if i == numPieces-1 {
			if lastLen > 0 {
				if sz >= lastLen {
					bitmap[i/8] |= 1 << uint(i%8)
					hasAny = true
				}
			} else if sz > 0 {
				bitmap[i/8] |= 1 << uint(i%8)
				hasAny = true
			}
			continue
		}
		if sz == pieceLength {
			bitmap[i/8] |= 1 << uint(i%8)
			hasAny = true
		}
	}
	if !hasAny {
		return bitmap
	}
	return bitmap
}
