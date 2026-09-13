package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/evidence"
)

const (
	exitOK       = 0
	exitReproved = 1
	exitUsage    = 2
)

const (
	envPostgres      = "DMPF_PG_DSN"
	envKafka         = "DMPF_KAFKA_BROKERS"
	envRedpandaAdmin = "DMPF_REDPANDA_ADMIN"
)

const modulePrefix = "github.com/mateusmacedo/dmpf/"

type subjectSpec struct {
	name     string
	packages []string
	tags     []string
	skip     []string
	env      []string
	postgres bool
	redpanda bool
	records  bool
	tools    bool
}

var catalog = []subjectSpec{
	{
		name:     "golden",
		packages: []string{modulePrefix + "libs/backend/go/dmpf-contracts/golden"},
		skip:     []string{"TestUpdateGolden"},
		records:  true,
		tools:    true,
	},
	{
		name: "provider",
		packages: []string{
			modulePrefix + "libs/backend/go/dmpf-provider-postgres",
			modulePrefix + "libs/backend/go/dmpf-application/example/memory",
		},
		tags:     []string{"integration"},
		env:      []string{envPostgres},
		postgres: true,
		records:  true,
	},
	{
		name:     "domain",
		packages: []string{modulePrefix + "libs/backend/go/dmpf-testkit/domainkit"},
		records:  true,
	},
	{
		name:     "services",
		packages: []string{modulePrefix + "libs/backend/go/dmpf-testkit/serviceskit"},
		records:  true,
	},
	{
		name:     "app",
		packages: []string{modulePrefix + "libs/backend/go/dmpf-testkit/appkit"},
		tags:     []string{"integration"},
		env:      []string{envPostgres},
		postgres: true,
	},
	{
		name:     "dist",
		packages: []string{modulePrefix + "libs/backend/go/dmpf-testkit/distkit"},
		tags:     []string{"integration", "distributed"},
		skip:     []string{"TestDistkitRole"},
		env:      []string{envPostgres, envKafka, envRedpandaAdmin},
		postgres: true,
		redpanda: true,
	},
	{
		name:     "reference",
		packages: []string{modulePrefix + "apps/backend/dmpf-reference/..."},
		tags:     []string{"integration"},
		env:      []string{envPostgres, envKafka, envRedpandaAdmin},
		postgres: true,
		redpanda: true,
	},
}

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	root       string
	release    string
	out        string
	subjects   []subjectSpec
	allowDirty bool
	ci         bool
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	opts, err := parse(args, stderr)
	if err != nil {
		say(stderr, "dmpf-evidence: %v\n", err)
		return exitUsage
	}

	goVersion, commit, err := preconditions(ctx, opts)
	if err != nil {
		say(stderr, "dmpf-evidence: %v\n", err)
		return exitUsage
	}

	missing := map[string]string{}
	for _, s := range opts.subjects {
		for _, name := range s.env {
			if os.Getenv(name) != "" {
				continue
			}
			if opts.ci {
				say(stderr, "dmpf-evidence: subject %s needs %s, empty in CI\n", s.name, name)
				return exitReproved
			}
			if _, seen := missing[s.name]; !seen {
				missing[s.name] = name
			}
		}
	}

	records, err := os.MkdirTemp("", "dmpf-evidence-records-")
	if err != nil {
		say(stderr, "dmpf-evidence: %v\n", err)
		return exitReproved
	}

	var published []evidence.Subject
	for _, s := range opts.subjects {
		header := evidence.Header{
			Schema:    evidence.Schema,
			Release:   opts.release,
			Subject:   s.name,
			Commit:    commit,
			GoVersion: goVersion,
			Tags:      append([]string{}, s.tags...),
			Packages:  append([]string{}, s.packages...),
			Modules:   []evidence.Module{},
			Externals: []evidence.External{},
		}
		if name, skipped := missing[s.name]; skipped {
			say(stderr, "dmpf-evidence: subject %s skipped: %s not set (does not count as evidence)\n", s.name, name)
			published = append(published, evidence.Subject{Header: header, Skipped: name})
			continue
		}
		subject, err := collect(ctx, opts, s, header, records)
		if err != nil {
			say(stderr, "dmpf-evidence: subject %s: %v\n", s.name, err)
			return exitReproved
		}
		published = append(published, subject)
	}

	names := make([]string, 0, len(opts.subjects))
	for _, s := range opts.subjects {
		names = append(names, s.name)
	}
	index, err := evidence.Publish(opts.out, names, published)
	if err != nil {
		say(stderr, "dmpf-evidence: %v\n", err)
		return exitReproved
	}
	for _, e := range index {
		say(stdout, "%s %s\n", e.SHA256, e.Subject)
	}
	return exitOK
}

