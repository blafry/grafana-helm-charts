package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"k8s-diff/pkg/differ"
	"os"
	"os/exec"

	"github.com/grafana/dskit/flagext"
	"gopkg.in/yaml.v2"
)

type Config struct {
	RuleFile flagext.StringSlice
	DirA     string
	DirB     string
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.Var(&c.RuleFile, "rules", "Rule file to load, can be specified multiple times")
	f.StringVar(&c.DirA, "dir-a", "", "Directory to read left state from")
	f.StringVar(&c.DirB, "dir-b", "", "Directory to read right state from")
}

func (c *Config) LoadRuleSet() (differ.RuleSet, error) {
	ruleSet := differ.RuleSet{}
	for _, v := range c.RuleFile {
		subRules := &differ.RuleSet{}
		configFile, err := os.Open(v)
		if err != nil {
			return ruleSet, fmt.Errorf("failed to open config file: %v", err)
		}
		defer configFile.Close()
		err = yaml.NewDecoder(configFile).Decode(subRules)
		if err != nil {
			return ruleSet, fmt.Errorf("failed to decode config file: %v", err)
		}

		ruleSet.Merge(subRules)
	}
	ruleSet.Desugar()
	return ruleSet, nil
}

func main() {
	config := &Config{}
	flagext.RegisterFlags(config)
	flag.Parse()

	if config.DirA == "" || config.DirB == "" {
		fmt.Println("dir-a and dir-b must be specified")
		flag.Usage()
		os.Exit(1)
	}

	cli, err := differ.NewDryRunK8sClient()
	if err != nil {
		fmt.Println("Error creating k8s client with default context:", err)
		os.Exit(1)
	}

	ruleSet, err := config.LoadRuleSet()
	if err != nil {
		fmt.Println("Error loading rule set:", err)
		os.Exit(1)
	}

	fmt.Println("Comparing", config.DirA, config.DirB)

	// Extract K8s Config
	objDiffer := differ.NewObjectDiffer()
	err = objDiffer.ReadLeftStateFromDirectory(config.DirA)
	if err != nil {
		fmt.Printf("failed to read left state: %v\n", err)
		os.Exit(1)
	}
	err = objDiffer.ReadRightStateFromDirectory(config.DirB)
	if err != nil {
		fmt.Printf("failed to read right state: %v\n", err)
		os.Exit(1)
	}

	// We apply all ignore rules, then set defaults, then apply patches Defaults
	// must be applied before patches, otherwise we may apply a patch that
	// causes the object to be invalid when sent to the api server for
	// defaulting.
	var mappers []differ.ObjectRule
	for _, ir := range ruleSet.IgnoreRules {
		mappers = append(mappers, ir)
	}
	mappers = append(mappers, differ.NewDefaultSettingRule(cli))
	for _, p := range ruleSet.Patches {
		mappers = append(mappers, p)
	}

	for i, om := range mappers {
		fmt.Printf("Applying rule: %v\n", om.Describe().Name)
		err := objDiffer.MapObjects(om)
		if err != nil {
			objDiffer.DebugInfo.RuleDebugInfos[i].Print()
			fmt.Printf("failed to apply patches: %v\n", err)
			os.Exit(1)
		}

		err = objDiffer.DebugInfo.ValidateAllRulesWereEffective()
		if err != nil {
			fmt.Println("ERROR: some rules are not effective")
			fmt.Println(err)
			os.Exit(1)
		}
	}

	// Compare K8s Config
	result := objDiffer.CalculateDifference()
	if len(result.MatchingObjects) > 0 {
		fmt.Println("Successfully Validated:")
		for _, rk := range result.MatchingObjects {
			fmt.Println("\t", rk)
		}
	}

	if len(result.MissingObjects) > 0 {
		fmt.Printf("Objects missing from %s\n", config.DirB)
		for _, rk := range result.MissingObjects {
			fmt.Println("\t", rk)
		}
	}

	if len(result.ExtraObjects) > 0 {
		fmt.Printf("Objects missing from %s\n", config.DirA)
		for _, rk := range result.ExtraObjects {
			fmt.Println("\t", rk)
		}
	}

	if len(result.DifferentObjects) > 0 {
		fmt.Println("Property-level Differences Detected")
		for _, diff := range result.DifferentObjects {
			err := formatDiffSideBySide(diff)
			if err != nil {
				fmt.Printf("failed to format diff: %v\n", err)
				os.Exit(1)
			}
		}
	}

	if len(result.DifferentObjects) > 0 || len(result.MissingObjects) > 0 || len(result.ExtraObjects) > 0 {
		os.Exit(1)
	}
}

func formatDiffSideBySide(diff differ.ObjectDifference) error {
	leftFile, err := os.CreateTemp("", fmt.Sprintf("%v-left.yaml", diff.Key))
	if err != nil {
		return err
	}
	leftFile.Write(jsonToYaml(diff.RightToLeftPatch.Current))
	leftFile.Close()
	defer os.Remove(leftFile.Name())

	rightFile, err := os.CreateTemp("", fmt.Sprintf("%v-right.yaml", diff.Key))
	if err != nil {
		return err
	}
	rightFile.Write(jsonToYaml(diff.LeftToRightPatch.Current))
	rightFile.Close()
	defer os.Remove(rightFile.Name())

	cmd := exec.Command("difft", leftFile.Name(), rightFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil && err.Error() != "exit status 1" {
		return err
	}

	return nil
}

// jsonToYaml converts a JSON string to a YAML string.
func jsonToYaml(jsonBytes []byte) []byte {
	var obj interface{}
	err := json.Unmarshal([]byte(jsonBytes), &obj)
	if err != nil {
		panic(err)
	}

	yaml, err := yaml.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return (yaml)
}
