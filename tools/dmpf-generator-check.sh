#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova, por que não usa a árvore de trabalho e quais armadilhas de ambiente ele neutraliza, no mesmo molde de tools/dmpf-cell-check.sh.
# Prova mecânica do generator `bounded-context`: o bounded context gerado passa a
# cadeia Go e o verificador de conformidade, e sai do gate byte a byte idêntico
# ao que o generator escreveu — nenhum hook, formatter ou target reescreve
# arquivo gerado pelo caminho.
#
# A prova roda num worktree descartável em HEAD, nunca na árvore de trabalho: o
# generator cria módulo novo, mexe no go.work e regrava o baseline de unidades,
# e nada disso pode vazar para o repositório de quem roda o gate.
#
# DMPF_GENERATOR_CHECK_WORKING_TREE=1 injeta a árvore de trabalho no worktree
# (diff de HEAD + cópia de tools/dmpf-plugin) e a commita antes de HEAD0. É o
# modo de desenvolvimento, para quando o plugin ainda não está commitado; sem a
# variável o gate prova apenas o HEAD, que é o que o CI vê.
#
# DMPF_GENERATOR_CHECK_BLOCKS (default `domain,port,application,provider,app`)
# restringe os blocos gerados.
#
# As quatro DMPF_GENERATOR_CHECK_SIMULAR_* sabotam um ponto da fase `structural`
# para que ela reprove; a fase `self-test` liga uma por execução e exige que cada
# uma reprove pelo motivo esperado. Ninguém as usa fora dela.
set -uo pipefail

# Resolvido antes do `cd` porque a fase `self-test` reinvoca este mesmo arquivo, e
# o $0 recebido pode ser relativo ao diretório de quem chamou.
ORIGEM="$(readlink -f "${BASH_SOURCE[0]}")" \
  || { echo "não consegui resolver o caminho do próprio script" >&2; exit 2; }

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

NOME=genproof
CONTEXTO=genproofctx
BLOCOS="${DMPF_GENERATOR_CHECK_BLOCKS:-domain,port,application,provider,app}"

# Um bloco só na fase `self-test`: nenhum dos quatro guards depende de quantos
# módulos existem, e cada vetor paga uma fase `structural` inteira.
BLOCOS_SELF_TEST=domain

# O daemon guarda estado entre execuções e o worktree é descartável; a nuvem não
# tem o que fazer num gate local.
export NX_DAEMON=false
export NX_NO_CLOUD=true

# O `verifyDepsBeforeRun` do pnpm nasce em `install` e o estado do workspace
# (node_modules/.pnpm-workspace-state-v1.json) guarda o caminho ABSOLUTO de cada
# projeto: dentro do worktree nenhum bate, o pnpm conclui que o node_modules
# está desatualizado e roda `pnpm install` sozinho — inclusive por dentro do
# `pnpm biome` do pre-commit. Ali isso é destrutivo, porque o node_modules do
# worktree é symlink para o da raiz e o install reescreve os links do
# repositório real apontando para o diretório temporário (observado: a raiz
# ficou com node_modules/* resolvendo para /tmp/dmpf-genproof-*).
#
# A variável é lida como `pnpm_config_<chave em snake_case>`
# (pnpm.mjs:149641 de pnpm@11.14.0); `npm_config_*` não alcança esta chave.
# CI=true fica de fora de propósito — é ele que autoriza o pnpm a apagar o
# modules dir sem perguntar — e `conferir_node_modules` confere a âncora depois
# de cada passo que chama o pnpm.
export pnpm_config_verify_deps_before_run=false

WT=""
HEAD0=""
MANIFESTO=""
NM_ANCORA=""
ARQUIVOS_GERADOS=()
ARQUIVOS_FORMATO=()
ARQUIVOS_GO=()
PROJETOS=()
BLOCOS_PEDIDOS=0

