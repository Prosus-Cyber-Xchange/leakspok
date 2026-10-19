package monitoring_test

import (
	"context"
	"strings"
	"testing"

	"github.com/Prosus-Cyber-Xchange/leakspok/monitoring"
)

// cacheTagRecorder records the spans started for cache span helpers so tests
// can assert the exact cache.token tag contract.
type cacheTagRecorder struct {
	spans []*cacheTagSpan
}

func (r *cacheTagRecorder) StartSpanFromContext(ctx context.Context, _ string) (context.Context, monitoring.Span) {
	span := &cacheTagSpan{tags: map[string]string{}}
	r.spans = append(r.spans, span)

	return ctx, span
}

type cacheTagSpan struct {
	tags map[string]string
}

func (s *cacheTagSpan) SetTag(key, value string) { s.tags[key] = value }
func (s *cacheTagSpan) Finish()                  {}

// TestCacheSpanTags_TokenDigestPrefix asserts the cache.token tag contract for
// both span helpers: the tag must equal the first 12 hex characters of the
// SHA-256 digest of the token and must never carry the raw token text, which
// is unsanitized PII candidate material. Span granularity is unchanged: each
// token-scoped operation starts exactly one span.
func TestCacheSpanTags_TokenDigestPrefix(t *testing.T) {
	// Expected digests are precomputed independently of the implementation
	// (sha256(token), first 12 lowercase hex characters) so the assertions
	// cannot pass by mirroring the implementation's own computation.
	cases := []struct {
		name       string
		trace      func(context.Context, string) (context.Context, func())
		token      string
		wantDigest string
	}{
		{
			name:       "get matches long token",
			trace:      monitoring.TraceCacheGetMatches,
			token:      "john.doe@example.com",
			wantDigest: "836f82db9912",
		},
		{
			name:       "save matches long token",
			trace:      monitoring.TraceCacheSaveMatches,
			token:      "jane.roe@example.com",
			wantDigest: "22fff12b355c",
		},
		{
			name:       "get matches short token",
			trace:      monitoring.TraceCacheGetMatches,
			token:      "abc",
			wantDigest: "ba7816bf8f01",
		},
		{
			name:       "save matches short token",
			trace:      monitoring.TraceCacheSaveMatches,
			token:      "abc",
			wantDigest: "ba7816bf8f01",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &cacheTagRecorder{}
			original := monitoring.GlobalTracer()
			monitoring.SetGlobalTracer(recorder)
			defer monitoring.SetGlobalTracer(original)

			_, finish := tc.trace(context.Background(), tc.token)
			finish()

			if len(recorder.spans) != 1 {
				t.Fatalf("exactly one span must be started per token-scoped operation, got %d", len(recorder.spans))
			}

			tag := recorder.spans[0].tags["cache.token"]
			if tag != tc.wantDigest {
				t.Errorf("cache.token tag = %q, want the 12-hex SHA-256 prefix %q", tag, tc.wantDigest)
			}
			if strings.Contains(tag, tc.token) {
				t.Errorf("cache.token tag %q must never contain the raw token text %q", tag, tc.token)
			}
		})
	}
}
