package service

import (
	"strings"

	"github.com/Tencent/WeKnora/internal/types"
)

const openAPIReferencePreviewLen = 80

// slimOpenAPIReferences converts raw pipeline references to the slim shape
// shown in the web UI (docInfo.vue): document title + truncated preview, no full chunk body.
func slimOpenAPIReferences(raw interface{}) []types.OpenAPIReference {
	results := parseSearchResults(raw)
	if len(results) == 0 {
		return nil
	}
	out := make([]types.OpenAPIReference, 0, len(results))
	for _, sr := range results {
		if sr == nil {
			continue
		}
		out = append(out, types.OpenAPIReference{
			ID:                sr.ID,
			KnowledgeID:       sr.KnowledgeID,
			KnowledgeBaseID:   sr.KnowledgeBaseID,
			KnowledgeTitle:    sr.KnowledgeTitle,
			KnowledgeFilename: sr.KnowledgeFilename,
			ChunkIndex:        sr.ChunkIndex,
			Score:             sr.Score,
			ChunkType:         sr.ChunkType,
			ContentPreview:    truncateReferencePreview(sr.Content, openAPIReferencePreviewLen),
		})
	}
	return out
}

func parseSearchResults(raw interface{}) []*types.SearchResult {
	if raw == nil {
		return nil
	}
	if searchResults, ok := raw.([]*types.SearchResult); ok {
		return searchResults
	}
	refs, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]*types.SearchResult, 0, len(refs))
	for _, ref := range refs {
		switch v := ref.(type) {
		case *types.SearchResult:
			out = append(out, v)
		case types.SearchResult:
			sr := v
			out = append(out, &sr)
		}
	}
	return out
}

func truncateReferencePreview(content string, maxLen int) string {
	if content == "" || maxLen <= 0 {
		return ""
	}
	text := strings.Join(strings.Fields(strings.ReplaceAll(content, "\n", " ")), " ")
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "..."
}
