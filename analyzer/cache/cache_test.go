package cache_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	analyzercache "github.com/ifood/leakspok/analyzer/cache"
	"github.com/ifood/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"go.uber.org/mock/gomock"
)

func TestNewRuleMatchingCache(t *testing.T) {
	ctx := context.Background()

	t.Run("creates cache with default TTL", func(t *testing.T) {
		ttl := 1 * time.Minute
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: ttl,
		}

		cache, err := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, err)
		require.NotNil(t, cache)
	})

	t.Run("creates cache with zero TTL", func(t *testing.T) {
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 0,
		}

		cache, err := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, err)
		require.NotNil(t, cache)
	})

	t.Run("creates cache with large TTL", func(t *testing.T) {
		ttl := 24 * time.Hour
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: ttl,
		}

		cache, err := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, err)
		require.NotNil(t, cache)
	})
}

func TestRuleMatchingCache_SaveMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	t.Run("saves a match result", func(t *testing.T) {
		entity, data := cacheEntity(ctrl), []byte("test@example.com")
		ctx := context.Background()

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))
	})

	t.Run("saves multiple matches for different data", func(t *testing.T) {
		entity := cacheEntityFor(ctrl, pattern.EntityEmail)
		ctx := context.Background()

		testData := []struct {
			name string
			data []byte
		}{
			{"email1", []byte("test1@example.com")},
			{"email2", []byte("test2@example.com")},
			{"email3", []byte("test3@example.com")},
		}

		for _, td := range testData {
			t.Run(td.name, func(t *testing.T) {
				require.NoError(t, cache.SaveMatch(ctx, entity, td.data, true))
			})
		}
	})

	t.Run("saves both true and false values", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		ctx := context.Background()

		require.NoError(t, cache.SaveMatch(ctx, entity, []byte("test-data"), true))
		require.NoError(t, cache.SaveMatch(ctx, entity, []byte("other-data"), false))
	})
}

func TestRuleMatchingCache_GetMatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	t.Run("retrieves saved match", func(t *testing.T) {
		entity, data := cacheEntity(ctrl), []byte("test@example.com")
		ctx := context.Background()

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)

		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("retrieves false value", func(t *testing.T) {
		entity, data := cacheEntity(ctrl), []byte("no-match-data")
		ctx := context.Background()

		require.NoError(t, cache.SaveMatch(ctx, entity, data, false))

		matched, getErr := cache.GetMatch(ctx, entity, data)

		require.NoError(t, getErr)
		assert.False(t, matched)
	})
}

