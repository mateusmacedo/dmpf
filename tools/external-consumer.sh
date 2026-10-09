#!/usr/bin/env bash
# Prova que um workspace Nx de fora consome o DMPF por versão: init, bounded-context e
# gates, com o plugin do PR (pack + proxy file://) ou publicado (--plugin-version).
# uso: external-consumer.sh [--work <dir>] [--plugin-version <v> [--upgrade-from <v>]]
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
KERNEL=github.com/mateusmacedo/dmpf
PLUGIN=@mateusmacedo/dmpf-plugin
WORK=""
PLUGIN_VERSION=""
UPGRADE_FROM=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --work) WORK="$2"; shift 2 ;;
    --plugin-version) PLUGIN_VERSION="$2"; shift 2 ;;
    --upgrade-from) UPGRADE_FROM="$2"; shift 2 ;;
    *) echo "argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done
[[ -z "$UPGRADE_FROM" ]] || [[ -n "$PLUGIN_VERSION" ]] || { echo "--upgrade-from exige --plugin-version" >&2; exit 2; }
[[ -z "$PLUGIN_VERSION" ]] || [[ -n "${NODE_AUTH_TOKEN:-}" ]] || { echo "--plugin-version exige NODE_AUTH_TOKEN com read:packages" >&2; exit 2; }
WORK="${WORK:-$(mktemp -d "${RUNNER_TEMP:-/tmp}/dmpf-consumer.XXXXXX")}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
case "$WORK/" in "$ROOT"/*) echo "--work precisa ficar fora da árvore do platform" >&2; exit 2 ;; *) ;; esac
CONSUMER="$WORK/consumer"
[[ ! -e "$CONSUMER" ]] || { echo "$CONSUMER já existe: use outro --work" >&2; exit 2; }

passo() { printf '\n== %s\n' "$*"; }
# Mesma decisão do dmpf-go-ci.yml: sem contexto com provider, não há DDL do kernel.
kernel_ddl() {
  local postgres="$KERNEL/libs/backend/go/postgres"
  if go list -m "$postgres" > /dev/null 2>&1; then
    go mod download -json "$postgres" | jq -r .Dir
  else
    echo none
  fi
}
versao_instalada() { local campo="$1"; jq -r ".$campo" "node_modules/$PLUGIN/versions.json"; }
commitar() { local mensagem="$1"; git add -A && git -c user.name=ci -c user.email=ci@example.com commit -q -m "$mensagem"; }

export NX_DAEMON=false NX_NO_CLOUD=true NX_TUI=false FORCE_COLOR=0

if [[ -z "$PLUGIN_VERSION" ]]; then
  passo "kernel e conformance do PR pelo proxy file://"
  VERSION="$(bash tools/kernel-file-proxy.sh --out "$WORK/proxy" | tail -1)"
  GOPROXY="file://$WORK/proxy,$(go env GOPROXY)"
  export GOPROXY GONOSUMDB="$KERNEL"

  passo "plugin empacotado, com o versions.json apontando para $VERSION"
  pnpm nx run "$PLUGIN:build"
  mkdir -p "$WORK/pack/unpacked"
  (cd tools/dmpf-plugin && pnpm pack --pack-destination "$WORK/pack")
  tar -xzf "$WORK"/pack/*.tgz -C "$WORK/pack/unpacked"
  versions="$WORK/pack/unpacked/package/versions.json"
  jq --arg v "$VERSION" --arg ref "$(git rev-parse HEAD)" '.kernel = $v | .conformance = $v | .workflowRef = $ref' \
    "$versions" > "$versions.novo"
  mv -f "$versions.novo" "$versions"
  TARBALL="$WORK/pack/dmpf-plugin-consumer.tgz"
  tar -czf "$TARBALL" -C "$WORK/pack/unpacked" package
  INSTALAR="$PLUGIN@file:$TARBALL"
else
  INSTALAR="$PLUGIN@${UPGRADE_FROM:-$PLUGIN_VERSION}"
fi

passo "workspace Nx vazio em $CONSUMER"
nx_version="$(jq -r .version node_modules/nx/package.json)"
mkdir -p "$CONSUMER"
jq -n --arg pm "$(jq -r .packageManager package.json)" --arg nx "$nx_version" \
  '{name: "@example/consumer", private: true, packageManager: $pm, devDependencies: {nx: $nx, "@nx/devkit": $nx}}' \
  > "$CONSUMER/package.json"
echo '{ "$schema": "./node_modules/nx/schemas/nx-schema.json" }' > "$CONSUMER/nx.json"
printf 'allowBuilds:\n  nx: true\n' > "$CONSUMER/pnpm-workspace.yaml"
printf 'node_modules\n.nx\n' > "$CONSUMER/.gitignore"
if [[ -n "$PLUGIN_VERSION" ]]; then
  printf '%s\n' '@mateusmacedo:registry=https://npm.pkg.github.com' 'fetch-retries=5' 'fetch-retry-mintimeout=10000' \
    > "$CONSUMER/.npmrc"
  # O pnpm 11.5.3+ não expande ${VAR} em credencial do .npmrc do projeto (GHSA-3qhv-2rgh-x77r).
  printf '%s\n' '//npm.pkg.github.com/:_authToken=${NODE_AUTH_TOKEN}' > "$WORK/npmrc"
  export NPM_CONFIG_USERCONFIG="$WORK/npmrc"
fi
cd "$CONSUMER"
# O consumidor roda como um dev: sem CI, o pnpm cria o lockfile que ainda não existe
# e o tb.Env pula os testes de integração, que exigem a infra do job.
unset CI
git init -q
commitar "chore: workspace vazio"
pnpm install

# Logo depois do publish, o pacote pode ainda não aparecer no registry.
if [[ -n "$PLUGIN_VERSION" ]]; then
  for tentativa in $(seq 1 10); do
    npm view "$INSTALAR" version > /dev/null 2>&1 && break
    [[ "$tentativa" -lt 10 ]] || { echo "$INSTALAR não apareceu no registry" >&2; exit 1; }
    sleep 15
  done
fi

passo "nx add $INSTALAR: init com o prefixo reservado"
pnpm nx add "$INSTALAR"
jq -e '.modulePrefix == "example.com/change-me" and .tooling.mode == "version"' dmpf.json > /dev/null

passo "init com o prefixo real, sem --force"
pnpm nx g "$PLUGIN:init" --modulePrefix=example.com/consumer --no-interactive
jq -e '.modulePrefix == "example.com/consumer"' dmpf.json > /dev/null
if grep -q 'example.com/change-me' .golangci.yml; then
  echo "o init deixou o prefixo reservado no .golangci.yml" >&2
  exit 1
fi
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" DMPF_KERNEL_DDL="$(kernel_ddl)" \
  bash "node_modules/$PLUGIN/scripts/dmpf-context-check.sh" --phase structural --allow-empty

passo "bounded-context com o agregado do template"
pnpm nx g "$PLUGIN:bounded-context" shop --boundedContext=shop --no-interactive
pnpm nx run-many -t tidy
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$(versao_instalada conformance)" --root . --write-baseline
# O generator deixa o .proto para o autor do contexto; um contrato vazio reprova no buf.
proto=apps/backend/shop/contract/proto/company/shop/service/v1
mkdir -p "$proto"
printf '%s\n' 'syntax = "proto3";' '' 'package company.shop.service.v1;' '' \
  'service ShopService {' '  rpc Ping(PingRequest) returns (PingResponse);' '}' '' \
  'message PingRequest {}' '' 'message PingResponse {}' > "$proto/shop_service.proto"

if [[ -n "$UPGRADE_FROM" ]]; then
  passo "nx migrate de $UPGRADE_FROM para $PLUGIN_VERSION"
  commitar "chore: consumidor em $UPGRADE_FROM"
  pnpm nx migrate "$PLUGIN@$PLUGIN_VERSION"
  [[ -f migrations.json ]] || { echo "$PLUGIN@$PLUGIN_VERSION não trouxe migrations a partir de $UPGRADE_FROM" >&2; exit 1; }
  pnpm install
  pnpm nx migrate --run-migrations | tee "$WORK/migrate.log"
  # O Nx lista os nextSteps das migrations sob esse título, um por linha com "- ".
  mapfile -t passos < <(awk '/Some migrations have additional information/ {f = 1; next}
    f && /^[[:space:]]*- / {sub(/^[[:space:]]*- /, ""); print}' "$WORK/migrate.log")
  for comando in "${passos[@]}"; do
    case "$comando" in
      "pnpm install" | "pnpm nx run-many -t tidy") ;;
      *) echo "nextStep fora do esperado: $comando" >&2; exit 1 ;;
    esac
    echo "nextStep: $comando"
    bash -c "$comando"
  done
  jq -e --arg v "$PLUGIN_VERSION" '.devDependencies["@mateusmacedo/dmpf-plugin"] | ltrimstr("^") | ltrimstr("~") == $v' package.json > /dev/null
  grep -qF "plugin-version: \"$PLUGIN_VERSION\"" .github/workflows/dmpf-ci.yml \
    || { echo "o chamador de CI não subiu para $PLUGIN_VERSION" >&2; exit 1; }
fi

KERNEL_VERSION="$(versao_instalada kernel)"
CONFORMANCE="$(versao_instalada conformance)"
passo "gates do consumidor (kernel $KERNEL_VERSION, conformance $CONFORMANCE)"
mapfile -t pacotes < <(go list -m -f '{{.Path}}/...')
go build "${pacotes[@]}"
go vet "${pacotes[@]}"
go test "${pacotes[@]}"
while read -r dir; do
  (cd "$dir" && go mod edit -json) | jq -e --arg k "$KERNEL/" --arg v "$KERNEL_VERSION" \
    '([.Replace[]? | select(.Old.Path | startswith($k))] | length == 0)
     and all(.Require[]? | select(.Path | startswith($k)); .Version == $v)' > /dev/null \
    || { echo "$dir: kernel fora de $KERNEL_VERSION ou com replace" >&2; exit 1; }
done < <(go list -m -f '{{.Dir}}')
for gerado in .github/workflows/dmpf-ci.yml .claude/rules/dmpf-bounded-context.md; do
  [[ -f "$gerado" ]] || { echo "init não escreveu $gerado" >&2; exit 1; }
done
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$CONFORMANCE" --root .
go run "$KERNEL/tools/dmpf-conformance/cmd/modsync@$CONFORMANCE" --root . --check
go run "$KERNEL/tools/dmpf-conformance/cmd/infrasync@$CONFORMANCE" --root . --check
ddl="$(kernel_ddl)"
[[ -d "$ddl" ]] || { echo "módulo postgres do kernel fora do cache: $ddl" >&2; exit 1; }
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" DMPF_KERNEL_DDL="$ddl" \
  bash "node_modules/$PLUGIN/scripts/dmpf-context-check.sh" --phase structural
pnpm nx run-many -t buf-lint

passo "consumidor externo conforme ($CONSUMER)"
