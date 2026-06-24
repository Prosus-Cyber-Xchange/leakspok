package pattern_test

import (
	"context"
	"testing"

	pattern2 "github.com/Prosus-Cyber-Xchange/leakspok/pattern"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper functions for testing
func alwaysTrue(context.Context, []byte) bool      { return true }
func alwaysFalse(context.Context, []byte) bool     { return false }
func isHello(_ context.Context, input []byte) bool { return string(input) == "hello" }
func isWorld(_ context.Context, input []byte) bool { return string(input) == "world" }
func isTest(_ context.Context, input []byte) bool  { return string(input) == "test" }

func TestPatternFunc(t *testing.T) {
	t.Run("PatternFunc implements Pattern interface", func(t *testing.T) {
		pf := pattern2.PatternFunc(alwaysTrue)
		assert.True(t, pf.Match(t.Context(), []byte("anything")))

		pf = pattern2.PatternFunc(alwaysFalse)
		assert.False(t, pf.Match(t.Context(), []byte("anything")))
	})

	t.Run("PatternFunc with custom logic", func(t *testing.T) {
		pf := pattern2.PatternFunc(isHello)
		assert.True(t, pf.Match(t.Context(), []byte("hello")))
		assert.False(t, pf.Match(t.Context(), []byte("world")))
	})
}

func TestNot(t *testing.T) {
	t.Run("Not negates true pattern", func(t *testing.T) {
		truePattern := pattern2.PatternFunc(alwaysTrue)
		notPattern := pattern2.Not(truePattern)

		assert.False(t, notPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Not negates false pattern", func(t *testing.T) {
		falsePattern := pattern2.PatternFunc(alwaysFalse)
		notPattern := pattern2.Not(falsePattern)

		assert.True(t, notPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Not with custom pattern", func(t *testing.T) {
		helloPattern := pattern2.PatternFunc(isHello)
		notHelloPattern := pattern2.Not(helloPattern)

		assert.False(t, notHelloPattern.Match(t.Context(), []byte("hello")))
		assert.True(t, notHelloPattern.Match(t.Context(), []byte("world")))
	})

	t.Run("Double negation", func(t *testing.T) {
		truePattern := pattern2.PatternFunc(alwaysTrue)
		doubleNot := pattern2.Not(pattern2.Not(truePattern))

		assert.True(t, doubleNot.Match(t.Context(), []byte("test")))
	})
}

func TestAll(t *testing.T) {
	t.Run("All with all true patterns", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)
		p3 := pattern2.PatternFunc(alwaysTrue)

		allPattern := pattern2.All(p1, p2, p3)
		assert.True(t, allPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("All with one false pattern", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)

		allPattern := pattern2.All(p1, p2, p3)
		assert.False(t, allPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("All with all false patterns", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)

		allPattern := pattern2.All(p1, p2)
		assert.False(t, allPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("All with single pattern", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		allPattern := pattern2.All(p1)
		assert.True(t, allPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("All with no patterns", func(t *testing.T) {
		allPattern := pattern2.All()
		// AtLeastN don't accept n=0, so it defaults to n=1, therefore returning false
		assert.False(t, allPattern.Match(t.Context(), []byte("test")))
	})
}

func TestAnd(t *testing.T) {
	t.Run("And with both true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)

		andPattern := pattern2.And(p1, p2)
		assert.True(t, andPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("And with first false", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysTrue)

		andPattern := pattern2.And(p1, p2)
		assert.False(t, andPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("And with second false", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)

		andPattern := pattern2.And(p1, p2)
		assert.False(t, andPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("And with both false", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)

		andPattern := pattern2.And(p1, p2)
		assert.False(t, andPattern.Match(t.Context(), []byte("test")))
	})
}

func TestAny(t *testing.T) {
	t.Run("Any with all true patterns", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)
		p3 := pattern2.PatternFunc(alwaysTrue)

		anyPattern := pattern2.Any(p1, p2, p3)
		assert.True(t, anyPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Any with one true pattern", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysTrue)
		p3 := pattern2.PatternFunc(alwaysFalse)

		anyPattern := pattern2.Any(p1, p2, p3)
		assert.True(t, anyPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Any with all false patterns", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)

		anyPattern := pattern2.Any(p1, p2)
		assert.False(t, anyPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Any with single pattern", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		anyPattern := pattern2.Any(p1)
		assert.True(t, anyPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Any with no patterns", func(t *testing.T) {
		anyPattern := pattern2.Any()
		// AtLeastN with n=1 and no patterns should return false
		assert.False(t, anyPattern.Match(t.Context(), []byte("test")))
	})
}

func TestOr(t *testing.T) {
	t.Run("Or with both true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)

		orPattern := pattern2.Or(p1, p2)
		assert.True(t, orPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Or with first true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)

		orPattern := pattern2.Or(p1, p2)
		assert.True(t, orPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Or with second true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysTrue)

		orPattern := pattern2.Or(p1, p2)
		assert.True(t, orPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("Or with both false", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)

		orPattern := pattern2.Or(p1, p2)
		assert.False(t, orPattern.Match(t.Context(), []byte("test")))
	})
}

func TestAtLeastN(t *testing.T) {
	t.Run("AtLeastN with n=1 and 3 patterns, 2 true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)

		atLeastPattern := pattern2.AtLeastN(1, p1, p2, p3)
		assert.True(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with n=2 and 3 patterns, 2 true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)

		atLeastPattern := pattern2.AtLeastN(2, p1, p2, p3)
		assert.True(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with n=3 and 3 patterns, 2 true", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)

		atLeastPattern := pattern2.AtLeastN(3, p1, p2, p3)
		assert.False(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with n=0 (should be normalized to 1)", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)

		atLeastPattern := pattern2.AtLeastN(0, p1, p2)
		assert.False(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with negative n (should be normalized to 1)", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)

		atLeastPattern := pattern2.AtLeastN(-5, p1, p2)
		assert.True(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with n greater than pattern count", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)

		// n=5 but only 2 patterns, should be normalized to 2
		atLeastPattern := pattern2.AtLeastN(5, p1, p2)
		assert.True(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN early termination - too many failures", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysFalse)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)

		// Need 2 passes, but after 2 failures we know it's impossible
		atLeastPattern := pattern2.AtLeastN(2, p1, p2, p3)
		assert.False(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN early termination - enough passes", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysTrue)
		p3 := pattern2.PatternFunc(alwaysFalse)

		// Need 2 passes, should return true after first 2
		atLeastPattern := pattern2.AtLeastN(2, p1, p2, p3)
		assert.True(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})

	t.Run("AtLeastN with no patterns", func(t *testing.T) {
		atLeastPattern := pattern2.AtLeastN(1)
		assert.False(t, atLeastPattern.Match(t.Context(), []byte("test")))
	})
}

func TestComplexCombinations(t *testing.T) {
	t.Run("Complex combination: (A AND B) OR (C AND D)", func(t *testing.T) {
		a := pattern2.PatternFunc(isHello)
		b := pattern2.PatternFunc(alwaysTrue)
		c := pattern2.PatternFunc(isWorld)
		d := pattern2.PatternFunc(alwaysTrue)

		combined := pattern2.Or(
			pattern2.And(a, b),
			pattern2.And(c, d),
		)

		assert.True(t, combined.Match(t.Context(), []byte("hello")))
		assert.True(t, combined.Match(t.Context(), []byte("world")))
		assert.False(t, combined.Match(t.Context(), []byte("test")))
	})

	t.Run("Complex combination: NOT(A OR B) AND C", func(t *testing.T) {
		a := pattern2.PatternFunc(isHello)
		b := pattern2.PatternFunc(isWorld)
		c := pattern2.PatternFunc(isTest)

		combined := pattern2.And(
			pattern2.Not(pattern2.Or(a, b)),
			c,
		)

		assert.False(t, combined.Match(t.Context(), []byte("hello")))
		assert.False(t, combined.Match(t.Context(), []byte("world")))
		assert.True(t, combined.Match(t.Context(), []byte("test")))
		assert.False(t, combined.Match(t.Context(), []byte("other")))
	})

	t.Run("Complex combination with AtLeastN", func(t *testing.T) {
		p1 := pattern2.PatternFunc(alwaysTrue)
		p2 := pattern2.PatternFunc(alwaysFalse)
		p3 := pattern2.PatternFunc(alwaysTrue)
		p4 := pattern2.PatternFunc(alwaysTrue)

		// At least 3 out of 4 should pass
		combined := pattern2.AtLeastN(3, p1, p2, p3, p4)
		assert.True(t, combined.Match(t.Context(), []byte("test")))

		// At least 4 out of 4 should fail (only 3 are true)
		combined = pattern2.AtLeastN(4, p1, p2, p3, p4)
		assert.False(t, combined.Match(t.Context(), []byte("test")))
	})
}

func TestEqual(t *testing.T) {
	t.Run("matches equal bytes", func(t *testing.T) {
		p := pattern2.Equal([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("hello")))
	})

	t.Run("does not match different bytes", func(t *testing.T) {
		p := pattern2.Equal([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("world")))
	})

	t.Run("does not match empty vs non-empty", func(t *testing.T) {
		p := pattern2.Equal([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("matches empty reference with empty input", func(t *testing.T) {
		p := pattern2.Equal([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("does not match different length", func(t *testing.T) {
		p := pattern2.Equal([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("hello!")))
	})

	t.Run("does not match nil vs non-nil", func(t *testing.T) {
		p := pattern2.Equal([]byte("hello"))
		assert.False(t, p.Match(t.Context(), nil))
	})

	t.Run("matches nil reference with nil input", func(t *testing.T) {
		p := pattern2.Equal(nil)
		assert.True(t, p.Match(t.Context(), nil))
	})

	t.Run("case-sensitive comparison", func(t *testing.T) {
		p := pattern2.Equal([]byte("Hello"))
		assert.False(t, p.Match(t.Context(), []byte("hello")))
		assert.True(t, p.Match(t.Context(), []byte("Hello")))
	})
}

func TestIgnoreCaseEqual(t *testing.T) {
	t.Run("matches same case", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("hello")))
	})

	t.Run("matches different case", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("HELLO")))
	})

	t.Run("matches mixed case", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("Hello"))
		assert.True(t, p.Match(t.Context(), []byte("hElLo")))
	})

	t.Run("does not match different content", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("world")))
	})

	t.Run("does not match different length", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("HELLO!")))
	})

	t.Run("matches empty with empty", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("matches nil with nil", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual(nil)
		assert.True(t, p.Match(t.Context(), nil))
	})

	t.Run("unicode case folding", func(t *testing.T) {
		p := pattern2.IgnoreCaseEqual([]byte("straße"))
		// bytes.EqualFold handles Unicode case folding
		assert.True(t, p.Match(t.Context(), []byte("Straße")))
	})
}

func TestStartsWith(t *testing.T) {
	t.Run("matches when prefix matches", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("hello world")))
	})

	t.Run("matches exact match", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("hello")))
	})

	t.Run("does not match when prefix wrong", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("world hello")))
	})

	t.Run("does not match shorter input", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("hel")))
	})

	t.Run("empty prefix matches everything", func(t *testing.T) {
		p := pattern2.StartsWith([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("anything")))
	})

	t.Run("empty prefix matches empty", func(t *testing.T) {
		p := pattern2.StartsWith([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("non-empty prefix does not match empty", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("prefix is case-sensitive", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("Hello"))
		assert.False(t, p.Match(t.Context(), []byte("hello world")))
	})

	t.Run("single byte prefix", func(t *testing.T) {
		p := pattern2.StartsWith([]byte("A"))
		assert.True(t, p.Match(t.Context(), []byte("ABC")))
		assert.False(t, p.Match(t.Context(), []byte("BCA")))
	})
}

func TestEndsWith(t *testing.T) {
	t.Run("matches when suffix matches", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("world"))
		assert.True(t, p.Match(t.Context(), []byte("hello world")))
	})

	t.Run("matches exact match", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("hello"))
		assert.True(t, p.Match(t.Context(), []byte("hello")))
	})

	t.Run("does not match when suffix wrong", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("world"))
		assert.False(t, p.Match(t.Context(), []byte("world hello")))
	})

	t.Run("does not match shorter input", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("hello"))
		assert.False(t, p.Match(t.Context(), []byte("lo")))
	})

	t.Run("empty suffix matches everything", func(t *testing.T) {
		p := pattern2.EndsWith([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("anything")))
	})

	t.Run("empty suffix matches empty", func(t *testing.T) {
		p := pattern2.EndsWith([]byte(""))
		assert.True(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("non-empty suffix does not match empty", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("world"))
		assert.False(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("suffix is case-sensitive", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("World"))
		assert.False(t, p.Match(t.Context(), []byte("hello world")))
	})

	t.Run("single byte suffix", func(t *testing.T) {
		p := pattern2.EndsWith([]byte("Z"))
		assert.True(t, p.Match(t.Context(), []byte("ABCZ")))
		assert.False(t, p.Match(t.Context(), []byte("ZABC")))
	})
}

func TestEdgeCases(t *testing.T) {
	t.Run("Empty byte slice", func(t *testing.T) {
		p := pattern2.PatternFunc(func(_ context.Context, input []byte) bool {
			return len(input) == 0
		})

		assert.True(t, p.Match(t.Context(), []byte("")))
		assert.False(t, p.Match(t.Context(), []byte("not empty")))
	})

	t.Run("Nil byte slice", func(t *testing.T) {
		p := pattern2.PatternFunc(func(_ context.Context, input []byte) bool {
			return input == nil
		})

		assert.True(t, p.Match(t.Context(), nil))
		assert.False(t, p.Match(t.Context(), []byte("")))
	})

	t.Run("Large byte slice", func(t *testing.T) {
		largeData := make([]byte, 10000)
		for i := range largeData {
			largeData[i] = byte(i % 256)
		}

		p := pattern2.PatternFunc(func(_ context.Context, input []byte) bool {
			return len(input) > 5000
		})

		assert.True(t, p.Match(t.Context(), largeData))
		assert.False(t, p.Match(t.Context(), []byte("small")))
	})
}

func TestRegex(t *testing.T) {
	t.Run("matches simple pattern", func(t *testing.T) {
		p, err := pattern2.Regex(`^hello$`)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("hello")))
		assert.False(t, p.Match(t.Context(), []byte("world")))
	})

	t.Run("matches Go module version pattern", func(t *testing.T) {
		p, err := pattern2.Regex(`@v\d+\.\d+\.\d+$`)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("metric@v1.44.0")))
		assert.True(t, p.Match(t.Context(), []byte("otel/metric@v0.1.2")))
		assert.False(t, p.Match(t.Context(), []byte("user@example.com")))
		assert.False(t, p.Match(t.Context(), []byte("john@gmail.com")))
	})

	t.Run("matches partial content", func(t *testing.T) {
		p, err := pattern2.Regex(`foo`)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("foobar")))
		assert.True(t, p.Match(t.Context(), []byte("barfoo")))
		assert.False(t, p.Match(t.Context(), []byte("bar")))
	})

	t.Run("matches with character classes", func(t *testing.T) {
		p, err := pattern2.Regex(`\d{3}-\d{2}-\d{4}`)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("123-45-6789")))
		assert.False(t, p.Match(t.Context(), []byte("abc-de-fghi")))
	})

	t.Run("matches with case-insensitive flag", func(t *testing.T) {
		p, err := pattern2.Regex(`(?i)hello`)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("HELLO")))
		assert.True(t, p.Match(t.Context(), []byte("Hello")))
	})

	t.Run("empty regex matches any input", func(t *testing.T) {
		p, err := pattern2.Regex(``)
		require.NoError(t, err)
		assert.True(t, p.Match(t.Context(), []byte("")))
		assert.True(t, p.Match(t.Context(), []byte("anything")))
	})

	t.Run("non-matching pattern returns false for nil input", func(t *testing.T) {
		p, err := pattern2.Regex(`^hello$`)
		require.NoError(t, err)
		assert.False(t, p.Match(t.Context(), nil))
	})

	t.Run("invalid regex returns error", func(t *testing.T) {
		_, err := pattern2.Regex(`[invalid`)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to compile regex pattern")
	})

	t.Run("name includes expression", func(t *testing.T) {
		p, err := pattern2.Regex(`^test_\d+$`)
		require.NoError(t, err)
		assert.Equal(t, "Regex(^test_\\d+$)", p.Name())
	})
}
