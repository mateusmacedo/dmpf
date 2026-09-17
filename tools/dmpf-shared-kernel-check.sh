#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova e
# por que a fixture não entra na árvore de trabalho, no mesmo molde de
# tools/dmpf-cell-check.sh.
#
# Prova que o verificador decide shared kernel pela RFC §7.2 estendida
# (ADR-042): C2 aceita destino designado, DMPF-M004 cobre chave não resolvida
# no baseline, e o mecanismo mínimo de RFC §10.2 (DMPF-T002) reprova quando a
# designação muda no mesmo commit que código. Nem dmpf-gate-check.sh (decide
# por nome de diretório, sem aresta) nem dmpf-cell-check.sh (fixa blocos e
# bounded context, sem shared kernel) nem o `conformance --root .` do CI
# (sem aresta de outro contexto para o kernel enquanto o consumidor não existe,
# e a designação muda raramente) exercitam este caminho.
#
# Dois módulos sintéticos entram num worktree descartável, nunca na árvore de
# trabalho: o verificador inventaria por `git ls-files` (exige go.mod
# rastreado) mas lê imports por `go list` (exige o módulo em go.work). Cada
# cenário abre e descarta o próprio worktree — como cada um precisa de um
# histórico git próprio (o commit que mistura normativo com código é o próprio
# objeto de teste do quarto cenário), reaproveitar um único worktree entre
# cenários journalizaria o commit de um no histórico avaliado pelos outros.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositorio git" >&2; exit 2; }
cd "$ROOT" || exit 2

VERIFICADOR="./tools/dmpf-conformance/cmd/conformance"
BASE_IMPORT="github.com/mateusmacedo/dmpf/libs/backend/go"

PROBE_X_DIR="libs/backend/go/probe-x"
PROBE_Y_DIR="libs/backend/go/probe-y"
PROBE_X_PKG="$BASE_IMPORT/probe-x"
PROBE_Y_PKG="$BASE_IMPORT/probe-y"

# kernel/domain e kernel/testkit-domain, os dois pela chave real dos manifestos
# de libs/backend/go/domain e libs/backend/go/testkit: o primeiro é a unidade
# designada nos vetores, o segundo a não designada do mesmo bounded context.
DESIGNADA="$BASE_IMPORT/domain"
NAO_DESIGNADA="$BASE_IMPORT/testkit/domainkit"

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

# Setup quebrado nunca vira "reprovou" nem "conforme": aborta o script inteiro
# com exit 2, para não confundir defeito no fixture com o verificador
# decidindo (correta ou incorretamente) sobre uma árvore válida.
falha_setup() {
  echo "FALHA DE SETUP: $*" >&2
  exit 2
}

abrir_worktree() {
  local destino
  destino="$(mktemp -d)" || falha_setup "mktemp -d nao funcionou"
  if ! git worktree add --detach "$destino" HEAD >/dev/null 2>&1; then
    falha_setup "nao consegui abrir o worktree em $destino"
  fi
  WORKTREE="$destino"
}

commitar() { # mensagem arquivo...
  local msg="$1"
  shift
  git -C "$WORKTREE" add -- "$@" || falha_setup "git add falhou para: $*"
  git -C "$WORKTREE" -c user.name=dmpf-shared-kernel-gate -c user.email=dmpf-shared-kernel-gate@localhost \
    commit -q -m "$msg" || falha_setup "git commit falhou: $msg"
}

escrever_probe() { # dir modulo pacote
  local dir="$1" modulo="$2" pacote="$3"
  mkdir -p "$WORKTREE/$dir" || falha_setup "mkdir $dir"
  printf 'module %s\n\ngo 1.26.6\n' "$modulo" > "$WORKTREE/$dir/go.mod" \
    || falha_setup "escrever $dir/go.mod"
  printf 'package %s\n\nconst Marker = %q\n' "$pacote" "$pacote" > "$WORKTREE/$dir/probe.go" \
    || falha_setup "escrever $dir/probe.go"
}

