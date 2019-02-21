package accesslog

import (
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AccessLog :
type AccessLog struct {
	zl *zap.Logger
}

func logWriter(logDir string, logRotate int64) (io.Writer, error) {
	if logDir == "stdout" {
		return os.Stdout, nil
	} else if logDir == "" {
		return os.Stderr, nil
	} else if logDir == "none" {
		return nil, nil
	}
	logFile := logDir
	linkName := logDir
	if !strings.HasSuffix(logDir, "/") {
		logFile += "/"
		linkName += "/"

	}
	logFile += "access_log.%Y%m%d%H%M"
	linkName += "current"

	rl, err := rotatelogs.New(
		logFile,
		rotatelogs.WithLinkName(linkName),
		rotatelogs.WithMaxAge(time.Duration(logRotate)*86400*time.Second),
		rotatelogs.WithRotationTime(time.Second*86400),
	)
	if err != nil {
		return nil, errors.Wrap(err, "rotatelogs.New failed")
	}
	return rl, nil
}

// New :
func New(logDir string, logRotate int64) (*AccessLog, error) {
	w, err := logWriter(logDir, logRotate)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return &AccessLog{}, nil
	}

	encoderConfig := zapcore.EncoderConfig{
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	zl := zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(w),
			zapcore.InfoLevel,
		),
	)
	return &AccessLog{
		zl: zl,
	}, nil
}

// WrapHandleFunc :
func (al *AccessLog) WrapHandleFunc(h http.Handler) http.Handler {
	if al.zl == nil {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := WrapWriter(w)
		defer func() {
			end := time.Now()
			ptime := end.Sub(start)
			remoteAddr := r.RemoteAddr
			if i := strings.LastIndexByte(remoteAddr, ':'); i > -1 {
				remoteAddr = remoteAddr[:i]
			}
			al.zl.Info(
				"-",
				zap.String("time", start.Format("02/Jan/2006:15:04:05 -0700")),
				zap.String("remote_addr", remoteAddr),
				zap.String("method", r.Method),
				zap.String("uri", r.URL.Path),
				zap.Int("status", ww.GetCode()),
				zap.Int("size", ww.GetSize()),
				zap.String("ua", r.UserAgent()),
				zap.Float64("ptime", ptime.Seconds()),
				zap.String("host", r.Host),
				zap.String("chocon_req", w.Header().Get("X-Chocon-Req")),
			)
		}()
		h.ServeHTTP(ww, r)
	})
}

// Writer :
type Writer struct {
	w    http.ResponseWriter
	size int
	code int
}

// WrapWriter :
func WrapWriter(w http.ResponseWriter) *Writer {
	return &Writer{
		w: w,
	}
}

// Header :
func (w *Writer) Header() http.Header {
	return w.w.Header()
}

// Write :
func (w *Writer) Write(b []byte) (int, error) {
	w.size += len(b)
	return w.w.Write(b)
}

// WriteHeader :
func (w *Writer) WriteHeader(statusCode int) {
	w.code = statusCode
	w.w.WriteHeader(statusCode)
}

// GetCode :
func (w *Writer) GetCode() int {
	return w.code
}

// GetSize :
func (w *Writer) GetSize() int {
	return w.size
}
