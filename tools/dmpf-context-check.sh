#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova e por que o critério de descoberta é esse, no mesmo molde de tools/dmpf-cell-check.sh.
# Prova que todo bounded context de apps/backend segue a forma canônica do
# ADR-053: um package por bloco sob o módulo do contexto, a borda gRPC em
# app/rpc, config e wiring em app/, o schema no provider, o binário em cmd/, os
# kits de teste e os targets que os servem. A borda (`bff`) tem ramo próprio.
#
# Além do layout, o gate confere os nomes de banco: o DDL dos contextos e do
# kernel segue a tabela de nomes canônicos, e todo DSN, POSTGRES_DB e script de
# criação de banco em infra/ e .github/workflows usa banco e role com o nome da
# app, ou o par administrativo `postgres`.
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
KERNEL_DDL="libs/backend/go/postgres"
FALHAS=0

falhar() {
  printf '  FALHA  %s\n' "$1" >&2
  FALHAS=$((FALHAS + 1))
}

aprovar() { printf '  ok     %s\n' "$1"; }

# contextos lista o que o gate governa: diretório de apps/backend com domain/.
contextos() {
  local raiz="$1" caminho
  for caminho in "$raiz"/*/; do
    [ -d "$caminho/domain" ] || continue
    basename "$caminho"
  done
}

# bordas lista as apps sem domínio que servem um binário: o `bff`.
bordas() {
  local raiz="$1" caminho
  for caminho in "$raiz"/*/; do
    [ -d "$caminho/domain" ] && continue
    [ -f "$caminho/cmd/main.go" ] || continue
    basename "$caminho"
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

exigir() {
  local caminho="$1" rotulo="$2"
  if [ -e "$caminho" ]; then
    aprovar "$rotulo"
  else
    falhar "falta $rotulo"
  fi
}

recusar() {
  local caminho="$1" rotulo="$2"
  if [ -e "$caminho" ]; then
    falhar "$rotulo"
  else
    aprovar "sem $rotulo"
  fi
}

raiz_sem_go() {
  local base="$1" ctx="$2" soltos
  soltos="$(find "$base" -maxdepth 1 -name '*.go' -printf '%f\n' 2>/dev/null)"
  if [ -n "$soltos" ]; then
    falhar "$ctx: código Go solto na raiz ($(echo "$soltos" | tr '\n' ' '))"
  else
    aprovar "$ctx: raiz sem código de bloco"
  fi
}

verificar_contexto() {
  local base="$1" ctx="$2"

  for bloco in domain application provider app; do
    exigir "$base/$bloco" "$ctx: o package do bloco $bloco"
  done
  raiz_sem_go "$base" "$ctx"

  exigir "$base/cmd/main.go" "$ctx: o binário em cmd/main.go"
  exigir "$base/app/config.go" "$ctx: a configuração em app/config.go"
  exigir "$base/app/wiring.go" "$ctx: o composition root em app/wiring.go"
  exigir "$base/app/rpc" "$ctx: a borda gRPC em app/rpc"
  recusar "$base/app/http" "$ctx: borda HTTP em app/http (o REST público é do bff, ADR-044)"
  exigir "$base/provider/schema.sql" "$ctx: o schema do contexto em provider/schema.sql"

  for kit in appkit distkit; do
    exigir "$base/$kit" "$ctx: o $kit"
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

verificar_borda() {
  local base="$1" app="$2"
  raiz_sem_go "$base" "$app"
  exigir "$base/app/config.go" "$app: a configuração em app/config.go"
  exigir "$base/app/wiring.go" "$app: o composition root em app/wiring.go"
  exigir "$base/app/api" "$app: as rotas REST em app/api"
  exigir "$base/app/rpc" "$app: os clientes gRPC em app/rpc"
  recusar "$base/provider" "$app: provider na borda (a borda não tem banco)"
}

# verificar_ddl confere os nomes do DDL (ADR-053): nada com dmpf ou example, índice
# `<tabela>_..._idx`, constraint `<tabela>_..._{pkey,key,check,fkey}`; num
# contexto, os nomes das tabelas do kernel são proibidos.
verificar_ddl() {
  local papel="$1"
  shift
  python3 - "$papel" "$@" <<'PY'
import re
import sys

papel, arquivos = sys.argv[1], sys.argv[2:]
KERNEL = {'outbox', 'inbox', 'quarantine'}
PROIBIDO = re.compile(r'(^|_)(dmpf|example)(_|$)')
achados = []
for arquivo in arquivos:
    try:
        texto = open(arquivo, encoding='utf-8').read()
    except OSError as erro:
        achados.append(f'{arquivo}: {erro}')
        continue
    sql = re.sub(r'--[^\n]*', '', texto)
    tabela = None
    for token in re.finditer(
        r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?'
        r'|CONSTRAINT\s+"?(\w+)"?'
        r'|CREATE\s+(?:UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?\s+ON\s+"?(\w+)"?',
        sql,
        re.IGNORECASE,
    ):
        criada, restricao, indice, alvo = token.groups()
        for nome in (criada, restricao, indice):
            if nome and PROIBIDO.search(nome):
                achados.append(f'{arquivo}: {nome} leva dmpf ou example no nome')
        if criada:
            tabela = criada
            if papel == 'contexto' and tabela in KERNEL:
                achados.append(f'{arquivo}: tabela {tabela} usa um nome do kernel')
        elif restricao:
            if tabela is None or not re.fullmatch(rf'{tabela}_\w*(pkey|key|check|fkey)', restricao):
                achados.append(f'{arquivo}: constraint {restricao} fora de <tabela>_<colunas>_{{pkey,key,check,fkey}}')
        elif indice:
            if not re.fullmatch(rf'{alvo}_\w+_idx', indice):
                achados.append(f'{arquivo}: índice {indice} fora de <tabela>_<colunas>_idx')
for achado in achados:
    print(achado)
sys.exit(1 if achados else 0)
PY
}

# verificar_infra confere banco e role onde a infra e a CI os declaram: DSN,
# POSTGRES_DB e o laço que cria banco e role por app.
verificar_infra() {
  local repo="$1"
  shift
  python3 - "$repo" "$@" <<'PY'
import pathlib
import re
import sys

repo, apps = pathlib.Path(sys.argv[1]), set(sys.argv[2:])
ADMIN = 'postgres'
DSN = re.compile(r'postgres(?:ql)?://((?:\$\{[^}]*\}|[^:@/\s"\'])+)(?::[^@\s"\']*)?@[^/\s"\']+/([^?\s"\']+)')
DEFAULT = re.compile(r'\$\{[A-Z_]+:-([^}]*)\}')
POSTGRES_DB = re.compile(r'POSTGRES_DB\s*[:=]\s*["\']?([^\s"\'}]+)')
LOOP = re.compile(r'for pair in (.*?);\s*do', re.DOTALL)
PAIR = re.compile(r'([a-z][a-z0-9-]*):\$')

def literal(value):
    return DEFAULT.sub(lambda m: m.group(1), value)

achados = []
arquivos = []
for raiz in ('infra', '.github/workflows'):
    base = repo / raiz
    if base.is_dir():
        arquivos += [p for p in sorted(base.rglob('*')) if p.is_file() and p.suffix != '.md']
for arquivo in arquivos:
    try:
        texto = arquivo.read_text(encoding='utf-8')
    except (OSError, UnicodeDecodeError):
        continue
    nome = arquivo.relative_to(repo)
    for user, db in DSN.findall(texto):
        user, db = literal(user), literal(db)
        if (user, db) == (ADMIN, ADMIN) or (user == db and db in apps):
            continue
        achados.append(f'{nome}: DSN com role {user} e banco {db} (esperado o nome da app nos dois, ou {ADMIN}/{ADMIN})')
    for db in POSTGRES_DB.findall(texto):
        if literal(db) != ADMIN:
            achados.append(f'{nome}: POSTGRES_DB={db} (o banco administrativo é {ADMIN})')
    if re.search(r'CREATE DATABASE', texto):
        declaradas = {app for laco in LOOP.findall(texto) for app in PAIR.findall(laco)}
        if declaradas != apps:
            achados.append(
                f'{nome}: cria banco e role para {sorted(declaradas)}, esperado um por contexto: {sorted(apps)}'
            )
for achado in achados:
    print(achado)
sys.exit(1 if achados else 0)
PY
}

relatar() {
  local rotulo="$1" saida="$2" status="$3"
  if [ "$status" -eq 0 ]; then
    aprovar "$rotulo"
    return
  fi
  while IFS= read -r linha; do
    [ -n "$linha" ] && falhar "$linha"
  done <<<"$saida"
}

fase_estrutural() {
  local repo="${1:-$ROOT}" raiz ctx app achou=0 saida status
  raiz="$repo/$APPS"
  local -a ctxs=()
  while IFS= read -r ctx; do
    achou=1
    ctxs+=("$ctx")
    printf '\n== %s ==\n' "$ctx"
    verificar_contexto "$raiz/$ctx" "$ctx"
  done < <(contextos "$raiz")

  if [ "$achou" -eq 0 ]; then
    echo "nenhum bounded context encontrado em $raiz" >&2
    return 2
  fi

  while IFS= read -r app; do
    printf '\n== %s (borda) ==\n' "$app"
    verificar_borda "$raiz/$app" "$app"
  done < <(bordas "$raiz")

  printf '\n== nomes no DDL ==\n'
  for ctx in "${ctxs[@]}"; do
    [ -f "$raiz/$ctx/provider/schema.sql" ] || continue
    saida="$(verificar_ddl contexto "$raiz/$ctx/provider/schema.sql")"
    status=$?
    relatar "$ctx: DDL nos nomes canônicos" "$saida" "$status"
  done
  local -a kernel=()
  mapfile -t kernel < <(find "$repo/$KERNEL_DDL" -maxdepth 1 -name '*.sql' 2>/dev/null | sort)
  if [ "${#kernel[@]}" -gt 0 ]; then
    saida="$(verificar_ddl kernel "${kernel[@]}")"
    status=$?
    relatar "kernel: DDL nos nomes canônicos" "$saida" "$status"
  fi

  printf '\n== banco e role em infra/ e .github/workflows ==\n'
  saida="$(verificar_infra "$repo" "${ctxs[@]}")"
  status=$?
  relatar "banco e role com o nome da app, ou o par administrativo" "$saida" "$status"

  if [ "$FALHAS" -gt 0 ]; then
    printf '\nGate de estrutura: REPROVADO com %d achado(s).\n' "$FALHAS" >&2
    return 1
  fi
  printf '\nGate de estrutura: OK — todo contexto segue a forma canônica.\n'
  return 0
}

# fase_contexto confere um único contexto, fora do inventário da infra: é o que
# o generator-check roda sobre o módulo recém-gerado, que ainda não tem banco.
fase_contexto() {
  local base="$1" ctx saida status
  ctx="$(basename "$base")"
  [ -d "$base/domain" ] || { echo "$base não é um bounded context (falta domain/)" >&2; return 2; }
  printf '\n== %s ==\n' "$ctx"
  verificar_contexto "$base" "$ctx"
  if [ -f "$base/provider/schema.sql" ]; then
    saida="$(verificar_ddl contexto "$base/provider/schema.sql")"
    status=$?
    relatar "$ctx: DDL nos nomes canônicos" "$saida" "$status"
  fi
  if [ "$FALHAS" -gt 0 ]; then
    printf '\nGate de estrutura: REPROVADO com %d achado(s) em %s.\n' "$FALHAS" "$ctx" >&2
    return 1
  fi
  printf '\nGate de estrutura: OK — %s segue a forma canônica.\n' "$ctx"
  return 0
}

# A fixture nasce completa e cada vetor remove ou estraga uma peça: assim o gate
# é provado contra o que ele deve recusar, e não só contra o que já passa.
montar_fixture() {
  local repo="$1"
  local ctx="$repo/$APPS/probe" bff="$repo/$APPS/bff"
  mkdir -p "$ctx"/{domain,application,provider,app/rpc,appkit,distkit,cmd}
  : > "$ctx/domain/doc.go"
  : > "$ctx/application/doc.go"
  : > "$ctx/provider/doc.go"
  : > "$ctx/app/doc.go"
  : > "$ctx/app/config.go"
  : > "$ctx/app/wiring.go"
  : > "$ctx/app/rpc/service.go"
  : > "$ctx/appkit/doc.go"
  : > "$ctx/distkit/doc.go"
  : > "$ctx/cmd/main.go"
  cat > "$ctx/provider/schema.sql" <<'SQL'
CREATE TABLE IF NOT EXISTS probes (
  tenant_id text NOT NULL,
  probe_id  text NOT NULL,
  CONSTRAINT probes_pkey PRIMARY KEY (tenant_id, probe_id)
);
CREATE INDEX IF NOT EXISTS probes_probe_id_idx ON probes (probe_id);
SQL
  cat > "$ctx/project.json" <<'JSON'
{"targets":{"serve-api":{},"serve-relay":{},"test-distributed":{}}}
JSON
  cat > "$ctx/dmpf-units.json" <<'JSON'
{"units":[{"id":"probe/app"},{"id":"probe/appkit"},{"id":"probe/distkit"}]}
JSON

  mkdir -p "$bff"/{app/api,app/rpc,cmd}
  : > "$bff/app/config.go"
  : > "$bff/app/wiring.go"
  : > "$bff/app/api/routes.go"
  : > "$bff/app/rpc/clients.go"
  : > "$bff/cmd/main.go"

  mkdir -p "$repo/$KERNEL_DDL"
  cat > "$repo/$KERNEL_DDL/outbox.sql" <<'SQL'
CREATE TABLE IF NOT EXISTS outbox (
  id bigint NOT NULL,
  CONSTRAINT outbox_pkey PRIMARY KEY (id)
);
CREATE INDEX IF NOT EXISTS outbox_claim_idx ON outbox (id);
SQL

  mkdir -p "$repo/infra/local" "$repo/.github/workflows"
  cat > "$repo/infra/local/compose.yml" <<'YAML'
services:
  postgres:
    environment:
      POSTGRES_DB: postgres
  init:
    command: for pair in probe:${PROBE_PG_PASSWORD:-probe-local}; do psql -c "CREATE DATABASE x"; done
  probe:
    environment:
      PG_DSN: postgres://probe:${PROBE_PG_PASSWORD:-probe-local}@postgres:5432/probe?sslmode=disable
YAML
  cat > "$repo/.github/workflows/ci.yml" <<'YAML'
env:
  PG_DSN: "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
YAML
}

sabotar() {
  local repo="$1" tipo="$2" ctx="$1/$APPS/probe"
  case "$tipo" in
    sem-binario) trash "$ctx/cmd/main.go" ;;
    sem-appkit) trash "$ctx/appkit" ;;
    sem-distkit) trash "$ctx/distkit" ;;
    sem-rpc) trash "$ctx/app/rpc" ;;
    sem-config) trash "$ctx/app/config.go" ;;
    sem-wiring) trash "$ctx/app/wiring.go" ;;
    sem-schema) trash "$ctx/provider/schema.sql" ;;
    com-http) mkdir -p "$ctx/app/http" && : > "$ctx/app/http/routes.go" ;;
    solto) : > "$ctx/wiring.go" ;;
    sem-target) printf '{"targets":{"serve-relay":{},"test-distributed":{}}}\n' > "$ctx/project.json" ;;
    sem-unidade) printf '{"units":[{"id":"probe/app"},{"id":"probe/distkit"}]}\n' > "$ctx/dmpf-units.json" ;;
    bff-sem-api) trash "$repo/$APPS/bff/app/api" ;;
    bff-com-provider) mkdir -p "$repo/$APPS/bff/provider" ;;
    tabela-prefixada) sed -i 's/probes/dmpf_probes/g' "$ctx/provider/schema.sql" ;;
    tabela-example) printf 'CREATE TABLE IF NOT EXISTS dmpf_example_x (id bigint);\n' >> "$ctx/provider/schema.sql" ;;
    indice-example) sed -i 's/probes_probe_id_idx/probes_example_idx/' "$ctx/provider/schema.sql" ;;
    tabela-do-kernel) printf 'CREATE TABLE IF NOT EXISTS outbox (id bigint);\n' >> "$ctx/provider/schema.sql" ;;
    indice-fora) sed -i 's/probes_probe_id_idx/idx_probe/' "$ctx/provider/schema.sql" ;;
    constraint-fora) sed -i 's/probes_pkey/pk_probes/' "$ctx/provider/schema.sql" ;;
    kernel-prefixado) sed -i 's/outbox/dmpf_outbox/g' "$repo/$KERNEL_DDL/outbox.sql" ;;
    dsn-compartilhado) sed -i 's#@postgres:5432/probe#@postgres:5432/dmpf#' "$repo/infra/local/compose.yml" ;;
    postgres-db) sed -i 's/POSTGRES_DB: postgres/POSTGRES_DB: dmpf/' "$repo/infra/local/compose.yml" ;;
    init-incompleto) sed -i 's/for pair in probe:/for pair in other:/' "$repo/infra/local/compose.yml" ;;
    ci-compartilhado) sed -i 's#localhost:5432/postgres#localhost:5432/dmpf#' "$repo/.github/workflows/ci.yml" ;;
  esac
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
    ( FALHAS=0; fase_estrutural "$base" 2>&1 | grep FALHA ) >&2
    status=1
  fi

  local -a sabotagens=(
    sem-binario sem-appkit sem-distkit sem-rpc sem-config sem-wiring sem-schema
    com-http solto sem-target sem-unidade bff-sem-api bff-com-provider
    tabela-prefixada tabela-example indice-example tabela-do-kernel indice-fora constraint-fora kernel-prefixado
    dsn-compartilhado postgres-db init-incompleto ci-compartilhado
  )
  for sabotagem in "${sabotagens[@]}"; do
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    sabotar "$tmp" "$sabotagem"
    if ( FALHAS=0; fase_estrutural "$tmp" >/dev/null 2>&1 ); then
      falhar "a sabotagem '$sabotagem' passou pelo gate"
      status=1
    else
      aprovar "reprovou: $sabotagem"
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
uso: tools/dmpf-context-check.sh [--phase structural|self-test] [--root <dir>]

  structural  (default) verifica todo bounded context de apps/backend, a borda,
              os nomes do DDL e banco e role em infra/ e .github/workflows
  self-test   prova o gate contra fixture sintética, um vetor por sabotagem
  --root      raiz do repositório a verificar (default: a do git)
  --context   verifica só o layout e o DDL de um contexto, sem a infra
TXT
}

FASE="structural"
REPO="$ROOT"
CONTEXTO=""
while [ $# -gt 0 ]; do
  case "$1" in
    --phase) shift; FASE="${1:-}"; [ -n "$FASE" ] || { uso >&2; exit 2; } ;;
    --phase=*) FASE="${1#--phase=}" ;;
    --root) shift; REPO="${1:-}"; [ -n "$REPO" ] || { uso >&2; exit 2; } ;;
    --root=*) REPO="${1#--root=}" ;;
    --context) shift; CONTEXTO="${1:-}"; [ -n "$CONTEXTO" ] || { uso >&2; exit 2; } ;;
    --context=*) CONTEXTO="${1#--context=}" ;;
    -h|--help) uso; exit 0 ;;
    *) printf 'argumento desconhecido: %s\n' "$1" >&2; uso >&2; exit 2 ;;
  esac
  shift
done

case "$FASE" in
  structural)
    if [ -n "$CONTEXTO" ]; then fase_contexto "$CONTEXTO"; else fase_estrutural "$REPO"; fi
    ;;
  self-test) fase_self_test ;;
  *) printf 'fase desconhecida: %s\n' "$FASE" >&2; uso >&2; exit 2 ;;
esac
