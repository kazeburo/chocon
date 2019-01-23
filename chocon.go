package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	stats_api "github.com/fukata/golang-stats-api-handler"
	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/chocon/proxy"
	apachelog "github.com/lestrrat/go-apache-logformat"
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	ss "github.com/lestrrat/go-server-starter-listener"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
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
	KeepaliveConns   int    `short:"c" default:"2" long:"keepalive-conns" description:"maximum keepalive connections for upstream"`
	ReadTimeout      int    `long:"read-timeout" default:"30" description:"timeout of reading request"`
	WriteTimeout     int    `long:"write-timeout" default:"90" description:"timeout of writing response"`
	ProxyReadTimeout int    `long:"proxy-read-timeout" default:"60" description:"timeout of reading response from upstream"`
	Upstream         string `long:"upstream" default:"" description:"upstream server: http://upstream-server/"`
	UpstreamH2C      bool   `long:"upstream-h2c" description:"use h2c for upstream"`
}

type upstreamRoundTripper struct {
	rt http.RoundTripper
}

func (upstr *upstreamRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return upstr.rt.RoundTrip(req)
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

func addLogHandler(h http.Handler, logDir string, logRotate int64) http.Server {
	apacheLog, err := apachelog.New(`%h %l %u %t "%r" %>s %b "%v" %T.%{msec_frac}t %{X-Chocon-Req}i`)
	h2s := &http2.Server{}

	if err != nil {
		panic(fmt.Sprintf("could not create logger: %v", err))
	}

	if logDir == "stdout" {
		return http.Server{
			Handler: h2c.NewHandler(apacheLog.Wrap(h, os.Stdout), h2s),
		}
	} else if logDir == "" {
		return http.Server{
			Handler: h2c.NewHandler(apacheLog.Wrap(h, os.Stderr), h2s),
		}
	} else if logDir == "none" {
		return http.Server{
			Handler: h2c.NewHandler(h, h2s),
		}
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
		panic(fmt.Sprintf("rotatelogs.New failed: %v", err))
	}
	return http.Server{
		Handler: h2c.NewHandler(apacheLog.Wrap(h, rl), h2s),
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

	upstreamURL := new(url.URL)
	if opts.Upstream != "" {
		upstreamURL, err = url.Parse(opts.Upstream)
		if err != nil {
			panic(fmt.Sprintf("upsteam url is invalid: %v", err))
		}
		if upstreamURL.Scheme != "http" && upstreamURL.Scheme != "https" {
			panic(fmt.Sprintf("upsteam url is invalid: %s", "upsteam url scheme should be http or https"))
		}
		if upstreamURL.Host == "" {
			panic(fmt.Sprintf("upsteam url is invalid: %s", "no hostname"))
		}
	}

	requestConverter := func(r *http.Request, pr *http.Request, ps *proxy.ProxyStatus) {
		if r.Host == "" {
			ps.Status = http.StatusBadRequest
			return
		}
		hostPortSplit := strings.Split(r.Host, ":")
		host := hostPortSplit[0]
		port := ""
		if len(hostPortSplit) > 1 {
			port = ":" + hostPortSplit[1]
		}
		hostSplit := strings.Split(host, ".")
		lastPartIndex := 0
		for i, hostPart := range hostSplit {
			if hostPart == "ccnproxy-ssl" || hostPart == "ccnproxy-secure" || hostPart == "ccnproxy-https" || hostPart == "ccnproxy" {
				lastPartIndex = i
			}
		}
		if lastPartIndex == 0 {
			ps.Status = http.StatusBadRequest
			return
		}

		pr.URL.Host = strings.Join(hostSplit[0:lastPartIndex], ".") + port
		pr.Host = pr.URL.Host
		if hostSplit[lastPartIndex] == "ccnproxy-https" || hostSplit[lastPartIndex] == "ccnproxy-secure" || hostSplit[lastPartIndex] == "ccnproxy-ssl" {
			pr.URL.Scheme = "https"
		}
	}

	var transports []http.RoundTripper
	if upstreamURL.Scheme != "" && opts.UpstreamH2C {
		c := 10
		for c > 0 {
			transports = append(transports, &upstreamRoundTripper{
				rt: &http2.Transport{
					AllowHTTP: true,
					DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
						return net.Dial(network, addr)
					},
				},
			})
			c--
		}
	} else {
		transports = append(transports, &upstreamRoundTripper{
			rt: &http.Transport{
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
			},
		})
	}

	proxyHandler := addStatsHandler(proxy.NewProxyWithRequestConverter(requestConverter, transports, upstreamURL))

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
