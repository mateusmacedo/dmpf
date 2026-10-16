package infrasync_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fakeRpk = `#!/bin/sh
printf 'rpk %s\n' "$*" >> "$ARGV_LOG"
case "$*" in
"security user list"*) printf 'NAME\nalpha\nbeta\nconsole\n' ;;
esac
`

const fakeCurl = `#!/bin/sh
printf 'curl %s\n' "$*" >> "$ARGV_LOG"
cat >> "$BODY_LOG"
printf '\n' >> "$BODY_LOG"
`

type scramUser struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Algorithm string `json:"algorithm"`
}

func TestRedpandaInitTakesThePasswordsOutOfTheArguments(t *testing.T) {
	dir := t.TempDir()
	passwords := map[string]string{
		"alpha":   `al"p\ha $(touch ` + dir + `/x) *`,
		"beta":    `be'ta ; touch ` + dir + `/y`,
		"console": `con\"sole`,
	}
	script := literalCommand(t, render(t, workspace(t))["infra/local/compose/provisioning.generated.yml"], "redpanda-init")
	argv, users := runRedpandaInit(t, script, []string{
		"KAFKA_ALPHA_PASSWORD=" + passwords["alpha"],
		"KAFKA_BETA_PASSWORD=" + passwords["beta"],
		"KAFKA_CONSOLE_PASSWORD=" + passwords["console"],
	})
	if len(users) != len(passwords) {
		t.Fatalf("usuários criados = %+v, want %d", users, len(passwords))
	}
	for _, u := range users {
		if u.Password != passwords[u.Username] || u.Algorithm != "SCRAM-SHA-256" {
			t.Errorf("usuário %q criado com %+v, want a senha do env", u.Username, u)
		}
	}
	rejectCallsWith(t, argv, passwords["alpha"], passwords["beta"], passwords["console"])
	if last := argv[len(argv)-1]; last != "rpk cluster config set kafka_enable_authorization true -X admin.hosts=redpanda:9644" {
		t.Errorf("a autorização não liga por último: %s", last)
	}
	for _, name := range []string{"x", "y"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("a senha executou comando: %s criado", name)
		}
	}
}

func literalCommand(t *testing.T, content, service string) string {
	t.Helper()
	_, after, ok := strings.Cut(content, "\n  "+service+":\n")
	if !ok {
		t.Fatalf("sem o serviço %s em:\n%s", service, content)
	}
	_, block, ok := strings.Cut(after, "\n      - |\n")
	if !ok {
		t.Fatalf("sem bloco | no serviço %s", service)
	}
	var lines []string
	for _, line := range strings.Split(block, "\n") {
		body, inside := strings.CutPrefix(line, "        ")
		if !inside && line != "" {
			break
		}
		lines = append(lines, body)
	}
	return strings.ReplaceAll(strings.Join(lines, "\n"), "$$", "$")
}

func runRedpandaInit(t *testing.T, script string, env []string) ([]string, []scramUser) {
	t.Helper()
	bin := t.TempDir()
	for name, body := range map[string]string{"rpk": fakeRpk, "curl": fakeCurl} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	argvLog, bodyLog := filepath.Join(bin, "argv.log"), filepath.Join(bin, "body.log")
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append([]string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "ARGV_LOG=" + argvLog, "BODY_LOG=" + bodyLog}, env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("redpanda-init falhou: %v\n%s\n--- script\n%s", err, out, script)
	}
	var users []scramUser
	for _, line := range readLines(t, bodyLog) {
		var u scramUser
		if err := json.Unmarshal([]byte(line), &u); err != nil {
			t.Fatalf("corpo do POST não é JSON: %v\n%s", err, line)
		}
		users = append(users, u)
	}
	return readLines(t, argvLog), users
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}
