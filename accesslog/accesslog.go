package accesslog

import (
	"io"
	"os"
	"strings"
	"time"

	rotatelogs "github.com/lestrrat-go/file-rotatelogs"
	"github.com/pkg/errors"
	"github.com/valyala/fasthttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AccessLog :
type AccessLog struct {
	logger *zap.Logger
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

	logger := zap.New(
		zapcore.NewCore(
			zapcore.NewJSONEncoder(encoderConfig),
			zapcore.AddSync(w),
			zapcore.InfoLevel,
		),
	)
	return &AccessLog{
		logger: logger,
	}, nil
}

// Wrap adds logging to a request handler.
func (al *AccessLog) Wrap(h fasthttp.RequestHandler) fasthttp.RequestHandler {
	if al.logger == nil {
		return h
	}

	return func(ctx *fasthttp.RequestCtx) {
		start := time.Now()
		defer func() {
			end := time.Now()
			ptime := end.Sub(start)
			al.logger.Info(
				"-",
				zap.String("time", start.Format("2006/01/02 15:04:05 MST")),
				zap.String("remote_addr", ctx.RemoteIP().String()),
				zap.ByteString("method", ctx.Method()),
				zap.ByteString("uri", ctx.RequestURI()),
				zap.Int("status", ctx.Response.StatusCode()),
				zap.Int("size", len(ctx.Response.Body())),
				zap.ByteString("ua", ctx.UserAgent()),
				zap.Float64("ptime", ptime.Seconds()),
				zap.ByteString("host", ctx.Host()),
				zap.ByteString("chocon_req", ctx.Response.Header.Peek("X-Chocon-Id")),
			)
		}()

		h(ctx)
	}
}