escrever_manifesto() { # dir id contexto include
  local dir="$1" id="$2" contexto="$3" include="$4"
  cat > "$WORKTREE/$dir/dmpf-units.json" <<JSON || falha_setup "escrever $dir/dmpf-units.json"
{
  "schema": "dmpf/units@1",
  "units": [
    {
      "id": "$id",
      "block": "domain",
      "bounded_context": "$contexto",
      "public_integration_surface": false,
      "include": [
        "$include"
      ]
    }
  ],
  "external": [],
  "exceptions": []
}
JSON
}

regravar_baseline() {
  go run "$VERIFICADOR" --root "$WORKTREE" --write-baseline >/dev/null 2>&1 \
    || falha_setup "--write-baseline falhou"
}

# Regravar preserva shared_kernel_units apenas se a chave já existe no arquivo
# (HasSharedKernelUnits): por isso a designação exige regravar, editar com jq e
# regravar de novo — editar o JSON à mão sem o segundo passe deixa o digest
# sem fechar e todo vetor positivo cairia em DMPF-T001.
designar_shared_kernel() { # lista-json, ex: ["kernel/domain"]
  local baseline="$WORKTREE/tools/dmpf-baseline/units-baseline.json" lista="$1"
  regravar_baseline
  jq --argjson lista "$lista" '.shared_kernel_units += ($lista - (.shared_kernel_units // []))' "$baseline" > "$baseline.tmp" \
    || falha_setup "jq nao conseguiu editar shared_kernel_units"
  mv "$baseline.tmp" "$baseline" || falha_setup "mv do baseline editado"
  jq -e --argjson lista "$lista" '($lista - .shared_kernel_units) == []' "$baseline" >/dev/null \
    || falha_setup "jq nao encontrou shared_kernel_units apos a edicao"
  regravar_baseline
}

# Monta o worktree comum aos quatro cenários: dois módulos sintéticos com
# go.mod e fonte de produção (commit 1, sem manifesto), depois manifesto e
# baseline designando kernel/domain como shared kernel (commit 2).
# BASE_REF é capturado ANTES do commit 1: CommitsQueTocaram(base) precisa
# enxergar os commits do próprio worktree, o que NX_BASE=develop não faria
# aqui, porque os commits nunca existiram em develop.
preparar_ambiente() {
  abrir_worktree
  BASE_REF="$(git -C "$WORKTREE" rev-parse HEAD)" || falha_setup "rev-parse HEAD no worktree"

  escrever_probe "$PROBE_X_DIR" "$PROBE_X_PKG" probex
  escrever_probe "$PROBE_Y_DIR" "$PROBE_Y_PKG" probey
  ( cd "$WORKTREE" && go work use "./$PROBE_X_DIR" "./$PROBE_Y_DIR" ) \
    || falha_setup "go work use falhou"
  commitar "chore(probe): scaffold synthetic modules for shared kernel gate" \
    go.work "$PROBE_X_DIR" "$PROBE_Y_DIR"

  escrever_manifesto "$PROBE_X_DIR" "probe-x/domain" gateprobe "$PROBE_X_PKG"
  escrever_manifesto "$PROBE_Y_DIR" "probe-y/domain" gateprobe-y "$PROBE_Y_PKG"
  designar_shared_kernel '["kernel/domain"]'
  commitar "docs(probe): declare manifest and designate shared kernel" \
    "$PROBE_X_DIR/dmpf-units.json" "$PROBE_Y_DIR/dmpf-units.json" tools/dmpf-baseline/units-baseline.json
}

verificar() {
  go run "$VERIFICADOR" --root "$WORKTREE" --base "$BASE_REF" 2>&1
}

# Conjunto exato de códigos, não grep de um único: exit 1 sai igual para
# DMPF-D002, DMPF-M004, DMPF-T001 e DMPF-T002, e um vetor que espera um deles
# passaria por acidente se o outro aparecesse no lugar.
codigos() {
  grep -oE 'DMPF-[A-Z][0-9]{3}' <<<"$1" | sort -u
}

falhas=0
exercitados=0
TOTAL_CENARIOS=4

