package main


import(
	"fmt"
	"os"
	"strings"
	"net"
	"net/http"
	"github.com/lestrrat/go-apache-logformat"
	"github.com/lestrrat/go-server-starter-listener"
	"github.com/fukata/golang-stats-api-handler"
	"github.com/kazeburo/chocon/proxy"
)


func addStatsHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Index(r.URL.Path, "/.api/stats") == 0  {
			stats_api.Handler(w, r)
		} else {
			h.ServeHTTP(w, r)
		}
	})
}

func main() {
	requestConverter := func(r *http.Request, pr *http.Request, ps *proxy_handler.ProxyStatus)  {
		if r.Host == "" {
			ps.Status = http.StatusBadRequest
			return
		}
		originalHost := r.Host
		hostSplited := strings.Split(originalHost,".")
		lastPartIndex := 0
		for i, hostPart := range hostSplited {
			if hostPart == "ccnproxy-https" || hostPart == "ccnproxy" {
				lastPartIndex = i
			}
		}
		if lastPartIndex == 0 {
			ps.Status = http.StatusBadRequest
			return
		}

		pr.URL.Host = strings.Join(hostSplited[0:lastPartIndex], ".")
		if hostSplited[lastPartIndex] == "ccnproxy-https" {
			pr.URL.Scheme = "https";
		}
	}
	proxyHandler := addStatsHandler(proxy_handler.NewProxyWithRequestConverter(requestConverter))

	l, err := ss.NewListener()
	if l == nil || err != nil {
		// Fallback if not running under Server::Starter
		l, err = net.Listen("tcp", ":3000")
		if err != nil {
			panic("Failed to listen to port 8080")
		}
	}

	apache_log, err := apachelog.New(`%h %l %u %t "%r" %>s %b "%v" %{X-Chocon-Req}i`)
	if err != nil {
		panic(fmt.Sprintf("could not create logger: %v",err))
	}
	// logger := apachelog.CombinedLog.Clone()
	server := http.Server{
		Handler: apache_log.Wrap(proxyHandler, os.Stderr),
	}
	server.Serve(l)
}


