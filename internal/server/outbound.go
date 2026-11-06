package server

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// outbound is the client every request the server makes goes through: the sources, the covers
// and open copies they point to. It reaches the public internet only. A source's record, or a
// repository's redirect, names the address, so it could otherwise make the server ask its own
// loopback services, the droplet's other apps, or the cloud's metadata address. The check is
// made as each connection is dialled, so a redirect or a name that resolves elsewhere later is
// held to it too. A redirect may not fall from https to http.
func outbound() *http.Client {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: 10 * time.Second, Control: publicOnly}).DialContext
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: t,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
				return fmt.Errorf("refused a redirect from https to %s", req.URL.Scheme)
			}
			return nil
		},
	}
}

// cgnat is the shared address space carriers put behind their NAT, not the public internet.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// publicOnly refuses to dial any address that is not a public one.
func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || cgnat.Contains(ip) {
		return fmt.Errorf("refused to connect to %s: not a public address", ip)
	}
	return nil
}
