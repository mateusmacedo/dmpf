#!/usr/bin/env bash
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

SCOPE=".github/actions/dmpf-scope/scope.sh"
FALHAS=0

uso() {
  cat <<'TXT'
uso: tools/dmpf-scope-check.sh --self-test

  --self-test  prova a decisão da dmpf-scope contra o grafo do workspace, com
               afetados e diff injetados, um vetor por cenário
TXT
}

falhar() {
  printf '  FALHA  %s\n' "$1" >&2
  FALHAS=$((FALHAS + 1))
}

aprovar() { printf '  ok     %s\n' "$1"; }

conferir() {
  local saida="$1" expectativa="$2" chave valor atual
  case "$expectativa" in
    *~*) chave="${expectativa%%~*}"; valor="${expectativa#*~}"
      atual="$(sed -n "s/^$chave=//p" <<<"$saida")"
      [[ " ${atual//,/ } " == *" $valor "* ]] ;;
    *!*) chave="${expectativa%%!*}"; valor="${expectativa#*!}"
      atual="$(sed -n "s/^$chave=//p" <<<"$saida")"
      [[ " ${atual//,/ } " != *" $valor "* ]] ;;
    *) grep -qxF -- "$expectativa" <<<"$saida" ;;
  esac
}

cenario() {
  local nome="$1" afetados="$2" mudados="$3" base="$4" saida expectativa antes="$FALHAS"
  shift 4
  printf '%s\n' "$afetados" > "$base/afetados.json"
  printf '%s' "$mudados" > "$base/mudados.txt"
  if ! saida="$(NX_EXCLUDE=@mateusmacedo/dmpf-source DMPF_APPS_DIR=apps/backend DMPF_PLUGIN_DIR=tools/dmpf-plugin \
    DMPF_TOOLING_MODE="${MODO:-local}" KEYCLOAK=true bash "$SCOPE" --graph "$base/grafo.json" --affected "$base/afetados.json" \
    --changed "$base/mudados.txt" 2>&1)"; then
    falhar "$nome: scope.sh saiu com erro: $saida"
    return
  fi
  for expectativa in "$@"; do
    conferir "$saida" "$expectativa" || falhar "$nome: esperava $expectativa"
  done
  [ "$FALHAS" -eq "$antes" ] && aprovar "$nome"
}

fase_self_test() {
  local base
  [ -f "$SCOPE" ] || { falhar "$SCOPE ausente"; return 1; }
  base="$(mktemp -d)" || return 2
  trap 'trash "$base" >/dev/null 2>&1 || true' RETURN
  pnpm --silent nx graph --file="$base/grafo.json" >/dev/null || { falhar "nx graph falhou"; return 1; }

  printf '\n== cenários da dmpf-scope ==\n'
  cenario "só bookings" '["bookings"]' 'apps/backend/bookings/app/rpc/server.go' "$base" \
    'projects=bookings' 'scope=run-many -p bookings' 'go=true' 'layer=true' 'context=apps/backend/bookings' \
    'modsync=true' 'conformance=true' 'infrasync=false' 'generator=false' 'init=false' 'versions=false' \
    'observability=false' 'gate-check=false' 'external-consumer=false' 'distributed=true' 'sonar=true' \
    'biome=changed' 'self-tests=harness' 'self-tests-job=true' 'infra=true' 'redis=false' 'keycloak=false' 'serial='

  cenario "ports" '["ports","application","app","bookings","bff"]' 'libs/backend/go/ports/doc.go' "$base" \
    'projects~ports' 'projects~bff' 'context=global' 'generator=true' 'external-consumer=true' 'redis=true' \
    'layer=true' 'conformance=true' 'gate-check=false' 'self-tests='

  cenario "só docs" '[]' 'docs/adr/README.md' "$base" \
    'projects=' 'scope=' 'go=false' 'layer=false' 'context=' 'modsync=false' 'conformance=false' 'infra=false' \
    'distributed=false' 'sonar=false' 'biome=changed' 'self-tests=' 'self-tests-job=false'

  cenario "só cell check" '[]' 'tools/dmpf-cell-check.sh' "$base" \
    'projects=' 'self-tests=cell' 'self-tests-job=true' 'gate-check=false' 'conformance=false'

  cenario "biome.json" '[]' 'biome.json' "$base" 'biome=global' 'projects='

  cenario "diff vazio" '[]' '' "$base" \
    'projects=' 'scope=' 'biome=false' 'context=' 'self-tests=' 'self-tests-job=false' 'conformance=false'

  cenario "autoproteção" '[]' '.github/actions/dmpf-scope/scope.sh' "$base" \
    'projects~bookings' 'projects~ports' 'projects~@mateusmacedo/dmpf-plugin' 'projects!@mateusmacedo/dmpf-source' \
    'context=global' 'biome=global' 'modsync=true' 'infrasync=true' 'conformance=true' 'init=true' \
    'versions=true' 'observability=true' 'gate-check=true' 'external-consumer=true' 'distributed=true' \
    'self-tests~scope' 'self-tests~cell' 'self-tests~harness' 'self-tests-job=true' 'keycloak=true'

  cenario "infra de teste" '[]' 'infra/test/compose.yml' "$base" \
    'projects~postgres' 'projects~authn' 'projects~bookings' 'projects!ports' 'infra=true' 'keycloak=true' \
    'infrasync=true' 'context=global' 'serial~postgres' 'serial~redis' 'serial~sqs' 'serial~kafka' \
    'serial~testkit' 'serial~authn' 'serial!bookings' 'serial!app'

  cenario "borda e deploy" '["bff"]' $'apps/backend/bff/app/main.go\napps/backend/bookings/deploy/infra.json' "$base" \
    'context=global' 'infrasync=true' 'observability=true' 'redis=true'

  cenario "dmpf.json" '[]' 'dmpf.json' "$base" 'infrasync=true' 'init=true' 'context=global'

  cenario "overlay de deploy" '["bookings"]' 'apps/backend/bookings/deploy/k8s/overlays/dev/kustomization.yaml' "$base" \
    'infrasync=true' 'context=global'

  cenario "regra do conformance" '["conformance"]' 'tools/dmpf-conformance/verifier/rules.go' "$base" \
    'self-tests~cell' 'self-tests~shared-kernel' 'self-tests~harness' 'conformance=true'

  cenario "versão do Biome" '[]' 'package.json' "$base" 'biome=global'

  cenario "init renderiza .env.example" '[]' '.env.example' "$base" 'init=true' 'infra=true' \
    'projects~postgres' 'projects~bookings'

  MODO=version cenario "plugin atualizado no consumidor" '["bookings"]' 'pnpm-lock.yaml' "$base" \
    'gate-check=true' 'conformance=true' 'modsync=true' 'infrasync=true' 'context=global'

  cenario "lockfile no platform" '["bookings"]' 'pnpm-lock.yaml' "$base" 'gate-check=false' 'infrasync=false'

  if [ "$FALHAS" -gt 0 ]; then
    printf '\nProva da dmpf-scope: REPROVADA com %d achado(s).\n' "$FALHAS" >&2
    return 1
  fi
  printf '\nProva da dmpf-scope: OK.\n'
}

case "${1:-}" in
  --self-test) fase_self_test ;;
  -h|--help) uso ;;
  *) uso >&2; exit 2 ;;
esac
