package auth

import (
	"os"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

// TestPublicRoutesMatchOpenAPI ties the middleware allowlist to the contract
// (F003-R12): an operation is public in Go if and only if it declares
// `security: []` in api/openapi.yaml, and everything else inherits a global
// security requirement.
func TestPublicRoutesMatchOpenAPI(t *testing.T) {
	raw, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Security []map[string][]string           `yaml:"security"`
		Paths    map[string]map[string]yaml.Node `yaml:"paths"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Security) == 0 {
		t.Fatal("openapi.yaml has no global security requirement: new operations would be public by default")
	}

	methods := []string{"get", "put", "post", "delete", "options", "head", "patch", "trace"}
	var public []string
	for path, item := range doc.Paths {
		for method, node := range item {
			if !slices.Contains(methods, method) {
				continue
			}
			var op struct {
				Security *[]map[string][]string `yaml:"security"`
			}
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s: %v", method, path, err)
			}
			if op.Security != nil && len(*op.Security) == 0 {
				public = append(public, strings.ToUpper(method)+" "+path)
			}
		}
	}

	got := PublicRoutes()
	slices.Sort(public)
	slices.Sort(got)
	if !slices.Equal(public, got) {
		t.Errorf("public operations in openapi.yaml = %v\nauth.publicRoutes          = %v", public, got)
	}
}
