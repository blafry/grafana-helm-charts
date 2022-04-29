package differ

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
)

func newDeploymentWithLabels(name string, labels map[string]string) *unstructured.Unstructured {
	dep := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "querier",
							Image: "gcr.io/nginx/nginx:1.7.9",
						},
					},
				},
			},
		},
	}
	// scheme.Scheme.Default(&dep)

	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(dep)
	if err != nil {
		panic(err)
	}

	var output = new(unstructured.Unstructured)
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(buf.Bytes(), nil, output)
	if err != nil {
		panic(err)
	}
	return output
}

func TestSimplePatch(t *testing.T) {
	t.Run("when the patch succeeds", func(t *testing.T) {
		objectA := newDeploymentWithLabels("querier2", map[string]string{
			"name": "querier",
		})

		objectB := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})

		patch := Json6902Patch{
			Json6902Operation{
				Op:    "replace",
				Path:  "/metadata/name",
				Value: "querier",
			},
		}
		err := patch.ApplyToObject(objectA, nil)
		assert.NoError(t, err, "patch should not fail")

		jsonPatch := createPatch(objectA, objectB)
		assert.Equal(t, jsonPatch, []byte("{}"), "patch should be empty")
	})

	t.Run("when the patch fails", func(t *testing.T) {
		object := newDeploymentWithLabels("querier", map[string]string{
			"app.kubernetes.io/name":      "mimir",
			"app.kubernetes.io/component": "querier",
		})

		patch := Json6902Patch{
			Json6902Operation{
				Op:   "remove",
				Path: "/metadata/labels/name",
			},
		}
		err := patch.ApplyToObject(object, nil)
		assert.Error(t, err, "patch should fail")
	})
}

func TestSimpleMatch(t *testing.T) {
	t.Run("when the object matches", func(t *testing.T) {
		object := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})

		patch := Json6902Patch{
			Json6902Operation{
				Op:    "test",
				Path:  "/metadata/labels/name",
				Value: "querier",
			},
		}
		match, err := patch.Matches(object, nil)
		assert.NoError(t, err, "patch should not fail")
		assert.True(t, match, "patch should match")
	})

	t.Run("when the object does not match", func(t *testing.T) {
		object := newDeploymentWithLabels("querier", map[string]string{
			"name": "querier",
		})

		patch := Json6902Patch{
			Json6902Operation{
				Op:    "test",
				Path:  "/metadata/labels/name",
				Value: "querier2",
			},
		}
		match, err := patch.Matches(object, nil)
		assert.NoError(t, err, "patch should not fail")
		assert.False(t, match, "patch should not match")
	})
}