uso() {
  cat <<'FIM'
uso: tools/dmpf-generator-check.sh --phase structural|self-test

  structural   gera o bounded context de prova, commita classificação e código,
               roda fmt-check/vet/build/lint e o verificador de conformidade
  self-test    roda a fase structural quatro vezes, cada uma com um ponto
               sabotado, e exige que todas reprovem pelo motivo esperado
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

# Identidade própria: o job do CI não tem user.name/user.email configurados, e
# sem eles o commit da prova falha antes de exercitar o que o gate mede.
git_gate() {
  git -C "$WT" \
    -c user.name=dmpf-generator-check \
    -c user.email=dmpf-generator-check@lidercap.local "$@"
}

# A corrupção observada (ADR-041) reescreveu 39 links de uma vez, entre eles
# @biomejs/biome, @nx/*, @swc/* e typescript. O glob de um nível não casa pacote
# com escopo, então a assinatura desce em node_modules/@*/* também.
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
    || falha "node_modules ausente na raiz; rode 'pnpm install' antes do gate"
  destino="$(readlink -m "$ROOT/node_modules/nx")"
  case "$destino" in
    "$ROOT"/*) ;;
    *) falha "node_modules/nx da raiz resolve para fora de $ROOT ($destino); rode 'pnpm install' na raiz antes do gate" ;;
  esac
  [ -f "$destino/package.json" ] \
    || falha "node_modules/nx da raiz está quebrado ($destino sem package.json); rode 'pnpm install' na raiz antes do gate"

  NM_ANCORA="$(mktemp)" || falha "não consegui criar o arquivo da âncora de node_modules"
  assinar_node_modules >"$NM_ANCORA" || falha "não consegui assinar o node_modules da raiz"
  [ -s "$NM_ANCORA" ] \
    || falha "a assinatura do node_modules da raiz saiu vazia; rode 'pnpm install' antes do gate"
}

conferir_node_modules() {
  local divergencia
  divergencia="$(diff "$NM_ANCORA" <(assinar_node_modules) | head -12)"
  [ -z "$divergencia" ] || falha "o node_modules da raiz foi reescrito durante o gate: o pnpm instalou por dentro do symlink do worktree; rode 'pnpm install' na raiz para recuperar. Primeiras divergências:
$divergencia"
}

injetar_arvore_de_trabalho() {
  if ! git diff --quiet HEAD; then
    git diff HEAD | git -C "$WT" apply \
      || falha "não consegui aplicar o diff da árvore de trabalho no worktree"
  fi
  if [ -d "$ROOT/tools/dmpf-plugin" ]; then
    mkdir -p "$WT/tools" || falha "não consegui criar tools/ no worktree"
    tar -C "$ROOT/tools" \
      --exclude=dmpf-plugin/dist \
      --exclude=dmpf-plugin/out-tsc \
      --exclude=dmpf-plugin/node_modules \
      -cf - dmpf-plugin | tar -C "$WT/tools" -xf - \
      || falha "não consegui copiar tools/dmpf-plugin para o worktree"
  fi
  git -C "$WT" add -A >/dev/null || falha "git add do estado de trabalho no worktree"
  git_gate commit -q -m "chore(workspace): estado de trabalho da prova (efêmero)" \
    || falha "commit efêmero do estado de trabalho reprovou (os hooks estão ativos)"
}

abrir_worktree() {
  WT="$(mktemp -d)" || falha "não consegui criar o diretório do worktree"
  git worktree add --detach "$WT" HEAD >/dev/null 2>&1 \
    || falha "não consegui abrir o worktree em $WT"
  ln -s "$ROOT/node_modules" "$WT/node_modules" \
    || falha "não consegui ligar node_modules no worktree"
  if [ -n "${DMPF_GENERATOR_CHECK_WORKING_TREE:-}" ]; then
    injetar_arvore_de_trabalho
  fi
  # O plugin é workspace package do pnpm e resolve as dependências pelo próprio
  # node_modules; sem este segundo link o generator não carrega.
  if [ -d "$ROOT/tools/dmpf-plugin/node_modules" ] && [ ! -e "$WT/tools/dmpf-plugin/node_modules" ]; then
    ln -s "$ROOT/tools/dmpf-plugin/node_modules" "$WT/tools/dmpf-plugin/node_modules" \
      || falha "não consegui ligar tools/dmpf-plugin/node_modules no worktree"
  fi
  HEAD0="$(git -C "$WT" rev-parse HEAD)" || falha "não consegui ler o HEAD do worktree"
}

gerar() {
  local blocos=() bloco args=() saida status
  IFS=',' read -r -a blocos <<<"$BLOCOS"
  for bloco in "${blocos[@]}"; do
    [ -n "$bloco" ] || continue
    args+=("--blocks=$bloco")
    BLOCOS_PEDIDOS=$((BLOCOS_PEDIDOS + 1))
  done
  [ "$BLOCOS_PEDIDOS" -gt 0 ] || falha "DMPF_GENERATOR_CHECK_BLOCKS não nomeou nenhum bloco"

  saida="$(cd "$WT" && pnpm nx g @lidercap-apps/dmpf-plugin:bounded-context "$NOME" \
    --bounded-context "$CONTEXTO" "${args[@]}" --no-interactive 2>&1)"
  status=$?
  if [ "$status" -ne 0 ]; then
    printf '%s\n' "$saida" >&2
    falha "o generator reprovou (exit $status; saída acima)"
  fi

  if [ -n "${DMPF_GENERATOR_CHECK_SIMULAR_SEM_INSTRUCAO:-}" ]; then
    saida="$(printf '%s\n' "$saida" | grep -v -e 'write-baseline' -e 'AUT-01')"
  fi

  grep -qF -- '--write-baseline' <<<"$saida" \
    || falha "a saída do generator não instrui a regravar o baseline (--write-baseline ausente)"
  grep -qF 'AUT-01' <<<"$saida" \
    || falha "a saída do generator não cita AUT-01: unidade nova é ato de classificação"
  ok "generator concluído e instrução do baseline (AUT-01 + --write-baseline) presente"
}

coletar_gerados() {
  local entrada caminho
  ARQUIVOS_GERADOS=()
  ARQUIVOS_FORMATO=()
  ARQUIVOS_GO=()
  while IFS= read -r -d '' entrada; do
    caminho="${entrada:3}"
    [ -n "$caminho" ] || continue
    ARQUIVOS_GERADOS+=("$caminho")
    case "$caminho" in
      *.json | *.jsonc | *.ts | *.tsx | *.js | *.mjs | *.cjs | *.css) ARQUIVOS_FORMATO+=("$caminho") ;;
      *.go) ARQUIVOS_GO+=("$caminho") ;;
    esac
  done < <(git -C "$WT" status --porcelain -z --untracked-files=all)

  [ "${#ARQUIVOS_GERADOS[@]}" -gt 0 ] \
    || falha "o generator não deixou nenhum arquivo novo ou modificado no worktree"
  printf '%s\n' "${ARQUIVOS_GERADOS[@]}" | grep -qx 'go.work' \
    || falha "go.work não aparece entre os arquivos tocados: o módulo gerado não foi registrado"
  ok "${#ARQUIVOS_GERADOS[@]} arquivo(s) gerado(s), go.work incluído"
}

projetos_gerados() {
  local caminho nome
  PROJETOS=()
  for caminho in "${ARQUIVOS_GERADOS[@]}"; do
    case "$caminho" in
      */project.json)
        nome="$(awk -F'"' '/"name":/ { print $4; exit }' "$WT/$caminho")"
        [ -n "$nome" ] || falha "project.json gerado sem campo name: $caminho"
        PROJETOS+=("$nome")
        ;;
    esac
  done
  [ "${#PROJETOS[@]}" -eq "$BLOCOS_PEDIDOS" ] \
    || falha "${#PROJETOS[@]} módulo(s) gerado(s) para $BLOCOS_PEDIDOS bloco(s) pedido(s): a lista --blocks não chegou inteira ao generator"
  ok "módulos gerados: ${PROJETOS[*]}"
}

