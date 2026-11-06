// Package ratelimit is a small per-key token bucket for the sign-in and write routes.
package ratelimit

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Limiter tracks one bucket per key.
type Limiter struct {
	mu      sync.Mutex
	limit   rate.Limit
	burst   int
	entries map[string]*entry
	pruned  time.Time
}

type entry struct {
	bucket *rate.Limiter
	seen   time.Time
}

// New allows perMinute sustained events per key with the given burst.
func New(perMinute float64, burst int) *Limiter {
	return &Limiter{limit: rate.Limit(perMinute / 60), burst: burst, entries: map[string]*entry{}}
}

// Past pruneAt keys, idle buckets are swept at most once a minute, so a flood of new keys
// cannot make every request a scan of the map. At the ceiling the sweep takes anything
// idle a minute, and a key not seen before is refused until there is room.
const (
	pruneAt = 4096
	ceiling = 65536
)

// Allow reports whether the key may proceed now.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	e, ok := l.entries[key]
	if !ok {
		if len(l.entries) >= pruneAt && now.Sub(l.pruned) > time.Minute {
			idle := 10 * time.Minute
			if len(l.entries) >= ceiling {
				idle = time.Minute
			}
			l.prune(now.Add(-idle))
			l.pruned = now
		}
		if len(l.entries) >= ceiling {
			return false
		}
		e = &entry{bucket: rate.NewLimiter(l.limit, l.burst)}
		l.entries[key] = e
	}
	e.seen = now
	return e.bucket.Allow()
}

func (l *Limiter) prune(cutoff time.Time) {
	for k, e := range l.entries {
		if e.seen.Before(cutoff) {
			delete(l.entries, k)
		}
	}
}

// Middleware rejects requests over the limit with a problem response. keyFn
// picks the bucket; an empty key is never limited.
func (l *Limiter) Middleware(keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if key := keyFn(r); key != "" && !l.Allow(key) {
				w.Header().Set("Content-Type", "application/problem+json")
				w.Header().Set("Retry-After", "10")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"status":429,"title":"Too Many Requests","detail":"slow down and try again shortly"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientKey is the caller as the limits count them: their address, or for IPv6 its /64,
// which one household or one attacker has whole.
func ClientKey(r *http.Request) string {
	ip := net.ParseIP(ClientIP(r))
	if ip == nil || ip.To4() != nil {
		return ClientIP(r)
	}
	return ip.Mask(net.CIDRMask(64, 128)).String() + "/64"
}

// ClientIP is the caller's address. Behind the reverse proxy the connection comes from
// loopback and the real address is the last X-Forwarded-For entry: the one our own proxy
// appended, which nothing upstream of it can forge. Apache names the connecting client
// there, or, with mod_remoteip, the visitor Cloudflare passed on.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			if i := strings.LastIndexByte(fwd, ','); i >= 0 {
				fwd = fwd[i+1:]
			}
			if last := strings.TrimSpace(fwd); last != "" {
				return last
			}
		}
	}
	return host
}