func say(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func parse(args []string, stderr io.Writer) (options, error) {
	flags := flag.NewFlagSet("dmpf-evidence", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("root", ".", "workspace root (holds go.work)")
	release := flags.String("release", "", "release semver, or latest for the highest bom/dmpf/<semver>.json")
	out := flags.String("out", "", "directory to publish into; must not exist")
	subjects := flags.String("subjects", "", "comma-separated subjects (default: all)")
	allowDirty := flags.Bool("allow-dirty", false, "run on a tree with uncommitted changes; the header commit gets a -dirty suffix")
	if err := flags.Parse(args); err != nil {
		return options{}, err
	}
	if flags.NArg() > 0 {
		return options{}, fmt.Errorf("unexpected arguments %v", flags.Args())
	}
	if *release == "" || *out == "" {
		return options{}, errors.New("--release and --out are required")
	}

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return options{}, err
	}
	resolved, err := resolveRelease(absRoot, *release)
	if err != nil {
		return options{}, err
	}
	selected, err := selectSubjects(*subjects)
	if err != nil {
		return options{}, err
	}
	absOut, err := filepath.Abs(*out)
	if err != nil {
		return options{}, err
	}
	if _, err := os.Stat(absOut); !errors.Is(err, fs.ErrNotExist) {
		return options{}, fmt.Errorf("--out %s already exists (or cannot be checked: %v)", absOut, err)
	}
	return options{
		root:       absRoot,
		release:    resolved,
		out:        absOut,
		subjects:   selected,
		allowDirty: *allowDirty,
		ci:         os.Getenv("CI") != "",
	}, nil
}

var releaseFile = regexp.MustCompile(`^(\d+)\.(\d+)\.(\d+)\.json$`)

func resolveRelease(root, release string) (string, error) {
	dir := filepath.Join(root, "bom", "dmpf")
	if release != "latest" {
		if _, err := os.Stat(filepath.Join(dir, release+".json")); err != nil {
			return "", fmt.Errorf("release %s: %w", release, err)
		}
		return release, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var best []int
	latest := ""
	for _, e := range entries {
		m := releaseFile.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		v := make([]int, 3)
		for i := range v {
			v[i], _ = strconv.Atoi(m[i+1])
		}
		if best == nil || slices.Compare(v, best) > 0 {
			best, latest = v, strings.TrimSuffix(e.Name(), ".json")
		}
	}
	if latest == "" {
		return "", fmt.Errorf("no bom/dmpf/<semver>.json under %s", dir)
	}
	return latest, nil
}

func selectSubjects(list string) ([]subjectSpec, error) {
	if list == "" {
		return slices.Clone(catalog), nil
	}
	wanted := map[string]bool{}
	for _, name := range strings.Split(list, ",") {
		name = strings.TrimSpace(name)
		if !slices.ContainsFunc(catalog, func(s subjectSpec) bool { return s.name == name }) {
			return nil, fmt.Errorf("unknown subject %q", name)
		}
		wanted[name] = true
	}
	var out []subjectSpec
	for _, s := range catalog {
		if wanted[s.name] {
			out = append(out, s)
		}
	}
	return out, nil
}

func preconditions(ctx context.Context, opts options) (goVersion, commit string, err error) {
	goWork, err := os.ReadFile(filepath.Join(opts.root, "go.work"))
	if err != nil {
		return "", "", err
	}
	toolchain, err := output(ctx, opts.root, "go", "env", "GOVERSION")
	if err != nil {
		return "", "", err
	}
	if goVersion, err = evidence.GoVersion(goWork, toolchain); err != nil {
		return "", "", err
	}
	if commit, err = output(ctx, opts.root, "git", "rev-parse", "HEAD"); err != nil {
		return "", "", err
	}
	status, err := output(ctx, opts.root, "git", "status", "--porcelain")
	if err != nil {
		return "", "", err
	}
	if status != "" {
		if !opts.allowDirty {
			return "", "", errors.New("the working tree has uncommitted changes: evidence must name a commit (use --allow-dirty only outside a release)")
		}
		commit += "-dirty"
	}
	return goVersion, commit, nil
}

func collect(ctx context.Context, opts options, s subjectSpec, header evidence.Header, records string) (evidence.Subject, error) {
	modules, externals, err := evidence.Reach(ctx, opts.root, s.tags, s.packages)
	if err != nil {
		return evidence.Subject{}, err
	}
	header.Modules, header.Externals = modules, externals

	infra := map[string]string{}
	if s.postgres {
		if infra["postgres"], err = postgresVersion(ctx, os.Getenv(envPostgres)); err != nil {
			return evidence.Subject{}, fmt.Errorf("postgres version: %w", err)
		}
	}
	if s.redpanda {
		if infra["redpanda"], err = redpandaVersion(ctx, os.Getenv(envRedpandaAdmin)); err != nil {
			return evidence.Subject{}, fmt.Errorf("redpanda version: %w", err)
		}
	}
	if len(infra) > 0 {
		header.Infra = infra
	}
	if s.tools {
		bufScript, err := os.ReadFile(filepath.Join(opts.root, "tools", "buf.sh"))
		if err != nil {
			return evidence.Subject{}, err
		}
		bufGen, err := os.ReadFile(filepath.Join(opts.root, "contracts", "buf.gen.yaml"))
		if err != nil {
			return evidence.Subject{}, err
		}
		if header.Tools, err = evidence.ToolVersions(bufScript, bufGen); err != nil {
			return evidence.Subject{}, err
		}
	}

	results, err := evidence.RunGoTest(ctx, evidence.GoTest{
		Dir:      opts.root,
		Tags:     s.tags,
		Packages: s.packages,
		Skip:     s.skip,
		Env:      []string{evidence.DirEnv + "=" + records},
	})
	if err != nil {
		return evidence.Subject{}, err
	}
	if failures := evidence.JudgeResults(results, opts.ci); len(failures) > 0 {
		return evidence.Subject{}, fmt.Errorf("reproved:\n  %s", strings.Join(failures, "\n  "))
	}
	body := &evidence.Body{Tests: results}
	if s.records {
		if body.Records, err = evidence.ReadRecords(records, s.name); err != nil {
			return evidence.Subject{}, err
		}
	}
	return evidence.Subject{Header: header, Body: body}, nil
}

func postgresVersion(ctx context.Context, dsn string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var version string
	if err := conn.QueryRow(ctx, "SHOW server_version").Scan(&version); err != nil {
		return "", err
	}
	return version, nil
}

func redpandaVersion(ctx context.Context, admin string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(admin, "/")+"/v1/brokers", nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GET %s: %s", req.URL, resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return evidence.ParseRedpandaBrokers(raw)
}

func output(ctx context.Context, dir, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	raw, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return "", fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return strings.TrimSpace(string(raw)), nil
}