checar_sem_escrita() {
  local saida status
  if [ "${#ARQUIVOS_FORMATO[@]}" -gt 0 ]; then
    saida="$(cd "$WT" && pnpm biome ci --no-errors-on-unmatched "${ARQUIVOS_FORMATO[@]}" 2>&1)"
    status=$?
    if [ "$status" -ne 0 ]; then
      printf '%s\n' "$saida" >&2
      falha "biome ci reprovou arquivo gerado (o arquivo e o diagnóstico estão acima)"
    fi
  fi
  if [ "${#ARQUIVOS_GO[@]}" -gt 0 ]; then
    # gofmt lista o desalinhado em stdout com status 0 e, diante de erro de
    # sintaxe, escreve em stderr e sai 2 com stdout vazio; saída e status são
    # guardados em separado, como no lefthook.yml.
    saida="$(cd "$WT" && gofmt -l "${ARQUIVOS_GO[@]}" 2>&1)"
    status=$?
    if [ "$status" -ne 0 ]; then
      printf '%s\n' "$saida" >&2
      falha "gofmt não conseguiu ler arquivo gerado (exit $status; saída acima)"
    fi
    [ -z "$saida" ] \
      || falha "gofmt -l apontou arquivo gerado fora do formato: $(printf '%s' "$saida" | tr '\n' ' ')"
  fi
  ok "biome ci e gofmt -l sem apontamento nos arquivos gerados"
}

