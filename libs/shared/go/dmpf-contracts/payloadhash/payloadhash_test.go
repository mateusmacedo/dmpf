package payloadhash_test

import (
	"regexp"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/shared/go/dmpf-contracts/payloadhash"
)

var lowerHex64 = regexp.MustCompile(`^[0-9a-f]{64}$`)

func TestSum(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "empty payload hashes the empty string",
			input: []byte{},
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "nil payload is the same as empty",
			input: nil,
			want:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		},
		{
			name:  "known SHA-256 vector",
			input: []byte("abc"),
			want:  "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
		},
		{
			name:  "protobuf-looking bytes are hashed as transported",
			input: []byte{0x0a, 0x03, 0x6f, 0x2d, 0x31, 0x18, 0x2a},
			want:  "fe4309a178728bb59f6b65e677322981ebc9af9b8d7a20869478ee6d75ce659a",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := payloadhash.Sum(tc.input)
			if !lowerHex64.MatchString(got) {
				t.Fatalf("Sum() = %q, want 64 lowercase hex characters", got)
			}
			if tc.want != "" && got != tc.want {
				t.Fatalf("Sum() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSumIsDeterministicAndPrefixSensitive(t *testing.T) {
	t.Parallel()

	payload := []byte{0x0a, 0x03, 0x6f, 0x2d, 0x31}
	if payloadhash.Sum(payload) != payloadhash.Sum(append([]byte(nil), payload...)) {
		t.Fatal("Sum() must depend only on the bytes")
	}
	if payloadhash.Sum(payload) == payloadhash.Sum(payload[:len(payload)-1]) {
		t.Fatal("Sum() must change when the transported bytes change")
	}
}

func TestFormulaVersion(t *testing.T) {
	t.Parallel()

	if payloadhash.FormulaVersion != 1 {
		t.Fatalf("FormulaVersion = %d, want 1 (ENV-19)", payloadhash.FormulaVersion)
	}
}
