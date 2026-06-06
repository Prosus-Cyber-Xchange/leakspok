package monitoring

import (
	"context"
	"sync"
	"testing"
)

func TestGlobalTracer_IsNotNil(t *testing.T) {
	if GlobalTracer() == nil {
		t.Fatal("GlobalTracer must not be nil")
	}
}

func TestNoopTracer_StartSpanFromContext_ReturnsSameContext(t *testing.T) {
	nt := &noopTracer{}
	ctx := context.Background()
	newCtx, span := nt.StartSpanFromContext(ctx, "test-operation")
	if newCtx != ctx {
		t.Errorf("noop tracer must return the same context")
	}
	if span == nil {
		t.Fatal("returned span must not be nil")
	}
}

func TestNoopSpan_SetTag_DoesNotPanic(t *testing.T) {
	_, span := (&noopTracer{}).StartSpanFromContext(context.Background(), "test")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("SetTag panicked: %v", r)
		}
	}()
	span.SetTag("key", "value")
}

func TestNoopSpan_Finish_DoesNotPanic(t *testing.T) {
	_, span := (&noopTracer{}).StartSpanFromContext(context.Background(), "test")
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Finish panicked: %v", r)
		}
	}()
	span.Finish()
}

func TestTracePattern_ReturnsFinishFunc(t *testing.T) {
	ctx, finish := TracePattern(context.Background(), "test-pattern")
	if ctx == nil {
		t.Fatal("TracePattern must return a non-nil context")
	}
	if finish == nil {
		t.Fatal("TracePattern must return a non-nil finish function")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("finish func panicked: %v", r)
		}
	}()
	finish()
}

func TestTraceCacheGet_ReturnsFinishFunc(t *testing.T) {
	ctx, finish := TraceCacheGet(context.Background(), "email")
	if ctx == nil {
		t.Fatal("TraceCacheGet must return a non-nil context")
	}
	if finish == nil {
		t.Fatal("TraceCacheGet must return a non-nil finish function")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("finish func panicked: %v", r)
		}
	}()
	finish()
}

func TestTraceCacheSave_ReturnsFinishFunc(t *testing.T) {
	ctx, finish := TraceCacheSave(context.Background(), "email")
	if ctx == nil {
		t.Fatal("TraceCacheSave must return a non-nil context")
	}
	if finish == nil {
		t.Fatal("TraceCacheSave must return a non-nil finish function")
	}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("finish func panicked: %v", r)
		}
	}()
	finish()
}

func TestNoopTracer_StartSpanFromContext_ContextValuesArePreserved(t *testing.T) {
	type testKey struct{}
	ctx := context.WithValue(context.Background(), testKey{}, "test-value")

	nt := &noopTracer{}
	newCtx, _ := nt.StartSpanFromContext(ctx, "test-op")

	if v, ok := newCtx.Value(testKey{}).(string); !ok || v != "test-value" {
		t.Errorf("context value must be preserved, got %v", v)
	}
}

func TestTracerInterfaces(_ *testing.T) {
	var _ Tracer = &noopTracer{}
	var _ Span = noopSpan{}
}

func TestSetGlobalTracer_ReplacesTracer(t *testing.T) {
	original := GlobalTracer()
	custom := &noopTracer{}

	SetGlobalTracer(custom)
	defer SetGlobalTracer(original)

	if GlobalTracer() != custom {
		t.Fatal("SetGlobalTracer must replace the global tracer")
	}
}

func TestSetGlobalTracer_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup
	tracers := []Tracer{&noopTracer{}, &noopTracer{}, &noopTracer{}}

	for i := range 20 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				SetGlobalTracer(tracers[i%len(tracers)])
			} else {
				_ = GlobalTracer()
			}
		}(i)
	}
	wg.Wait()

	if GlobalTracer() == nil {
		t.Fatal("GlobalTracer must not be nil after concurrent access")
	}
}
