package infrasync_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/tools/dmpf-conformance/infrasync"
)

const (
	alphaDatabase = `"dev": "alpha-dev" }`
	alphaMigrator = `"dev": "alpha-dev", "migrator": { "passwordEnv": "ALPHA_PG_MIGRATOR_PASSWORD", "local": "alpha-migrator-local", "dev": "alpha-migrator-dev" } }`
	alphaOverlay  = "apps/backend/alpha/deploy/k8s/overlays/dev/kustomization.yaml"
	migratorDSN   = "PG_MIGRATE_DSN=postgres://alpha_migrator:alpha-migrator-dev@postgres-0.postgres:5432/alpha?sslmode=disable"
	runtimeAttrs  = "NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS"
	migratorAttrs = "LOGIN " + runtimeAttrs
)

func alterRole(role, env, pass, attrs string) string {
	return "-v ON_ERROR_STOP=1 -v role=" + role + ` <<< \getenv pass ` + env +
		` SET log_min_error_statement = panic; SET log_statement = none; ALTER ROLE :"role" PASSWORD :'pass' ` + attrs + "; [pass=" + pass + "]"
}

func withMigrator(t *testing.T) string {
	t.Helper()
	root := workspace(t)
	rewrite(t, root, "apps/backend/alpha/deploy/infra.json", alphaDatabase, alphaMigrator)
	return root
}

func appendMigratorSecret(t *testing.T, root, literal string) {
	t.Helper()
	rewrite(t, root, alphaOverlay, "sslmode=disable\n", "sslmode=disable\n  - name: alpha-migrator\n    literals:\n      - "+literal+"\n")
}

func TestLoadRefusesAnIncompleteMigrator(t *testing.T) {
	for _, field := range []string{
		`"passwordEnv": "ALPHA_PG_MIGRATOR_PASSWORD"`,
		`"local": "alpha-migrator-local"`,
		`"dev": "alpha-migrator-dev"`,
	} {
		t.Run(field, func(t *testing.T) {
			root := withMigrator(t)
			key, _, _ := strings.Cut(field, ":")
			rewrite(t, root, "apps/backend/alpha/deploy/infra.json", field, key+`: ""`)
			if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "database.migrator exige passwordEnv, local e dev") {
				t.Fatalf("Load = %v, want the incomplete migrator refused", err)
			}
		})
	}
}

func TestLoadRefusesAMigratorValueThatWouldBreakOutOfTheGeneratedFiles(t *testing.T) {
	for _, tc := range []struct{ old, new, field string }{
		{`"passwordEnv": "ALPHA_PG_MIGRATOR_PASSWORD"`, `"passwordEnv": "X; id"`, "database.migrator.passwordEnv"},
		{`"local": "alpha-migrator-local"`, `"local": "alpha'local"`, "database.migrator.local"},
		{`"dev": "alpha-migrator-dev"`, `"dev": "alpha dev"`, "database.migrator.dev"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			root := withMigrator(t)
			rewrite(t, root, "apps/backend/alpha/deploy/infra.json", tc.old, tc.new)
			if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), tc.field+" ") || !strings.Contains(err.Error(), "fora do formato") {
				t.Fatalf("Load = %v, want the value refused naming %q", err, tc.field)
			}
		})
	}
}

func TestLoadRefusesAMigratorVariableDeclaredTwice(t *testing.T) {
	for _, taken := range []string{"ALPHA_PG_PASSWORD", "BETA_PG_PASSWORD"} {
		t.Run(taken, func(t *testing.T) {
			root := withMigrator(t)
			rewrite(t, root, "apps/backend/alpha/deploy/infra.json", `"ALPHA_PG_MIGRATOR_PASSWORD"`, `"`+taken+`"`)
			if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "variável "+taken+" declarada mais de uma vez") {
				t.Fatalf("Load = %v, want the variable declared twice named", err)
			}
		})
	}
}

