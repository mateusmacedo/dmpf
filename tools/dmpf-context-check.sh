#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova e por que o critério de descoberta é esse, no mesmo molde de tools/dmpf-cell-check.sh.
# Prova que todo bounded context de apps/backend segue o layout canônico: um
# package por bloco sob o módulo do contexto, a borda em subpacote de `app/`,
# o binário em `cmd/`, os kits de teste e os targets que os servem.
#
# O critério de descoberta é a presença de `domain/`: um contexto tem domínio,
# uma borda como o `bff` não tem. Isso separa os dois sem lista fixa e passa a
# valer para contexto novo no dia em que ele nasce.
#
# A fase `self-test` prova o próprio gate: sobre uma fixture sintética, sabota
# um ponto por execução e exige que o gate reprove por aquele motivo. Sem ela,
# um gate que nunca morde passaria por gate.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

APPS="apps/backend"
FALHAS=0

falhar() {
  printf '  FALHA  %s\n' "$1" >&2
  FALHAS=$((FALHAS + 1))
}

aprovar() { printf '  ok     %s\n' "$1"; }

# contextos lista o que o gate governa: diretório de apps/backend com domain/.
contextos() {
  local raiz="${1:-$APPS}" caminho ctx
  for caminho in "$raiz"/*/; do
    [ -d "$caminho" ] || continue
    ctx="$(basename "$caminho")"
    [ -d "$caminho/domain" ] || continue
    printf '%s\n' "$ctx"
  done
}

# tem_target responde se o project.json declara o alvo, sem depender do Nx: o
# gate roda antes de qualquer grafo estar carregado.
tem_target() {
  local arquivo="$1" alvo="$2"
  python3 - "$arquivo" "$alvo" <<'PY'
import json
import sys

try:
    with open(sys.argv[1], encoding='utf-8') as f:
        targets = json.load(f).get('targets', {})
except (OSError, ValueError):
    sys.exit(2)
sys.exit(0 if sys.argv[2] in targets else 1)
PY
}

# tem_unidade responde se o manifesto declara a unidade cujo id termina no
# sufixo dado, seja qual for o bounded context que o contexto declarou.
tem_unidade() {
  local arquivo="$1" sufixo="$2"
  python3 - "$arquivo" "$sufixo" <<'PY'
import json
import sys

try:
    with open(sys.argv[1], encoding='utf-8') as f:
        units = json.load(f).get('units', [])
except (OSError, ValueError):
    sys.exit(2)
sys.exit(0 if any(u.get('id', '').endswith('/' + sys.argv[2]) for u in units) else 1)
PY
}

verificar_contexto() {
  local raiz="$1" ctx="$2" base="$1/$2"

  for bloco in domain application provider app; do
    if [ -d "$base/$bloco" ]; then
      aprovar "$ctx: bloco $bloco"
    else
      falhar "$ctx: falta o package do bloco $bloco"
    fi
  done

  # A raiz do contexto guarda só configuração e manifesto: código de bloco
  # vive no package do bloco, e a migração para app/ existiu para isso.
  local soltos
  soltos="$(find "$base" -maxdepth 1 -name '*.go' -printf '%f\n' 2>/dev/null)"
  if [ -n "$soltos" ]; then
    falhar "$ctx: código Go solto na raiz do contexto ($(echo "$soltos" | tr '\n' ' '))"
  else
    aprovar "$ctx: raiz sem código de bloco"
  fi

  if [ -f "$base/cmd/main.go" ]; then
    aprovar "$ctx: binário em cmd/main.go"
  else
    falhar "$ctx: falta o binário em cmd/main.go"
  fi

  if find "$base/app" -mindepth 1 -maxdepth 1 -type d 2>/dev/null | read -r; then
    aprovar "$ctx: borda em subpacote de app/"
  else
    falhar "$ctx: a borda do transporte não está em subpacote de app/"
  fi

  for kit in appkit distkit; do
    if [ -d "$base/$kit" ]; then
      aprovar "$ctx: $kit"
    else
      falhar "$ctx: falta o $kit"
    fi
    if tem_unidade "$base/dmpf-units.json" "$kit"; then
      aprovar "$ctx: unidade do $kit declarada"
    else
      falhar "$ctx: o manifesto não declara a unidade do $kit"
    fi
  done

  for alvo in serve-api serve-relay test-distributed; do
    if tem_target "$base/project.json" "$alvo"; then
      aprovar "$ctx: target $alvo"
    else
      falhar "$ctx: falta o target $alvo"
    fi
  done
}

fase_estrutural() {
  local raiz="${1:-$APPS}" achou=0 ctx
  while IFS= read -r ctx; do
    achou=1
    printf '\n== %s ==\n' "$ctx"
    verificar_contexto "$raiz" "$ctx"
  done < <(contextos "$raiz")

  if [ "$achou" -eq 0 ]; then
    echo "nenhum bounded context encontrado em $raiz" >&2
    return 2
  fi
  if [ "$FALHAS" -gt 0 ]; then
    printf '\nGate de estrutura: REPROVADO com %d achado(s).\n' "$FALHAS" >&2
    return 1
  fi
  printf '\nGate de estrutura: OK — todo contexto segue o layout canônico.\n'
  return 0
}

# A fixture nasce completa e cada vetor remove uma peça: assim o gate é provado
# contra o que ele deve recusar, e não só contra o que já passa.
montar_fixture() {
  local raiz="$1"
  local ctx="$raiz/probe"
  mkdir -p "$ctx"/{domain,application,provider,app/http,appkit,distkit,cmd}
  : > "$ctx/domain/doc.go"
  : > "$ctx/application/doc.go"
  : > "$ctx/provider/doc.go"
  : > "$ctx/app/doc.go"
  : > "$ctx/app/http/doc.go"
  : > "$ctx/appkit/doc.go"
  : > "$ctx/distkit/doc.go"
  : > "$ctx/cmd/main.go"
  cat > "$ctx/project.json" <<'JSON'
{"targets":{"serve-api":{},"serve-relay":{},"test-distributed":{}}}
JSON
  cat > "$ctx/dmpf-units.json" <<'JSON'
{"units":[{"id":"probe/app"},{"id":"probe/appkit"},{"id":"probe/distkit"}]}
JSON
}

fase_self_test() {
  local base tmp status=0
  base="$(mktemp -d)" || return 2
  trap 'trash "$base" >/dev/null 2>&1 || true' RETURN

  montar_fixture "$base"
  printf '\n== vetor positivo ==\n'
  if ( FALHAS=0; fase_estrutural "$base" >/dev/null 2>&1 ); then
    aprovar "a fixture completa passa"
  else
    falhar "a fixture completa deveria passar"
    status=1
  fi

  local -a sabotagens=(
    "remover:cmd/main.go:binário"
    "remover:appkit:appkit"
    "remover:distkit:distkit"
    "remover-sub:app/http:borda em subpacote"
    "solto:raiz com código de bloco"
    "target:serve-api"
    "unidade:appkit"
  )
  for sabotagem in "${sabotagens[@]}"; do
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    local tipo alvo
    tipo="${sabotagem%%:*}"
    alvo="${sabotagem#*:}"
    case "$tipo" in
      remover | remover-sub) trash "$tmp/probe/${alvo%%:*}" ;;
      solto) : > "$tmp/probe/wiring.go" ;;
      target) printf '{"targets":{"serve-relay":{},"test-distributed":{}}}\n' > "$tmp/probe/project.json" ;;
      unidade) printf '{"units":[{"id":"probe/app"},{"id":"probe/distkit"}]}\n' > "$tmp/probe/dmpf-units.json" ;;
    esac

    if ( FALHAS=0; fase_estrutural "$tmp" >/dev/null 2>&1 ); then
      falhar "a sabotagem '$tipo ${alvo}' passou pelo gate"
      status=1
    else
      aprovar "reprovou: $tipo ${alvo}"
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done

  if [ "$status" -ne 0 ]; then
    printf '\nAutoteste do gate: REPROVADO.\n' >&2
    return 1
  fi
  printf '\nAutoteste do gate: OK — cada vetor negativo foi recusado.\n'
  return 0
}

uso() {
  cat <<'TXT'
uso: tools/dmpf-context-check.sh [--phase structural|self-test]

  structural  (default) verifica todo bounded context de apps/backend
  self-test   prova o gate contra fixture sintética, um vetor por sabotagem
TXT
}

FASE="structural"
while [ $# -gt 0 ]; do
  case "$1" in
    --phase) shift; FASE="${1:-}"; [ -n "$FASE" ] || { uso >&2; exit 2; } ;;
    --phase=*) FASE="${1#--phase=}" ;;
    -h|--help) uso; exit 0 ;;
    *) printf 'argumento desconhecido: %s\n' "$1" >&2; uso >&2; exit 2 ;;
  esac
  shift
done

case "$FASE" in
  structural) fase_estrutural ;;
  self-test) fase_self_test ;;
  *) printf 'fase desconhecida: %s\n' "$FASE" >&2; uso >&2; exit 2 ;;
esac
