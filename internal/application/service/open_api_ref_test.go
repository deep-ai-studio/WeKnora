package service

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateReferencePreview(t *testing.T) {
	short := truncateReferencePreview("hello world", 80)
	assert.Equal(t, "hello world", short)

	long := truncateReferencePreview("a"+string(make([]byte, 200)), 10)
	assert.True(t, len(long) <= 13)
	assert.True(t, len(long) > 10)
}

func TestSlimOpenAPIReferences(t *testing.T) {
	raw := []*types.SearchResult{{
		ID:                "chunk-1",
		Content:           "full chunk body that should not appear in open api response",
		KnowledgeID:       "doc-1",
		KnowledgeTitle:    "Report.pdf",
		KnowledgeFilename: "Report.pdf",
		ChunkIndex:        3,
		Score:             0.88,
	}}
	refs := slimOpenAPIReferences(raw)
	require.Len(t, refs, 1)
	assert.Equal(t, "chunk-1", refs[0].ID)
	assert.Equal(t, "Report.pdf", refs[0].KnowledgeTitle)
	assert.Contains(t, refs[0].ContentPreview, "full chunk")
}
