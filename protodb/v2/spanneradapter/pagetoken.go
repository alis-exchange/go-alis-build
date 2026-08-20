package spanneradapter

import (
	"bytes"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// PageToken is the decoded form of an opaque keyset page-token cursor: the
// last row's ORDER BY column values (for resuming the scan) plus its key
// values (as a tiebreaker for rows with equal order values).
type PageToken struct {
	OrderValues []any // last row's ORDER BY column values, in clause order
	KeyValues   []any // last row's key values (tiebreaker)
}

// tokenPayload is the gob-encoded wire form of a page token. gob (not JSON)
// is used deliberately: JSON decodes numeric values into float64, which
// silently corrupts int64 keys/order values that exceed float64's exact
// integer range. gob preserves concrete Go types across the []any fields.
type tokenPayload struct {
	Fingerprint uint64
	OrderValues []any
	KeyValues   []any
}

func init() {
	// string, int64, bool, and float64 are gob's built-in basic types and
	// are auto-registered by the gob package itself, so they need no
	// registration here even though they cross the OrderValues/KeyValues
	// interface (any) boundary. time.Time is the one non-primitive,
	// non-auto-registered type ListOptions order-by values may carry (e.g.
	// timestamp columns), so it must be registered explicitly.
	gob.Register(time.Time{})
}

// EncodePageToken serializes t together with fingerprint into an opaque,
// base64url page-token string.
func EncodePageToken(t PageToken, fingerprint uint64) string {
	var buf bytes.Buffer
	// Encode errors here only arise from unregistered/unsupported types
	// carried in OrderValues/KeyValues, which is a programmer error in the
	// caller, not a runtime condition callers can recover from; encoding
	// intentionally proceeds best-effort per the brief's reference
	// implementation.
	_ = gob.NewEncoder(&buf).Encode(tokenPayload{fingerprint, t.OrderValues, t.KeyValues})
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

// DecodePageToken decodes a page-token string produced by EncodePageToken
// and validates it against fingerprint. It returns an InvalidArgument
// status error if the token is malformed or was minted for a different
// parent/filter/orderBy/tail combination than fingerprint represents.
func DecodePageToken(s string, fingerprint uint64) (PageToken, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return PageToken{}, status.Error(codes.InvalidArgument, "invalid page token")
	}
	var p tokenPayload
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&p); err != nil {
		return PageToken{}, status.Error(codes.InvalidArgument, "invalid page token")
	}
	if p.Fingerprint != fingerprint {
		return PageToken{}, status.Error(codes.InvalidArgument,
			"page token does not match the request; tokens are only valid for the exact parent, filter, orderBy and tail that produced them")
	}
	return PageToken{OrderValues: p.OrderValues, KeyValues: p.KeyValues}, nil
}

// Fingerprint computes a stable fnv64a hash over the canonical tuple that a
// page token must have been minted for: the request's parent, filter,
// orderBy, and tail. Page tokens are only valid for the exact tuple that
// produced them; presenting a token against a request with any different
// parent, filter, orderBy, or tail yields a fingerprint mismatch.
func Fingerprint(parent, filter, orderBy string, tail bool) uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%q|%q|%q|%v", parent, filter, orderBy, tail)
	return h.Sum64()
}
