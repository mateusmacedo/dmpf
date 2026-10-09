#!/usr/bin/env bash
# GOPROXY file:// com o kernel e o dmpf-conformance da árvore numa versão efêmera:
# o consumidor de prova resolve o código do PR pelo caminho por tag de uma release.
# uso: kernel-file-proxy.sh --out <dir> [--version <v>]; a versão sai na última linha.
set -euo pipefail

ROOT="$(git rev-parse --show-toplevel)"
cd "$ROOT"
KERNEL=github.com/mateusmacedo/dmpf
OUT=""
VERSION=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --out) OUT="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    *) echo "argumento desconhecido: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$OUT" ]] || { echo "uso: kernel-file-proxy.sh --out <dir> [--version <v>]" >&2; exit 2; }
# O prefixo `g` impede um identificador só de dígitos com zero à esquerda, que o
# SemVer recusa; o sufixo da árvore suja evita que o cache sirva um zip antigo.
sujo="$(git diff HEAD -- libs/backend/go tools/dmpf-conformance | sha256sum | cut -c1-8)"
[[ -n "$(git diff HEAD --name-only -- libs/backend/go tools/dmpf-conformance)" ]] || sujo=""
VERSION="${VERSION:-v0.0.0-ci.g$(git rev-parse --short=12 HEAD)${sujo:+.w$sujo}}"
mkdir -p "$OUT"
OUT="$(cd "$OUT" && pwd)"
STAGE="$(mktemp -d)"
INFO_TIME="$(date -u -d "@$(git log -1 --format=%ct HEAD)" +%Y-%m-%dT%H:%M:%SZ)"

mapfile -t MODULES < <(
  find libs/backend/go -mindepth 2 -maxdepth 2 -name go.mod -printf '%h\n' | sort
  echo tools/dmpf-conformance
)

arquivos_do_modulo() {
  local dir="$1" f n
  mapfile -t aninhados < <(cd "$dir" && git ls-files -- '*/go.mod' | xargs -r -n1 dirname)
  while IFS= read -r -d '' f; do
    for n in "${aninhados[@]}"; do
      [[ "$f" == "$n"/* ]] && continue 2
    done
    printf '%s\0' "$f"
  done < <(cd "$dir" && git ls-files -z)
}

for dir in "${MODULES[@]}"; do
  path="$KERNEL/$dir"
  [[ "$path" == "${path,,}" ]] || { echo "$path: caminho com maiúscula exige escape de módulo" >&2; exit 1; }
  stage="$STAGE/$path@$VERSION"
  mkdir -p "$stage"
  arquivos_do_modulo "$dir" | tar -C "$dir" --null -T - -cf - | tar -C "$stage" -xf -
  # Sem a reescrita, o MVS escolheria a tag publicada dos irmãos, maior que a
  # versão efêmera, e o consumidor provaria parte do kernel de uma release.
  irmaos="$(cd "$stage" && GOWORK=off go mod edit -json | jq -r --arg k "$KERNEL/" '.Require[]?.Path | select(startswith($k))')"
  for irmao in $irmaos; do
    (cd "$stage" && GOWORK=off go mod edit -require="$irmao@$VERSION")
  done
  dest="$OUT/$path/@v"
  mkdir -p "$dest"
  cp "$stage/go.mod" "$dest/$VERSION.mod"
  printf '{"Version":"%s","Time":"%s"}\n' "$VERSION" "$INFO_TIME" > "$dest/$VERSION.info"
  zipfile="$STAGE/$(basename "$dir").zip"
  (cd "$STAGE" && zip -q -r -D -X "$zipfile" "$path@$VERSION")
  mv -f "$zipfile" "$dest/$VERSION.zip"
  grep -qxF "$VERSION" "$dest/list" 2>/dev/null || echo "$VERSION" >> "$dest/list"
done

verifica="$(mktemp -d)"
for dir in "${MODULES[@]}"; do
  erro="$(cd "$verifica" && GOWORK=off GOFLAGS=-mod=mod GOPROXY="file://$OUT" GONOSUMDB="$KERNEL" \
    go mod download -json "$KERNEL/$dir@$VERSION" | jq -r '.Error // empty')"
  [[ -z "$erro" ]] || { echo "$KERNEL/$dir@$VERSION: $erro" >&2; exit 1; }
done
echo "proxy file://$OUT com ${#MODULES[@]} módulo(s)" >&2
echo "$VERSION"
