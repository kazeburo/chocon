package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	stats_api "github.com/fukata/golang-stats-api-handler"
	"github.com/jessevdk/go-flags"
	"github.com/kazeburo/chocon/accesslog"
	"github.com/kazeburo/chocon/pidfile"
	"github.com/kazeburo/chocon/proxy"
	"github.com/kazeburo/chocon/stats"
	"github.com/kazeburo/chocon/upstream"
	ss "github.com/lestrrat/go-server-starter-listener"
	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttpadaptor"
	"go.uber.org/zap"
)

var (
	// Version chocon version
	Version string
)

type cmdOpts struct {
	Listen           string        `short:"l" long:"listen" default:"0.0.0.0" description:"address to bind"`
	Port             string        `short:"p" long:"port" default:"3000" description:"Port number to bind"`
	LogDir           string        `long:"access-log-dir" default:"" description:"directory to store logfiles"`
	LogRotate        int64         `long:"access-log-rotate" default:"30" description:"Number of day before remove logs"`
	Version          bool          `short:"v" long:"version" description:"Show version"`
	PidFile          string        `long:"pid-file" default:"" description:"filename to store pid. disabled by default"`
	MaxConnsPerHost  int           `long:"max-conns-per-host" default:"0" description:"maximum connections per host"`
	ReadTimeout      int           `long:"read-timeout" default:"30" description:"timeout of reading request"`
	WriteTimeout     int           `long:"write-timeout" default:"90" description:"timeout of writing response"`
	ProxyReadTimeout int           `long:"proxy-read-timeout" default:"60" description:"timeout of reading response from upstream"`
	ShutdownTimeout  time.Duration `long:"shutdown-timeout" default:"1h"  description:"timeout to wait for all connections to be closed."`
	Upstream         string        `long:"upstream" default:"" description:"upstream server: http://upstream-server/"`
	StatsBufsize     int           `long:"stsize" default:"1000" description:"buffer size for http stats"`
	StatsSpfactor    int           `long:"spfactor" default:"3" description:"sampling factor for http stats"`
	Insecure         bool          `long:"insecure" description:"disable certificate verifications (only for debugging)"`
}

func addStatsHandler(h fasthttp.RequestHandler, mw *stats.Metrics) fasthttp.RequestHandler {
	return func(ctx *fasthttp.RequestCtx) {
		switch string(ctx.Path()) {
		case "/.api/stats":
			fasthttpadaptor.NewFastHTTPHandlerFunc(stats_api.Handler)(ctx)
		case "/.api/http-stats":
			d, err := mw.Data()
			if err != nil {
				ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
			}
			if err := json.NewEncoder(ctx.Response.BodyWriter()).Encode(d); err != nil {
				ctx.Error(err.Error(), fasthttp.StatusInternalServerError)
			}
		default:
			h(ctx)
		}
	}
}

func printVersion() {
	fmt.Printf(`chocon %s
Compiler: %s %s
`,
		Version,
		runtime.Compiler,
		runtime.Version())
}

func main() {
	os.Exit(_main())
}

func _main() int {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		return 1
	}

	if opts.Version {
		printVersion()
		return 0
	}

	logger, _ := zap.NewProduction()

	upstream, err := upstream.New(opts.Upstream, logger)

	if err != nil {
		log.Fatal(err)
	}

	if opts.PidFile != "" {
		if err := pidfile.WritePid(opts.PidFile); err != nil {
			log.Fatal(err)
		}
	}

	var tlsClientConfig *tls.Config

	if opts.Insecure {
		tlsClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	proxy := proxy.New(&fasthttp.Client{
		ReadTimeout:         time.Duration(opts.ProxyReadTimeout) * time.Second,
		TLSConfig:           tlsClientConfig,
		MaxConnsPerHost:     opts.MaxConnsPerHost,
		MaxIdleConnDuration: 30 * time.Second,
	}, Version, logger, upstream)

	al, err := accesslog.New(opts.LogDir, opts.LogRotate)

	if err != nil {
		logger.Fatal("could not init accesslog", zap.Error(err))
	}

	statsChocon, err := stats.NewCapa(opts.StatsBufsize, opts.StatsSpfactor)

	if err != nil {
		log.Fatal(err)
	}

	handler := addStatsHandler(proxy.Handler, statsChocon)
	handler = al.Wrap(handler)
	handler = statsChocon.WrapHandler(handler)

	server := &fasthttp.Server{
		ReadTimeout:  time.Duration(opts.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(opts.WriteTimeout) * time.Second,
		Handler:      handler,
	}

	idleConnsClosed := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGTERM)
		<-sigChan

		// The process gets shut down forcedly after timeout.
		timer := time.NewTimer(opts.ShutdownTimeout)
		go func() {
			<-timer.C
			logger.Warn("shutdown timeout")
			os.Exit(1)
		}()

		// Graceful shutdown.
		if es := server.Shutdown(); es != nil {
			logger.Warn("Shutdown error", zap.Error(es))
		}

		close(idleConnsClosed)
	}()

	l, err := ss.NewListener()
	if l == nil || err != nil {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%s", opts.Listen, opts.Port))
		if err != nil {
			logger.Fatal("Failed to listen to port",
				zap.Error(err),
				zap.String("listen", opts.Listen),
				zap.String("port", opts.Port))
		}
	}

	if err := server.Serve(l); err != nil {
		logger.Error("Error in Serve", zap.Error(err))
	}

	<-idleConnsClosed
	return 0
}