func TestRenderDeclaresTheMigratorPasswordAndSecret(t *testing.T) {
	files := render(t, withMigrator(t))
	env := files["infra/local/.env.example"]
	if !strings.Contains(env, "ALPHA_PG_PASSWORD=alpha-local\nALPHA_PG_MIGRATOR_PASSWORD=alpha-migrator-local\nBETA_PG_PASSWORD=beta-local\n") {
		t.Errorf(".env.example sem a senha do migrador depois da senha da app:\n%s", env)
	}
	dbs := files["infra/k8s/overlays/dev/databases/kustomization.yaml"]
	if !strings.Contains(dbs, "      - alpha=alpha-dev\n      - alpha_migrator=alpha-migrator-dev\n      - beta=beta-dev\n") {
		t.Errorf("Secret databases sem a chave do migrador:\n%s", dbs)
	}
	job := files["infra/k8s/overlays/dev/databases/job.yaml"]
	if !strings.Contains(job, "            - name: ALPHA_PG_MIGRATOR_PASSWORD\n              valueFrom:\n                secretKeyRef:\n                  name: databases\n                  key: alpha_migrator\n") {
		t.Errorf("Job sem a senha do migrador lida do Secret databases:\n%s", job)
	}
	if strings.Contains(job, "beta_migrator") || strings.Contains(dbs, "beta_migrator") {
		t.Error("app sem migrador ganhou chave de migrador")
	}
}

