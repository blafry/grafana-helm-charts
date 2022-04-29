package differ

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNarrowPhase(t *testing.T) {
	t.Run("Matching objects returns an empty result", func(t *testing.T) {
		left := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})
		right := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})
		result := NarrowPhaseComparison(left, right)
		require.True(t, result.IsEmpty())
	})
	t.Run("Different objects returns a non-empty result", func(t *testing.T) {
		left := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})
		right := newDeploymentWithLabels("ingester", map[string]string{
			"name": "ingester",
		})
		result := NarrowPhaseComparison(left, right)
		require.False(t, result.IsEmpty())
	})
}
