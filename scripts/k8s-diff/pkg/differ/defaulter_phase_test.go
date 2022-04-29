package differ

import (
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDefaultMapper(t *testing.T) {
	dep := newDeploymentWithLabels("querier", map[string]string{
		"app": "querier",
	})

	cli, err := NewDryRunK8sClient()
	require.NoError(t, err)

	defaulter := DefaultSettingRule{client: cli}
	result, err := defaulter.MapObject(dep, nil)
	require.NoError(t, err)
	strategy, exists, err := unstructured.NestedString(result.Object, "spec", "strategy", "type")
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, "RollingUpdate", strategy)
}

func TestDefaultMapperSetsNamespace(t *testing.T) {
	// The defaulter will internally set a namespace if it's missing, but we
	// don't want to impact the final output, so it should not be set in the
	// output.
	dep := newDeploymentWithLabels("querier", map[string]string{
		"app": "querier",
	})
	dep.SetNamespace("")

	cli, err := NewDryRunK8sClient()
	require.NoError(t, err)

	defaulter := DefaultSettingRule{client: cli}
	result, err := defaulter.MapObject(dep, nil)
	require.NoError(t, err)
	require.Equal(t, "", result.GetNamespace())
}
