package spanneradapter

import (
	"reflect"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestPageTokenRoundtrip checks that a page token decodes to the values it was encoded from,
// with int64 values keeping their type.
func TestPageTokenRoundtrip(t *testing.T) {
	fp := Fingerprint("p", "f", "o", false)
	in := PageToken{OrderValues: []any{int64(9), "z"}, KeyValues: []any{"k1", int64(7)}}
	out, err := DecodePageToken(EncodePageToken(in, fp), fp)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %#v", out) // int64 must survive as int64
	}
}

// TestPageTokenFingerprintMismatch checks that decoding a token under a different fingerprint
// fails with InvalidArgument.
func TestPageTokenFingerprintMismatch(t *testing.T) {
	tok := EncodePageToken(PageToken{}, Fingerprint("p", "f", "o", false))
	_, err := DecodePageToken(tok, Fingerprint("p", "DIFFERENT", "o", false))
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("want InvalidArgument, got %v", err)
	}
}

// TestPageTokenGarbage checks that decoding a malformed token fails with InvalidArgument.
func TestPageTokenGarbage(t *testing.T) {
	if _, err := DecodePageToken("!!!", 1); status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
}

// TestPageTokenTimeValue checks that a time.Time order value survives a page token round trip.
func TestPageTokenTimeValue(t *testing.T) {
	fp := Fingerprint("", "", "ts desc", false)
	ts := time.Date(2026, 8, 20, 1, 2, 3, 400, time.UTC)
	out, err := DecodePageToken(EncodePageToken(PageToken{OrderValues: []any{ts}}, fp), fp)
	if err != nil || !out.OrderValues[0].(time.Time).Equal(ts) {
		t.Fatalf("%v %v", out, err)
	}
}
