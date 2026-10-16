//go:build integration

package infrasync_test

import (
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

const testDSN = "PG_DSN"

func TestProvisioningConvergesAgainstPostgres(t *testing.T) {
	admin := adminEnv(t)
	if got := psqlAs(t, admin, "postgres", "SHOW cluster_name"); got != "test" {
		t.Fatalf("cluster_name = %q: o provisionamento cria e apaga roles, só roda no servidor de testes", got)
	}
	for _, target := range []struct {
		name, path string
		compose    bool
	}{
		{"compose", "infra/local/compose/provisioning.generated.yml", true},
		{"job", "infra/k8s/overlays/dev/databases/job.yaml", false},
	} {
		t.Run(target.name, func(t *testing.T) {
			app := "prov" + randomHex(t)
			migrator := app + "_migrator"
			t.Cleanup(func() {
				for _, sql := range []string{
					`DROP DATABASE IF EXISTS "` + app + `" WITH (FORCE)`,
					`DROP ROLE IF EXISTS "` + app + `"`,
					`DROP ROLE IF EXISTS "` + migrator + `"`,
				} {
					psqlAs(t, admin, "postgres", sql)
				}
			})
			script := foldedCommand(t, render(t, migratorWorkspace(t, app))[target.path])
			if target.compose {
				script = strings.ReplaceAll(script, "$$", "$")
			}
			for run, pass := range []struct{ runtime, migrator string }{
				{`run'ti;me`, `mig'ra;"tor`},
				{`run'2;x`, `mig'2;"y`},
				{`run 3 $(id) *`, `mig 3 ; id | *`},
			} {
				provision(t, script, admin, "PROV_PG_PASSWORD="+pass.runtime, "PROV_PG_MIGRATOR_PASSWORD="+pass.migrator)
				for _, check := range []struct{ sql, want string }{
					{`SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = '` + app + `'`, migrator},
					{`SELECT has_schema_privilege('` + app + `', 'public', 'CREATE')`, "f"},
					{`SELECT has_database_privilege('` + app + `', '` + app + `', 'TEMPORARY')`, "f"},
					{`SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls FROM pg_roles WHERE rolname = '` + app + `'`, "f"},
					{`SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR NOT rolcanlogin FROM pg_roles WHERE rolname = '` + migrator + `'`, "f"},
					{`SELECT has_database_privilege('public', '` + app + `', 'CONNECT')`, "f"},
					{`SELECT has_database_privilege('` + migrator + `', '` + app + `', 'CONNECT')`, "t"},
				} {
					if got := psqlAs(t, admin, app, check.sql); got != check.want {
						t.Errorf("execução %d: %s = %q, want %q", run+1, check.sql, got, check.want)
					}
				}
				if got := psqlAs(t, as(admin, app, pass.runtime), app, "SELECT current_user"); got != app {
					t.Errorf("execução %d: a role de runtime não entra com a senha reconciliada: %q", run+1, got)
				}
				psqlAs(t, as(admin, migrator, pass.migrator), app, "CREATE TABLE run"+strconv.Itoa(run+1)+" (id int)")
			}
		})
	}
}

func adminEnv(t *testing.T) []string {
	t.Helper()
	raw := os.Getenv(testDSN)
	if raw == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s is empty in CI: integration tests must not skip silently", testDSN)
		}
		t.Skipf("%s not set", testDSN)
	}
	if _, err := exec.LookPath("psql"); err != nil {
		t.Fatalf("psql: %v", err)
	}
	dsn, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("%s: %v", testDSN, err)
	}
	pass, _ := dsn.User.Password()
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"PGHOST=" + dsn.Hostname(),
		"PGPORT=" + dsn.Port(),
		"PGUSER=" + dsn.User.Username(),
		"PGPASSWORD=" + pass,
		"PGDATABASE=" + strings.TrimPrefix(dsn.Path, "/"),
	}
	if mode := dsn.Query().Get("sslmode"); mode != "" {
		env = append(env, "PGSSLMODE="+mode)
	}
	return env
}

func as(env []string, user, pass string) []string {
	others := slices.DeleteFunc(slices.Clone(env), func(kv string) bool {
		return strings.HasPrefix(kv, "PGUSER=") || strings.HasPrefix(kv, "PGPASSWORD=")
	})
	return append(others, "PGUSER="+user, "PGPASSWORD="+pass)
}

func psqlAs(t *testing.T, env []string, db, sql string) string {
	t.Helper()
	cmd := exec.Command("psql", "-XAtq", "-v", "ON_ERROR_STOP=1", "-d", db, "-c", sql)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql -d %s -c %q: %v\n%s", db, sql, err, out)
	}
	return strings.TrimSpace(string(out))
}

func provision(t *testing.T, script string, env []string, passwords ...string) {
	t.Helper()
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append(slices.Clone(env), passwords...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("provisionamento falhou: %v\n%s", err, out)
	}
}

func migratorWorkspace(t *testing.T, app string) string {
	t.Helper()
	root := t.TempDir()
	platform, err := os.ReadFile("testdata/workspace/infra/local/env.platform.example")
	if err != nil {
		t.Fatal(err)
	}
	manifest := `{ "schema": "dmpf/infra@1", "app": "` + app + `",
  "image": { "env": "PROV_IMAGE", "local": "prov:local" },
  "database": { "passwordEnv": "PROV_PG_PASSWORD", "local": "prov-local", "dev": "prov-dev",
    "migrator": { "passwordEnv": "PROV_PG_MIGRATOR_PASSWORD", "local": "prov-migrator-local", "dev": "prov-migrator-dev" } },
  "grpc": {} }`
	for path, content := range map[string]string{
		"infra/local/env.platform.example":           string(platform),
		"apps/backend/" + app + "/deploy/infra.json": manifest,
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := infrasync.Load(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}
