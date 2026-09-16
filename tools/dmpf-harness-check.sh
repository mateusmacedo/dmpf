#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova, por que roda em worktree e como o agente é acionado, no mesmo molde de tools/dmpf-generator-check.sh.
# Prova de regressão do harness de bounded contexts: o golden `bookings`,
# regenerado a partir da mesma spec pelo mesmo harness, passa nos mesmos gates
# que o golden commitado passou — cadeia Go, rito Buf, classificação em commit
# próprio e verificador de conformidade. A garantia é pelos gates, não por bytes
# idênticos (ADR-041): dois runs do agente podem divergir em forma.
#
# Roda num worktree descartável em HEAD, nunca na árvore de trabalho: a fase
# `regen` remove o golden, aciona o agente e regrava o baseline, e nada disso
# pode vazar para o repositório de quem roda a prova. Não roda no CI (exige
# LLM, credenciais e tempo); o CI prova o golden como módulos normais.
#
# DMPF_HARNESS_CHECK_AGENT_CMD é o comando que regenera o contexto dentro do
# worktree (default: `claude -p --dangerously-skip-permissions "/dmpf-new-context
# <SPEC>"`). Qualquer agente serve, desde que escreva no cwd. `true` liga o modo
# manual: a prova imprime o worktree, espera Enter e julga o que estiver lá.
#
# DMPF_PG_DSN, quando definido, acrescenta o test-race das suítes Postgres,
# com --parallel=1 (os harnesses truncam tabelas do kernel).
#
# A fase `self-test` não usa LLM: sobre o golden commitado, retira
# kernel/domain de shared_kernel_units e exige que o verificador reprove
# com DMPF-D002 apontando bookings/domain — o cenário 4 da spec do harness.
set -uo pipefail

ORIGEM="$(readlink -f "${BASH_SOURCE[0]}")" \
  || { echo "não consegui resolver o caminho do próprio script" >&2; exit 2; }

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

SPEC=SPEC-AHPRBZCT # ephemeral-ref-ok: a prova regenera o golden a partir da sua spec, por natureza
NOME=bookings
CONTEXTO=resource-scheduling
# Um módulo por contexto e um package por bloco (ADR-045): o golden é o
# projeto Nx `bookings`, e cada bloco é um subdiretório do módulo.
MODULO="libs/backend/go/$NOME"
BLOCOS=(domain ports application provider app)
PROJETOS=("$NOME")
PROJETOS_POSTGRES=("$NOME")
CAMINHOS_GOLDEN=(
  "contracts/proto/company/$NOME"
  "contracts/openapi/$NOME"
  "libs/backend/go/contracts/gen/go/company/$NOME"
)
MANIFESTO_CONTRATOS=libs/backend/go/contracts/dmpf-units.json
BASELINE=tools/dmpf-baseline/units-baseline.json
UNIDADE_CONTRATO="$CONTEXTO/contract"
UNIDADE_SABOTADA=kernel/domain

AGENT_CMD="${DMPF_HARNESS_CHECK_AGENT_CMD:-claude -p --dangerously-skip-permissions \"/dmpf-new-context $SPEC — prova em worktree: não rode pnpm install\"}"

export NX_DAEMON=false
export NX_NO_CLOUD=true
# Ver tools/dmpf-generator-check.sh: sem isto o pnpm reinstala por dentro do
# symlink do worktree e reescreve o node_modules da raiz.
export pnpm_config_verify_deps_before_run=false

WT=""
HEAD0=""
NM_ANCORA=""
ARQUIVOS_GERADOS=()
ARQUIVOS_FORMATO=()
ARQUIVOS_GO=()

uso() {
  cat <<'FIM'
uso: tools/dmpf-harness-check.sh --phase regen|self-test

  regen      remove o golden bookings num worktree, aciona o agente sobre a mesma
             spec, roda o rito Buf, classifica em commit próprio, roda a cadeia
             Go e o verificador, e reporta divergências contra o golden
  self-test  sem LLM: sabota shared_kernel_units sobre o golden commitado e
             exige DMPF-D002 em bookings/domain
FIM
}

