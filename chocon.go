package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/fukata/golang-stats-api-handler"
	"github.com/jessevdk/go-flags"
	"github.com/kazeburo/chocon/proxy"
	"github.com/lestrrat/go-apache-logformat"
	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/go-server-starter-listener"
)

var (
	Version string
)

type cmdOpts struct {
	Listen           string `short:"l" long:"listen" default:"0.0.0.0" description:"address to bind"`
	Port             string `short:"p" long:"port" default:"3000" description:"Port number to bind"`
	LogDir           string `long:"access-log-dir" default:"" description:"directory to store logfiles"`
	LogRotate        int64  `long:"access-log-rotate" default:"30" description:"Number of day before remove logs"`
	Version          bool   `short:"v" long:"version" description:"Show version"`
	KeepaliveConns   int    `short:"c" default:"2" long:"keepalive-conns" description:"maximun keepalive connections for upstream"`
	ReadTimeout      int    `long:"read-timeout" default:"30" description:"timeout of reading request"`
	WriteTimeout     int    `long:"write-timeout" default:"90" description:"timeout of writing response"`
	ProxyReadTimeout int    `long:"proxy-read-timeout" default:"60" description:"timeout of reading response from upstream"`
}

func addStatsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Index(r.URL.Path, "/.api/stats") == 0 {
			stats_api.Handler(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

func addLogHandler(h http.Handler, log_dir string, log_rotate int64) http.Server {
	apache_log, err := apachelog.New(`%h %l %u %t "%r" %>s %b "%v" %T.%{msec_frac}t %{X-Chocon-Req}i`)
	if err != nil {
		panic(fmt.Sprintf("could not create logger: %v", err))
	}

	if log_dir == "stdout" {
		return http.Server{
			Handler: apache_log.Wrap(h, os.Stdout),
		}
	} else if log_dir == "" {
		return http.Server{
			Handler: apache_log.Wrap(h, os.Stderr),
		}
	} else if log_dir == "none" {
		return http.Server{
			Handler: h,
		}
	}

	log_file := log_dir
	link_name := log_dir
	if !strings.HasSuffix(log_dir, "/") {
		log_file += "/"
		link_name += "/"

	}
	log_file += "access_log.%Y%m%d%H%M"
	link_name += "current"

	rl, err := rotatelogs.New(
		log_file,
		rotatelogs.WithLinkName(link_name),
		rotatelogs.WithMaxAge(time.Duration(log_rotate)*86400*time.Second),
		rotatelogs.WithRotationTime(time.Second*86400),
	)
	if err != nil {
		panic(fmt.Sprintf("rotatelogs.New failed: %v", err))
	}

	return http.Server{
		Handler: apache_log.Wrap(h, rl),
	}
}

func main() {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf(`chocon %s
Compiler: %s %s
`,
			Version,
			runtime.Compiler,
			runtime.Version())
		return

	}

	requestConverter := func(r *http.Request, pr *http.Request, ps *proxy_handler.ProxyStatus) {
		if r.Host == "" {
			ps.Status = http.StatusBadRequest
			return
		}
		host, _, err := net.SplitHostPort(r.Host)
		if err != nil {
			ps.Status = http.StatusBadRequest
			return
		}
		hostSplited := strings.Split(host, ".")
		lastPartIndex := 0
		for i, hostPart := range hostSplited {
			if hostPart == "ccnproxy-ssl" || hostPart == "ccnproxy-secure" || hostPart == "ccnproxy-https" || hostPart == "ccnproxy" {
				lastPartIndex = i
			}
		}
		if lastPartIndex == 0 {
			ps.Status = http.StatusBadRequest
			return
		}

		pr.URL.Host = strings.Join(hostSplited[0:lastPartIndex], ".")
		pr.Host = pr.URL.Host
		if hostSplited[lastPartIndex] == "ccnproxy-https" || hostSplited[lastPartIndex] == "ccnproxy-secure" || hostSplited[lastPartIndex] == "ccnproxy-ssl" {
			pr.URL.Scheme = "https"
		}
	}

	var transport http.RoundTripper = &http.Transport{
		// inherited http.DefaultTransport
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// self-customized values
		MaxIdleConnsPerHost:   opts.KeepaliveConns,
		ResponseHeaderTimeout: time.Duration(opts.ProxyReadTimeout) * time.Second,
	}

	proxyHandler := addStatsHandler(proxy_handler.NewProxyWithRequestConverter(requestConverter, &transport))

	l, err := ss.NewListener()
	if l == nil || err != nil {
		// Fallback if not running under Server::Starter
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%s", opts.Listen, opts.Port))
		if err != nil {
			panic(fmt.Sprintf("Failed to listen to port %s:%s", opts.Listen, opts.Port))
		}
	}

	server := addLogHandler(proxyHandler, opts.LogDir, opts.LogRotate)
	server.ReadTimeout = time.Duration(opts.ReadTimeout) * time.Second
	server.WriteTimeout = time.Duration(opts.WriteTimeout) * time.Second
	server.Serve(l)
}
