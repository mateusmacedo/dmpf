#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova e por que não usa a árvore de trabalho, no mesmo molde de tools/dmpf-gate-check.sh.
# Prova que o verificador reprova as células 26 (provider → application) e 12
# (application → contract) da RFC §7.3. O `dmpf-gate-check.sh` não as alcança:
# o depguard decide por nome de diretório e não conhece aresta entre unidades.
#
# O fixture entra num worktree descartável, nunca na árvore de trabalho, e não é
# adicionado ao git: o verificador inventaria por `git ls-files` mas lê imports
# por `go list`. Motivo completo em docs/adr/035-realizacao-postgres-da-outbox.md.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 1; }
cd "$ROOT" || exit 1

BASE="gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go"

# O verificador exige a base para avaliar mudança de classificação; sem ela
# reprova como "não verificado", e o vetor positivo nunca passaria.
NX_BASE="${NX_BASE:-develop}"

WORKTREE=""
descartar_worktree() {
  local status=$?
  if [ -n "$WORKTREE" ] && [ -d "$WORKTREE" ]; then
    git worktree remove --force "$WORKTREE" >/dev/null 2>&1 \
      || echo "AVISO: nao consegui remover o worktree $WORKTREE" >&2
  fi
  WORKTREE=""
  return $status
}
trap descartar_worktree EXIT INT TERM

# célula|diretório do fixture|import proibido|package de origem|package alvo
#
# Origem e alvo são import paths de package, não a chave `módulo#unidade`: o
# D001 é diagnóstico de ARESTA, e aresta liga packages. A chave por unidade
# aparece nos diagnósticos de classificação (T001), que são outra coisa.
VETORES=(
  "26|libs/backend/go/dmpf-provider-postgres|$BASE/dmpf-application|$BASE/dmpf-provider-postgres|$BASE/dmpf-application"
  "12|libs/backend/go/dmpf-application|$BASE/dmpf-contracts/envelope|$BASE/dmpf-application|$BASE/dmpf-contracts/envelope"
)

abrir_worktree() {
  local destino
  destino="$(mktemp -d)" || return 1
  # mktemp cria o diretório; `git worktree add` exige que ele esteja vazio, que
  # é o caso, e reaproveita o caminho em vez de recusar.
  if ! git worktree add --detach "$destino" HEAD >/dev/null 2>&1; then
    echo "FALHA: nao consegui abrir o worktree em $destino" >&2
    return 1
  fi
  WORKTREE="$destino"
  return 0
}

verificar() {
  go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance \
    --root "$WORKTREE" --base "$NX_BASE" 2>&1
}

falhas=0
exercitados=0

for vetor in "${VETORES[@]}"; do
  IFS='|' read -r celula dir_fixture import_proibido chave alvo <<<"$vetor"

  abrir_worktree || { falhas=$((falhas + 1)); continue; }

  if [ ! -d "$WORKTREE/$dir_fixture" ]; then
    echo "FALHA  celula $celula: $dir_fixture nao existe no HEAD; o vetor nao pode ser exercitado"
    falhas=$((falhas + 1))
    descartar_worktree
    continue
  fi

  pkg_clause="$(awk '/^package /{print; exit}' "$WORKTREE/$dir_fixture"/*.go 2>/dev/null)"
  if [ -z "$pkg_clause" ]; then
    echo "FALHA  celula $celula: nenhum .go com clausula de package em $dir_fixture"
    falhas=$((falhas + 1))
    descartar_worktree
    continue
  fi

  fixture="$WORKTREE/$dir_fixture/zz_cell_${celula}.go"
  printf '%s\n\nimport _ "%s"\n' "$pkg_clause" "$import_proibido" > "$fixture"

  saida="$(verificar)"
  exercitados=$((exercitados + 1))

  if ! grep -q "DMPF-D001" <<<"$saida"; then
    echo "  FALHA  celula $celula: o verificador nao emitiu DMPF-D001 para $import_proibido"
    falhas=$((falhas + 1))
  elif ! grep -qF "$chave -> $alvo" <<<"$saida"; then
    echo "  FALHA  celula $celula: DMPF-D001 emitido com chave/alvo diferentes do esperado"
    echo "         esperado: $chave -> $alvo"
    falhas=$((falhas + 1))
  else
    echo "  ok     celula $celula: $chave -> $alvo reprovada com DMPF-D001"
  fi

  descartar_worktree
done

# Vetor positivo: sem ele o gate só sabe dizer não, e um verificador que
# reprovasse tudo passaria nos dois vetores acima sem proteger nada.
if abrir_worktree; then
  if verificar >/dev/null 2>&1; then
    echo "  ok     positivo: arvore limpa aprovada"
  else
    echo "  FALHA  positivo: arvore limpa reprovada, mas deveria passar"
    echo "         (o HEAD precisa ter o manifesto e o baseline ja commitados)"
    falhas=$((falhas + 1))
  fi
  descartar_worktree
else
  falhas=$((falhas + 1))
fi

if [ "$exercitados" -ne "${#VETORES[@]}" ]; then
  echo
  echo "FALHA: $exercitados de ${#VETORES[@]} vetor(es) exercitado(s); o gate nao cobriu o que promete." >&2
  exit 1
fi

if [ "$falhas" -gt 0 ]; then
  echo
  echo "$falhas verificacao(oes) falharam: o verificador nao esta decidindo as celulas 26 e 12."
  exit 1
fi

echo
echo "Gate de celula: ${#VETORES[@]} vetor(es) negativo(s) e 1 positivo. Todos conformes."