passo() { printf '\n== %s ==\n' "$1"; }
ok() { printf '  ok     %s\n' "$1"; }
falha() { printf 'FALHA: %s\n' "$1" >&2; exit 1; }

descartar() {
  local status=$?
  cd "$ROOT" 2>/dev/null || true
  if [ -n "$WT" ] && [ -d "$WT" ]; then
    git worktree remove --force "$WT" >/dev/null 2>&1 \
      || echo "AVISO: não consegui remover o worktree $WT" >&2
    git worktree prune >/dev/null 2>&1
  fi
  WT=""
  return $status
}
trap descartar EXIT INT TERM

git_gate() {
  git -C "$WT" \
    -c user.name=dmpf-harness-check \
    -c user.email=dmpf-harness-check@dmpf.local "$@"
}

assinar_node_modules() {
  {
    find "$ROOT/node_modules" -mindepth 1 -maxdepth 1 -printf '%P\t%y\t%l\n'
    find "$ROOT/node_modules" -mindepth 2 -maxdepth 2 -path "$ROOT/node_modules/@*" \
      -printf '%P\t%y\t%l\n'
  } 2>/dev/null | LC_ALL=C sort
}

ancorar_node_modules() {
  local destino
  [ -d "$ROOT/node_modules" ] \
    || falha "node_modules ausente na raiz; rode 'pnpm install' antes da prova"
  destino="$(readlink -m "$ROOT/node_modules/nx")"
  case "$destino" in
    "$ROOT"/*) ;;
    *) falha "node_modules/nx da raiz resolve para fora de $ROOT ($destino); rode 'pnpm install' na raiz antes da prova" ;;
  esac
  NM_ANCORA="$(mktemp)" || falha "não consegui criar o arquivo da âncora de node_modules"
  assinar_node_modules >"$NM_ANCORA" || falha "não consegui assinar o node_modules da raiz"
  [ -s "$NM_ANCORA" ] || falha "a assinatura do node_modules da raiz saiu vazia"
}

conferir_node_modules() {
  local divergencia
  divergencia="$(diff "$NM_ANCORA" <(assinar_node_modules) | head -12)"
  [ -z "$divergencia" ] || falha "o node_modules da raiz foi reescrito durante a prova: algo rodou 'pnpm install' por dentro do symlink do worktree; rode 'pnpm install' na raiz para recuperar. Primeiras divergências:
$divergencia"
}

abrir_worktree() {
  WT="$(mktemp -d)" || falha "não consegui criar o diretório do worktree"
  git worktree add --detach "$WT" HEAD >/dev/null 2>&1 \
    || falha "não consegui abrir o worktree em $WT"
  ln -s "$ROOT/node_modules" "$WT/node_modules" \
    || falha "não consegui ligar node_modules no worktree"
  if [ -d "$ROOT/tools/dmpf-plugin/node_modules" ] && [ ! -e "$WT/tools/dmpf-plugin/node_modules" ]; then
    ln -s "$ROOT/tools/dmpf-plugin/node_modules" "$WT/tools/dmpf-plugin/node_modules" \
      || falha "não consegui ligar tools/dmpf-plugin/node_modules no worktree"
  fi
  HEAD0="$(git -C "$WT" rev-parse HEAD)" || falha "não consegui ler o HEAD do worktree"
}

conferir_golden_presente() {
  local bloco
  [ -f "$ROOT/$MODULO/go.mod" ] \
    || falha "o golden não está no HEAD: $MODULO/go.mod ausente — a prova compara contra ele"
  for bloco in "${BLOCOS[@]}"; do
    [ -d "$ROOT/$MODULO/$bloco" ] \
      || falha "o golden não está no HEAD: $MODULO/$bloco ausente — a prova compara contra ele"
  done
  jq -e --arg u "$UNIDADE_CONTRATO" '.units[] | select(.id == $u)' "$ROOT/$MANIFESTO_CONTRATOS" >/dev/null \
    || falha "a unidade $UNIDADE_CONTRATO não está em $MANIFESTO_CONTRATOS"
  ok "golden presente: $MODULO com ${BLOCOS[*]} e a unidade $UNIDADE_CONTRATO"
}

# O estado "sem golden" é o ponto de partida do agente e o --base do
# verificador. A unidade `contract` sai do manifesto junto com o gen/go, senão
# o --write-baseline vê unidade sem package.
remover_golden() {
  local caminho tmp
  git -C "$WT" rm -rq -- "$MODULO" || falha "não consegui remover $MODULO do worktree"
  sed -i "\#^\t./$MODULO\$#d" "$WT/go.work" || falha "não consegui editar o go.work"
  go run ./libs/backend/go/conformance/cmd/modsync --root "$WT" --write >/dev/null 2>&1 \
    || falha "o modsync não conseguiu retirar $NOME do replace do go.work"
  for caminho in "${CAMINHOS_GOLDEN[@]}"; do
    [ -e "$WT/$caminho" ] || continue
    git -C "$WT" rm -rq -- "$caminho" || falha "não consegui remover $caminho do worktree"
  done
  ! grep -q "$NOME" "$WT/go.work" || falha "go.work ainda cita $NOME depois da remoção"

  tmp="$(mktemp)" || falha "não consegui criar arquivo temporário"
  jq --arg u "$UNIDADE_CONTRATO" '.units |= map(select(.id != $u))' "$WT/$MANIFESTO_CONTRATOS" >"$tmp" \
    && mv "$tmp" "$WT/$MANIFESTO_CONTRATOS" || falha "não consegui retirar $UNIDADE_CONTRATO do manifesto"
  (cd "$WT" && pnpm biome format --write "$MANIFESTO_CONTRATOS" >/dev/null 2>&1) || true

  (cd "$WT" && go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline >/dev/null 2>&1) \
    || falha "--write-baseline reprovou ao classificar o estado sem o golden"
  git -C "$WT" add -A >/dev/null || falha "git add do estado sem o golden"
  git_gate commit -q -m "chore(workspace): estado sem o golden bookings (efêmero)" \
    || falha "o commit do estado sem o golden reprovou (os hooks estão ativos)"
  HEAD0="$(git -C "$WT" rev-parse HEAD)"
  ok "golden removido; HEAD0=$HEAD0 é o estado que o agente recebe"
}

acionar_agente() {
  local status
  if [ "$AGENT_CMD" = "true" ]; then
    printf '\nModo manual. Gere o contexto %s a partir de %s dentro de:\n  %s\n(sem pnpm install — o node_modules é o da raiz). Pressione Enter ao terminar.\n' \
      "$NOME" "$SPEC" "$WT"
    read -r _ </dev/tty || falha "sem terminal para o modo manual"
    return 0
  fi
  printf '  agente: %s\n' "$AGENT_CMD"
  (cd "$WT" && bash -c "$AGENT_CMD")
  status=$?
  [ "$status" -eq 0 ] || falha "o comando do agente saiu com $status"
  ok "agente concluiu"
}

coletar_gerados() {
  local entrada caminho modulo
  ARQUIVOS_GERADOS=(); ARQUIVOS_FORMATO=(); ARQUIVOS_GO=()
  while IFS= read -r -d '' entrada; do
    caminho="${entrada:3}"
    [ -n "$caminho" ] || continue
    ARQUIVOS_GERADOS+=("$caminho")
    case "$caminho" in
      *.json | *.jsonc | *.ts | *.tsx | *.js | *.mjs | *.cjs | *.css) ARQUIVOS_FORMATO+=("$caminho") ;;
      *.go) ARQUIVOS_GO+=("$caminho") ;;
    esac
  done < <(git -C "$WT" status --porcelain -z --untracked-files=all)
  [ "${#ARQUIVOS_GERADOS[@]}" -gt 0 ] || falha "o agente não deixou nenhum arquivo no worktree"
  [ -f "$WT/$MODULO/go.mod" ] || falha "o agente não produziu o módulo $MODULO"
  for modulo in "${BLOCOS[@]}"; do
    [ -d "$WT/$MODULO/$modulo" ] || falha "o agente não produziu $MODULO/$modulo"
  done
  grep -q "$NOME" "$WT/go.work" || falha "go.work não registra o módulo de $NOME: o esqueleto não passou pelo generator"
  [ -d "$WT/contracts/proto/company/$NOME" ] || falha "o agente não escreveu contracts/proto/company/$NOME"
  ok "${#ARQUIVOS_GERADOS[@]} arquivo(s) tocado(s); um módulo com cinco blocos, go.work e .proto presentes"
}

checar_sem_escrita() {
  local saida status
  if [ "${#ARQUIVOS_FORMATO[@]}" -gt 0 ]; then
    saida="$(cd "$WT" && pnpm biome ci --no-errors-on-unmatched "${ARQUIVOS_FORMATO[@]}" 2>&1)"
    status=$?
    [ "$status" -eq 0 ] || { printf '%s\n' "$saida" >&2; falha "biome ci reprovou arquivo regenerado (acima)"; }
  fi
  if [ "${#ARQUIVOS_GO[@]}" -gt 0 ]; then
    saida="$(cd "$WT" && gofmt -l "${ARQUIVOS_GO[@]}" 2>&1)"
    status=$?
    [ "$status" -eq 0 ] || { printf '%s\n' "$saida" >&2; falha "gofmt não conseguiu ler arquivo regenerado (exit $status)"; }
    [ -z "$saida" ] || falha "gofmt -l apontou arquivo regenerado fora do formato: $(printf '%s' "$saida" | tr '\n' ' ')"
  fi
  ok "biome ci e gofmt -l sem apontamento"
}

rito_buf() {
  jq -e --arg u "$UNIDADE_CONTRATO" '.units[] | select(.id == $u)' "$WT/$MANIFESTO_CONTRATOS" >/dev/null \
    || falha "o agente não declarou a unidade $UNIDADE_CONTRATO em $MANIFESTO_CONTRATOS (sem ela o gen/go cai em DMPF-U001)"
  (cd "$WT/contracts" && bash ../tools/buf.sh generate) || falha "buf generate reprovou"
  (cd "$WT" && pnpm nx run contracts:buf-lint && pnpm nx run contracts:buf-pins \
    && pnpm nx run contracts:buf-generate-check) || falha "um gate Buf reprovou"
  [ -d "$WT/libs/backend/go/contracts/gen/go/company/$NOME" ] \
    || falha "o rito Buf não produziu gen/go/company/$NOME"
  ok "rito Buf: generate, buf-lint, buf-pins, buf-generate-check"
}

commitar_classificacao() {
  local saida status
  git -C "$WT" add -- $(git -C "$WT" ls-files -o -m --exclude-standard -- '*/dmpf-units.json' "$MANIFESTO_CONTRATOS") \
    || falha "git add dos manifestos de unidade"
  saida="$(cd "$WT" && go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline 2>&1)"
  status=$?
  [ "$status" -eq 0 ] || { printf '%s\n' "$saida" >&2; falha "--write-baseline reprovou (exit $status)"; }
  git -C "$WT" add -- "$BASELINE" || falha "git add do baseline"
  git_gate commit -q -m "chore(workspace): classificar o golden regenerado" \
    || falha "o commit da classificação reprovou (os hooks estão ativos)"
  ok "commit 1: manifestos + baseline (DMPF-T002)"
}

