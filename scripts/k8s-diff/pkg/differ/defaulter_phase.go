package differ

import (
	"context"
	"log"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/scale/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewDryRunK8sClient() (client.Client, error) {
	sch := runtime.NewScheme()
	scheme.AddToScheme(sch)
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		log.Fatal(err)
	}

	clientConfig := clientcmd.NewDefaultClientConfig(*config, nil)
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		log.Fatal(err)
	}

	cli, err := client.New(restConfig, client.Options{
		Scheme: sch,
	})
	if err != nil {
		log.Fatal(err)
	}

	return client.NewDryRunClient(cli), nil
}

type DefaultSettingRule struct {
	client client.Client
}

func NewDefaultSettingRule(client client.Client) *DefaultSettingRule {
	return &DefaultSettingRule{client: client}
}

func (d *DefaultSettingRule) Describe() ObjectRuleDescription {
	return ObjectRuleDescription{
		Name:       "Fill in default values",
		MatchRules: nil,
		PatchRules: Json6902Patch{{Op: "set_defaults"}},
	}
}

func (d *DefaultSettingRule) MapObject(obj *unstructured.Unstructured, debug *RuleDebugInfo) (*unstructured.Unstructured, error) {
	oldObj := obj.DeepCopy()
	didSetNamespace := false
	if obj.GetNamespace() == "" {
		didSetNamespace = true
		obj.SetNamespace("default")
	}

	err := d.client.Create(context.Background(), obj)
	if err != nil {
		return nil, err
	}
	// These fields are added by the server-side dry-run. They are unique to the
	// object, so they will always appear as different
	patch := Json6902Patch{
		{Op: "remove", Path: "/metadata/creationTimestamp"},
		{Op: "remove", Path: "/metadata/managedFields"},
		{Op: "remove", Path: "/metadata/uid"},
	}
	err = patch.ApplyToObject(obj, nil)
	if err != nil {
		return nil, err
	}
	if didSetNamespace {
		obj.SetNamespace("")
	}
	// We don't need to record each individual step change in the debug info,
	// instead we just record the whole patch as one step.
	debug.RecordIncrementalPatch(0, oldObj, obj)

	return obj, nil
}
