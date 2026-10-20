#!/usr/bin/env bash
set -euo pipefail

uso() {
  cat <<'TXT'
uso: scope.sh [--graph <arquivo> --affected <arquivo> --changed <arquivo>]

Sem argumentos, coleta o grafo e os afetados pelo Nx e o diff NX_BASE...NX_HEAD.
Com os três argumentos, decide sobre os insumos dados (a prova os injeta).
Imprime uma linha chave=valor por output da action.
TXT
}

GRAFO="" AFETADOS="" MUDADOS=""
while [ $# -gt 0 ]; do
  case "$1" in
    --graph) shift; GRAFO="${1:-}" ;;
    --affected) shift; AFETADOS="${1:-}" ;;
    --changed) shift; MUDADOS="${1:-}" ;;
    -h|--help) uso; exit 0 ;;
    *) uso >&2; exit 2 ;;
  esac
  shift
done

APPS="${DMPF_APPS_DIR:-apps/backend}"
PLUGIN="${DMPF_PLUGIN_DIR:-tools/dmpf-plugin}"
MODO="${DMPF_TOOLING_MODE:-local}"
EXCLUIR="${NX_EXCLUDE:-}"
KEYCLOAK="${KEYCLOAK:-false}"

dir="$(mktemp -d "${RUNNER_TEMP:-${TMPDIR:-/tmp}}/dmpf-scope.XXXXXX")"
if [ -z "$GRAFO" ]; then
  : "${NX_BASE:?NX_BASE ausente}" "${NX_HEAD:?NX_HEAD ausente}"
  GRAFO="$dir/grafo.json" AFETADOS="$dir/afetados.json" MUDADOS="$dir/mudados.txt"
  pnpm --silent nx graph --file="$GRAFO" >/dev/null
  pnpm --silent nx show projects --affected --base="$NX_BASE" --head="$NX_HEAD" --exclude="$EXCLUIR" --json > "$AFETADOS"
  # Sem --no-renames, um arquivo movido para fora de um gatilho some do diff.
  git diff --name-only --no-renames "$NX_BASE...$NX_HEAD" > "$MUDADOS"
elif [ -z "$AFETADOS" ] || [ -z "$MUDADOS" ]; then
  uso >&2
  exit 2
fi

mapfile -t mudados < "$MUDADOS"

toca() {
  local f p
  for f in "${mudados[@]}"; do
    [ -n "$f" ] || continue
    for p in "$@"; do
      # shellcheck disable=SC2053 # $p é padrão glob de propósito
      [[ $f == $p ]] && return 0
    done
  done
  return 1
}

booleano() { if "$@"; then echo true; else echo false; fi; }

tudo=false
toca '.github/actions/dmpf-scope/*' '.github/workflows/dmpf-go-ci.yml' && tudo=true
infra_teste=false
toca 'infra/test/*' 'infra/keycloak/*' && infra_teste=true
# No modo version as ferramentas e os scripts dos gates vêm do plugin em
# node_modules, fora do diff: trocar a versão dele muda o que os gates verificam.
ferramentas=false
[ "$MODO" = version ] && toca 'package.json' 'pnpm-lock.yaml' && ferramentas=true

jq -c --arg ex "$EXCLUIR" '[.graph.nodes[] | select(.name != $ex)]' "$GRAFO" > "$dir/nos.json"

projetos="$(jq -r -n --slurpfile nos "$dir/nos.json" --slurpfile af "$AFETADOS" \
  --argjson tudo "$tudo" --argjson infra "$infra_teste" '
  def usa_infra: [.data.targets[]?.dependsOn[]? | if type == "string" then . else .target end]
    | any(. == "test-infra-up" or . == "test-keycloak-up");
  ($nos[0] | map(.name)) as $todos
  | if $tudo then $todos
    else ($af[0] | map(select(. as $n | $todos | index($n))))
      + (if $infra then [$nos[0][] | select(usa_infra) | .name] else [] end)
    end
  | unique | join(",")')"

sel="$(jq -c --arg p "$projetos" '($p | split(",")) as $l | [.[] | select(.name as $n | $l | index($n))]' "$dir/nos.json")"

consulta() { jq -r --arg apps "$APPS" --arg plugin "$PLUGIN" "$1" <<<"$sel"; }

go="$(consulta 'any(.[]; .data.tags | index("stack:go"))')"
serial="$(consulta '[.[] | select(.data.targets["test-race"].cache == false and (.data.tags | index("layer:providers"))) | .name] | join(",")')"
infra="$(consulta 'any(.[]; .data.targets["test-race"].cache == false or .data.targets["test-integration"] != null or .data.targets.e2e != null)')"
plugin="$(consulta 'any(.[]; .data.root == $plugin)')"
kernel="$(consulta 'any(.[]; .data.root | startswith("libs/backend/go/"))')"
conformance_afetado="$(consulta 'any(.[]; .name == "conformance")')"
distribuido="$(consulta 'any(.[]; .data.targets["test-distributed"] != null)')"
buf_selftest="$(consulta 'any(.[]; .data.targets["buf-gate-selftest"] != null)')"