func TestRuleMatchingCache_CacheHitAndMiss(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	entity := cacheEntity(ctrl)
	data := []byte("cached-value")
	ctx := context.Background()

	t.Run("cache miss returns error", func(t *testing.T) {
		otherEntity := cacheEntityFor(ctrl, pattern.EntityPhone)
		otherData := []byte("non-existent-data-that-was-never-saved")
		_, missErr := cache.GetMatch(ctx, otherEntity, otherData)

		assert.Error(t, missErr)
	})

	t.Run("cache hit after save", func(t *testing.T) {
		saveErr := cache.SaveMatch(ctx, entity, data, true)
		require.NoError(t, saveErr)

		matched, getErr := cache.GetMatch(ctx, entity, data)

		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("different data is different cache entry", func(t *testing.T) {
		data1, data2 := []byte("data1"), []byte("data2")

		require.NoError(t, cache.SaveMatch(ctx, entity, data1, true))
		require.NoError(t, cache.SaveMatch(ctx, entity, data2, false))

		matched1, getErr := cache.GetMatch(ctx, entity, data1)
		require.NoError(t, getErr)
		assert.True(t, matched1)

		matched2, getErr := cache.GetMatch(ctx, entity, data2)
		require.NoError(t, getErr)
		assert.False(t, matched2)
	})
}

func TestRuleMatchingCache_DifferentRules(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	entity1 := cacheEntityFor(ctrl, pattern.EntityEmail)
	entity2 := cacheEntityFor(ctrl, pattern.EntityPhone)
	data := []byte("test-data")
	ctx := context.Background()

	t.Run("different rules have separate cache entries", func(t *testing.T) {
		require.NoError(t, cache.SaveMatch(ctx, entity1, data, true))
		require.NoError(t, cache.SaveMatch(ctx, entity2, data, false))

		matched1, getErr := cache.GetMatch(ctx, entity1, data)
		require.NoError(t, getErr)
		assert.True(t, matched1)

		matched2, getErr := cache.GetMatch(ctx, entity2, data)
		require.NoError(t, getErr)
		assert.False(t, matched2)
	})
}

func TestRuleMatchingCache_ContextHandling(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	entity := cacheEntity(ctrl)
	data := []byte("test-data")

	t.Run("works with background context", func(t *testing.T) {
		ctx := context.Background()

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("works with timeout context", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("works with cancel context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)

		cancel()

		matched, getErr = cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})
}

func TestRuleMatchingCache_LargeData(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	entity := cacheEntity(ctrl)
	ctx := context.Background()

	t.Run("handles small data", func(t *testing.T) {
		data := []byte("x")

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("handles medium data", func(t *testing.T) {
		data := make([]byte, 1024)
		for i := range data {
			data[i] = byte(i % 256)
		}

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("handles data with special characters", func(t *testing.T) {
		data := []byte{0, 1, 2, 255, 254, 253, 127}

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})
}

func TestRuleMatchingCache_KeyGeneration(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("same entity and data produce same cache behavior", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		data := []byte("test-data")

		err1 := cache.SaveMatch(ctx, entity, data, true)
		require.NoError(t, err1)

		matched1, err2 := cache.GetMatch(ctx, entity, data)
		require.NoError(t, err2)
		assert.True(t, matched1)

		err3 := cache.SaveMatch(ctx, entity, data, false)
		require.NoError(t, err3)

		matched2, err4 := cache.GetMatch(ctx, entity, data)
		require.NoError(t, err4)
		assert.False(t, matched2)
	})

	t.Run("similar data with different entities", func(t *testing.T) {
		entity1 := cacheEntityFor(ctrl, pattern.EntityEmail)
		entity2 := cacheEntityFor(ctrl, pattern.EntityPhone)
		data := []byte("same-data")

		err1 := cache.SaveMatch(ctx, entity1, data, true)
		require.NoError(t, err1)

		err2 := cache.SaveMatch(ctx, entity2, data, false)
		require.NoError(t, err2)

		matched1, _ := cache.GetMatch(ctx, entity1, data)
		matched2, _ := cache.GetMatch(ctx, entity2, data)

		assert.NotEqual(t, matched1, matched2)
	})
}

func TestRuleMatchingCache_BufferPoolReuse(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	entity := cacheEntity(ctrl)
	ctx := context.Background()

	t.Run("buffer pool reuses buffers for key generation", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			data := []byte("test-data-" + string(rune(i)))
			require.NoError(t, cache.SaveMatch(ctx, entity, data, true))
		}

		for i := 0; i < 100; i++ {
			data := []byte("test-data-" + string(rune(i)))
			matched, getErr := cache.GetMatch(ctx, entity, data)
			require.NoError(t, getErr)
			assert.True(t, matched)
		}
	})

	t.Run("buffer pool handles varying data sizes", func(t *testing.T) {
		testCases := []struct {
			name string
			size int
		}{
			{"tiny", 1},
			{"small", 10},
			{"medium", 100},
			{"large", 1000},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				data := bytes.Repeat([]byte("x"), tc.size)
				require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

				matched, getErr := cache.GetMatch(ctx, entity, data)
				require.NoError(t, getErr)
				assert.True(t, matched)
			})
		}
	})
}

func TestRuleMatchingCache_EdgeCases(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cache, err := analyzercache.NewRuleMatchingCache(context.Background(), analyzercache.RuleMatchingCacheOptions{
		CacheTTL: 10 * time.Second,
	})
	require.NoError(t, err)

	ctx := context.Background()

	t.Run("empty data", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		data := []byte("")

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("data with only special characters", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		data := []byte("!@#$%^&*()")

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("data with unicode characters", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		data := []byte("测试数据🔒")

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("data with null bytes", func(t *testing.T) {
		entity := cacheEntity(ctrl)
		data := []byte("test\x00data\x00here")

		require.NoError(t, cache.SaveMatch(ctx, entity, data, true))

		matched, getErr := cache.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})
}

// Redis Integration Tests

func TestRuleMatchingCache_RedisBackend(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "docker.io/redis:7")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, redisContainer.Terminate(ctx))
	})

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisAddr := host + ":" + port.Port()

	t.Run("creates cache with redis backend enabled", func(t *testing.T) {
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, cacheErr := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, cacheErr)
		require.NotNil(t, cacheObj)
	})

	t.Run("saves and retrieves from redis backend", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 1 * time.Minute,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, cacheErr := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, cacheErr)

		entity := cacheEntity(ctrl)
		data := []byte("redis-test@example.com")

		require.NoError(t, cacheObj.SaveMatch(ctx, entity, data, true))

		matched, getErr := cacheObj.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("redis cache respects TTL", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		ttl := 100 * time.Millisecond
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: ttl,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, cacheErr := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, cacheErr)

		entity := cacheEntity(ctrl)
		data := []byte("ttl-test@example.com")

		require.NoError(t, cacheObj.SaveMatch(ctx, entity, data, true))

		matched, getErr := cacheObj.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)

		time.Sleep(ttl + 50*time.Millisecond)

		_, missErr := cacheObj.GetMatch(ctx, entity, data)
		require.Error(t, missErr)
		assert.True(t, analyzercache.IsCacheNotFoundError(missErr))
	})
}

