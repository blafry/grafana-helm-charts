package differ

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fluxcd/pkg/ssa"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func (o *ObjectDiffer) ReadLeftStateFromDirectory(path string) error {
	var err error
	o.leftState, err = readStateFromDirectory(path)
	o.DebugInfo.InitialObjects = append(o.DebugInfo.InitialObjects, o.leftState...)
	return err
}

func (o *ObjectDiffer) ReadRightStateFromDirectory(path string) error {
	var err error
	o.rightState, err = readStateFromDirectory(path)
	o.DebugInfo.InitialObjects = append(o.DebugInfo.InitialObjects, o.rightState...)
	return err
}

func readStateFromDirectory(path string) ([]*unstructured.Unstructured, error) {
	state := []*unstructured.Unstructured{}
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".yaml" {
			fmt.Printf("%s: skipping non-yaml file\n", path)
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to read k8s resource from yaml: %w", err)
		}

		obj, err := ssa.ReadObject(f)
		if err != nil {
			fmt.Printf("%s: failed to decode k8s resource from yaml: %v\n", path, err)
			return nil
		}

		state = append(state, obj)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return state, nil
}
