package cache_test

import (
	"context"
	"errors"
	"testing"

	analyzercache "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache"
	cachemock "github.com/Prosus-Cyber-Xchange/leakspok/analyzer/cache/mocks"
	"github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestCacheStoreTracer_GetMatch(t *testing.T) {
	t.Parallel()

	errCache := errors.New("cache error")

	tests := []struct {
		name        string
		wantMatched bool
		wantErr     error
	}{
		{
			name:        "returns matched=true with no error",
			wantMatched: true,
			wantErr:     nil,
		},
		{
			name:        "returns matched=false with no error",
			wantMatched: false,
			wantErr:     nil,
		},
		{
			name:        "propagates error from underlying store",
			wantMatched: false,
			wantErr:     errCache,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUnderlying := cachemock.NewMockCacheStore(ctrl)

			entity := pattern.EntityEmail
			data := []byte("test-data")

			mockUnderlying.EXPECT().
				GetMatch(gomock.Any(), entity, data).
				Return(tt.wantMatched, tt.wantErr)

			tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
			matched, err := tracer.GetMatch(context.Background(), entity, data)

			assert.Equal(t, tt.wantMatched, matched)
			assert.Equal(t, tt.wantErr, err)
		})
	}
}

func TestCacheStoreTracer_SaveMatch(t *testing.T) {
	t.Parallel()

	errSave := errors.New("save error")

	tests := []struct {
		name    string
		matched bool
		wantErr error
	}{
		{
			name:    "saves matched=true with no error",
			matched: true,
			wantErr: nil,
		},
		{
			name:    "saves matched=false with no error",
			matched: false,
			wantErr: nil,
		},
		{
			name:    "propagates error from underlying store",
			matched: true,
			wantErr: errSave,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockUnderlying := cachemock.NewMockCacheStore(ctrl)

			entity := pattern.EntityEmail
			data := []byte("test-data")

			mockUnderlying.EXPECT().
				SaveMatch(gomock.Any(), entity, data, tt.matched).
				Return(tt.wantErr)

			tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
			err := tracer.SaveMatch(context.Background(), entity, data, tt.matched)

			assert.Equal(t, tt.wantErr, err)
		})
	}
}

type cacheTracerTestKey struct{}

func TestCacheStoreTracer_GetMatch_ForwardsSpanContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUnderlying := cachemock.NewMockCacheStore(ctrl)

	entity := pattern.EntityEmail
	data := []byte("test-data")

	var capturedCtx context.Context
	mockUnderlying.EXPECT().
		GetMatch(gomock.Any(), entity, data).
		DoAndReturn(func(ctx context.Context, _ pattern.Entity, _ []byte) (bool, error) {
			capturedCtx = ctx
			return false, nil
		})

	callerCtx := context.WithValue(context.Background(), cacheTracerTestKey{}, "marker")
	tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
	_, _ = tracer.GetMatch(callerCtx, entity, data)

	assert.Equal(t, "marker", capturedCtx.Value(cacheTracerTestKey{}))
}

func TestCacheStoreTracer_SaveMatch_ForwardsSpanContext(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockUnderlying := cachemock.NewMockCacheStore(ctrl)

	entity := pattern.EntityEmail
	data := []byte("test-data")

	var capturedCtx context.Context
	mockUnderlying.EXPECT().
		SaveMatch(gomock.Any(), entity, data, true).
		DoAndReturn(func(ctx context.Context, _ pattern.Entity, _ []byte, _ bool) error {
			capturedCtx = ctx
			return nil
		})

	callerCtx := context.WithValue(context.Background(), cacheTracerTestKey{}, "marker")
	tracer := analyzercache.NewCacheStoreTracer(mockUnderlying)
	_ = tracer.SaveMatch(callerCtx, entity, data, true)

	assert.Equal(t, "marker", capturedCtx.Value(cacheTracerTestKey{}))
}