commitar_codigo() {
  git -C "$WT" add -A || falha "git add do código regenerado"
  git_gate commit -q -m "feat(workspace): golden bookings regenerado pelo harness" \
    || falha "o commit do código reprovou (os hooks estão ativos)"
  ok "commit 2: código, contrato, go.work"
}

cadeia_nx() {
  local lista
  lista="$(IFS=,; printf '%s' "${PROJETOS[*]}")"
  (cd "$WT" && pnpm nx run-many -t fmt-check,vet,build,lint --projects="$lista" --parallel=3) \
    || falha "a cadeia fmt-check,vet,build,lint reprovou em $lista"
  ok "cadeia fmt-check,vet,build,lint aprovada em $lista"
  if [ -n "${DMPF_PG_DSN:-}" ]; then
    lista="$(IFS=,; printf '%s' "${PROJETOS_POSTGRES[*]}")"
    (cd "$WT" && pnpm nx run-many -t test-race --projects="$lista" --parallel=1) \
      || falha "test-race reprovou em $lista"
    ok "test-race aprovado em $lista (--parallel=1)"
  else
    printf '  aviso  DMPF_PG_DSN ausente: test-race das suítes Postgres não rodou\n'
  fi
}

verificar_conformidade() {
  local saida status
  saida="$(go run ./libs/backend/go/conformance/cmd/conformance --root "$WT" --base "$HEAD0" 2>&1)"
  status=$?
  printf '%s\n' "$saida"
  [ "$status" -eq 0 ] || falha "o verificador de conformidade reprovou o contexto regenerado (exit $status)"
  ok "verificador de conformidade aprovado com --base $HEAD0"
}

