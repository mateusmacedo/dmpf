package boot

import "testing"

func TestAMessageOfGRPCKeepsItsStatementAndNotItsValues(t *testing.T) {
	cases := map[string]string{
		"[core] [Channel #1] Channel Connectivity change to READY\n":                                    "[core] [Channel #1] Channel Connectivity change to READY",
		"[core] [Server #3] grpc: server failed to encode response: proto: dial tcp 10.0.0.7:4317\n":    "[core] [Server #3] grpc: server failed to encode response <redacted>",
		"[core] [Server #1] grpc: Server.Serve failed to complete security handshake from \"10.0.0.7\"": "[core] [Server #1] grpc: Server.Serve failed to complete security handshake from <redacted>",
		"[transport] transport: loopyWriter exiting with error: connection reset by peer\n":             "[transport] transport: loopyWriter exiting with error <redacted>",
		"grpc: failed dns A record lookup due to lookup orders on 10.96.0.10:53\n":                      "grpc: failed dns A record lookup due to lookup orders on <redacted>",
		"[core] [fe80::1]:443 refused": "[core] <redacted>",
		"10.0.0.7:4317 refused":        "<redacted>",
	}
	for message, want := range cases {
		if got := withoutValues(message); got != want {
			t.Errorf("withoutValues(%q) = %q, want %q", message, got, want)
		}
	}
}