func TestProvisioningMakesTheMigratorOwnerAndTheRuntimeRoleUnprivileged(t *testing.T) {
	files := render(t, withMigrator(t))
	for _, tc := range []struct {
		name, script string
		env          []string
	}{
		{"compose", strings.ReplaceAll(foldedCommand(t, files["infra/local/compose/provisioning.generated.yml"]), "$$", "$"), nil},
		{"job", foldedCommand(t, files["infra/k8s/overlays/dev/databases/job.yaml"]), []string{"BETA_PG_PASSWORD=beta-local"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := append([]string{"ALPHA_PG_PASSWORD=run:time", "ALPHA_PG_MIGRATOR_PASSWORD=mig:rator"}, tc.env...)
			calls := runProvisioning(t, tc.script, false, env)
			requireCallsInOrder(t, calls, []string{
				`-c CREATE ROLE "alpha_migrator" LOGIN`,
				alterRole("alpha_migrator", "ALPHA_PG_MIGRATOR_PASSWORD", "mig:rator", migratorAttrs),
				`-c CREATE DATABASE "alpha" OWNER "alpha_migrator"`,
				`-d alpha -c GRANT CREATE ON SCHEMA public TO "alpha_migrator"`,
				`-c CREATE ROLE "alpha" LOGIN`,
				alterRole("alpha", "ALPHA_PG_PASSWORD", "run:time", runtimeAttrs),
				`-c GRANT CONNECT ON DATABASE "alpha" TO "alpha"`,
				`-c REVOKE CONNECT ON DATABASE "alpha" FROM PUBLIC`,
				`-c REVOKE TEMPORARY ON DATABASE "alpha" FROM PUBLIC`,
				`-d alpha -c REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
				`-d alpha -c REVOKE CREATE ON SCHEMA public FROM "alpha"`,
				`-d alpha -c GRANT USAGE ON SCHEMA public TO "alpha"`,
			})
			expectCalls(t, calls, []string{
				`-c CREATE ROLE "beta" LOGIN`,
				`-c ALTER ROLE "beta" PASSWORD 'beta-local'`,
				`-c CREATE DATABASE "beta" OWNER "beta"`,
			})
			rejectCallsWith(t, calls, `OWNER "alpha"`, "beta_migrator", `TO "beta"`, "PASSWORD 'mig:rator'", "PASSWORD 'run:time'")
			rejectPasswordsInArgv(t, calls, "mig:rator", "run:time")
		})
	}
}

func requireCallsInOrder(t *testing.T, calls, want []string) {
	t.Helper()
	last := -1
	for _, w := range want {
		i := slices.Index(calls, w)
		if i < 0 {
			t.Fatalf("provisionamento sem %q:\n%s", w, strings.Join(calls, "\n"))
		}
		if i < last {
			t.Errorf("%q fora de ordem:\n%s", w, strings.Join(calls, "\n"))
		}
		last = i
	}
}

func expectCalls(t *testing.T, calls, want []string) {
	t.Helper()
	for _, w := range want {
		if !slices.Contains(calls, w) {
			t.Errorf("provisionamento sem %q:\n%s", w, strings.Join(calls, "\n"))
		}
	}
}

func rejectCallsWith(t *testing.T, calls []string, unwanted ...string) {
	t.Helper()
	for _, u := range unwanted {
		for _, c := range calls {
			if strings.Contains(c, u) {
				t.Errorf("chamada inesperada com %q: %s", u, c)
			}
		}
	}
}

func rejectPasswordsInArgv(t *testing.T, calls []string, passwords ...string) {
	t.Helper()
	for _, c := range calls {
		argv, _, _ := strings.Cut(c, " <<< ")
		if slices.ContainsFunc(passwords, func(p string) bool { return strings.Contains(argv, p) }) {
			t.Errorf("senha nos argumentos do psql: %s", c)
		}
	}
}

func TestProvisioningReconcilesWithoutRecreating(t *testing.T) {
	files := render(t, withMigrator(t))
	script := foldedCommand(t, files["infra/k8s/overlays/dev/databases/job.yaml"])
	calls := runProvisioning(t, script, true, []string{"ALPHA_PG_PASSWORD=a", "ALPHA_PG_MIGRATOR_PASSWORD=m", "BETA_PG_PASSWORD=b"})
	for _, c := range calls {
		if strings.Contains(c, "CREATE ROLE") || strings.Contains(c, "CREATE DATABASE") || strings.Contains(c, "OWNER TO") {
			t.Errorf("recriou o que já existia: %s", c)
		}
	}
	for _, w := range []string{
		alterRole("alpha_migrator", "ALPHA_PG_MIGRATOR_PASSWORD", "m", migratorAttrs),
		alterRole("alpha", "ALPHA_PG_PASSWORD", "a", runtimeAttrs),
		`-c ALTER ROLE "beta" PASSWORD 'b'`,
		`-c REVOKE TEMPORARY ON DATABASE "alpha" FROM PUBLIC`,
		`-d alpha -c REVOKE CREATE ON SCHEMA public FROM "alpha"`,
	} {
		if !slices.Contains(calls, w) {
			t.Errorf("reexecução sem reconciliar %q:\n%s", w, strings.Join(calls, "\n"))
		}
	}
}

func TestProvisioningMovesADatabaseOwnedByAnotherRole(t *testing.T) {
	files := render(t, withMigrator(t))
	script := foldedCommand(t, files["infra/k8s/overlays/dev/databases/job.yaml"])
	calls := runProvisioning(t, script, true, []string{"ALPHA_PG_PASSWORD=a", "ALPHA_PG_MIGRATOR_PASSWORD=m", "BETA_PG_PASSWORD=b"}, "PSQL_OTHER_OWNER=1")
	if !slices.Contains(calls, `-c ALTER DATABASE "alpha" OWNER TO "alpha_migrator"`) {
		t.Fatalf("banco de outro dono não passou à migradora:\n%s", strings.Join(calls, "\n"))
	}
}

func TestProvisioningLoopsOnlyOverTheAppsWithoutAMigrator(t *testing.T) {
	files := render(t, withMigrator(t))
	for path, content := range map[string]string{
		"compose": files["infra/local/compose/provisioning.generated.yml"],
		"job":     files["infra/k8s/overlays/dev/databases/job.yaml"],
	} {
		script := foldedCommand(t, content)
		var keys []string
		for _, loop := range strings.Split(script, "for pair in")[1:] {
			list, _, _ := strings.Cut(loop, "; do")
			for _, item := range strings.Fields(list) {
				key, _, _ := strings.Cut(item, ":")
				keys = append(keys, key)
			}
		}
		if got := strings.Join(keys, ","); got != "beta" {
			t.Errorf("%s: chaves dos laços = %s, want beta", path, got)
		}
	}
}

func TestProvisioningKeepsTheMigratedPasswordsOutOfTheScript(t *testing.T) {
	files := render(t, withMigrator(t))
	compose := files["infra/local/compose/provisioning.generated.yml"]
	for _, line := range []string{
		"      ALPHA_PG_PASSWORD: ${ALPHA_PG_PASSWORD:-alpha-local}\n",
		"      ALPHA_PG_MIGRATOR_PASSWORD: ${ALPHA_PG_MIGRATOR_PASSWORD:-alpha-migrator-local}\n",
	} {
		if !strings.Contains(compose, line) {
			t.Errorf("environment do postgres-init sem %q:\n%s", line, compose)
		}
	}
	for path, content := range map[string]string{"compose": compose, "job": files["infra/k8s/overlays/dev/databases/job.yaml"]} {
		if script := foldedCommand(t, content); strings.Contains(script, "$ALPHA_PG") || strings.Contains(script, "{ALPHA_PG") {
			t.Errorf("%s: senha de app com migrador expandida no script:\n%s", path, script)
		}
	}
}

func TestProvisioningTakesAHostilePasswordAsData(t *testing.T) {
	files := render(t, withMigrator(t))
	dir := t.TempDir()
	runtime, migrator := "abc def * $(touch "+dir+"/x)", "ghi jkl ; touch "+dir+"/y"
	for path, content := range map[string]string{
		"compose": strings.ReplaceAll(foldedCommand(t, files["infra/local/compose/provisioning.generated.yml"]), "$$", "$"),
		"job":     foldedCommand(t, files["infra/k8s/overlays/dev/databases/job.yaml"]),
	} {
		calls := runProvisioning(t, content, false, []string{"ALPHA_PG_PASSWORD=" + runtime, "ALPHA_PG_MIGRATOR_PASSWORD=" + migrator, "BETA_PG_PASSWORD=b"})
		for _, w := range []string{
			alterRole("alpha_migrator", "ALPHA_PG_MIGRATOR_PASSWORD", migrator, migratorAttrs),
			alterRole("alpha", "ALPHA_PG_PASSWORD", runtime, runtimeAttrs),
		} {
			if !slices.Contains(calls, w) {
				t.Errorf("%s: sem %q:\n%s", path, w, strings.Join(calls, "\n"))
			}
		}
		for _, c := range calls {
			if strings.Contains(c, "def_migrator") || strings.Contains(c, `ROLE "def"`) || strings.Contains(c, `ROLE "jkl"`) {
				t.Errorf("%s: a senha virou role: %s", path, c)
			}
		}
	}
	for _, name := range []string{"x", "y"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("a senha executou comando: %s criado", name)
		}
	}
}

func TestCheckDevSecretsRequiresTheMigratorSecret(t *testing.T) {
	root := withMigrator(t)
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	findings := infrasync.CheckDevSecrets(root, manifests)
	if len(findings) != 1 || !strings.Contains(findings[0].String(), "alpha-migrator") {
		t.Fatalf("CheckDevSecrets = %v, want the missing alpha-migrator Secret named", findings)
	}

	appendMigratorSecret(t, root, migratorDSN)
	if findings := infrasync.CheckDevSecrets(root, manifests); len(findings) != 0 {
		t.Fatalf("CheckDevSecrets = %v, want none with the migrator Secret in place", findings)
	}
}

func TestCheckDevSecretsReportsAMigratorPasswordThatDiverges(t *testing.T) {
	for _, tc := range []struct{ name, literal string }{
		{"senha", strings.Replace(migratorDSN, "alpha-migrator-dev@", "outra@", 1)},
		{"role", strings.Replace(migratorDSN, "alpha_migrator:", "alpha:", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := withMigrator(t)
			appendMigratorSecret(t, root, tc.literal)
			manifests, err := infrasync.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			findings := infrasync.CheckDevSecrets(root, manifests)
			if len(findings) != 1 || !strings.Contains(findings[0].String(), "PG_MIGRATE_DSN") {
				t.Fatalf("CheckDevSecrets = %v, want the diverging PG_MIGRATE_DSN named", findings)
			}
		})
	}
}

func TestCheckDevSecretsRefusesTheMigratorDSNInTheRuntimeSecret(t *testing.T) {
	root := withMigrator(t)
	appendMigratorSecret(t, root, migratorDSN)
	rewrite(t, root, alphaOverlay, "sslmode=disable\n", "sslmode=disable\n      - "+migratorDSN+"\n")
	manifests, err := infrasync.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	findings := infrasync.CheckDevSecrets(root, manifests)
	if len(findings) != 1 || !strings.Contains(findings[0].String(), "PG_MIGRATE_DSN fora do Secret alpha-migrator") {
		t.Fatalf("CheckDevSecrets = %v, want only the DSN outside the alpha-migrator Secret refused", findings)
	}
}

func TestCheckDevSecretsReadsAQuotedSecretName(t *testing.T) {
	for _, quoted := range []string{`"alpha-migrator"`, `'alpha-migrator'`} {
		t.Run(quoted, func(t *testing.T) {
			root := withMigrator(t)
			appendMigratorSecret(t, root, migratorDSN)
			rewrite(t, root, alphaOverlay, "- name: alpha-migrator", "- name: "+quoted)
			manifests, err := infrasync.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			if findings := infrasync.CheckDevSecrets(root, manifests); len(findings) != 0 {
				t.Fatalf("CheckDevSecrets = %v, want the quoted Secret name read", findings)
			}
		})
	}
}

func TestLoadRefusesAMigratorPasswordEqualToTheRuntimeOne(t *testing.T) {
	for _, tc := range []struct{ old, new, field string }{
		{`"local": "alpha-migrator-local"`, `"local": "alpha-local"`, "database.migrator.local igual a database.local"},
		{`"dev": "alpha-migrator-dev"`, `"dev": "alpha-dev"`, "database.migrator.dev igual a database.dev"},
	} {
		t.Run(tc.field, func(t *testing.T) {
			root := withMigrator(t)
			rewrite(t, root, "apps/backend/alpha/deploy/infra.json", tc.old, tc.new)
			if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("Load = %v, want %q", err, tc.field)
			}
		})
	}
}

func TestLoadLimitsTheAppWithAMigratorTo54Bytes(t *testing.T) {
	for _, tc := range []struct {
		size    int
		refused bool
	}{{54, false}, {55, true}} {
		t.Run(strconv.Itoa(tc.size), func(t *testing.T) {
			root := withMigrator(t)
			app := strings.Repeat("a", tc.size)
			if err := os.Rename(filepath.Join(root, "apps/backend/alpha"), filepath.Join(root, "apps/backend", app)); err != nil {
				t.Fatal(err)
			}
			rewrite(t, root, "apps/backend/"+app+"/deploy/infra.json", `"app": "alpha"`, `"app": "`+app+`"`)
			_, err := infrasync.Load(root)
			if refused := err != nil && strings.Contains(err.Error(), "passa de 54 bytes"); refused != tc.refused || (!tc.refused && err != nil) {
				t.Fatalf("Load = %v, want refused=%v", err, tc.refused)
			}
		})
	}
}

func TestLoadRefusesAnAppNamedAfterTheMigratorSecretOfAnother(t *testing.T) {
	root := withMigrator(t)
	dir := filepath.Join(root, "apps/backend/alpha-migrator/deploy")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	manifest := `{ "schema": "dmpf/infra@1", "app": "alpha-migrator", "image": { "env": "ALPHA_MIGRATOR_IMAGE", "local": "alpha-migrator:local" }, "grpc": {} }`
	if err := os.WriteFile(filepath.Join(dir, "infra.json"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := infrasync.Load(root); err == nil || !strings.Contains(err.Error(), "app alpha-migrator tem o nome do Secret do migrador da app alpha") {
		t.Fatalf("Load = %v, want the collision with the alpha-migrator Secret refused", err)
	}
}

func TestDatabasesJobWithAMigratorRunsWithoutTheServiceAccountToken(t *testing.T) {
	job := render(t, withMigrator(t))["infra/k8s/overlays/dev/databases/job.yaml"]
	if !strings.Contains(job, "\n    spec:\n      automountServiceAccountToken: false\n") {
		t.Fatalf("Job com o token da ServiceAccount montado:\n%s", job)
	}
}

func TestDatabasesJobWithAMigratorExpiresAfterFinishing(t *testing.T) {
	job := render(t, withMigrator(t))["infra/k8s/overlays/dev/databases/job.yaml"]
	if !strings.Contains(job, "\n  ttlSecondsAfterFinished: ") {
		t.Fatalf("Job sem ttlSecondsAfterFinished:\n%s", job)
	}
}

func foldedCommand(t *testing.T, content string) string {
	t.Helper()
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "- >-" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		var parts []string
		for _, next := range lines[i+1:] {
			if len(next)-len(strings.TrimLeft(next, " ")) <= indent {
				break
			}
			parts = append(parts, strings.TrimSpace(next))
		}
		return strings.Join(parts, " ")
	}
	t.Fatalf("sem bloco >- em:\n%s", content)
	return ""
}

const fakePsql = `#!/bin/sh
case " $* " in
*" -c "*|*" -tAc "*) printf '%s\n' "$*" >> "$PSQL_LOG" ;;
*) sql=$(cat); var=$(printf '%s\n' "$sql" | sed -n 's/^\\getenv pass //p')
  printf '%s <<< %s [pass=%s]\n' "$*" "$(printf '%s' "$sql" | tr '\n' ' ')" "$(printenv "$var")" >> "$PSQL_LOG" ;;
esac
case "$*" in
*pg_get_userbyid*) [ -n "$PSQL_OTHER_OWNER" ] || [ -z "$PSQL_EXISTS" ] || echo 1 ;;
*SELECT*) [ -z "$PSQL_EXISTS" ] || echo 1 ;;
esac
`

func runProvisioning(t *testing.T, script string, exists bool, env []string, extra ...string) []string {
	t.Helper()
	bin := t.TempDir()
	for name, body := range map[string]string{"psql": fakePsql, "pg_isready": "#!/bin/sh\n"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	log := filepath.Join(bin, "psql.log")
	cmd := exec.Command("sh", "-c", script)
	cmd.Env = append([]string{"PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH"), "PSQL_LOG=" + log}, env...)
	if exists {
		cmd.Env = append(cmd.Env, "PSQL_EXISTS=1")
	}
	cmd.Env = append(cmd.Env, extra...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("provisionamento falhou: %v\n%s\n--- script\n%s", err, out, script)
	}
	raw, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	var calls []string
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if !strings.HasPrefix(line, "-tAc ") {
			calls = append(calls, line)
		}
	}
	return calls
}

func TestProvisioningWithEveryAppMigratedHasNoEmptyLoop(t *testing.T) {
	root := withMigrator(t)
	rewrite(t, root, "apps/backend/beta/deploy/infra.json", `"dev": "beta-dev" }`,
		`"dev": "beta-dev", "migrator": { "passwordEnv": "BETA_PG_MIGRATOR_PASSWORD", "local": "beta-migrator-local", "dev": "beta-migrator-dev" } }`)
	files := render(t, root)

	for _, name := range []string{"infra/local/compose/provisioning.generated.yml", "infra/k8s/overlays/dev/databases/job.yaml"} {
		script := foldedCommand(t, files[name])
		if strings.Contains(script, "for pair in;") {
			t.Errorf("%s carries a loop without pairs:\n%s", name, script)
		}
	}
	calls := runProvisioning(t, foldedCommand(t, files["infra/k8s/overlays/dev/databases/job.yaml"]), false,
		[]string{"ALPHA_PG_PASSWORD=a", "ALPHA_PG_MIGRATOR_PASSWORD=m", "BETA_PG_PASSWORD=b", "BETA_PG_MIGRATOR_PASSWORD=n"})
	if !slices.Contains(calls, `-c CREATE DATABASE "beta" OWNER "beta_migrator"`) {
		t.Errorf("beta not created under its migrator:\n%s", strings.Join(calls, "\n"))
	}
}
