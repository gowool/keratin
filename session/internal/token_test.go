package internal

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToken(t *testing.T) {
	t.Run("generates non-empty token", func(t *testing.T) {
		token, err := Token()
		require.NoError(t, err)
		assert.NotEmpty(t, token)
	})

	t.Run("generates valid base64 URL-safe encoding", func(t *testing.T) {
		token, err := Token()
		require.NoError(t, err)

		base64Pattern := `^[A-Za-z0-9_-]+$`
		matched, err := regexp.MatchString(base64Pattern, token)
		require.NoError(t, err)
		assert.True(t, matched, "token should contain only base64 URL-safe characters")
	})

	t.Run("generates tokens of correct length", func(t *testing.T) {
		token, err := Token()
		require.NoError(t, err)

		expectedLength := 43
		assert.Len(t, token, expectedLength, "32 bytes encoded in base64 should produce 43 characters")
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		const numTokens = 100

		for range numTokens {
			token, err := Token()
			require.NoError(t, err)
			assert.False(t, tokens[token], "token %q should be unique", token)
			tokens[token] = true
		}

		assert.Len(t, tokens, numTokens, "all generated tokens should be unique")
	})

	t.Run("tokens have consistent format", func(t *testing.T) {
		tokens := make([]string, 10)
		for i := range tokens {
			token, err := Token()
			require.NoError(t, err)
			tokens[i] = token
		}

		for i, token := range tokens {
			assert.Len(t, token, 43, "token %d should have correct length", i)
		}
	})
}

func BenchmarkToken(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Token()
	}
}

func BenchmarkToken_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = Token()
		}
	})
}
