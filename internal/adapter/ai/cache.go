// Package ai provides AI client adapters and wrappers used by the application.
package ai

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/domain"
)

// embedCacheClient wraps an AIClient and caches embedding vectors by text hash.
// It is safe for concurrent use.
// Only the Embed method is cached; ChatJSON is passed through.
// Cache is a simple LRU-ish with FIFO eviction for simplicity.

type embedCacheClient struct {
	base     domain.AIClient
	capacity int
	mu       sync.RWMutex
	m        map[string][]float32
	ord      []string
}

// NewEmbedCache wraps base with an embedding cache of given capacity (number of entries).
// If capacity <= 0, base is returned unmodified.
func NewEmbedCache(base domain.AIClient, capacity int) domain.AIClient {
	if capacity <= 0 || base == nil {
		return base
	}
	return &embedCacheClient{base: base, capacity: capacity, m: make(map[string][]float32), ord: make([]string, 0, capacity)}
}

func (c *embedCacheClient) Embed(ctx domain.Context, texts []string) ([][]float32, error) {
	res := make([][]float32, len(texts))
	missIdx := make([]int, 0)
	missTexts := make([]string, 0)
	// Lookup cache
	for i, t := range texts {
		k := keyFor(t)
		c.mu.RLock()
		v, ok := c.m[k]
		c.mu.RUnlock()
		if ok {
			res[i] = v
			continue
		}
		missIdx = append(missIdx, i)
		missTexts = append(missTexts, t)
	}
	if len(missIdx) > 0 {
		vecs, err := c.base.Embed(ctx, missTexts)
		if err != nil {
			return nil, err
		}
		for j, idx := range missIdx {
			res[idx] = vecs[j]
			c.put(missTexts[j], vecs[j])
		}
	}
	return res, nil
}

func (c *embedCacheClient) ChatJSON(ctx domain.Context, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	return c.base.ChatJSON(ctx, systemPrompt, userPrompt, maxTokens)
}

func (c *embedCacheClient) CleanCoTResponse(ctx domain.Context, response string) (string, error) {
	return c.base.CleanCoTResponse(ctx, response)
}

func (c *embedCacheClient) put(text string, vec []float32) {
	k := keyFor(text)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.m[k]; exists {
		c.m[k] = vec
		return
	}
	if len(c.ord) >= c.capacity {
		old := c.ord[0]
		c.ord = c.ord[1:]
		delete(c.m, old)
	}
	c.m[k] = vec
	c.ord = append(c.ord, k)
}

func keyFor(text string) string {
	s := strings.TrimSpace(text)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}