conferir_arvore_limpa() {
  local sobras
  git -C "$WT" diff --exit-code >/dev/null \
    || falha "os checks mudaram arquivo versionado: $(git -C "$WT" diff --name-only | tr '\n' ' ')"
  sobras="$(git -C "$WT" status --porcelain)"
  [ -z "$sobras" ] || { printf '%s\n' "$sobras" >&2; falha "sobrou arquivo não versionado depois dos checks (acima)"; }
  ok "git diff vazio e git status --porcelain vazio"
}

# Informativo: a garantia é pelos gates; a lista mostra onde o agente divergiu
# em forma do golden commitado, para quem for ler.
relatar_divergencia() {
  local so_golden so_regen
  so_golden="$(comm -23 \
    <(git -C "$ROOT" ls-tree -r --name-only HEAD -- "libs/backend/go/$NOME" "${CAMINHOS_GOLDEN[@]}" 2>/dev/null | sort) \
    <(git -C "$WT" ls-tree -r --name-only HEAD -- "libs/backend/go/$NOME" "${CAMINHOS_GOLDEN[@]}" 2>/dev/null | sort))"
  so_regen="$(comm -13 \
    <(git -C "$ROOT" ls-tree -r --name-only HEAD -- "libs/backend/go/$NOME" "${CAMINHOS_GOLDEN[@]}" 2>/dev/null | sort) \
    <(git -C "$WT" ls-tree -r --name-only HEAD -- "libs/backend/go/$NOME" "${CAMINHOS_GOLDEN[@]}" 2>/dev/null | sort))"
  printf '\nDivergência de forma contra o golden commitado (informativo):\n'
  printf '  só no golden:      %s\n' "${so_golden:-nenhum}"
  printf '  só no regenerado:  %s\n' "${so_regen:-nenhum}"
}