# O manifesto fica fora do worktree porque o worktree é removido no trap, e
# antes do commit porque o pre-commit roda `biome check --write` com
# stage_fixed: um arquivo reescrito ali tem de aparecer como divergência.
gravar_manifesto() {
  MANIFESTO="$(mktemp)" || falha "não consegui criar o arquivo do manifesto"
  (cd "$WT" && sha256sum -- "${ARQUIVOS_GERADOS[@]}") >"$MANIFESTO" \
    || falha "não consegui calcular o SHA-256 dos arquivos gerados"
  ok "manifesto SHA-256 de ${#ARQUIVOS_GERADOS[@]} arquivo(s) em $MANIFESTO"
}

conferir_manifesto() {
  local saida status
  [ -s "$MANIFESTO" ] || falha "o manifesto SHA-256 está vazio ou não foi gravado"
  saida="$(cd "$WT" && sha256sum -c --quiet -- "$MANIFESTO" 2>&1)"
  status=$?
  if [ "$status" -ne 0 ]; then
    printf '%s\n' "$saida" >&2
    falha "arquivo gerado mudou depois da geração (o que divergiu está acima)"
  fi
  ok "manifesto conferido: nenhum arquivo gerado foi reescrito"
}

commitar_classificacao() {
  local caminho manifestos=() saida status
  for caminho in "${ARQUIVOS_GERADOS[@]}"; do
    case "$caminho" in */dmpf-units.json) manifestos+=("$caminho") ;; esac
  done
  [ "${#manifestos[@]}" -gt 0 ] || falha "nenhum dmpf-units.json entre os arquivos gerados"

  git -C "$WT" add -- "${manifestos[@]}" || falha "git add dos manifestos de unidade"
  saida="$(cd "$WT" && go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance \
    --root . --write-baseline 2>&1)"
  status=$?
  if [ "$status" -ne 0 ]; then
    printf '%s\n' "$saida" >&2
    falha "--write-baseline reprovou (exit $status; saída acima)"
  fi
  git -C "$WT" add -- tools/dmpf-baseline/units-baseline.json || falha "git add do baseline"

  # Vetor (b) do self-test: o código entra no mesmo commit da classificação, que
  # é o que DMPF-T002 recusa.
  if [ -n "${DMPF_GENERATOR_CHECK_SIMULAR_CLASSIFICACAO_MISTURADA:-}" ]; then
    git -C "$WT" add -A || falha "git add do código junto da classificação"
    git_gate commit -q -m "chore(genproof): classificar unidades e escrever o código" \
      || falha "o commit misturado reprovou (os hooks estão ativos)"
    ok "sabotagem: commit único com classificação e código"
    return 0
  fi

  git_gate commit -q -m "chore(genproof): classificar unidades" \
    || falha "o commit da classificação reprovou (os hooks estão ativos)"
  ok "commit 1: ${#manifestos[@]} manifesto(s) de unidade + baseline"
}

commitar_codigo() {
  if [ -n "${DMPF_GENERATOR_CHECK_SIMULAR_CLASSIFICACAO_MISTURADA:-}" ]; then
    ok "commit 2 dispensado: a sabotagem já commitou o código com a classificação"
    return 0
  fi
  git -C "$WT" add -A || falha "git add do código gerado"
  git_gate commit -q -m "feat(genproof): scaffold" \
    || falha "o commit do código reprovou (os hooks estão ativos)"
  ok "commit 2: código gerado, go.work e o que o pnpm ajustou"
}

