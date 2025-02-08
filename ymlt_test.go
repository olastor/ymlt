package ymlt

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	"gopkg.in/yaml.v3"
)

func TestReplaceInNode(t *testing.T) {
	tests := []struct {
		name          string
		templateStr   string
		yamlContent   string
		config        Config
		expectedValue string
		expectErr     bool
	}{
		{
			name:          "Single node replace",
			templateStr:   "{{ t \"$.name\" }}",
			yamlContent:   "name: Bob",
			config:        Config{},
			expectedValue: "Bob",
			expectErr:     false,
		},
		{
			name:        "Multiple nodes replace",
			templateStr: "{{ join (tt \"$.items.*\") }}",
			yamlContent: "items:\n  - item1\n  - item2",
			config: Config{
				FuncMap: template.FuncMap{
					"join": func(arr []string) string {
						return strings.Join(arr, ",")
					},
				},
			},
			expectedValue: "item1,item2",
			expectErr:     false,
		},
		{
			name:          "Invalid path",
			templateStr:   "{{ t \"$.invalid\" }}",
			yamlContent:   "name: Bob",
			config:        Config{},
			expectedValue: "",
			expectErr:     true,
		},
		{
			name:        "Custom FuncMap",
			templateStr: "{{ customFunc }}",
			yamlContent: "name: Bob",
			config: Config{
				FuncMap: template.FuncMap{
					"customFunc": func() string { return "Custom Function" },
				},
			},
			expectedValue: "Custom Function",
			expectErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &yaml.Node{}
			err := yaml.Unmarshal([]byte(tt.yamlContent), root)
			if err != nil {
				t.Fatalf("failed to unmarshal yaml: %v", err)
			}
			got, err := executeTemplate(tt.templateStr, root, &tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("executeTemplate() error = %v, expectErr %v", err, tt.expectErr)
				return
			}
			if got != tt.expectedValue {
				t.Errorf("executeTemplate() = %v, expectedValue %v", got, tt.expectedValue)
			}
		})
	}
}

func TestGetDependentNodes(t *testing.T) {
	tests := []struct {
		name          string
		templateStr   string
		yamlStr       string
		funcMap       template.FuncMap
		expectErr     bool
		expectedNodes []string // string representations of expected nodes
	}{
		{
			name:          "Basic Dependency",
			templateStr:   `{{t "root.key"}}`,
			yamlStr:       `root: { key: value1 }`,
			funcMap:       template.FuncMap{},
			expectErr:     false,
			expectedNodes: []string{"value1"},
		},
		{
			name:        "Invalid ref",
			templateStr: `{{t "root"}}`,
			yamlStr:     `root: { key: value1 }`,
			funcMap:     template.FuncMap{},
			expectErr:   true,
		},
		{
			name:          "Multiple Dependencies",
			templateStr:   `{{t "root.key"}} {{tt "root.items[*]"}}`,
			yamlStr:       `root: { key: value1, items: [item1, item2] }`,
			funcMap:       template.FuncMap{},
			expectErr:     false,
			expectedNodes: []string{"value1", "item1", "item2"},
		},
		{
			name:        "Missing Path",
			templateStr: `{{t "root.missing"}}`,
			yamlStr:     `root: { key: value1 }`,
			funcMap:     template.FuncMap{},
			expectErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := &yaml.Node{}
			err := yaml.Unmarshal([]byte(tt.yamlStr), root)
			if err != nil {
				t.Fatalf("failed to unmarshal yaml: %v", err)
			}

			config := &Config{FuncMap: tt.funcMap}

			nodes, err := getDependentNodes(tt.templateStr, root, config)
			if (err != nil) != tt.expectErr {
				t.Errorf("getDependentNodes() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if err == nil {
				nodeValues := []string{}
				for _, node := range nodes {
					nodeValues = append(nodeValues, node.Value)
				}
				if len(nodeValues) != len(tt.expectedNodes) {
					t.Fatalf("expected %d nodes, got %d", len(tt.expectedNodes), len(nodeValues))
				}

				for i, expected := range tt.expectedNodes {
					if expected != nodeValues[i] {
						t.Errorf("expected node value %v, got %v", expected, nodeValues[i])
					}
				}
			}
		})
	}
}

func TestReplaceAllTemplateVariables(t *testing.T) {
	tests := []struct {
		name      string
		inputYML  string
		expected  map[string]interface{}
		config    *Config
		expectErr bool
	}{
		{
			name: "simple replace",
			inputYML: `
root: hello world
replace_me: '{{t "root"}}'`,
			expected: map[string]interface{}{
				"root":       "hello world",
				"replace_me": "hello world",
			},
			config:    &Config{},
			expectErr: false,
		},
		{
			name: "cycle error",
			inputYML: `
a: '{{t "b"}}'
b: '{{t "a"}}'`,
			config:    &Config{},
			expectErr: true,
		},
		{
			name: "no replacement",
			inputYML: `
non_related: value
no_replace: other_value`,
			expected: map[string]interface{}{
				"non_related": "value",
				"no_replace":  "other_value",
			},
			config:    &Config{},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var root yaml.Node
			err := yaml.Unmarshal([]byte(tt.inputYML), &root)
			if err != nil {
				t.Fatalf("Failed to unmarshal input YAML: %v", err)
			}

			err = Apply(&root, tt.config)
			if (err != nil) != tt.expectErr {
				t.Errorf("Apply() error = %v, expectErr %v", err, tt.expectErr)
			}

			if !tt.expectErr {
				var got map[string]interface{}
				yaml.Unmarshal([]byte(yamlToString(&root)), &got)
				if !mapsEqual(got, tt.expected) {
					t.Errorf("Apply() got = %v, expected %v", got, tt.expected)
				}
			}
		})
	}
}

func yamlToString(node *yaml.Node) string {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	defer enc.Close()
	enc.Encode(node)
	return buf.String()
}

func mapsEqual(a, b map[string]interface{}) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}