func TestRuleMatchingCache_MultiTierCaching(t *testing.T) {
	ctx := context.Background()

	redisContainer, err := redis.Run(ctx, "docker.io/redis:7")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, redisContainer.Terminate(ctx))
	})

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisAddr := host + ":" + port.Port()

	t.Run("memory and redis cache chaining", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Memory: analyzercache.MemoryOptions{
				Enabled:   true,
				CacheSize: 1 * 1024 * 1024,
			},
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, cacheErr := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, cacheErr)

		entity := cacheEntity(ctrl)
		data := []byte("multi-tier@example.com")

		require.NoError(t, cacheObj.SaveMatch(ctx, entity, data, true))

		matched, getErr := cacheObj.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})

	t.Run("redis only cache when memory disabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Memory: analyzercache.MemoryOptions{
				Enabled:   false,
				CacheSize: 0,
			},
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, cacheErr := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, cacheErr)

		entity := cacheEntity(ctrl)
		data := []byte("redis-only@example.com")

		require.NoError(t, cacheObj.SaveMatch(ctx, entity, data, true))

		matched, getErr := cacheObj.GetMatch(ctx, entity, data)
		require.NoError(t, getErr)
		assert.True(t, matched)
	})
}

func TestRuleMatchingCache_RedisErrors(t *testing.T) {
	t.Run("returns error on invalid redis address", func(t *testing.T) {
		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               "invalid-redis:6379",
				DisableClusterMode: true,
				PingOnConnect:      true,
			},
		}

		cache, err := analyzercache.NewRuleMatchingCache(context.Background(), options)
		require.Error(t, err)
		assert.Nil(t, cache)
	})

	t.Run("detects redis nil as cache miss", func(t *testing.T) {
		ctx := context.Background()

		redisContainer, err := redis.Run(ctx, "docker.io/redis:7")
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, redisContainer.Terminate(ctx))
		})

		host, err := redisContainer.Host(ctx)
		require.NoError(t, err)

		port, err := redisContainer.MappedPort(ctx, "6379")
		require.NoError(t, err)

		redisAddr := host + ":" + port.Port()

		options := analyzercache.RuleMatchingCacheOptions{
			CacheTTL: 10 * time.Second,
			Redis: analyzercache.RedisOptions{
				Enabled:            true,
				Addr:               redisAddr,
				DisableClusterMode: true,
			},
		}

		cacheObj, err := analyzercache.NewRuleMatchingCache(ctx, options)
		require.NoError(t, err)

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		entity := cacheEntity(ctrl)
		data := []byte("nonexistent@example.com")

		_, getErr := cacheObj.GetMatch(ctx, entity, data)
		require.Error(t, getErr)
		assert.True(t, analyzercache.IsCacheNotFoundError(getErr))
	})
}

// Helper functions

func cacheEntity(_ *gomock.Controller) pattern.Entity {
	return pattern.EntityEmail
}

func cacheEntityFor(_ *gomock.Controller, entity pattern.Entity) pattern.Entity {
	return entity
}
