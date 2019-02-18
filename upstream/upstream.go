package upstream

import (
	"context"
	"math/rand"
	"net"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
)

// Upstream struct
type Upstream struct {
	scheme string
	port   string
	host   string
	ipwcs  []*IPwc
	csum   string
	logger *zap.Logger
	mu     sync.Mutex
	// current resolved record version
	version uint64
}

// IPwc : IP with counter
type IPwc struct {
	ip string
	// # requerst in busy
	busy int64
	// resolved record version
	version uint64
}

// New :
func New(upstream string, logger *zap.Logger) (*Upstream, error) {
	var h string
	var p string
	var err error
	u := new(url.URL)

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
	}

	um := &Upstream{
		scheme:  u.Scheme,
		host:    h,
		port:    p,
		version: 0,
		logger:  logger,
	}

	if um.Enabled() {
		ctx := context.Background()
		ipwcs, err := um.RefreshIP(ctx)
		if err != nil {
			return nil, errors.Wrap(err, "failed initial resolv hostname")
		}
		if len(ipwcs) < 1 {
			return nil, errors.New("Could not resolv hostname")
		}
		go um.Run(ctx)
	}
	return um, nil
}

// Enabled : upstream is enabled
func (u *Upstream) Enabled() bool {
	return u.scheme != ""
}

// GetScheme : get upstream's scheme
func (u *Upstream) GetScheme() string {
	return u.scheme
}

// RefreshIP : resolve hostname
func (u *Upstream) RefreshIP(ctx context.Context) ([]*IPwc, error) {
	u.mu.Lock()
	u.version++
	u.mu.Unlock()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, u.host)
	cancel()
	if err != nil {
		return nil, err
	}

	sort.Slice(addrs, func(i, j int) bool {
		return addrs[i].IP.String() > addrs[j].IP.String()
	})

	ips := make([]string, len(addrs))
	ipwcs := make([]*IPwc, len(addrs))
	for i, ia := range addrs {
		ips[i] = ia.IP.String()
		ipwcs[i] = &IPwc{
			ip:      ia.IP.String(),
			version: u.version,
			busy:    0, // dummy
		}
	}
	csum := strings.Join(ips, ",")
	u.mu.Lock()
	defer u.mu.Unlock()
	if csum != u.csum {
		u.csum = csum
		u.ipwcs = ipwcs
	}

	return ipwcs, nil
}

// Run : resolv hostname in background
func (u *Upstream) Run(ctx context.Context) {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _ = <-ticker.C:
			//for _, ipwc := range u.ipwcs {
			//	log.Printf("%v", ipwc)
			//}
			_, err := u.RefreshIP(ctx)
			if err != nil {
				u.logger.Error("failed refresh ip", zap.Error(err))
			}
		}
	}
}

// Get : wild
func (u *Upstream) Get() (string, *IPwc, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	if len(u.ipwcs) < 1 {
		return "", &IPwc{}, errors.New("No upstream hosts")
	}

	sort.Slice(u.ipwcs, func(i, j int) bool {
		if u.ipwcs[i].busy == u.ipwcs[j].busy {
			return rand.Intn(1) == 0
		}
		return u.ipwcs[i].busy < u.ipwcs[j].busy
	})

	u.ipwcs[0].busy++
	h := u.ipwcs[0].ip
	if u.port != "" {
		h = h + ":" + u.port
	}
	ipwc := &IPwc{
		ip:      u.ipwcs[0].ip,
		version: u.ipwcs[0].version,
		busy:    0,
	}
	return h, ipwc, nil
}

// Release : decrement counter
func (u *Upstream) Release(o *IPwc) {
	u.mu.Lock()
	defer u.mu.Unlock()
	for i, ipwc := range u.ipwcs {
		if ipwc.ip == o.ip && ipwc.version == o.version {
			u.ipwcs[i].busy = u.ipwcs[i].busy - 1
		}
	}
}