cadeia_nx() {
  local lista
  lista="$(
    IFS=,
    printf '%s' "${PROJETOS[*]}"
  )"
  (cd "$WT" && pnpm nx run-many -t fmt-check,vet,build,lint --projects="$lista" --parallel=3) \
    || falha "a cadeia fmt-check,vet,build,lint reprovou em $lista"
  ok "cadeia fmt-check,vet,build,lint aprovada em $lista"
}

verificar_conformidade() {
  local saida status
  saida="$(go run ./libs/backend/go/dmpf-conformance/cmd/dmpf-conformance \
    --root "$WT" --base "$HEAD0" 2>&1)"
  status=$?
  printf '%s\n' "$saida"
  [ "$status" -eq 0 ] \
    || falha "o verificador de conformidade reprovou o bounded context gerado (exit $status)"
  ok "verificador de conformidade aprovado com --base $HEAD0"
}

conferir_arvore_limpa() {
  local sobras
  git -C "$WT" diff --exit-code >/dev/null \
    || falha "os checks mudaram arquivo versionado: $(git -C "$WT" diff --name-only | tr '\n' ' ')"
  sobras="$(git -C "$WT" status --porcelain)"
  if [ -n "$sobras" ]; then
    printf '%s\n' "$sobras" >&2
    falha "sobrou arquivo não versionado depois dos checks (acima)"
  fi
  ok "git diff vazio e git status --porcelain vazio"
}

# Vetor (d) do self-test. Roda depois da coleta e antes de `checar_sem_escrita`,
# que é onde o biome ci tem de acusar: o manifesto só é gravado adiante e
# assinaria o arquivo já sabotado.
sabotar_json_gerado() {
  local caminho alvo="" antes
  for caminho in "${ARQUIVOS_GERADOS[@]}"; do
    case "$caminho" in */dmpf-units.json) alvo="$caminho"; break ;; esac
  done
  [ -n "$alvo" ] || falha "nenhum dmpf-units.json entre os gerados para sabotar"
  antes="$(sha256sum <"$WT/$alvo")" || falha "não consegui ler $alvo"
  sed -i '2s/^/ /' "$WT/$alvo" || falha "não consegui sabotar $alvo"
  [ "$antes" != "$(sha256sum <"$WT/$alvo")" ] \
    || falha "a sabotagem não mudou $alvo: sem ela o vetor mediria a fase intacta"
  ok "sabotagem: espaço extra na indentação de $alvo"
}

# Vetor (a) do self-test. O alvo é o README, e não um .go, porque a cadeia Go roda
# entre este ponto e o manifesto: sabotar código faria a prova reprovar em
# fmt-check, e não no guard que este vetor mede.
sabotar_edicao_pos_commit() {
  local caminho alvo=""
  for caminho in "${ARQUIVOS_GERADOS[@]}"; do
    case "$caminho" in */README.md) alvo="$caminho"; break ;; esac
  done
  [ -n "$alvo" ] || falha "nenhum README.md entre os gerados para sabotar"
  printf '\nlinha inserida pelo vetor de self-test\n' >>"$WT/$alvo" \
    || falha "não consegui sabotar $alvo"
  ok "sabotagem: $alvo reescrito depois do commit 2"
}

