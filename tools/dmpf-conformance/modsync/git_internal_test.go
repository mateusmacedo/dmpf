package modsync

import "testing"

func TestMaiorReleasePrefereEstavelEUsaPreReleaseSoSemEstavel(t *testing.T) {
	casos := map[string]struct {
		tags []string
		want string
	}{
		"estável vence pré-release maior":     {[]string{"d/v0.2.0", "d/v1.0.0-rc.1", "d/v1.0.0-rc.2"}, "v0.2.0"},
		"só pré-release":                      {[]string{"d/v1.0.0-rc.1"}, "v1.0.0-rc.1"},
		"identificador numérico por valor":    {[]string{"d/v1.0.0-rc.2", "d/v1.0.0-rc.10", "d/v1.0.0-rc.1"}, "v1.0.0-rc.10"},
		"alfanumérico por ordem léxica":       {[]string{"d/v1.0.0-alpha.1", "d/v1.0.0-beta", "d/v1.0.0-alpha"}, "v1.0.0-beta"},
		"mais campos vence com prefixo igual": {[]string{"d/v1.0.0-alpha", "d/v1.0.0-alpha.1"}, "v1.0.0-alpha.1"},
		"núcleo maior vence":                  {[]string{"d/v1.0.0-rc.9", "d/v1.1.0-rc.1"}, "v1.1.0-rc.1"},
		"pré-release malformada ignorada":     {[]string{"d/v1.0.0-", "d/v1.0.0-rc..1", "d/v1.0.0-01", "d/v1.0.0+meta", "d/v1.0.0-rc_1"}, versaoInicial},
		"outro diretório ignorado":            {[]string{"dx/v2.0.0", "d/v0.1.0"}, "v0.1.0"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			if got := maiorRelease(c.tags, "d"); got != c.want {
				t.Errorf("maiorRelease(%v) = %s, esperado %s", c.tags, got, c.want)
			}
		})
	}
}
