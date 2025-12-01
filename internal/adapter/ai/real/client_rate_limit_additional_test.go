package real

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fairyhunter13/ai-cv-evaluator/internal/config"
)

func newTestClient() *Client {
	return &Client{
		cfg: config.Config{
			OpenRouterAPIKey:  "key1",
			OpenRouterAPIKey2: "key2",
			GroqAPIKey:        "g1",
			GroqAPIKey2:       "g2",
			AIWorkerReplicas:  1,
		},
	}
}

func TestClient_BlockAndCheckOpenRouterGlobal(t *testing.T) {
	c := newTestClient()

	require.False(t, c.isOpenRouterBlocked())

	c.blockOpenRouter(10 * time.Millisecond)
	require.True(t, c.isOpenRouterBlocked())

	time.Sleep(20 * time.Millisecond)
	require.False(t, c.isOpenRouterBlocked())
}

func TestClient_BlockAndCheckOpenRouterAccounts(t *testing.T) {
	c := newTestClient()

	// Initially not blocked
	require.False(t, c.isOpenRouterAccountBlocked("key1"))
	require.False(t, c.isOpenRouterAccountBlocked("key2"))

	c.blockOpenRouterAccount("key1", 10*time.Millisecond)
	require.True(t, c.isOpenRouterAccountBlocked("key1"))
	require.False(t, c.isOpenRouterAccountBlocked("key2"))

	time.Sleep(20 * time.Millisecond)
	require.False(t, c.isOpenRouterAccountBlocked("key1"))
}

func TestClient_BlockAndCheckGroqAccounts(t *testing.T) {
	c := newTestClient()

	require.False(t, c.isGroqAccountBlocked("g1"))
	require.False(t, c.isGroqAccountBlocked("g2"))

	c.blockGroqAccount("g1", 10*time.Millisecond)
	require.True(t, c.isGroqAccountBlocked("g1"))
	require.False(t, c.isGroqAccountBlocked("g2"))

	time.Sleep(20 * time.Millisecond)
	require.False(t, c.isGroqAccountBlocked("g1"))
}
