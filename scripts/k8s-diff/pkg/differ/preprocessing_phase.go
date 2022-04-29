package differ

import (
	"bytes"
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ObjectRule interface {
	Describe() ObjectRuleDescription
	MapObject(obj *unstructured.Unstructured, debug *RuleDebugInfo) (*unstructured.Unstructured, error)
}

type ObjectRuleDescription struct {
	Name       string
	MatchRules Json6902Patch
	PatchRules Json6902Patch
}

type RuleSet struct {
	IgnoreRules []IgnoreRule        `yaml:"ignore_rules"`
	Patches     []Json6902PatchRule `yaml:"patches"`
}

func (r *RuleSet) Merge(other *RuleSet) {
	r.IgnoreRules = append(r.IgnoreRules, other.IgnoreRules...)
	r.Patches = append(r.Patches, other.Patches...)
}

func (r *RuleSet) Desugar() {
	for i := range r.Patches {
		r.Patches[i] = Desugar(r.Patches[i])
	}
}

type Json6902PatchRule struct {
	Name  string        `yaml:"name,omitempty"`
	Match Json6902Patch `yaml:"match,omitempty"`
	Steps Json6902Patch `yaml:"steps,omitempty"`

	// These fields exist to support syntax sugar.
	// They are converted to the above fields when the rule is created.
	MatchKind    string      `yaml:"match_kind,omitempty"`
	RemoveField  string      `yaml:"remove_field,omitempty"`
	RenameField  *RenameRule `yaml:"rename_field,omitempty"`
	RenameObject *RenameRule `yaml:"rename_object,omitempty"`
}

type RenameRule struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
}

func (j Json6902PatchRule) Describe() ObjectRuleDescription {
	return ObjectRuleDescription{
		Name:       j.Name,
		MatchRules: j.Match,
		PatchRules: j.Steps,
	}
}

func (j Json6902PatchRule) MapObject(obj *unstructured.Unstructured, debug *RuleDebugInfo) (*unstructured.Unstructured, error) {
	ok, err := j.Match.Matches(obj, debug)
	if err != nil {
		// Intentionally not returning the error here. The test operator will
		// return an error if the match fails. It's also useful to allow errors
		// in the match to be ignored so that property existence can be tested.
		return obj, nil
	}
	if !ok {
		return obj, nil
	}

	err = j.Steps.ApplyToObject(obj, debug)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

type Json6902Patch []Json6902Operation

func (j Json6902Patch) String() string {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(buf).Encode(j)
	if err != nil {
		panic(err)
	}
	return buf.String()
}

func (j Json6902Patch) Matches(obj *unstructured.Unstructured, debug *RuleDebugInfo) (bool, error) {
	obj = obj.DeepCopy()
	for i, step := range j {
		matches, err := step.Matches(obj)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil
		}
		debug.RecordIncrementalMatch(i, obj)
	}
	return true, nil
}

func (j Json6902Patch) ApplyToObject(obj *unstructured.Unstructured, debug *RuleDebugInfo) error {
	for i, step := range j {
		originalObj := obj.DeepCopy()
		err := step.ApplyToObject(obj)
		if err != nil {
			return err
		}
		debug.RecordIncrementalPatch(i, originalObj, obj)
	}
	return nil
}

type Json6902Operation struct {
	Op    string      `yaml:"op" json:"op"`                 // Required for all
	Path  string      `yaml:"path" json:"path"`             // Required for all
	From  string      `yaml:"from" json:"from,omitempty"`   // Required for copy / move
	Value interface{} `yaml:"value" json:"value,omitempty"` // Required for add / replace / test
}

func (j Json6902Operation) String() string {
	switch j.Op {
	case "add":
		return "add " + j.Path + ": " + j.Value.(string)
	case "remove":
		return "remove " + j.Path
	case "replace":
		return "replace " + j.Path + ": " + j.Value.(string)
	case "move":
		return "move " + j.Path + " from " + j.From
	case "copy":
		return "copy " + j.Path + " from " + j.From
	case "test":
		return "test " + j.Path + ": " + j.Value.(string)
	default:
		return j.Op
	}
}

func (j Json6902Operation) Matches(obj *unstructured.Unstructured) (bool, error) {
	buf := new(bytes.Buffer)
	err := unstructured.UnstructuredJSONScheme.Encode(obj, buf)
	if err != nil {
		return false, err
	}

	_, err = j.Apply(buf.Bytes())
	return err == nil, nil
}

func (j Json6902Operation) ApplyToObject(obj *unstructured.Unstructured) error {
	buf := new(bytes.Buffer)
	err := unstructured.UnstructuredJSONScheme.Encode(obj, buf)
	if err != nil {
		return err
	}

	objBuf, err := j.Apply(buf.Bytes())
	if err != nil {
		return err
	}

	// Decode the json back into an object
	_, _, err = unstructured.UnstructuredJSONScheme.Decode(objBuf, nil, obj)
	return err
}

func (j Json6902Operation) Apply(buf []byte) ([]byte, error) {
	patchJson, err := json.Marshal(Json6902Patch{j})
	if err != nil {
		return nil, err
	}

	op := jsonpatch.Patch{}
	err = json.Unmarshal(patchJson, &op)
	if err != nil {
		return nil, err
	}

	return op.Apply(buf)
}

type IgnoreRule struct {
	Match Json6902Patch `yaml:"match"`
	Name  string        `yaml:"name"`
}

func (e IgnoreRule) Describe() ObjectRuleDescription {
	return ObjectRuleDescription{
		Name:       e.Name,
		MatchRules: e.Match,
		PatchRules: nil,
	}
}

func (e IgnoreRule) MapObject(obj *unstructured.Unstructured, debug *RuleDebugInfo) (*unstructured.Unstructured, error) {
	ok, err := e.Match.Matches(obj, debug)
	if err != nil {
		// Intentionally not returning the error here. The test operator will
		// return an error if the match fails. It's also useful to allow errors
		// in the match to be ignored so that property existence can be tested.
		return obj, nil
	}
	if !ok {
		return obj, nil
	}
	return nil, nil
}