keycloak=false
[ "$KEYCLOAK" = true ] && [ "$infra" = true ] && [[ ",$projetos," == *",authn,"* ]] && keycloak=true
redis=false
[ "$infra" = true ] && [[ ",$projetos," == *",redis,"* || ",$projetos," == *",bff,"* ]] && redis=true

# A borda é app Go sem domain/: o modo --context não a cobre, só o estrutural.
contextos=() borda=false
while IFS= read -r raiz; do
  [ -n "$raiz" ] || continue
  if [ -d "$raiz/domain" ]; then contextos+=("$raiz"); else borda=true; fi
done < <(consulta '.[] | select((.data.tags | index("stack:go")) and (.data.root | startswith($apps + "/"))
  and (.data.root | ltrimstr($apps + "/") | contains("/") | not)) | .data.root')

if [ "$tudo" = true ] || [ "$borda" = true ] || [ "$ferramentas" = true ] || toca 'dmpf.json' 'infra/*' \
  "$APPS/*/deploy/*" '.github/workflows/*' 'libs/backend/go/postgres/*' "$PLUGIN/scripts/dmpf-context-check.sh"; then
  context=global
else
  context="${contextos[*]:-}"
fi

if [ "$tudo" = true ] || toca 'biome.json' 'biome.jsonc' 'package.json' 'pnpm-lock.yaml' '.gitignore'; then
  biome=global
elif [ "${#mudados[@]}" -gt 0 ] && [ -n "${mudados[0]}" ]; then
  biome=changed
else
  biome=false
fi

se() { [ "$tudo" = true ] || [ "$ferramentas" = true ] || "$@"; }
ou() { [ "$1" = true ] || { shift; [ $# -gt 0 ] && toca "$@"; }; }

vetores=()
declare -A gatilhos=(
  [cell]='tools/dmpf-cell-check.sh tools/dmpf-conformance/* *dmpf-units.json'
  [shared-kernel]='tools/dmpf-shared-kernel-check.sh tools/dmpf-conformance/*'
  [generator]="tools/dmpf-generator-check.sh $PLUGIN/src/*"
  [context]="$PLUGIN/scripts/dmpf-context-check.sh"
  [grafana]='tools/grafana-provisioning-check.sh'
  [otel]='tools/otel-env-check.sh'
  [harness]="tools/dmpf-harness-check.sh tools/dmpf-conformance/* tools/dmpf-baseline/* $APPS/bookings/*"
  [scope]='.github/actions/dmpf-scope/* tools/dmpf-scope-check.sh'
)
for vetor in cell shared-kernel generator context grafana otel harness scope; do
  read -ra padroes <<<"${gatilhos[$vetor]}"
  if se toca "${padroes[@]}"; then vetores+=("$vetor"); fi
done

gate_check="$(booleano se toca '.golangci.yml' '*dmpf-units.json' "$PLUGIN/scripts/dmpf-gate-check.sh")"

self_tests_job=false
if [ "${#vetores[@]}" -gt 0 ] || [ "$gate_check" = true ] || [ "$buf_selftest" = true ]; then
  self_tests_job=true
fi

kernel_ou_plugin=false
[ "$kernel" = true ] || [ "$plugin" = true ] && kernel_ou_plugin=true

cat <<SAIDA
graph=$GRAFO
projects=$projetos
scope=${projetos:+run-many -p $projetos}
serial=$serial
infra=$infra
keycloak=$keycloak
redis=$redis
go=$go
layer=$go
context=$context
biome=$biome
modsync=$(booleano se ou "$go" 'go.work' 'go.mod' '*/go.mod' 'tools/dmpf-conformance/*')
infrasync=$(booleano se toca 'dmpf.json' "$APPS/*/deploy/*" 'infra/*' 'tools/dmpf-conformance/*')
conformance=$(booleano se ou "$go" '*dmpf-units.json' 'tools/dmpf-baseline/*' 'tools/dmpf-conformance/*')
generator=$(booleano se ou "$kernel_ou_plugin" 'tools/dmpf-generator-check.sh')
init=$(booleano se ou "$plugin" 'dmpf.json' '.env.example' '.golangci.yml' 'infra/*' '.claude/*' '.agents/*' \
  '.github/actions/*')
versions=$(booleano se toca 'go.work' "$APPS/*/Dockerfile" 'libs/backend/go/contracts/buf.gen.yaml' 'nx.json' \
  'pnpm-workspace.yaml' 'libs/backend/go/domain/project.json' "$PLUGIN/versions.json" "$PLUGIN/migrations.json" \
  "$PLUGIN/scripts/sync-versions.mjs")
observability=$(booleano se toca 'infra/observability/*' 'infra/local/*' "$APPS/*/deploy/*" \
  "$PLUGIN/templates/bounded-context/deploy/*" 'tools/grafana-provisioning-check.sh' 'tools/otel-env-check.sh')
gate-check=$gate_check
self-tests=${vetores[*]:-}
self-tests-job=$self_tests_job
external-consumer=$(booleano se ou "$( [ "$kernel_ou_plugin" = true ] || [ "$conformance_afetado" = true ] && echo true || echo false)" \
  'tools/external-consumer.sh' 'tools/kernel-file-proxy.sh' '.github/actions/*')
distributed=$distribuido
sonar=$(booleano se ou "$( [ "$go" = true ] || [ "$plugin" = true ] && echo true || echo false)")
SAIDA
