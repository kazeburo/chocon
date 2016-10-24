package main

import (
	"fmt"
	"github.com/cubicdaiya/chocon/proxy"
	"github.com/fukata/golang-stats-api-handler"
	"github.com/jessevdk/go-flags"
	"github.com/lestrrat/go-apache-logformat"
	"github.com/lestrrat/go-file-rotatelogs"
	"github.com/lestrrat/go-server-starter-listener"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	Version string
)

type cmdOpts struct {
	Listen    string `short:"l" long:"listen" default:"0.0.0.0" description:"address to bind"`
	Port      string `short:"p" long:"port" default:"3000" description:"Port number to bind"`
	LogDir    string `long:"access-log-dir" default:"" description:"directory to store logfiles"`
	LogRotate int64  `long:"access-log-rotate" default:"30" description:"Number of day before remove logs"`
	Version   bool   `short:"v" long:"version" description:"Show version"`
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
	apache_log, err := apachelog.New(`%h %l %u %t "%r" %>s %b "%v" %{X-Chocon-Req}i`)
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

	rl := rotatelogs.New(
		log_file,
		rotatelogs.WithLinkName(link_name),
		rotatelogs.WithMaxAge(time.Duration(log_rotate)*86400*time.Second),
		rotatelogs.WithRotationTime(time.Second*86400),
	)

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
		originalHost := r.Host
		hostSplited := strings.Split(originalHost, ".")
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
		if hostSplited[lastPartIndex] == "ccnproxy-https" || hostSplited[lastPartIndex] == "ccnproxy-secure" || hostSplited[lastPartIndex] == "ccnproxy-ssl" {
			pr.URL.Scheme = "https"
		}
	}
	proxyHandler := addStatsHandler(proxy_handler.NewProxyWithRequestConverter(requestConverter))

	l, err := ss.NewListener()
	if l == nil || err != nil {
		// Fallback if not running under Server::Starter
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%s", opts.Listen, opts.Port))
		if err != nil {
			panic(fmt.Sprintf("Failed to listen to port %s:%s", opts.Listen, opts.Port))
		}
	}

	server := addLogHandler(proxyHandler, opts.LogDir, opts.LogRotate)
	server.Serve(l)
}
