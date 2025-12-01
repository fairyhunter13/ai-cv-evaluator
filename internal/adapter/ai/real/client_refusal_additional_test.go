package real

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRefusalResponse_PatternsAndNonRefusal(t *testing.T) {
	refusal := "I'm sorry, but I cannot help with that request due to our safety guidelines."
	nonRefusal := "Here is the code snippet you requested for your Go unit tests."

	require.True(t, isRefusalResponse(refusal))
	require.False(t, isRefusalResponse(nonRefusal))
}

func TestIsRefusalByPattern_ShortAndPolicyBased(t *testing.T) {
	shortRefusal := "Sorry, I can't."
	policyRefusal := "This violates our guidelines and internal policy, so I cannot proceed."
	nonRefusal := "This is a detailed explanation about your project without any refusal language."

	require.True(t, isRefusalByPattern(shortRefusal))
	require.True(t, isRefusalByPattern(policyRefusal))
	require.False(t, isRefusalByPattern(nonRefusal))
}

func TestClient_DetectRefusalWithValidation_CodeBasedAndLowQuality(t *testing.T) {
	c := newTestClient()

	codeBased := "I'm sorry, I cannot help with that due to policy."
	isRefusal, reason := c.detectRefusalWithValidation(context.Background(), codeBased)
	require.True(t, isRefusal)
	require.Equal(t, "code-based pattern detection", reason)

	lowQuality := "ok ok ok ok ok" // short, low-content response without explicit refusal phrases
	isRefusal2, reason2 := c.detectRefusalWithValidation(context.Background(), lowQuality)
	require.True(t, isRefusal2)
	require.Equal(t, "low quality response detected", reason2)
}

func TestClient_IsLowQualityResponse_TrueAndFalse(t *testing.T) {
	c := newTestClient()

	// Very short / low-content
	require.True(t, c.isLowQualityResponse("short"))

	// Reasonable, detailed response should not be considered low quality
	good := "Here is a detailed, multi-sentence answer that explains the reasoning and provides concrete steps."
	require.False(t, c.isLowQualityResponse(good))
}
