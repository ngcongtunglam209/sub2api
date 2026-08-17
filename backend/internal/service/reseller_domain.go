package service

import (
	"context"
	"net"
	"strings"
	"time"
)

// ResellerDomainStatusActive is the only status that both issues a certificate
// and admits traffic. Anything else is off.
const ResellerDomainStatusActive = "active"

// ResellerDomain is a reseller-owned hostname pointed at this deployment.
type ResellerDomain struct {
	ID        int64
	Domain    string
	UserID    int64
	Status    string
	Notes     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ResellerDomainRepository is the persistence contract. Declared here rather
// than in `repository` so the service layer owns its own dependency shape.
type ResellerDomainRepository interface {
	// ListActiveDomains returns every active hostname, lowercased.
	//
	// Deliberately whole-set rather than a per-host lookup: this answers on
	// every TLS handshake with an unknown host *and* on every proxied request,
	// and the row count is operator-scale (tens), not user-scale. One query
	// feeding an in-memory set beats a cache entry per hostname, and it removes
	// the need for a negative cache — a host absent from the set is denied
	// without a round trip, which is exactly the case an attacker generates.
	ListActiveDomains(ctx context.Context) ([]string, error)

	Create(ctx context.Context, domain *ResellerDomain) (*ResellerDomain, error)
	ListByUser(ctx context.Context, userID int64) ([]*ResellerDomain, error)
	List(ctx context.Context) ([]*ResellerDomain, error)
	SetStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

// NormalizeDomain canonicalises a hostname for storage and lookup.
//
// The Host header is client-controlled, so nothing about its shape can be
// assumed: it may carry a port, a trailing root dot, mixed case, or
// surrounding space. All four have to collapse to the same key or an attacker
// picks whichever spelling misses the allowlist.
func NormalizeDomain(host string) string {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return ""
	}

	// net.SplitHostPort rather than a hand-rolled colon split: it is the only
	// form that gets bracketed IPv6 right, where the address itself is full of
	// colons. It errors when there is no port at all, which is the common case
	// here, so the original string stands.
	if stripped, _, err := net.SplitHostPort(h); err == nil && stripped != "" {
		h = stripped
	}

	// SplitHostPort already unwraps the brackets around an IPv6 literal, but it
	// only runs when a port was present. Without this the same address
	// normalises two different ways depending on whether the client sent a
	// port, which is exactly the inconsistency this function exists to remove.
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		h = h[1 : len(h)-1]
	}

	return strings.TrimSuffix(h, ".")
}
