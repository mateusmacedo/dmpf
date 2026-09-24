package bom

import (
	"strings"
	"testing"
	"testing/fstest"
)

func registros() fstest.MapFS {
	return fstest.MapFS{
		"go.work":      {Data: []byte("// workspace\ngo 1.26.6\n\nuse (\n\t./libs/a\n)\n")},
		"tools/buf.sh": {Data: []byte("#!/usr/bin/env bash\n# pin antigo: github.com/bufbuild/buf/cmd/buf@v1.60.0\nexec go run github.com/bufbuild/buf/cmd/buf@v1.72.0 \"$@\"\n")},
		"contracts/buf.gen.yaml": {Data: []byte("version: v2\nplugins:\n  - local:\n      - go\n      - run\n" +
			"      - google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1\n  - local:\n      - go\n      - run\n" +
			"      # - google.golang.org/protobuf/cmd/protoc-gen-go@v1.30.0\n" +
			"      - google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.12\n    out: gen/go\n")},
		"nx.json": {Data: []byte(`{"targetDefaults": {"@nx-go/nx-go:lint": {"options": {"args": ` +
			`["run", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.2", "run"]}}}}`)},
		"libs/a/go.mod": {Data: []byte("module x\n\ngo 1.26.6\n\nreplace (\n\tgithub.com/jackc/pgx/v5 v5.9.0 => ../pgx\n)\n\n" +
			"exclude github.com/jackc/pgx/v5 v5.8.0\n\nrequire (\n\tgithub.com/jackc/pgx/v5 v5.10.0 // indirect\n)\n")},
		"libs/a/package.json": {Data: []byte(`{"name": "a", "version": "0.0.0"}`)},
		"tmp/fake/go.mod":     {Data: []byte("module fake\n\nrequire github.com/jackc/pgx/v5 v5.10.0\n")},
		"package.json":        {Data: []byte(`{"packageManager": "pnpm@11.14.0+sha512.abc", "engines": {"node": "^24"}}`)},
		"pnpm-workspace.yaml": {Data: []byte("packages:\n  - libs/*\ncatalog:\n  \"@biomejs/biome\": 2.4.16 # pin\n  typescript: 5.9.3\n")},
		semconvFile:           {Data: []byte("package otelboot\n\nimport semconv \"go.opentelemetry.io/otel/semconv/v1.43.0\"\n")},
	}
}

func TestCadaSeletorResolve(t *testing.T) {
	casos := []struct {
		ref      RegistryRef
		identity string
		valor    string
	}{
		{RegistryRef{File: "go.work", Selector: "go"}, "go", "1.26.6"},
		{RegistryRef{File: "tools/buf.sh", Selector: "buf"}, "github.com/bufbuild/buf/cmd/buf", "v1.72.0"},
		{RegistryRef{File: "contracts/buf.gen.yaml", Selector: "plugin:protoc-gen-go"}, "google.golang.org/protobuf/cmd/protoc-gen-go", "v1.36.12"},
		{RegistryRef{File: "nx.json", Selector: "golangci-lint"}, "github.com/golangci/golangci-lint/v2/cmd/golangci-lint", "v2.13.2"},
		{RegistryRef{File: "libs/a/go.mod", Selector: "require:github.com/jackc/pgx/v5"}, "github.com/jackc/pgx/v5", "v5.10.0"},
		{RegistryRef{File: "libs/a/package.json", Selector: "version"}, "a", "0.0.0"},
		{RegistryRef{File: "package.json", Selector: "packageManager"}, "pnpm", "pnpm@11.14.0+sha512.abc"},
		{RegistryRef{File: "package.json", Selector: "engines.node"}, "node", "^24"},
		{RegistryRef{File: "pnpm-workspace.yaml", Selector: "catalog:@biomejs/biome"}, "@biomejs/biome", "2.4.16"},
		{RegistryRef{File: semconvFile, Selector: "semconv"}, "go.opentelemetry.io/otel/semconv", "v1.43.0"},
	}
	for _, c := range casos {
		t.Run(c.ref.File+"#"+c.ref.Selector, func(t *testing.T) {
			got, problem, err := resolve(registros(), c.ref, c.identity)
			if err != nil || problem != "" || got != c.valor {
				t.Fatalf("resolve = %q, problema %q, erro %v; esperado %q", got, problem, err, c.valor)
			}
		})
	}
}

func TestSeletorQueNaoResolveViraProblema(t *testing.T) {
	casos := map[string]struct {
		ref      RegistryRef
		identity string
	}{
		"fora do conjunto aceito":       {RegistryRef{File: "go.mod", Selector: "go"}, "go"},
		"plugin inexistente":            {RegistryRef{File: "contracts/buf.gen.yaml", Selector: "plugin:protoc-gen-ts"}, "protoc-gen-ts"},
		"registro ausente":              {RegistryRef{File: "libs/b/go.mod", Selector: "require:github.com/jackc/pgx/v5"}, "github.com/jackc/pgx/v5"},
		"caminho fora da raiz":          {RegistryRef{File: "../go.work", Selector: "go"}, "go"},
		"pacote fora do catálogo":       {RegistryRef{File: "pnpm-workspace.yaml", Selector: "catalog:nx"}, "nx"},
		"go.mod fora do go.work":        {RegistryRef{File: "tmp/fake/go.mod", Selector: "require:github.com/jackc/pgx/v5"}, "github.com/jackc/pgx/v5"},
		"require de outra identity":     {RegistryRef{File: "libs/a/go.mod", Selector: "require:github.com/jackc/pgx/v5"}, "github.com/aws/aws-sdk-go-v2"},
		"catálogo de outra identity":    {RegistryRef{File: "pnpm-workspace.yaml", Selector: "catalog:typescript"}, "pnpm"},
		"gerenciador de outra identity": {RegistryRef{File: "package.json", Selector: "packageManager"}, "npm"},
		"go.work para outra identity":   {RegistryRef{File: "go.work", Selector: "go"}, "node"},
		"plugin de outra identity":      {RegistryRef{File: "contracts/buf.gen.yaml", Selector: "plugin:protoc-gen-go"}, "github.com/bufbuild/buf/cmd/buf"},
		"package.json de outro pacote":  {RegistryRef{File: "libs/a/package.json", Selector: "version"}, "b"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			got, problem, err := resolve(registros(), c.ref, c.identity)
			if err != nil || problem == "" {
				t.Fatalf("resolve = %q, problema %q, erro %v; esperado problema", got, problem, err)
			}
		})
	}
}

func TestComparacaoNormalizaEResolveFaixaCaret(t *testing.T) {
	casos := []struct {
		resolvido, declarado string
		passa                bool
	}{
		{"v1.72.0", "1.72.0", true},
		{"pnpm@11.14.0", "11.14.0", true},
		{"pnpm@11.14.0+sha512.abc", "11.14.0", true},
		{"^24", "24.11.1", true},
		{"^24", "25.0.0", false},
		{"^24", "23.9.0", false},
		{"^0.3", "0.4.0", false},
		{"~24.1", "24.1.0", false},
		{"1.26.6", "1.26.5", false},
	}
	for _, c := range casos {
		detail := compareVersion(c.resolvido, c.declarado)
		if (detail == "") != c.passa {
			t.Errorf("compareVersion(%q, %q) = %q, esperado passar=%v", c.resolvido, c.declarado, detail, c.passa)
		}
	}
	if d := compareVersion("~24.1", "24.1.0"); !strings.Contains(d, "faixa não suportada") {
		t.Errorf("faixa ~ não declarada como não suportada: %q", d)
	}
}
