package main

import (
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/hashicorp/hcl"
	"github.com/stelligent/config-lint/assertion"
	"io/ioutil"
)

// TerraformLinter implements a Linter for Terraform configuration files
type TerraformLinter struct {
	Log assertion.LoggingFunction
}

// TerraformResourceLoader converts Terraform configuration files into JSON objects
type TerraformResourceLoader struct {
	Log assertion.LoggingFunction
}

func parsePolicy(templateResource interface{}) (map[string]interface{}, error) {
	firstResource := templateResource.([]interface{})[0] // FIXME does this array always have 1 element?
	properties := firstResource.(map[string]interface{})
	for _, attribute := range []string{"assume_role_policy", "policy"} {
		if policyAttribute, hasPolicyString := properties[attribute]; hasPolicyString {
			if policyString, isString := policyAttribute.(string); isString {
				var policy interface{}
				err := json.Unmarshal([]byte(policyString), &policy)
				if err != nil {
					return properties, err
				}
				properties[attribute] = policy
			}
		}
	}
	return properties, nil
}

func loadHCL(filename string, log assertion.LoggingFunction) ([]interface{}, error) {
	results := make([]interface{}, 0)
	template, err := ioutil.ReadFile(filename)
	if err != nil {
		return results, nil
	}

	var v interface{}
	err = hcl.Unmarshal([]byte(template), &v)
	if err != nil {
		return results, nil
	}
	jsonData, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return results, nil
	}
	log(string(jsonData))

	var hclData interface{}
	err = yaml.Unmarshal(jsonData, &hclData)
	if err != nil {
		return results, nil
	}
	m := hclData.(map[string]interface{})
	for _, key := range []string{"resource", "data"} {
		if m[key] != nil {
			log(fmt.Sprintf("Adding %s", key))
			results = append(results, m[key].([]interface{})...)
		}
	}
	return results, nil
}

// Load parses an HCL file into a collection or Resource objects
func (l TerraformResourceLoader) Load(filename string) ([]assertion.Resource, error) {
	resources := make([]assertion.Resource, 0)
	hclResources, err := loadHCL(filename, l.Log)
	if err != nil {
		return resources, err
	}
	for _, resource := range hclResources {
		for resourceType, templateResources := range resource.(map[string]interface{}) {
			if templateResources != nil {
				for _, templateResource := range templateResources.([]interface{}) {
					for resourceID, templateResource := range templateResource.(map[string]interface{}) {
						properties, err := parsePolicy(templateResource)
						if err != nil {
							return resources, err
						}
						tr := assertion.Resource{
							ID:         resourceID,
							Type:       resourceType,
							Properties: properties,
							Filename:   filename,
						}
						resources = append(resources, tr)
					}
				}
			}
		}
	}
	return resources, nil
}

// Validate uses a RuleSet to validate resources in a collection of Terraform configuration files
func (l TerraformLinter) Validate(filenames []string, ruleSet assertion.RuleSet, tags []string, ruleIDs []string) ([]string, []assertion.Violation, error) {
	loader := TerraformResourceLoader{Log: l.Log}
	f := FileLinter{Log: l.Log}
	return f.ValidateFiles(filenames, ruleSet, tags, ruleIDs, loader)
}

// Search applies a JMESPath expression to the resources in a collection of Terraform configuration files
func (l TerraformLinter) Search(filenames []string, ruleSet assertion.RuleSet, searchExpression string) {
	loader := TerraformResourceLoader{Log: l.Log}
	f := FileLinter{Log: l.Log}
	f.SearchFiles(filenames, ruleSet, searchExpression, loader)
}
