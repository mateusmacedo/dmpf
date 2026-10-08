#!/usr/bin/env bash
# Prova que um workspace Nx de fora consome o DMPF por versão: plugin por pnpm pack,
# kernel e conformance do PR pelo proxy file://, init, bounded-context e os gates.
# uso: external-consumer.sh [--work <dir>]; o workspace fica em <dir>, fora da árvore.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
KERNEL=github.com/mateusmacedo/dmpf
PLUGIN=@mateusmacedo/dmpf-plugin
WORK=""

while [ $# -gt 0 ]; do
  case "$1" in
    --work) WORK="$2"; shift 2 ;;
    *) echo "argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done
WORK="${WORK:-$(mktemp -d "${RUNNER_TEMP:-/tmp}/dmpf-consumer.XXXXXX")}"
mkdir -p "$WORK"
WORK="$(cd "$WORK" && pwd)"
case "$WORK/" in "$ROOT"/*) echo "--work precisa ficar fora da árvore do platform" >&2; exit 2 ;; esac
CONSUMER="$WORK/consumer"
[ ! -e "$CONSUMER" ] || { echo "$CONSUMER já existe: use outro --work" >&2; exit 2; }

passo() { printf '\n== %s\n' "$*"; }

export NX_DAEMON=false NX_NO_CLOUD=true NX_TUI=false

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
jq --arg v "$VERSION" '.kernel = $v | .conformance = $v' "$versions" > "$versions.novo"
mv -f "$versions.novo" "$versions"
TARBALL="$WORK/pack/dmpf-plugin-consumer.tgz"
tar -czf "$TARBALL" -C "$WORK/pack/unpacked" package

passo "workspace Nx vazio em $CONSUMER"
nx_version="$(jq -r .version node_modules/nx/package.json)"
mkdir -p "$CONSUMER"
jq -n --arg pm "$(jq -r .packageManager package.json)" --arg nx "$nx_version" \
  '{name: "@example/consumer", private: true, packageManager: $pm, devDependencies: {nx: $nx, "@nx/devkit": $nx}}' \
  > "$CONSUMER/package.json"
echo '{ "$schema": "./node_modules/nx/schemas/nx-schema.json" }' > "$CONSUMER/nx.json"
printf 'allowBuilds:\n  nx: true\n' > "$CONSUMER/pnpm-workspace.yaml"
printf 'node_modules\n.nx\n' > "$CONSUMER/.gitignore"
cd "$CONSUMER"
# O consumidor roda como um dev: sem CI, o pnpm cria o lockfile que ainda não existe
# e o tb.Env pula os testes de integração, que exigem a infra do job.
unset CI
git init -q
git -c user.name=ci -c user.email=ci@example.com commit -q --allow-empty -m "chore: workspace vazio"
pnpm install

passo "nx add: init com o prefixo reservado"
pnpm nx add "$PLUGIN@file:$TARBALL"
jq -e '.modulePrefix == "example.com/change-me" and .tooling.mode == "version"' dmpf.json > /dev/null

passo "init com o prefixo real, sem --force"
pnpm nx g "$PLUGIN:init" --modulePrefix=example.com/consumer --no-interactive
jq -e '.modulePrefix == "example.com/consumer"' dmpf.json > /dev/null
! grep -rq 'example.com/change-me' .golangci.yml

passo "bounded-context com o agregado do template"
pnpm nx g "$PLUGIN:bounded-context" shop --boundedContext=shop --no-interactive
pnpm nx run-many -t tidy
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$VERSION" --root . --write-baseline

passo "gates do consumidor"
mapfile -t pacotes < <(go list -m -f '{{.Path}}/...')
go build "${pacotes[@]}"
go vet "${pacotes[@]}"
go test "${pacotes[@]}"
while read -r dir; do
  (cd "$dir" && go mod edit -json) | jq -e --arg k "$KERNEL/" --arg v "$VERSION" \
    '([.Replace[]? | select(.Old.Path | startswith($k))] | length == 0)
     and all(.Require[]? | select(.Path | startswith($k)); .Version == $v)' > /dev/null \
    || { echo "$dir: kernel fora de $VERSION ou com replace" >&2; exit 1; }
done < <(go list -m -f '{{.Dir}}')
for gerado in .github/workflows/dmpf-ci.yml .claude/rules/dmpf-bounded-context.md; do
  [ -f "$gerado" ] || { echo "init não escreveu $gerado" >&2; exit 1; }
done
go run "$KERNEL/tools/dmpf-conformance/cmd/conformance@$VERSION" --root .
go run "$KERNEL/tools/dmpf-conformance/cmd/modsync@$VERSION" --root . --check
go run "$KERNEL/tools/dmpf-conformance/cmd/infrasync@$VERSION" --root . --check
DMPF_APPS_DIR="$(jq -r .appsDir dmpf.json)" \
  DMPF_KERNEL_DDL="$(go mod download -json "$KERNEL/libs/backend/go/postgres@$VERSION" | jq -r .Dir)" \
  bash "node_modules/$PLUGIN/scripts/dmpf-context-check.sh" --phase structural
# O generator deixa o .proto para o autor do contexto; um contrato vazio reprova no buf.
proto=apps/backend/shop/contract/proto/company/shop/service/v1
mkdir -p "$proto"
printf '%s\n' 'syntax = "proto3";' '' 'package company.shop.service.v1;' '' \
  'service ShopService {' '  rpc Ping(PingRequest) returns (PingResponse);' '}' '' \
  'message PingRequest {}' '' 'message PingResponse {}' > "$proto/shop_service.proto"
pnpm nx run-many -t buf-lint

passo "consumidor externo conforme ($CONSUMER)"
