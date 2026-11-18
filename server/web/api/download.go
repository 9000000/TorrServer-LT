package api

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type fileReader struct {
	pos  int64
	size int64
}

func newFR(size int64) *fileReader {
	return &fileReader{
		pos:  0,
		size: size,
	}
}

func (f *fileReader) Read(p []byte) (n int, err error) {
	if f.pos >= f.size {
		return 0, io.EOF
	}
	rem := f.size - f.pos
	if int64(len(p)) > rem {
		p = p[:int(rem)]
	}
	clear(p)
	f.pos += int64(len(p))
	return len(p), nil
}

func (f *fileReader) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = f.pos + offset
	case io.SeekEnd:
		next = f.size + offset
	default:
		return 0, errors.New("invalid whence")
	}
	if next < 0 {
		return 0, errors.New("negative seek position")
	}
	f.pos = next
	return f.pos, nil
}

// download godoc
//
//	@Summary		Generates test file of given size
//	@Description	Download the test file of given size (for speed testing purpose).
//
//	@Tags			API
//
//	@Param			size	path	string	true	"Test file size (in MB)"
//
//	@Produce		application/octet-stream
//	@Success		200 {file} file
//	@Router			/download/{size} [get]
func download(c *gin.Context) {
	szStr := c.Param("size")
	sz, err := strconv.Atoi(szStr)
	if err != nil || sz <= 0 || sz > 10240 {
		abortWithJSONError(c, http.StatusBadRequest, errors.New("size must be between 1 and 10240 MB"))
		return
	}

	http.ServeContent(c.Writer, c.Request, strconv.Itoa(sz)+"mb.bin", time.Now(), newFR(int64(sz)*1024*1024))
}