fase_structural() {
  ancorar_node_modules

  passo "worktree descartável em HEAD"
  abrir_worktree
  conferir_node_modules
  ok "worktree em $WT (HEAD0=$HEAD0)"

  passo "gerar o bounded context de prova ($NOME/$CONTEXTO, blocos $BLOCOS)"
  gerar
  conferir_node_modules
  coletar_gerados
  projetos_gerados

  passo "checagem sem escrita e manifesto dos arquivos gerados"
  [ -z "${DMPF_GENERATOR_CHECK_SIMULAR_JSON_FORA_DO_PADRAO:-}" ] || sabotar_json_gerado
  checar_sem_escrita
  conferir_node_modules
  gravar_manifesto

  passo "commit 1 — classificação"
  commitar_classificacao

  passo "commit 2 — código"
  commitar_codigo
  [ -z "${DMPF_GENERATOR_CHECK_SIMULAR_EDICAO_POS_COMMIT:-}" ] || sabotar_edicao_pos_commit

  passo "cadeia Go dos módulos gerados"
  cadeia_nx
  conferir_node_modules

  passo "verificador de conformidade"
  verificar_conformidade

  passo "nada foi reescrito nem sobrou"
  conferir_manifesto
  conferir_arvore_limpa

  printf '\nProva do generator (structural): OK — %s módulo(s) e %s arquivo(s) gerado(s).\n' \
    "${#PROJETOS[@]}" "${#ARQUIVOS_GERADOS[@]}"
}

# Cada vetor roda num processo próprio porque `falha` encerra o processo: capturar
# a reprovação daqui exigiria desmontar o contrato de erro do script. O processo
# próprio também dá worktree e trap próprios, que é o isolamento que o vetor pede.
vetor() {
  local nome="$1" variavel="$2" esperado="$3"
  local saida status
  local ambiente=("$variavel=1" "DMPF_GENERATOR_CHECK_BLOCKS=$BLOCOS_SELF_TEST")
  [ -z "${DMPF_GENERATOR_CHECK_WORKING_TREE:-}" ] \
    || ambiente+=("DMPF_GENERATOR_CHECK_WORKING_TREE=$DMPF_GENERATOR_CHECK_WORKING_TREE")
  saida="$(env "${ambiente[@]}" bash "$ORIGEM" --phase structural 2>&1)"
  status=$?
  if [ "$status" -eq 0 ]; then
    printf '%s\n' "$saida" >&2
    falha "vetor $nome: a fase structural passou, e devia ter reprovado (saída acima)"
  fi
  if ! grep -qF -- "$esperado" <<<"$saida"; then
    printf '%s\n' "$saida" >&2
    falha "vetor $nome: reprovou por outro motivo — esperado o trecho \"$esperado\" (saída acima)"
  fi
  ok "$nome: reprovou como esperado (\"$esperado\")"
}

fase_self_test() {
  passo "vetor 1 — arquivo gerado reescrito depois do commit 2"
  vetor "edição pós-commit" DMPF_GENERATOR_CHECK_SIMULAR_EDICAO_POS_COMMIT \
    "arquivo gerado mudou depois da geração"

  passo "vetor 2 — classificação e código no mesmo commit"
  vetor "classificação misturada" DMPF_GENERATOR_CHECK_SIMULAR_CLASSIFICACAO_MISTURADA \
    "DMPF-T002"

  passo "vetor 3 — instrução do baseline ausente na saída do generator"
  vetor "instrução ausente" DMPF_GENERATOR_CHECK_SIMULAR_SEM_INSTRUCAO \
    "não instrui a regravar o baseline"

  passo "vetor 4 — JSON gerado fora do padrão do Biome"
  vetor "JSON fora do padrão" DMPF_GENERATOR_CHECK_SIMULAR_JSON_FORA_DO_PADRAO \
    "biome ci reprovou arquivo gerado"

  printf '\nProva do generator (self-test): OK — 4 vetor(es) reprovaram pelo motivo esperado.\n'
}

FASE=""
while [ $# -gt 0 ]; do
  case "$1" in
    --phase)
      FASE="${2:-}"
      [ -n "$FASE" ] || { echo "FALHA: --phase exige um valor" >&2; uso >&2; exit 2; }
      shift 2
      ;;
    --phase=*)
      FASE="${1#--phase=}"
      shift
      ;;
    -h | --help)
      uso
      exit 0
      ;;
    *)
      echo "FALHA: argumento desconhecido: $1" >&2
      uso >&2
      exit 2
      ;;
  esac
done

case "$FASE" in
  structural)
    fase_structural
    ;;
  self-test)
    fase_self_test
    ;;
  "")
    echo "FALHA: --phase é obrigatório" >&2
    uso >&2
    exit 2
    ;;
  *)
    echo "FALHA: fase desconhecida: $FASE" >&2
    uso >&2
    exit 2
    ;;
esac
