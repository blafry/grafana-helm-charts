package differ

import (
	"bytes"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type DebugInfo struct {
	RuleDebugInfos []*RuleDebugInfo
	InitialObjects []*unstructured.Unstructured
}

func (d *DebugInfo) ValidateAllRulesWereEffective() error {
	for _, ruleDebugInfo := range d.RuleDebugInfos {
		if err := ruleDebugInfo.ValidateAllStepsWereEffective(); err != nil {
			return err
		}
	}
	return nil
}

func (d *DebugInfo) NewRuleDebugInfo(rule ObjectRule) *RuleDebugInfo {
	ruleDebugInfo := &RuleDebugInfo{
		Parent:  d,
		Rule:    rule,
		Matches: make([]IncrementalMatchDebugInfo, len(rule.Describe().MatchRules)),
		Patches: make([]IncrementalPatchDebugInfo, len(rule.Describe().PatchRules)),
	}
	d.RuleDebugInfos = append(d.RuleDebugInfos, ruleDebugInfo)
	return ruleDebugInfo
}

// RuleDebugInfo is created during the rule application process and can be used to
// understand the state of the system after each rule application or to debug
// the rule application process.
type RuleDebugInfo struct {
	Parent  *DebugInfo
	Rule    ObjectRule
	Matches []IncrementalMatchDebugInfo
	Patches []IncrementalPatchDebugInfo
}

type IneffectiveMatchError struct {
	RuleName  string
	Step      int
	Matched   []*unstructured.Unstructured
	MatchRule Json6902Operation
}

func (e IneffectiveMatchError) Error() string {
	candidateStrings := []string{}
	for _, u := range e.Matched {
		candidateStrings = append(candidateStrings, resourceKeyForObject(u).String())
	}

	return fmt.Sprintf("rule %q matching step %d:\n\t %s did not match any objects in:\n\t\t%s", e.RuleName, e.Step, e.MatchRule, strings.Join(candidateStrings, "\n\t\t"))
}

type IneffectivePatchError struct {
	RuleName  string
	Step      int
	Matched   []*unstructured.Unstructured
	PatchRule Json6902Operation
}

func (e IneffectivePatchError) Error() string {
	candidateStrings := []string{}
	for _, u := range e.Matched {
		candidateStrings = append(candidateStrings, resourceKeyForObject(u).String())
	}

	return fmt.Sprintf("rule %q patching step %d:\n\t %s did not change any objects in:\n\t\t%s", e.RuleName, e.Step, e.PatchRule, strings.Join(candidateStrings, "\n\t\t"))
}

func (d *RuleDebugInfo) ValidateAllStepsWereEffective() error {
	previousMatchedObjects := d.Parent.InitialObjects
	// Validate that all matches matched at least one object.
	for step, debugInfo := range d.Matches {
		if len(debugInfo.matchedObjects) == 0 {
			return IneffectiveMatchError{
				RuleName:  d.Rule.Describe().Name,
				Step:      step,
				Matched:   previousMatchedObjects,
				MatchRule: d.Rule.Describe().MatchRules[step],
			}
		}
		previousMatchedObjects = debugInfo.matchedObjects
	}

	// Validate that all patches changed at least one object.
	for step, debugInfo := range d.Patches {
		if len(debugInfo.patchedObjects) == 0 {
			return IneffectivePatchError{
				RuleName:  d.Rule.Describe().Name,
				Step:      step,
				Matched:   previousMatchedObjects,
				PatchRule: d.Rule.Describe().PatchRules[step],
			}
		}

		// Validate that patches aren't all empty.
		atLeastOneNonEmptyPatch := false
		for _, op := range debugInfo.patchedObjects {
			if !bytes.Equal(op.patch, []byte("{}")) {
				atLeastOneNonEmptyPatch = true
			}
		}

		if !atLeastOneNonEmptyPatch {
			return IneffectivePatchError{
				RuleName:  d.Rule.Describe().Name,
				Step:      step,
				Matched:   previousMatchedObjects,
				PatchRule: d.Rule.Describe().PatchRules[step],
			}
		}
	}

	return nil
}

func (d *RuleDebugInfo) Print() {
	if d == nil {
		return
	}
	fmt.Printf("Rule: %s\n", d.Rule.Describe().Name)
	for step, debugInfo := range d.Matches {
		fmt.Printf("Step %d: %v\n", step, d.Rule)
		fmt.Printf("  Matched:\n")
		for _, u := range debugInfo.matchedObjects {
			fmt.Printf("    %s\n", resourceKeyForObject(u))
		}
	}

	for step, debugInfo := range d.Patches {
		fmt.Printf("Step %d: %v\n", step, d.Rule)
		fmt.Printf("  Patched:\n")
		for _, op := range debugInfo.patchedObjects {
			fmt.Printf("    %s -> %s\n", resourceKeyForObject(op.oldObj), resourceKeyForObject(op.newObj))
		}
	}
}

func (d *RuleDebugInfo) RecordIncrementalMatch(step int, obj *unstructured.Unstructured) {
	if d == nil {
		return
	}
	d.Matches[step].matchedObjects = append(d.Matches[step].matchedObjects, obj)
}

func (d *RuleDebugInfo) RecordIncrementalPatch(step int, oldObj, newObj *unstructured.Unstructured) {
	if d == nil {
		return
	}
	d.Patches[step].patchedObjects = append(d.Patches[step].patchedObjects, objectPatch{
		oldObj: oldObj,
		newObj: newObj,
		patch:  createPatch(oldObj, newObj),
	})
}

type IncrementalMatchDebugInfo struct {
	matchedObjects []*unstructured.Unstructured
}

type IncrementalPatchDebugInfo struct {
	patchedObjects []objectPatch
}

type objectPatch struct {
	oldObj, newObj *unstructured.Unstructured
	patch          []byte
}

func createPatch(oldObj, newObj *unstructured.Unstructured) []byte {
	oldObjBuf := new(bytes.Buffer)
	err := unstructured.UnstructuredJSONScheme.Encode(oldObj, oldObjBuf)
	if err != nil {
		panic(err)
	}

	newObjBuf := new(bytes.Buffer)
	err = unstructured.UnstructuredJSONScheme.Encode(newObj, newObjBuf)
	if err != nil {
		panic(err)
	}

	patchResult, err := jsonpatch.CreateMergePatch(oldObjBuf.Bytes(), newObjBuf.Bytes())
	if err != nil {
		panic(err)
	}

	return patchResult
}
