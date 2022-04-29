package differ

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type BroadPhaseResult struct {
	ObjectsInBoth      []ObjectPair
	ObjectsOnlyOnLeft  []*unstructured.Unstructured
	ObjectsOnlyOnRight []*unstructured.Unstructured
}

type ObjectPair struct {
	Left  *unstructured.Unstructured
	Right *unstructured.Unstructured
}

// BroadPhaseComparison compares two slices of objects and returns the following:
// - Objects in both
// - Objects only on left
// - Objects only on right
// The objects in each list must be uniquely identified by their GroupVersionKind and Name.
func BroadPhaseComparison(leftSide, rightSide []*unstructured.Unstructured) *BroadPhaseResult {
	leftSideMap := make(map[ResourceKey]*unstructured.Unstructured)
	rightSideMap := make(map[ResourceKey]*unstructured.Unstructured)

	for _, obj := range leftSide {
		key := resourceKeyForObject(obj)
		leftSideMap[key] = obj
	}
	for _, obj := range rightSide {
		key := resourceKeyForObject(obj)
		rightSideMap[key] = obj
	}

	result := &BroadPhaseResult{}
	for key, lhs := range leftSideMap {
		if rhs, ok := rightSideMap[key]; ok {
			result.ObjectsInBoth = append(result.ObjectsInBoth, ObjectPair{
				Left:  lhs,
				Right: rhs,
			})
		} else {
			result.ObjectsOnlyOnLeft = append(result.ObjectsOnlyOnLeft, lhs)
		}
	}

	for key, rhs := range rightSideMap {
		if _, ok := leftSideMap[key]; !ok {
			result.ObjectsOnlyOnRight = append(result.ObjectsOnlyOnRight, rhs)
		}
	}

	return result
}
