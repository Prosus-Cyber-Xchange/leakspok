package monitoring

import (
	"context"
)

// TraceCacheGetMatches starts a DataDog APM span for a token-scoped cache read
// (GetMatches) and returns a context with the span attached and a finish
// function. The caller must call the finish function when the operation
// completes.
func TraceCacheGetMatches(ctx context.Context, token string) (context.Context, func()) {
	newCtx, span := GlobalTracer().StartSpanFromContext(ctx, "anonymize_cache_get_matches")
	span.SetTag("cache.token", token)

	return newCtx, func() {
		span.Finish()
	}
}

// TraceCacheSaveMatches starts a DataDog APM span for a token-scoped cache
// write (SaveMatches) and returns a context with the span attached and a
// finish function. The caller must call the finish function when the
// operation completes.
func TraceCacheSaveMatches(ctx context.Context, token string) (context.Context, func()) {
	newCtx, span := GlobalTracer().StartSpanFromContext(ctx, "anonymize_cache_save_matches")
	span.SetTag("cache.token", token)

	return newCtx, func() {
		span.Finish()
	}
}
