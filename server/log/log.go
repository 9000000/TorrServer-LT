package log

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	logPath    = ""
	webLogPath = ""
)

var webLog *log.Logger

var (
	logFile    *os.File
	webLogFile *os.File
)

func Init(path, webpath string) {
	logPath = path
	webLogPath = webpath

	shared := path != "" && webpath != "" && filepath.Clean(path) == filepath.Clean(webpath)
	if path != "" {
		path = filepath.Clean(path)
	}
	if webpath != "" {
		webpath = filepath.Clean(webpath)
	}

	if shared {
		ff, err := openLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error create log file: %v\n", err)
			return
		}
		logFile = ff
		webLogFile = ff
		webLog = log.New(ff, " ", log.LstdFlags)
		applyServerLog(ff)
		return
	}

	if webpath != "" {
		ff, err := os.OpenFile(webpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error create web log file: %v\n", err)
		} else {
			webLogFile = ff
			webLog = log.New(ff, " ", log.LstdFlags)
		}
	}

	if path != "" {
		ff, err := openLogFile(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error create log file: %v\n", err)
			return
		}
		logFile = ff
		applyServerLog(ff)
	}
}

func openLogFile(path string) (*os.File, error) {
	if fi, err := os.Lstat(path); err == nil {
		if fi.Size() >= 100*1024*1024 { // 100MB
			os.Remove(path)
		}
	}
	return os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o600)
}

func applyServerLog(ff *os.File) {
	if err := redirectStdFDs(ff); err != nil {
		fmt.Fprintf(os.Stderr, "Error redirect stdout/stderr: %v\n", err)
	}
	log.SetFlags(log.LstdFlags | log.LUTC | log.Lmsgprefix)
	log.SetPrefix("UTC0 ")
	log.SetOutput(ff)
}

func Close() {
	if logFile != nil {
		logFile.Close()
		if webLogFile == logFile {
			webLogFile = nil
			webLog = nil
		}
		logFile = nil
	}
	if webLogFile != nil {
		webLogFile.Close()
		webLogFile = nil
		webLog = nil
	}
}

func TLogln(v ...interface{}) {
	log.Println(v...)
}

func WebLogln(v ...interface{}) {
	if webLog != nil {
		webLog.Println(v...)
	}
}

const maxLoggedBodyBytes = 4096

type readCloser struct {
	io.Reader
	io.Closer
}

func isSensitiveLogPath(p string) bool {
	return p == "/settings" ||
		strings.HasPrefix(p, "/ssl") ||
		strings.HasPrefix(p, "/waf") ||
		p == "/gst/settings"
}

func sanitizeLogField(s string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(s)
}

func WebLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		if webLog == nil {
			c.Next()
			return
		}
		body := ""
		reqPath := c.Request.URL.Path
		if isSensitiveLogPath(reqPath) {
			body = "[redacted]"
		} else if strings.HasPrefix(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			body = "body hidden, too large"
		} else if c.Request.Body != nil {
			// Peek at most 4 KiB so large uploads never buffer in RAM, while
			// preserving the full unread stream for downstream handlers.
			peek, _ := io.ReadAll(io.LimitReader(c.Request.Body, maxLoggedBodyBytes+1))
			c.Request.Body = readCloser{
				Reader: io.MultiReader(bytes.NewReader(peek), c.Request.Body),
				Closer: c.Request.Body,
			}
			if len(peek) > maxLoggedBodyBytes {
				body = sanitizeLogField(string(peek[:maxLoggedBodyBytes])) + "...(truncated)"
			} else {
				body = sanitizeLogField(string(peek))
			}
		}
		c.Next()

		statusCode := c.Writer.Status()
		clientIP := sanitizeLogField(c.ClientIP())
		method := sanitizeLogField(c.Request.Method)
		path := reqPath
		raw := c.Request.URL.RawQuery
		if raw != "" {
			path = path + "?" + raw
		}
		path = sanitizeLogField(path)

		logStr := fmt.Sprintf("%3d | %12s | %-7s %#v %v",
			statusCode,
			clientIP,
			method,
			path,
			body,
		)
		WebLogln(logStr)
	}
}
