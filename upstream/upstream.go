package upstream

import (
	"context"
	"math/rand"
	"net/url"
	"strings"
	"time"

	"github.com/pkg/errors"
	dnscache "go.mercari.io/go-dnscache"
	"go.uber.org/zap"
)

// Upstream struct
type Upstream struct {
	scheme   string
	port     string
	host     string
	logger   *zap.Logger
	resolver *dnscache.Resolver
}

// New :
func New(upstream string, logger *zap.Logger) (*Upstream, error) {
	var h string
	var p string
	u := new(url.URL)

	resolver, err := dnscache.New(3*time.Second, 10*time.Second, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init dnscache")
	}

	if upstream != "" {
		u, err = url.Parse(upstream)
		if err != nil {
			return nil, errors.Wrap(err, "upsteam url is invalid")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return nil, errors.New("upsteam url is invalid: upsteam url scheme should be http or https")
		}
		if u.Host == "" {
			return nil, errors.New("upsteam url is invalid: no hostname")
		}

		hostPortSplit := strings.Split(u.Host, ":")
		h = hostPortSplit[0]
		p = ""
		if len(hostPortSplit) > 1 {
			p = hostPortSplit[1]
		}

		_, err = resolver.Fetch(context.Background(), h)
		if err != nil {
			return nil, errors.Wrap(err, "failed to resolve upstream")
		}
	}

	return &Upstream{
		scheme:   u.Scheme,
		host:     h,
		port:     p,
		logger:   logger,
		resolver: resolver,
	}, nil
}

// Enabled : upstream is enabled
func (u *Upstream) Enabled() bool {
	return u.scheme != ""
}

// GetScheme : get upstream's scheme
func (u *Upstream) GetScheme() string {
	return u.scheme
}

// GetHost :
func (u *Upstream) GetHost(ctx context.Context) (string, error) {
	ips, err := u.resolver.Fetch(ctx, u.host)
	if err != nil {
		return "", err
	}
	h := ips[rand.Intn(len(ips))].String()
	if u.port != "" {
		h = h + ":" + u.port
	}
	return h, nil
}
