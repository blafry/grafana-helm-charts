package differ

import (
	"fmt"
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/banzaicloud/k8s-objectmatcher/patch"
)

// ResourceKey represents a unique identifier for any object, whether it exists
// in cluster or not. In-cluster objects have a UID field for this purpose, but
// in this code we are dealing with objects that have not yet been persisted.
type ResourceKey struct {
	schema.GroupVersionKind
	Name string
}

func (r ResourceKey) String() string {
	if r.Group == "" {
		return fmt.Sprintf(`%s.%s-%s`, r.Version, r.Kind, r.Name)
	}
	return fmt.Sprintf(`%s-%s.%s-%s`, r.Group, r.Version, r.Kind, r.Name)
}

func resourceKeyForObject(obj *unstructured.Unstructured) ResourceKey {
	return ResourceKey{
		GroupVersionKind: obj.GetObjectKind().GroupVersionKind(),
		Name:             obj.GetName(),
	}
}

type DifferenceResult struct {
	MissingObjects   []ResourceKey
	ExtraObjects     []ResourceKey
	MatchingObjects  []ResourceKey
	DifferentObjects []ObjectDifference
}

func (d *DifferenceResult) RecordMissingObject(obj *unstructured.Unstructured) error {
	d.MissingObjects = append(d.MissingObjects, resourceKeyForObject(obj))
	return nil
}

func (d *DifferenceResult) RecordExtraObject(obj *unstructured.Unstructured) error {
	d.ExtraObjects = append(d.ExtraObjects, resourceKeyForObject(obj))
	return nil
}

func (d *DifferenceResult) RecordObjectWithDifferences(leftToRight, rightToLeft *patch.PatchResult, left, right *unstructured.Unstructured) error {
	d.DifferentObjects = append(d.DifferentObjects, ObjectDifference{
		Key:              resourceKeyForObject(left),
		LeftToRightPatch: leftToRight,
		RightToLeftPatch: rightToLeft,
	})
	return nil
}

func (d *DifferenceResult) Sort() {
	sort.Slice(d.MissingObjects, func(i, j int) bool {
		return d.MissingObjects[i].String() < d.MissingObjects[j].String()
	})
	sort.Slice(d.ExtraObjects, func(i, j int) bool {
		return d.ExtraObjects[i].String() < d.ExtraObjects[j].String()
	})
	sort.Slice(d.MatchingObjects, func(i, j int) bool {
		return d.MatchingObjects[i].String() < d.MatchingObjects[j].String()
	})
	sort.Slice(d.DifferentObjects, func(i, j int) bool {
		return d.DifferentObjects[i].Key.String() < d.DifferentObjects[j].Key.String()
	})
}

type ObjectDifference struct {
	Key              ResourceKey
	LeftToRightPatch *patch.PatchResult
	RightToLeftPatch *patch.PatchResult
}

func (c *DifferenceResult) RecordMatchingObject(left, right *unstructured.Unstructured) error {
	c.MatchingObjects = append(c.MatchingObjects, resourceKeyForObject(right))
	return nil
}

type ObjectDiffer struct {
	leftState  []*unstructured.Unstructured
	rightState []*unstructured.Unstructured

	DebugInfo *DebugInfo
}

func NewObjectDiffer() *ObjectDiffer {
	return &ObjectDiffer{
		DebugInfo: new(DebugInfo),
	}
}

func (o *ObjectDiffer) MapObjects(mapper ObjectRule) error {
	debugInfo := o.DebugInfo.NewRuleDebugInfo(mapper)

	newLeftState := []*unstructured.Unstructured{}
	for _, obj := range o.leftState {
		mapped, err := mapper.MapObject(obj, debugInfo)
		if err != nil {
			return err
		}
		if mapped != nil {
			newLeftState = append(newLeftState, mapped)
		}
	}

	newRightState := []*unstructured.Unstructured{}
	for _, obj := range o.rightState {
		mapped, err := mapper.MapObject(obj, debugInfo)
		if err != nil {
			return err
		}
		if mapped != nil {
			newRightState = append(newRightState, mapped)
		}
	}

	o.leftState = newLeftState
	o.rightState = newRightState
	return nil
}

func (o *ObjectDiffer) CalculateDifference() *DifferenceResult {
	result := &DifferenceResult{}

	broadPhaseResult := BroadPhaseComparison(o.leftState, o.rightState)
	for _, obj := range broadPhaseResult.ObjectsOnlyOnLeft {
		result.RecordMissingObject(obj)
	}
	for _, obj := range broadPhaseResult.ObjectsOnlyOnRight {
		result.RecordExtraObject(obj)
	}
	for _, obj := range broadPhaseResult.ObjectsInBoth {
		narrowPhaseResult := NarrowPhaseComparison(obj.Left, obj.Right)
		if narrowPhaseResult.IsEmpty() {
			result.RecordMatchingObject(obj.Left, obj.Right)
		} else {
			result.RecordObjectWithDifferences(narrowPhaseResult.LeftToRightPatch, narrowPhaseResult.RightToLeftPatch, obj.Left, obj.Right)
		}
	}

	result.Sort()

	return result
}
