package differ

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestBroadPhase(t *testing.T) {

	leftSide := []*unstructured.Unstructured{
		newDeploymentWithLabels("querier", map[string]string{
			"name": "querier-left",
		}),
		newDeploymentWithLabels("ingester", map[string]string{
			"name": "ingester",
		}),
	}
	rightSide := []*unstructured.Unstructured{
		newDeploymentWithLabels("querier", map[string]string{
			"name": "querier-right",
		}),
		newDeploymentWithLabels("distributor", map[string]string{
			"name": "distributor",
		}),
	}

	result := BroadPhaseComparison(leftSide, rightSide)
	require.Equal(t, 1, len(result.ObjectsInBoth), "querier should be in both")
	require.Equal(t, 1, len(result.ObjectsOnlyOnLeft), "ingester should be only on left")
	require.Equal(t, 1, len(result.ObjectsOnlyOnRight), "distributor should be only on right")
	require.Equal(t, "querier-left", result.ObjectsInBoth[0].Left.GetLabels()["name"])
	require.Equal(t, "querier-right", result.ObjectsInBoth[0].Right.GetLabels()["name"])
	require.Equal(t, "ingester", result.ObjectsOnlyOnLeft[0].GetLabels()["name"])
	require.Equal(t, "distributor", result.ObjectsOnlyOnRight[0].GetLabels()["name"])
}
