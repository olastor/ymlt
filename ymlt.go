package ymlt

import (
	"bytes"
	"fmt"
	"io"
	"maps"
	"text/template"

	"github.com/vmware-labs/yaml-jsonpath/pkg/yamlpath"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Defaults []byte
	FuncMap  template.FuncMap
}

// Does a depth-first tree traversal of a yaml parse tree and call `handle` for each child node
// of the root node. The initial root node is **not** passed to `handle`!
func traverse(root *yaml.Node, handle func(node *yaml.Node, parent *yaml.Node, index int) error) error {
	for i, n := range root.Content {
		handle(n, root, i)

		err := traverse(n, handle)
		if err != nil {
			return err
		}
	}

	return nil
}

// Execute the template with injected methods "t" (single path) and "tt" (multiple paths),
// which will lookup fields in the yaml document represented by `root`.
func executeTemplate(templateStr string, root *yaml.Node, config *Config) (string, error) {
	if _, exists := config.FuncMap["t"]; exists {
		return "", fmt.Errorf("function 't' is not allowed in FuncMap")
	}
	if _, exists := config.FuncMap["tt"]; exists {
		return "", fmt.Errorf("function 'tt' is not allowed in FuncMap")
	}

	funcMap := template.FuncMap{
		"t": func(path string) (string, error) {
			ypath, err := yamlpath.NewPath(path)
			if err != nil {
				return "", err
			}

			nodes, err := ypath.Find(root)
			if err != nil {
				return "", err
			}

			if len(nodes) == 0 {
				return "", fmt.Errorf("no matches found for path: %s", path)
			}

			return nodes[0].Value, nil
		},
		"tt": func(path string) ([]string, error) {
			ypath, err := yamlpath.NewPath(path)
			if err != nil {
				return nil, err
			}

			nodes, err := ypath.Find(root)
			if err != nil {
				return nil, err
			}

			if len(nodes) == 0 {
				return nil, fmt.Errorf("no matches found for path: %s", path)
			}

			result := []string{}
			for _, n := range nodes {
				result = append(result, n.Value)
			}

			return result, nil
		},
	}

	maps.Copy(funcMap, config.FuncMap)

	var result bytes.Buffer
	tmpl, err := template.New("ymlt").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return "", err
	}

	err = tmpl.Execute(&result, templateStr)
	if err != nil {
		return "", err
	}

	return result.String(), nil
}

// returns a list of pointers to nodes which must be evaluated before
// evaluating the given node.
func getDependentNodes(templateStr string, root *yaml.Node, config *Config) ([]*yaml.Node, error) {
	// list of paths that are being used by the template
	singlePaths := []string{}
	multiPaths := []string{}

	// the idea is to execute the template and collect the paths that would be called.
	// it would probably be cleaner to just traverse the parse tree properly...
	funcMap := template.FuncMap{
		"t": func(path string) string {
			singlePaths = append(singlePaths, path)
			return ""
		},
		"tt": func(path string) []string {
			multiPaths = append(multiPaths, path)
			return []string{}
		},
	}

	// TODO: evaluating the custom functions twice could become a perf problem maybe if for some
	//       reason someone decides to do something that takes very long or uses a lot of compute.
	maps.Copy(funcMap, config.FuncMap)

	tmpl, err := template.New("only checking").Funcs(funcMap).Parse(templateStr)
	if err != nil {
		return nil, err
	}

	if err := tmpl.Execute(io.Discard, templateStr); err != nil {
		return nil, err
	}

	nodes := []*yaml.Node{}
	for _, path := range singlePaths {
		curPath, err := yamlpath.NewPath(path)
		if err != nil {
			return nil, err
		}

		curNodes, err := curPath.Find(root)
		if err != nil {
			return nil, err
		}

		if len(curNodes) == 0 {
			return nil, fmt.Errorf("no matches found for path: %s", path)
		}

		if curNodes[0].Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("this path does not reference a raw value: %s", path)
		}

		nodes = append(nodes, curNodes[0])
	}

	for _, path := range multiPaths {
		curPath, err := yamlpath.NewPath(path)
		if err != nil {
			return nil, err
		}

		curNodes, err := curPath.Find(root)
		if err != nil {
			return nil, err
		}

		if len(curNodes) == 0 {
			return nil, fmt.Errorf("no matches found for path: %s", path)
		}

		for _, node := range curNodes {
			if node.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("this path does not reference a raw value: %s", path)
			}
		}

		nodes = append(nodes, curNodes...)
	}

	return nodes, nil
}

func addDefaults(doc *yaml.Node, defaults *yaml.Node) error {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		doc = doc.Content[0]
	}
	if defaults.Kind == yaml.DocumentNode && len(defaults.Content) == 1 {
		defaults = defaults.Content[0]
	}

	for i := 0; i < len(defaults.Content); i += 2 {
		keyDefault := defaults.Content[i]
		valueDefault := defaults.Content[i+1]

		found := false
		for j := 0; j < len(doc.Content); j += 2 {
			keyDoc := doc.Content[j]

			if keyDoc.Value == keyDefault.Value {
				found = true
				if doc.Content[j+1].Kind == yaml.MappingNode && valueDefault.Kind == yaml.MappingNode {
					if err := addDefaults(doc.Content[j+1], valueDefault); err != nil {
						return err
					}
				}
				break
			}
		}

		if !found {
			doc.Content = append(doc.Content, keyDefault, valueDefault)
		}
	}
	return nil
}

func Apply(root *yaml.Node, config *Config) error {
	if config.Defaults != nil {
		var defaults yaml.Node
		if err := yaml.Unmarshal(config.Defaults, &defaults); err != nil {
			return err
		}

		if err := addDefaults(root, &defaults); err != nil {
			return err
		}
	}

	nodeDeps := map[*yaml.Node][]*yaml.Node{}
	err := traverse(root, func(n *yaml.Node, parent *yaml.Node, index int) error {
		if n.Kind != yaml.ScalarNode || n.Value == "" {
			return nil
		}

		var err error
		nodeDeps[n], err = getDependentNodes(n.Value, root, config)
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	done := map[*yaml.Node]bool{}

	var recurseDeps func(n *yaml.Node, initialNode *yaml.Node) error
	recurseDeps = func(n *yaml.Node, initialNode *yaml.Node) error {
		if n == initialNode {
			return fmt.Errorf("Detected cyclic use of template var: %v", n)
		}

		if n == nil {
			n = initialNode
		}

		if done[n] {
			// already evaluated in different branch
			return nil
		}

		if nodeDeps[n] != nil {
			// when the template functions "t" or "tt" are called with paths to other fields,
			// then the dependent fields need to be evaluated first (recursively) since their
			// values could contain unprocessed templates.
			for _, nn := range nodeDeps[n] {
				err := recurseDeps(nn, initialNode)
				if err != nil {
					return err
				}
			}
		}

		n.Value, err = executeTemplate(n.Value, root, config)
		done[n] = true

		return nil
	}

	for n := range nodeDeps {
		err := recurseDeps(nil, n)
		if err != nil {
			return err
		}
	}

	return nil
}

func Parse(yamlDoc []byte, config *Config) ([]byte, error) {
	var rootNode yaml.Node
	err := yaml.Unmarshal(yamlDoc, &rootNode)
	if err != nil {
		return nil, err
	}

	err = Apply(&rootNode, config)
	if err != nil {
		return nil, err
	}

	return yaml.Marshal(&rootNode)
}
