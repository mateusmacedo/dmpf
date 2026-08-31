#!/usr/bin/env bash
# Prova que o gate de dependência do bloco `domain` reprova o que deve reprovar.
#
# Descobre os módulos pelo `dmpf-units.json` (a classificação autoritativa da
# RFC), e não por caminho fixo: quando o segundo módulo `domain` nascer, ele
# entra aqui sozinho. Sem isso o gate seguiria verde sem nunca ter sido exercido
# no módulo novo.
#
# Os fixtures .go são artefatos de runtime deste script: nascem e morrem dentro
# de uma execução. Para retirá-los do módulo o script os MOVE para um diretório
# temporário, em vez de apagá-los — assim não depende de utilitário de lixeira,
# que o runner de CI não tem. O diretório fica em /tmp, que o runner descarta ao
# fim do job e a máquina local limpa no boot.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 1; }
cd "$ROOT" || exit 1
DESCARTE="$(mktemp -d)" || { echo "falha ao criar diretorio temporario" >&2; exit 1; }

FIXTURE=""
retirar_fixture() {
  local status=$?
  if [ -n "$FIXTURE" ] && [ -e "$FIXTURE" ]; then
    mv -f "$FIXTURE" "$DESCARTE/descartado-$(date +%s%N).go" \
      || echo "AVISO: nao consegui retirar $FIXTURE do modulo" >&2
  fi
  FIXTURE=""
  return $status
}
trap retirar_fixture EXIT INT TERM

# pacote|capability|origem. `dentro` = listado na `deny` do .golangci.yml, prova
# a mensagem por capability. `fora` = ausente da `deny`, prova que a regra é
# fechada (`list-mode: strict`) e não uma mera denylist — sem estes dois, um
# `list-mode` removido por engano passaria despercebido.
VETORES=(
  "net/http|io.network|dentro"
  "os|io.filesystem|dentro"
  "encoding/json|wire.codec|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
  "encoding/xml|wire.codec|fora"
)

falhas=0
modulos=0

while IFS= read -r manifesto; do
  module_dir="$(dirname "$manifesto")"

  # só blocos `domain`; a política do .golangci.yml é específica deste bloco
  node -e '
    const m = require(process.argv[1]);
    process.exit((m.units ?? []).some((u) => u.block === "domain") ? 0 : 1);
  ' "$ROOT/$manifesto" 2>/dev/null || continue

  project="$(node -p 'require(process.argv[1]).name' "$ROOT/$module_dir/project.json" 2>/dev/null)"
  if [ -z "$project" ]; then
    echo "FALHA  $module_dir: nao consegui ler o nome do projeto no project.json"
    falhas=$((falhas + 1))
    continue
  fi

  # a cláusula de package vem do próprio módulo; fixá-la aqui quebraria o
  # fixture em qualquer módulo com outro nome de package
  pkg_clause="$(awk '/^package /{print; exit}' "$module_dir"/*.go 2>/dev/null)"
  if [ -z "$pkg_clause" ]; then
    echo "FALHA  $module_dir: nenhum .go com clausula de package"
    falhas=$((falhas + 1))
    continue
  fi

  modulos=$((modulos + 1))
  echo "== $project ($module_dir)"

  for vetor in "${VETORES[@]}"; do
    pkg="${vetor%%|*}"
    resto="${vetor#*|}"
    cap="${resto%%|*}"
    origem="${resto##*|}"

    FIXTURE="$(mktemp "$module_dir/zz_gate_XXXXXX.go")" || {
      echo "FALHA  $project: nao consegui criar o fixture"
      falhas=$((falhas + 1))
      break
    }
    printf '%s\n\nimport _ "%s"\n' "$pkg_clause" "$pkg" > "$FIXTURE"

    saida="$(pnpm nx run "$project":lint --skip-nx-cache 2>&1)"
    status=$?
    retirar_fixture

    if [ "$status" -eq 0 ]; then
      echo "  FALHA  $pkg ($cap, $origem): o lint passou, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -qi "depguard" <<<"$saida"; then
      echo "  FALHA  $pkg ($cap, $origem): reprovou por outro motivo que nao o depguard"
      falhas=$((falhas + 1))
    else
      echo "  ok     $pkg ($cap, $origem)"
    fi
  done

  # Vetor positivo: a árvore limpa precisa passar, senão o gate só sabe dizer não.
  if pnpm nx run "$project":lint --skip-nx-cache >/dev/null 2>&1; then
    echo "  ok     arvore limpa: aprovada"
  else
    echo "  FALHA  arvore limpa: reprovada, mas deveria passar"
    falhas=$((falhas + 1))
  fi
done < <(git ls-files '*dmpf-units.json')

if [ "$modulos" -eq 0 ]; then
  echo "FALHA: nenhum modulo com bloco domain encontrado; o gate nao exercitou nada." >&2
  exit 1
fi

if [ "$falhas" -gt 0 ]; then
  echo
  echo "$falhas verificacao(oes) falharam: o gate nao esta protegendo o bloco domain."
  exit 1
fi

echo
echo "Gate do bloco domain: $modulos modulo(s), ${#VETORES[@]} vetores negativos e 1 positivo cada, todos conformes."