VETORES_D002=(
  # nome|import-alvo|exit-esperado|codigos-esperados(sep. por espaço, vazio = nenhum)|par chave->alvo (vazio = não conferir)
  "designada (shared kernel)|$DESIGNADA|0||"
  "nao-designada (mesmo bounded context kernel)|$NAO_DESIGNADA|1|DMPF-D002|$PROBE_X_PKG -> $NAO_DESIGNADA"
  "outro bounded context (probe-y)|$PROBE_Y_PKG|1|DMPF-D002|$PROBE_X_PKG -> $PROBE_Y_PKG"
)

for vetor in "${VETORES_D002[@]}"; do
  IFS='|' read -r nome alvo exit_esperado esperados par <<<"$vetor"

  preparar_ambiente
  printf 'package probex\n\nimport _ "%s"\n' "$alvo" > "$WORKTREE/$PROBE_X_DIR/vetor.go" \
    || falha_setup "escrever vetor.go para $nome"
  commitar "test(probe): import $alvo from probe-x" "$PROBE_X_DIR/vetor.go"

  saida="$(verificar)"
  status=$?
  exercitados=$((exercitados + 1))
  obtidos="$(codigos "$saida")"
  esperados_ordenados="$(tr ' ' '\n' <<<"$esperados" | sed '/^$/d' | sort -u)"

  if [ "$status" -ne "$exit_esperado" ]; then
    echo "  FALHA  $nome: exit $status, esperado $exit_esperado"
    falhas=$((falhas + 1))
  elif [ "$obtidos" != "$esperados_ordenados" ]; then
    echo "  FALHA  $nome: codigos [$obtidos], esperado [$esperados_ordenados]"
    falhas=$((falhas + 1))
  elif [ -n "$par" ] && ! grep -qF "$par" <<<"$saida"; then
    echo "  FALHA  $nome: DMPF-D002 emitido sem o par chave->alvo esperado ($par)"
    falhas=$((falhas + 1))
  else
    echo "  ok     $nome: exit $status, codigos [$obtidos]"
  fi

  descartar_worktree
done

# Sexto ato: reprova o commit que muda shared_kernel_units de uma unidade já
# existente na MESMA mensagem que toca código. É a asserção mais frágil do
# gate — se DMPF-T002 não disparar aqui, é sinal de buraco na tarefa 1.5, não
# motivo para afrouxar a checagem até o script ficar verde.
preparar_ambiente
designar_shared_kernel '["kernel/domain", "kernel/testkit-domain"]'
printf '\nconst MarkerV2 = "probe-x-v2"\n' >> "$WORKTREE/$PROBE_X_DIR/probe.go" \
  || falha_setup "editar probe.go do sexto ato"
commitar "refactor(probe): rotate shared kernel designation and touch code" \
  tools/dmpf-baseline/units-baseline.json "$PROBE_X_DIR/probe.go"

saida="$(verificar)"
status=$?
exercitados=$((exercitados + 1))
obtidos="$(codigos "$saida")"

if [ "$status" -ne 1 ]; then
  echo "  FALHA  sexto ato: exit $status, esperado 1"
  falhas=$((falhas + 1))
elif ! grep -qx "DMPF-T002" <<<"$obtidos"; then
  echo "  FALHA  sexto ato: DMPF-T002 nao apareceu (codigos obtidos: [$obtidos]) -- possivel buraco na tarefa 1.5"
  falhas=$((falhas + 1))
else
  echo "  ok     sexto ato: exit $status, DMPF-T002 presente (codigos [$obtidos])"
fi

descartar_worktree

if [ "$exercitados" -ne "$TOTAL_CENARIOS" ]; then
  echo
  echo "FALHA: $exercitados de $TOTAL_CENARIOS cenario(s) exercitado(s); o gate nao cobriu o que promete." >&2
  exit 1
fi

if [ "$falhas" -gt 0 ]; then
  echo
  echo "$falhas verificacao(oes) falharam: o verificador nao esta decidindo shared kernel corretamente."
  exit 1
fi

echo
echo "Gate de shared kernel: $TOTAL_CENARIOS cenario(s) conformes."
