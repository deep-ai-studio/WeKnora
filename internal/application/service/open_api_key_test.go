package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashOpenAPIKeyDeterministic(t *testing.T) {
	h1 := HashOpenAPIKey("sk-open-test-key")
	h2 := HashOpenAPIKey("sk-open-test-key")
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 64)
}

func TestGenerateOpenAPIKeyFormat(t *testing.T) {
	plaintext, hash, err := GenerateOpenAPIKey()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(plaintext, openAPIKeyPrefix))
	assert.Equal(t, HashOpenAPIKey(plaintext), hash)
}

func TestBuildOpenAPIInternalUserID(t *testing.T) {
	id := BuildOpenAPIInternalUserID("client-1", "user-42")
	assert.Len(t, id, 36)
	assert.Equal(t, id, BuildOpenAPIInternalUserID("client-1", "user-42"))
	assert.NotEqual(t, id, BuildOpenAPIInternalUserID("client-1", "user-43"))
}

func TestIsKBAllowed(t *testing.T) {
	allowed := []string{"kb-a", "kb-b"}
	assert.True(t, isKBAllowed(allowed, "kb-a"))
	assert.False(t, isKBAllowed(allowed, "kb-c"))
	assert.False(t, isKBAllowed(nil, "kb-a"))
}

func TestResolveOpenAPIChatMode(t *testing.T) {
	mode, err := resolveOpenAPIChatMode("")
	require.NoError(t, err)
	assert.Equal(t, "wiki-qa", mode)

	mode, err = resolveOpenAPIChatMode("wiki-qa")
	require.NoError(t, err)
	assert.Equal(t, "wiki-qa", mode)

	mode, err = resolveOpenAPIChatMode("rag-qa")
	require.NoError(t, err)
	assert.Equal(t, "rag-qa", mode)

	_, err = resolveOpenAPIChatMode("invalid")
	require.Error(t, err)
}
