package infrasync_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const fakeRpk = `#!/bin/sh
printf 'rpk %s\n' "$*" >> "$ARGV_LOG"
case "$*" in
"security user list"*) printf 'USERNAME\n'; for u in ${FAKE_USERS-alpha beta console}; do printf '%s\n' "$u"; done ;;
esac
`

const fakeCurl = `#!/bin/sh
printf 'curl %s\n' "$*" >> "$ARGV_LOG"
while [ $# -gt 1 ]; do
  [ "$1" = -X ] && method=$2
  shift
done
body=$(cat)
printf '%s %s %s\n' "$method" "$1" "$body" >> "$BODY_LOG"
user=$(printf '%s' "$body" | sed -n 's/^{"username":"\([^"]*\)".*/\1/p')
status=200
for rule in $FAKE_STATUS; do
  case $rule in "$method:$user:"*) status=${rule##*:} ;; esac
done
printf '%s' "$status"
`

const usersAPI = "http://redpanda:9644/v1/security/users"

type scramUser struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Algorithm string `json:"algorithm"`
}

type scramRequest struct {
	method, url string
	scramUser
}

type initRun struct {
	argv     []string
	requests []scramRequest
	out      string
	failed   bool
}

func TestRedpandaInitTakesThePasswordsOutOfTheArguments(t *testing.T) {
	dir := t.TempDir()
	passwords := map[string]string{
		"alpha":   `al"p\ha $(touch ` + dir + `/x) *`,
		"beta":    `be'ta ; touch ` + dir + `/y`,
		"console": `con\"sole`,
	}
	run := runRedpandaInit(t, "KAFKA_ALPHA_PASSWORD="+passwords["alpha"], "KAFKA_BETA_PASSWORD="+passwords["beta"], "KAFKA_CONSOLE_PASSWORD="+passwords["console"])
	requireSucceeded(t, run)
	if len(run.requests) != len(passwords) {
		t.Fatalf("requisições = %+v, want %d", run.requests, len(passwords))
	}
	for _, r := range run.requests {
		if want := (scramRequest{"POST", usersAPI, scramUser{r.Username, passwords[r.Username], "SCRAM-SHA-256"}}); r != want {
			t.Errorf("requisição %+v, want %+v", r, want)
		}
	}
	rejectCallsWith(t, run.argv, passwords["alpha"], passwords["beta"], passwords["console"])
	if last := run.argv[len(run.argv)-1]; last != "rpk cluster config set kafka_enable_authorization true -X admin.hosts=redpanda:9644" {
		t.Errorf("a autorização não liga por último: %s", last)
	}
	for _, name := range []string{"x", "y"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("a senha executou comando: %s criado", name)
		}
	}
}

func TestRedpandaInitReconcilesThePasswordOfAnExistingUser(t *testing.T) {
	run := runRedpandaInit(t, "KAFKA_ALPHA_PASSWORD=nova senha", "FAKE_STATUS=POST:alpha:500")
	requireSucceeded(t, run)
	want := scramRequest{"PUT", usersAPI + "/alpha", scramUser{"alpha", "nova senha", "SCRAM-SHA-256"}}
	if !slices.Contains(run.requests, want) {
		t.Errorf("sem o PUT da senha nova:\n%+v", run.requests)
	}
	rejectCallsWith(t, run.argv, "nova senha")
}

func TestRedpandaInitStopsOnAnUnexpectedStatus(t *testing.T) {
	for _, tc := range []struct {
		name, user, status string
		env                []string
	}{
		{"POST recusado", "beta", "400", []string{"FAKE_STATUS=POST:beta:400"}},
		{"500 sem o usuário", "alpha", "500", []string{"FAKE_STATUS=POST:alpha:500", "FAKE_USERS=beta console"}},
		{"PUT recusado", "alpha", "503", []string{"FAKE_STATUS=POST:alpha:500 PUT:alpha:503"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runRedpandaInit(t, append(tc.env, "KAFKA_ALPHA_PASSWORD=alpha-secreta", "KAFKA_BETA_PASSWORD=beta-secreta")...)
			requireStopped(t, run, "recusou o usuário "+tc.user+" com status "+tc.status, "alpha-secreta", "beta-secreta")
		})
	}
}

func TestRedpandaInitRefusesAPasswordWithAControlCharacter(t *testing.T) {
	for _, tc := range []struct{ name, env, user, pass string }{
		{"tab", "KAFKA_ALPHA_PASSWORD", "alpha", "al\tpha-secreta"},
		{"quebra de linha", "KAFKA_CONSOLE_PASSWORD", "console", "con\nsole-secreta"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := runRedpandaInit(t, tc.env+"="+tc.pass)
			requireStopped(t, run, "a senha do usuário "+tc.user+" tem caractere de controle", tc.pass, "secreta")
			for _, r := range run.requests {
				if r.Username == tc.user {
					t.Errorf("a senha com caractere de controle chegou à Admin API: %s %s", r.method, r.url)
				}
			}
		})
	}
}

func requireSucceeded(t *testing.T, run initRun) {
	t.Helper()
	if run.failed {
		t.Fatalf("redpanda-init falhou:\n%s", run.out)
	}
}

func requireStopped(t *testing.T, run initRun, message string, secrets ...string) {
	t.Helper()
	if !run.failed {
		t.Fatalf("o redpanda-init não parou:\n%s", run.out)
	}
	if !strings.Contains(run.out, message) {
		t.Errorf("saída sem %q:\n%s", message, run.out)
	}
	rejectCallsWith(t, append([]string{run.out}, run.argv...), secrets...)
	rejectCallsWith(t, run.argv, "kafka_enable_authorization")
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

func runRedpandaInit(t *testing.T, env ...string) initRun {
	t.Helper()
	script := literalCommand(t, render(t, workspace(t))["infra/local/compose/provisioning.generated.yml"], "redpanda-init")
	bin := t.TempDir()
	for name, body := range map[string]string{"rpk": fakeRpk, "curl": fakeCurl} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	argvLog, bodyLog := filepath.Join(bin, "argv.log"), filepath.Join(bin, "body.log")
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append([]string{
		"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "ARGV_LOG=" + argvLog, "BODY_LOG=" + bodyLog,
		"KAFKA_ALPHA_PASSWORD=alpha-pass", "KAFKA_BETA_PASSWORD=beta-pass", "KAFKA_CONSOLE_PASSWORD=console-pass",
	}, env...)
	out, err := cmd.CombinedOutput()
	run := initRun{argv: readLines(t, argvLog), out: string(out), failed: err != nil}
	for _, line := range readLines(t, bodyLog) {
		parts := strings.SplitN(line, " ", 3)
		r := scramRequest{method: parts[0], url: parts[1]}
		if err := json.Unmarshal([]byte(parts[2]), &r.scramUser); err != nil {
			t.Fatalf("corpo do %s não é JSON: %v\n%s", parts[0], err, line)
		}
		run.requests = append(run.requests, r)
	}
	return run
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}
