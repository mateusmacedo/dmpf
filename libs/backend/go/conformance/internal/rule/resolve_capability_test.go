package rule

import "testing"

func TestResolveCapabilityStdlibReflect(t *testing.T) {
	policy := ExternalPolicy{}
	isStd := func(path string) bool { return path == "reflect" }
	cap, ok := ResolveCapability("reflect", policy, isStd)
	if !ok {
		t.Fatal("reflect should resolve via stdlib")
	}
	if cap != CapRuntimeFramework {
		t.Fatalf("got %q, want %q", cap, CapRuntimeFramework)
	}
}

func TestResolveCapabilityAllowlistPrecedence(t *testing.T) {
	policy := ExternalPolicy{
		Allowlist: []AllowlistEntry{
			{Package: "example.com/lib", Entrypoints: []string{"example.com/lib"}, Capability: CapWireCodec, Versions: ">=1"},
		},
	}
	isStd := func(string) bool { return false }
	cap, ok := ResolveCapability("example.com/lib", policy, isStd)
	if !ok {
		t.Fatal("allowlist entry should resolve")
	}
	if cap != CapWireCodec {
		t.Fatalf("got %q, want %q", cap, CapWireCodec)
	}
}

func TestResolveCapabilityAllowlistBeforeStdlib(t *testing.T) {
	policy := ExternalPolicy{
		Allowlist: []AllowlistEntry{
			{Package: "reflect", Entrypoints: []string{"reflect"}, Capability: CapWireCodec, Versions: ">=1"},
		},
	}
	isStd := func(path string) bool { return path == "reflect" }
	cap, ok := ResolveCapability("reflect", policy, isStd)
	if !ok {
		t.Fatal("should resolve via allowlist")
	}
	if cap != CapWireCodec {
		t.Fatalf("allowlist should win: got %q, want %q", cap, CapWireCodec)
	}
}
