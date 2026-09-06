package channel_test

import (
	"errors"
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

func TestValidateAddress(t *testing.T) {
	cases := map[string]struct {
		address string
		prefix  string
		want    error
	}{
		"canonical form":                   {"credito.proposta.aprovada.v1", "", nil},
		"hyphen and underscore":            {"credito.proposta_aprovada-v2.v1", "", nil},
		"empty":                            {"", "", channel.ErrInvalidAddress},
		"uppercase":                        {"Credito.Proposta.Aprovada.v1", "", channel.ErrInvalidAddress},
		"slash":                            {"credito/proposta", "", channel.ErrInvalidAddress},
		"space":                            {"credito proposta", "", channel.ErrInvalidAddress},
		"at the length limit":              {strings.Repeat("a", channel.MaxAddressLength), "", nil},
		"over the length limit":            {strings.Repeat("a", channel.MaxAddressLength+1), "", channel.ErrInvalidAddress},
		"prod segment without prefix":      {"prod.credito.proposta.aprovada.v1", "", channel.ErrEnvironmentInAddress},
		"hml segment without prefix":       {"credito.hml.proposta.v1", "", channel.ErrEnvironmentInAddress},
		"staging as a word, not a segment": {"credito.stagingarea.v1", "", nil},
		"declared prefix present":          {"hml.credito.proposta.aprovada.v1", "hml", nil},
		"declared prefix absent":           {"credito.proposta.aprovada.v1", "hml", channel.ErrEnvironmentInAddress},
		"declared prefix outside alphabet": {"hml.credito.proposta.v1", "HML", channel.ErrInvalidAddress},
		"prefix counts toward the limit":   {"hml." + strings.Repeat("a", channel.MaxAddressLength-3), "hml", channel.ErrInvalidAddress},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := channel.ValidateAddress(tc.address, tc.prefix)
			if tc.want == nil && err != nil {
				t.Fatalf("ValidateAddress() = %v, want nil", err)
			}
			if tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("ValidateAddress() = %v, want %v", err, tc.want)
			}
		})
	}
}