fase_regen() {
  ancorar_node_modules
  conferir_golden_presente

  passo "worktree descartável em HEAD"
  abrir_worktree
  conferir_node_modules
  ok "worktree em $WT"

  passo "estado sem o golden"
  remover_golden

  passo "agente regenera $NOME a partir de $SPEC"
  acionar_agente
  conferir_node_modules
  coletar_gerados

  passo "checagem sem escrita"
  checar_sem_escrita

  passo "rito Buf"
  rito_buf
  conferir_node_modules

  passo "commit 1 — classificação"
  commitar_classificacao

  passo "commit 2 — código"
  commitar_codigo

  passo "cadeia Go dos módulos regenerados"
  cadeia_nx
  conferir_node_modules

  passo "verificador de conformidade"
  verificar_conformidade

  passo "nada sobrou"
  conferir_arvore_limpa
  relatar_divergencia

  printf '\nProva do harness (regen): OK — %s regenerado e aprovado nos gates.\n' "$NOME"
}

fase_self_test() {
  local tmp saida status
  ancorar_node_modules
  conferir_golden_presente

  passo "worktree descartável em HEAD"
  abrir_worktree
  ok "worktree em $WT (HEAD0=$HEAD0)"

  passo "sabotagem: $UNIDADE_SABOTADA fora de shared_kernel_units"
  jq -e --arg u "$UNIDADE_SABOTADA" '.shared_kernel_units | index($u)' "$WT/$BASELINE" >/dev/null \
    || falha "$UNIDADE_SABOTADA não está em shared_kernel_units: o vetor mediria o baseline intacto"
  tmp="$(mktemp)" || falha "não consegui criar arquivo temporário"
  jq --arg u "$UNIDADE_SABOTADA" '.shared_kernel_units |= map(select(. != $u))' "$WT/$BASELINE" >"$tmp" \
    && mv "$tmp" "$WT/$BASELINE" || falha "não consegui sabotar o baseline"
  # Regravar preserva a lista do arquivo atual e recalcula o digest
  # (baseline.go, Regravar): sem este passo o verificador reprovaria por
  # digest, não por D002.
  (cd "$WT" && go run ./libs/backend/go/conformance/cmd/conformance --root . --write-baseline >/dev/null 2>&1) \
    || falha "--write-baseline reprovou ao fechar o digest do baseline sabotado"
  ! jq -e --arg u "$UNIDADE_SABOTADA" '.shared_kernel_units | index($u)' "$WT/$BASELINE" >/dev/null \
    || falha "o --write-baseline restaurou $UNIDADE_SABOTADA: a sabotagem não pegou"
  ok "baseline sem $UNIDADE_SABOTADA, digest fechado"

  passo "o verificador precisa reprovar com DMPF-D002 em $NOME/domain"
  saida="$(go run ./libs/backend/go/conformance/cmd/conformance --root "$WT" 2>&1)"
  status=$?
  if [ "$status" -eq 0 ]; then
    printf '%s\n' "$saida" >&2
    falha "o verificador aprovou sem o shared kernel, e devia ter reprovado (saída acima)"
  fi
  grep -qF 'DMPF-D002' <<<"$saida" \
    || { printf '%s\n' "$saida" >&2; falha "reprovou por outro motivo — esperado DMPF-D002 (saída acima)"; }
  grep -qF "$NOME/domain" <<<"$saida" \
    || { printf '%s\n' "$saida" >&2; falha "DMPF-D002 não aponta $NOME/domain (saída acima)"; }
  ok "DMPF-D002 em $NOME/domain, como esperado"

  printf '\nProva do harness (self-test): OK — o gate normativo ainda morde sem o shared kernel.\n'
}

FASE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --phase) FASE="${2:-}"; [ -n "$FASE" ] || { echo "FALHA: --phase exige um valor" >&2; uso >&2; exit 2; }; shift 2 ;;
    --phase=*) FASE="${1#--phase=}"; shift ;;
    -h | --help) uso; exit 0 ;;
    *) echo "FALHA: argumento desconhecido: $1" >&2; uso >&2; exit 2 ;;
  esac
done

case "$FASE" in
  regen) fase_regen ;;
  self-test) fase_self_test ;;
  "") echo "FALHA: --phase é obrigatório" >&2; uso >&2; exit 2 ;;
  *) echo "FALHA: fase desconhecida: $FASE" >&2; uso >&2; exit 2 ;;
esac
