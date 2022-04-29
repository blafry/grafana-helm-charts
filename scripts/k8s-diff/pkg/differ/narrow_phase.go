package differ

import (
	"github.com/banzaicloud/k8s-objectmatcher/patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type NarrowPhaseResult struct {
	LeftToRightPatch *patch.PatchResult
	RightToLeftPatch *patch.PatchResult
	DiffText         []byte
}

func (n NarrowPhaseResult) IsEmpty() bool {
	return n.LeftToRightPatch.IsEmpty() && n.RightToLeftPatch.IsEmpty()
}

func NarrowPhaseComparison(left, right *unstructured.Unstructured) *NarrowPhaseResult {
	rightToLeft, err := patch.DefaultPatchMaker.Calculate(left, right)
	if err != nil {
		panic(err)
	}
	leftToRight, err := patch.DefaultPatchMaker.Calculate(right, left)
	if err != nil {
		panic(err)
	}
	return &NarrowPhaseResult{
		LeftToRightPatch: leftToRight,
		RightToLeftPatch: rightToLeft,
	}
}
