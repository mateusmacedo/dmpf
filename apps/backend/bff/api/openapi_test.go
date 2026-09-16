package api_test

import (
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/api"
)

type document struct {
	Paths      map[string]map[string]operation `yaml:"paths"`
	Components struct {
		Parameters map[string]parameter `yaml:"parameters"`
	} `yaml:"components"`
}

type operation struct {
	Parameters []parameter          `yaml:"parameters"`
	Responses  map[string]yaml.Node `yaml:"responses"`
}

type parameter struct {
	Ref      string `yaml:"$ref"`
	Name     string `yaml:"name"`
	In       string `yaml:"in"`
	Required bool   `yaml:"required"`
}

func loadDocument(t *testing.T, file string) document {
	t.Helper()
	raw, err := os.ReadFile("../../../../" + file)
	if err != nil {
		t.Fatalf("ReadFile(%s) = %v", file, err)
	}
	var doc document
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s is not YAML: %v", file, err)
	}
	return doc
}

func (d document) resolve(p parameter) parameter {
	if name, ok := strings.CutPrefix(p.Ref, "#/components/parameters/"); ok {
		return d.Components.Parameters[name]
	}
	return p
}

func TestTheContractsDeclareEveryRouteTheEdgeServes(t *testing.T) {
	// WHY: rpc.Classify maps NotFound and Aborted for every method, not per
	// route, so every write declares both 404 and 409; a read never answers 409
	// because optimistic locking only rejects a write.
	statuses := map[string][]string{
		"addItem":         {"201", "400", "404", "409", "422", "429", "503", "504", "default"},
		"placeOrder":      {"200", "400", "404", "409", "422", "429", "503", "504", "default"},
		"findOrder":       {"200", "400", "404", "429", "503", "504", "default"},
		"findReservation": {"200", "400", "404", "429", "503", "504", "default"},
		"reserve":         {"200", "400", "404", "409", "422", "429", "503", "504", "default"},
		"cancel":          {"200", "400", "404", "409", "422", "429", "503", "504", "default"},
	}
	for _, route := range api.Routes(routeBudget) {
		t.Run(route.Name, func(t *testing.T) {
			file, pointer, found := strings.Cut(route.ContractRef, "#/paths/")
			if !found {
				t.Fatalf("ContractRef %q is not a pointer into paths", route.ContractRef)
			}
			escaped, method, _ := strings.Cut(pointer, "/")
			path := strings.ReplaceAll(escaped, "~1", "/")
			if path != route.Path || method != strings.ToLower(route.Method) {
				t.Fatalf("ContractRef names %s %s, want %s %s", method, path, route.Method, route.Path)
			}

			doc := loadDocument(t, file)
			op, declared := doc.Paths[path][method]
			if !declared {
				t.Fatalf("%s does not declare %s %s", file, method, path)
			}

			byName := map[string]parameter{}
			for _, p := range op.Parameters {
				resolved := doc.resolve(p)
				byName[resolved.Name] = resolved
			}
			if key, ok := byName[api.IdempotencyHeader]; route.Method == http.MethodPost && (!ok || key.In != "header" || !key.Required) {
				t.Fatalf("%s: %s must be a required header (RST-02), got %+v", route.Name, api.IdempotencyHeader, key)
			}
			if correlation, ok := byName[api.CorrelationHeader]; !ok || correlation.In != "header" || correlation.Required {
				t.Fatalf("%s: %s must be an optional header, got %+v", route.Name, api.CorrelationHeader, correlation)
			}

			var got []string
			for code := range op.Responses {
				got = append(got, code)
			}
			slices.Sort(got)
			want := slices.Clone(statuses[route.Name])
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("%s responses = %v, want %v", route.Name, got, want)
			}
		})
	}
}
