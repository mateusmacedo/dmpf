#!/usr/bin/env bash
# comment-discipline-ok-file: cabeçalho de gate; declara o que o script prova e por que o critério de descoberta é esse, no mesmo molde de tools/dmpf-cell-check.sh.
# Prova que todo bounded context de apps/backend segue a forma canônica do
# ADR-053: um package por bloco sob o módulo do contexto, a borda gRPC em
# app/rpc, config e wiring em app/, o schema no provider, o binário em cmd/, os
# kits de teste e os targets que os servem. A borda (`bff`) tem ramo próprio.
#
# Além do layout, o gate confere os nomes de banco: o DDL dos contextos e do
# kernel segue a tabela de nomes canônicos, e todo DSN, POSTGRES_DB e script de
# criação de banco em infra/, apps/backend/*/deploy e .github/workflows usa banco
# e role com o nome da app (ou a role `<app>_migrator` no banco da app, só no
# PG_MIGRATE_DSN), ou o par administrativo `postgres`; cada contexto declara o
# próprio banco no deploy/infra.json e um grant para cada tabela do schema.sql;
# cada CONSTRAINT ... UNIQUE do schema.sql tem postgres.Unique de mesmo nome no
# provider, e o Table que a declara está em provider.Uniques (ADR-066); a leitura
# entre tenants e a projeção só aparecem em provider/platform_*.go (ADR-068).
#
# O critério de descoberta é a presença de `domain/`: um contexto tem domínio,
# uma borda como o `bff` não tem. Isso separa os dois sem lista fixa e passa a
# valer para contexto novo no dia em que ele nasce.
#
# A fase `self-test` prova o próprio gate: sobre uma fixture sintética, sabota
# um ponto por execução e exige que o gate reprove por aquele motivo. Sem ela,
# um gate que nunca morde passaria por gate.
set -uo pipefail

ROOT="${DMPF_WORKSPACE_ROOT:-$(git rev-parse --show-toplevel)}" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

# Layout do dmpf.json, exportado pelo executor; os defaults são os do platform. O DDL
# do kernel é relativo à raiz (modo local) ou absoluto, no cache do Go (modo version).
APPS="${DMPF_APPS_DIR:-apps/backend}"
KERNEL_DDL="${DMPF_KERNEL_DDL:-libs/backend/go/postgres}"
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
    [[ -d "$caminho/domain" ]] || continue
    basename "$caminho"
  done
}

# bordas lista as apps sem domínio que servem um binário: o `bff`.
bordas() {
  local raiz="$1" caminho
  for caminho in "$raiz"/*/; do
    [[ -d "$caminho/domain" ]] && continue
    [[ -f "$caminho/cmd/main.go" ]] || continue
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
  if [[ -e "$caminho" ]]; then
    aprovar "$rotulo"
  else
    falhar "falta $rotulo"
  fi
}

recusar() {
  local caminho="$1" rotulo="$2"
  if [[ -e "$caminho" ]]; then
    falhar "$rotulo"
  else
    aprovar "sem $rotulo"
  fi
}

raiz_sem_go() {
  local base="$1" ctx="$2" soltos
  soltos="$(find "$base" -maxdepth 1 -name '*.go' -printf '%f\n' 2>/dev/null)"
  if [[ -n "$soltos" ]]; then
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
  exigir "$base/deploy/infra.json" "$app: o manifesto de infra em deploy/infra.json"
  recusar "$base/provider" "$app: provider na borda (a borda não tem banco)"
}

# verificar_ddl confere os nomes do DDL (ADR-053): nada com dmpf ou example, índice
# `<tabela>_..._idx`, constraint `<tabela>_..._{pkey,key,check,fkey}` da tabela
# do CREATE ou ALTER TABLE mais recente; num contexto, os nomes das tabelas do
# kernel são proibidos.
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
        r'(?:CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?|ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?)"?(\w+)"?'
        r'|\bCONSTRAINT\s+"?(\w+)"?'
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

# verificar_grants cruza o CREATE TABLE do schema.sql com o provider.Grants: a
# tabela sem grant recusa a partida (ADR-063), e o gate a pega antes da subida.
verificar_grants() {
  local base="$1"
  python3 - "$base/provider/schema.sql" "$base/provider/schema.go" <<'PY'
import re
import sys

sql_path, go_path = sys.argv[1:3]
try:
    sql = re.sub(r'--[^\n]*', '', open(sql_path, encoding='utf-8').read())
except OSError as erro:
    print(f'{sql_path}: {erro}')
    sys.exit(1)
try:
    go = re.sub(r'//[^\n]*', '', open(go_path, encoding='utf-8').read())
except OSError:
    go = ''
tabelas = re.findall(
    r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:"?public"?\.)?"?(\w+)"?', sql, re.IGNORECASE
)
concedidas = set(re.findall(r'\b(?:Mutable|AppendOnly)\(\s*"(\w+)"\s*\)', go))
concedidas |= set(re.findall(r'\bTable:\s*"(\w+)"', go))
faltam = [t for t in tabelas if t not in concedidas]
for tabela in faltam:
    print(f'{sql_path}: tabela {tabela} sem grant no provider.Grants')
sys.exit(1 if faltam else 0)
PY
  return $?
}

# verificar_uniques cruza o CONSTRAINT ... UNIQUE do schema.sql com o provider
# (ADR-066): UNIQUE sem nome, índice único ou restrição sem postgres.Unique de mesmo
# Name voltam INTERNAL, e Table com Unique fora de provider.Uniques escapa do VerifyUniques.
verificar_uniques() {
  local base="$1"
  python3 - "$base/provider/schema.sql" "$base/provider" <<'PY'
import pathlib
import re
import sys
from bisect import bisect_left, bisect_right
from itertools import accumulate, repeat

def sem_comentarios(texto):
    saida, i = [], 0
    while i < len(texto):
        if texto.startswith('//', i):
            fim = texto.find('\n', i)
            i = len(texto) if fim < 0 else fim
        elif texto.startswith('/*', i):
            fim = texto.find('*/', i + 2)
            fim = len(texto) if fim < 0 else fim + 2
            saida.append(' ' + '\n' * texto.count('\n', i, fim))
            i = fim
        elif texto[i] in '"\'`':
            aspa, fim = texto[i], i + 1
            while fim < len(texto) and texto[fim] != aspa:
                fim += 2 if aspa != '`' and texto[fim] == '\\' else 1
            fim = min(fim + 1, len(texto))
            saida.append(texto[i:fim])
            i = fim
        else:
            saida.append(texto[i])
            i += 1
    return ''.join(saida)


TOKEN = re.compile(
    r'--|/\*|\'|"|(?<![A-Za-z0-9_$\x80-\U0010ffff])(?:[Ee]\'|\$(?:(?!\d)[\w\x80-\U0010ffff]+)?\$)'
)
ASPA, ASPA_OU_BARRA, BLOCO = re.compile("'"), re.compile(r"[\\']"), re.compile(r'/\*|\*/')


def fim_do_literal(texto, i, barra):
    while achado := (ASPA_OU_BARRA if barra else ASPA).search(texto, i):
        i = achado.start()
        if texto[i] != "'" or texto.startswith("''", i):
            i += 2
        else:
            return i + 1
    return None


def fim_do_bloco(texto, i):
    nivel = 1
    for marca in BLOCO.finditer(texto, i):
        nivel += 1 if marca.group() == '/*' else -1
        if nivel == 0:
            return marca.end()
    return None


def lexico_sql(texto):
    nomes, palavras, i, aberto = [], [], 0, False
    while i < len(texto):
        token = TOKEN.search(texto, i)
        comum = texto[i:token.start() if token else len(texto)]
        if comum:
            nomes.append(comum)
            palavras.append(comum)
            i += len(comum)
            continue
        marca, identificador = token.group(), False
        if marca == '--':
            fim = texto.find('\n', i)
            fim = len(texto) if fim < 0 else fim
        elif marca == '/*':
            fim = fim_do_bloco(texto, i + 2)
        elif marca.startswith('$'):
            fim = texto.find(marca, token.end())
            fim = None if fim < 0 else fim + len(marca)
        elif marca == "'":
            fim = fim_do_literal(texto, i + 1, False)
        elif marca != '"':
            fim = fim_do_literal(texto, i + 2, True)
        else:
            fim, identificador = texto.find('"', i + 1), True
            while fim >= 0 and texto.startswith('""', fim):
                fim = texto.find('"', fim + 2)
            fim = None if fim < 0 else fim + 1
        if fim is None:
            fim, aberto = len(texto), True
        trecho = texto[i:fim]
        if identificador:
            nomes.append(trecho)
            palavras.append('"' + re.sub(r'[^\n]', '_', trecho[1:-1]) + trecho[-1:])
        else:
            apagado = re.sub(r'[^\n]', ' ', trecho)
            nomes.append(apagado)
            palavras.append(apagado)
        i = fim
    return ''.join(nomes), ''.join(palavras), aberto


sql_path, provider = sys.argv[1], pathlib.Path(sys.argv[2])
try:
    nomes, sql, aberto = lexico_sql(open(sql_path, encoding='utf-8').read())
except OSError as erro:
    print(f'{sql_path}: {erro}')
    sys.exit(1)


def original(casamento, grupo, deslocamento=0):
    return nomes[casamento.start(grupo) + deslocamento:casamento.end(grupo) + deslocamento]


go = ''
for arquivo in sorted(provider.glob('*.go')):
    if arquivo.name.endswith('_test.go'):
        continue
    try:
        go += sem_comentarios(arquivo.read_text(encoding='utf-8')) + '\n'
    except OSError:
        continue

def profundidade(linha, nivel, crua):
    i = 0
    while i < len(linha):
        c = linha[i]
        if crua:
            crua = c != '`'
        elif c == '`':
            crua = True
        elif c in '"\'':
            i += 1
            while i < len(linha) and linha[i] != c:
                i += 2 if linha[i] == '\\' else 1
        elif c in '([{':
            nivel += 1
        elif c in ')]}':
            nivel -= 1
        i += 1
    return nivel, crua


def itens_do_bloco(pedaco):
    itens, atual, nivel, crua = {}, None, 0, False
    for linha in pedaco.split('\n')[1:]:
        if nivel == 0 and not crua and linha.strip():
            if linha.strip() == ')':
                break
            item = re.match(r'(?:\t| {4})(\w+)\b[^=]*?=(.*)', linha)
            atual = item.group(1) if item else None
            if atual:
                itens[atual] = item.group(2) + '\n'
        elif atual:
            itens[atual] += linha + '\n'
        nivel, crua = profundidade(linha, nivel, crua)
    return itens


variaveis, funcoes = {}, {}
for pedaco in re.split(r'(?m)^(?=(?:var|func|type|const|import)\b)', go):
    if pedaco.startswith('func'):
        nome = re.match(r'func\s+(?:\([^)]*\)\s*)?(\w+)', pedaco)
        if nome:
            funcoes[nome.group(1)] = pedaco
    elif re.match(r'var\s*\(', pedaco):
        variaveis.update(itens_do_bloco(pedaco))
    elif pedaco.startswith('var'):
        item = re.match(r'var\s+(\w+)[^=\n]*=(.*)', pedaco, re.DOTALL)
        if item:
            variaveis[item.group(1)] = item.group(2)

DECLARA = re.compile(r'\bUnique\s*:|\.Unique\s*=')
com_unique = {f for f, corpo in funcoes.items() if DECLARA.search(corpo)}
tabelas = sorted(
    nome for nome, texto in variaveis.items()
    if nome != 'Uniques'
    and (DECLARA.search(texto) or com_unique & set(re.findall(r'\b(\w+)\s*\(', texto)))
)
listadas = set(re.findall(r'\b(\w+)\.Uniques\(\)', variaveis.get('Uniques', '')))

constantes = dict(re.findall(r'\bconst\s+(\w+)\s*(?:[\w.]+\s*)?=\s*"(\w+)"', go))
for bloco in re.findall(r'(?ms)^const\s*\((.*?)^\)', go):
    constantes.update(re.findall(r'(?m)^\s+(\w+)\s*(?:[\w.]+\s*)?=\s*"(\w+)"', bloco))
declaradas, opacos = set(), []
for literal in re.finditer(r'\b\w+\.Unique\s*\{', go):
    nivel, fim = 0, literal.end() - 1
    while fim < len(go):
        nivel += {'{': 1, '}': -1}.get(go[fim], 0)
        if nivel == 0:
            break
        fim += 1
    for valor in re.findall(r'\bName\s*:\s*([^,\n}]+)', go[literal.end():fim]):
        valor = valor.strip()
        if re.fullmatch(r'"\w+"', valor):
            declaradas.add(valor.strip('"'))
        elif valor in constantes:
            declaradas.add(constantes[valor])
        else:
            opacos.append(valor)

achados = [f'literal/comentário SQL não terminado em {sql_path}'] if aberto else []
achados += [
    f'{provider}: postgres.Unique com Name {valor}; o Name precisa ser literal ou const resolvível no provider'
    for valor in opacos
]
restricoes = (original(c, 1) for c in re.finditer(r'CONSTRAINT\s+"?(\w+)"?\s+UNIQUE\b', sql, re.IGNORECASE))
achados += [
    f'{sql_path}: constraint {restricao} UNIQUE sem postgres.Unique de mesmo Name no provider'
    for restricao in restricoes
    if restricao not in declaradas
]
FORMA = 'CONSTRAINT {}_<colunas>_key UNIQUE (tenant_id, ...), a única forma que postgres.Unique e VerifyUniques reconhecem'
ANTES_DO_UNIQUE = r'(?=UNIQUE(?![\w$\x80-\U0010ffff]))'
nomeadas = {c.end() for c in re.finditer(r'CONSTRAINT\s+"?\w+"?\s*' + ANTES_DO_UNIQUE, sql, re.IGNORECASE)}
de_indice = {c.end() for c in re.finditer(r'\bCREATE\s+' + ANTES_DO_UNIQUE, sql, re.IGNORECASE)}
INDICE = re.compile(
    r'\s+INDEX\s+(?:CONCURRENTLY\s+)?(?:IF\s+NOT\s+EXISTS\s+)?"?(\w+)"?\s+ON\s+(?:ONLY\s+)?(?:(?:public|"\w+")\.)?"?(\w+)"?',
    re.IGNORECASE,
)
ALTER_TABLE = re.compile(r'\s*ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?(?:(?:public|"\w+")\.)?"?(\w+)"?', re.IGNORECASE)
criacoes = list(re.finditer(r'CREATE\s+TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?(?:(?:public|"\w+")\.)?"?(\w+)"?', sql, re.IGNORECASE))
fins_de_criacao = [c.end() for c in criacoes]
pontos = [c.start() for c in re.finditer(';', sql)]
alteracoes = {}
for unique in re.finditer(r'(?<![\w$\x80-\U0010ffff])UNIQUE(?![\w$\x80-\U0010ffff])', sql, re.IGNORECASE):
    if unique.start() in nomeadas:
        continue
    indice = INDICE.match(sql, unique.end())
    if unique.start() in de_indice and indice:
        indexado, tabela = original(indice, 1), original(indice, 2)
        achados.append(
            f'{sql_path}: CREATE UNIQUE INDEX {indexado} em {tabela} não vira rejeição (índice não tem linha em pg_constraint); '
            f'declare no CREATE TABLE {FORMA.format(tabela)}'
        )
        continue
    ponto = bisect_left(pontos, unique.start())
    inicio = pontos[ponto - 1] + 1 if ponto else 0
    if inicio not in alteracoes:
        alteracoes[inicio] = ALTER_TABLE.match(sql, inicio)
    alterada = alteracoes[inicio]
    if alterada and alterada.end() > unique.start():
        alterada = ALTER_TABLE.match(sql, inicio, unique.start())
    criada = bisect_right(fins_de_criacao, unique.start())
    tabela = original(alterada, 1) if alterada else original(criacoes[criada - 1], 1) if criada else '?'
    achados.append(f'{sql_path}: UNIQUE sem nome na tabela {tabela}; declare-a como {FORMA.format(tabela)}')

FRONTEIRA = r'(?<![\w$\x80-\U0010ffff]){}(?![\w$\x80-\U0010ffff])'
NOME = r'(?:(?:public|"\w+")\.)?"?(\w+)"?'
CRIADA = re.compile(
    r'\s*CREATE\s+(?:(?:GLOBAL|LOCAL)\s+)?(?:(?:TEMPORARY|TEMP|UNLOGGED)\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?' + NOME, re.IGNORECASE,
)
ALTERADA = re.compile(r'\s*ALTER\s+TABLE\s+(?:IF\s+EXISTS\s+)?(?:ONLY\s+)?' + NOME + r'(?:\s*\*)?', re.IGNORECASE)
TEM_UNIQUE = re.compile(FRONTEIRA.format('UNIQUE'), re.IGNORECASE)
DA_CRIACAO = re.compile(FRONTEIRA.format(r'(PARTITION\s+(?:BY|OF)|INHERITS)'), re.IGNORECASE)
HERDA = re.compile(r'\s*(NO\s+)?INHERIT(?![\w$\x80-\U0010ffff])', re.IGNORECASE)
ANEXA = re.compile(FRONTEIRA.format(r'ATTACH\s+PARTITION') + r'\s+' + NOME, re.IGNORECASE)
SEGUINTE = re.compile(r'\s+' + NOME)
ABRE_LISTA = re.compile(r'\s*\(')
NA_LISTA = re.compile(r'[\s,]*' + NOME)
PARTICAO = 'o Postgres reporta a constraint da partição folha, não a declarada na tabela'
HERANCA = 'a UNIQUE vale por tabela e não cobre a hierarquia de herança'


nivel = [0, *accumulate(map({'(': 1, ')': -1}.get, sql, repeat(0)))]


def maes(texto, clausula, pos, ultimo_fecha=-1):
    if clausula in ('PARTITION OF', 'INHERIT'):
        mae = SEGUINTE.match(texto, pos)
        return [mae] if mae else []
    abre = ABRE_LISTA.match(texto, pos)
    if not abre or ultimo_fecha < abre.end():
        return []
    listadas, item = [], NA_LISTA.match(texto, abre.end())
    while item:
        listadas.append(item)
        item = NA_LISTA.match(texto, item.end())
    return listadas


com_unicidade, clausulas, exibidas = set(), {}, {}


def marcar(casamento, deslocamento, clausula=None, razao=None):
    nome = original(casamento, 1, deslocamento)
    chave = nome.lower()
    exibidas.setdefault(chave, nome)
    if clausula:
        clausulas.setdefault(chave, []).append((clausula, razao))
    return chave


def ligar(filha, mae, deslocamento, clausula, razao):
    marcar(filha, deslocamento, clausula, razao)
    marcar(mae, deslocamento, f'{clausula} como mãe de {original(filha, 1, deslocamento)}', razao)


inicio = 0
for comando in sql.split(';'):
    criada, alterada = CRIADA.match(comando), ALTERADA.match(comando)
    alvo = criada or alterada
    if alvo and TEM_UNIQUE.search(comando):
        com_unicidade.add(marcar(alvo, inicio))
    if criada:
        ultimo_fecha = comando.rfind(')')
        for achada in DA_CRIACAO.finditer(comando):
            if nivel[inicio + achada.start()] != nivel[inicio]:
                continue
            clausula = ' '.join(achada.group(1).upper().split())
            razao = HERANCA if clausula == 'INHERITS' else PARTICAO
            marcar(criada, inicio, clausula, razao)
            for mae in maes(comando, clausula, achada.end(), ultimo_fecha):
                ligar(criada, mae, inicio, clausula, razao)
    elif alterada:
        acoes = [alterada.end()] + [
            virgula.end() for virgula in re.finditer(',', comando) if nivel[inicio + virgula.start()] == nivel[inicio]
        ]
        for acao in acoes:
            herda = HERDA.match(comando, acao)
            if herda and not herda.group(1):
                marcar(alterada, inicio, 'INHERIT', HERANCA)
                for mae in maes(comando, 'INHERIT', herda.end()):
                    ligar(alterada, mae, inicio, 'INHERIT', HERANCA)
        for anexa in ANEXA.finditer(comando):
            ligar(anexa, alterada, inicio, 'ATTACH PARTITION', PARTICAO)
    inicio += len(comando) + 1
for chave, usadas in clausulas.items():
    if chave in com_unicidade:
        clausula, razao = usadas[0]
        achados.append(
            f'{sql_path}: tabela {exibidas[chave]} declara UNIQUE e usa {clausula}, que a rejeição por unicidade não cobre '
            f'({razao}); declare a UNIQUE em tabela sem partição nem herança'
        )
achados += [
    f'{provider}: {tabela} declara Unique e falta em provider.Uniques ({tabela}.Uniques())'
    for tabela in tabelas
    if tabela not in listadas
]
for achado in achados:
    print(achado)
sys.exit(1 if achados else 0)
PY
  return $?
}

# O forbidigo não vê o método chamado por interface local (IDN-14, ADR-068).
verificar_plataforma() {
  local base="$1" ctx="$2" arquivo linha trecho rel achou=0
  while IFS=: read -r arquivo linha trecho; do
    rel="${arquivo#"$base"/}"
    case "$rel" in
      appkit/* | distkit/* | testkit/*) continue ;;
    esac
    case "$trecho" in
      WithVerifiedPlatformReach) printf '%s: %s:%s usa WithVerifiedPlatformReach, que só a borda REST chama (IDN-14, ADR-068)\n' "$ctx" "$rel" "$linha" ;;
      *) printf '%s: %s:%s usa WithPlatformReach, que só o kernel de borda monta (IDN-14, ADR-068)\n' "$ctx" "$rel" "$linha" ;;
    esac
    achou=1
  done < <(find "$base" -type f -name '*.go' ! -name '*_test.go' -print0 2>/dev/null |
    sort -z | xargs -0 -r grep -HnoE 'With(Verified)?PlatformReach')
  while IFS=: read -r arquivo linha trecho; do
    rel="${arquivo#"$base"/}"
    case "$rel" in
      provider/platform_*/*) ;;
      provider/platform_*.go | appkit/* | distkit/* | testkit/*) continue ;;
    esac
    printf '%s: %s:%s usa %s fora de provider/platform_*.go (IDN-14, ADR-068)\n' \
      "$ctx" "$rel" "$linha" "$(grep -oE 'PlatformReader|Projector' <<<"$trecho" | head -n 1)"
    achou=1
  done < <(find "$base" -type f -name '*.go' ! -name '*_test.go' -print0 2>/dev/null |
    sort -z | xargs -0 -r grep -HnoE '(^|[^A-Za-z0-9_])(PlatformReader|Projector)[[:space:]]*\(|\.(PlatformReader|Projector)([^[A-Za-z0-9_]|$)')
  return "$achou"
}

# verificar_infra confere DSN, POSTGRES_DB, o laço ou bloco que cria banco e role
# por app e onde a credencial da migradora aparece (ADR-063). Lê o YAML pela
# indentação para não exigir PyYAML de quem roda o gate fora do platform.
verificar_infra() {
  local repo="$1"
  shift
  python3 - "$repo" "$@" <<'PY'
import json
import os
import pathlib
import re
import sys

repo, apps = pathlib.Path(sys.argv[1]), set(sys.argv[2:])
APPS = os.environ.get('DMPF_APPS_DIR', 'apps/backend')
ADMIN = 'postgres'
DSN = re.compile(r'postgres(?:ql)?://((?:\$\{[^}]*\}|[^:@/\s"\'])+)(?::[^@\s"\']*)?@[^/\s"\']+/([^?\s"\']+)')
DEFAULT = re.compile(r'\$\{[A-Z_]+:-([^}]*)\}')
POSTGRES_DB = re.compile(r'POSTGRES_DB\s*[:=]\s*["\']?([^\s"\'}]+)')
LOOP = re.compile(r'for pair in (.*?);\s*do', re.DOTALL)
CHAVE = re.compile(r'([A-Z_][A-Z0-9_]*)["\']?\s*[:=]\s*["\']?$')
PAIR = re.compile(r'(?<![\w-])([a-z][a-z0-9_-]*):\$')
BLOCO = re.compile(r'CREATE DATABASE \\?"([a-z0-9][a-z0-9_-]*)\\?"')
KV_USER = re.compile(r'(?<![\w.-])user=(\$\{[^}]*\}|[^\s"\'&;]+)')
KV_DB = re.compile(r'(?<![\w.-])dbname=(\$\{[^}]*\}|[^\s"\'&;]+)')
PGUSER = re.compile(r'(?<![\w])PGUSER["\']?\s*[:=]\s*["\']?(\$\{[^}]*\}|[\w-]+)')
MIGRATE_KEY = re.compile(r'(?<![\w])PG_MIGRATE_DSN["\']?\s*[:=]')
CHAVE_DA_LINHA = re.compile(r'^\s*(?:-\s+)?(?:export\s+)?["\']?([A-Z_][A-Z0-9_]*)["\']?\s*[:=]')
ITEM = re.compile(r'^("[^"]*"|\'[^\']*\'|[^\s:#\'"{}\[\],][^:#]*?)\s*:(?:\s+(.*))?$')
ESCALAR = re.compile(r'^[|>][-+0-9]*$')
ALIAS = re.compile(r'(?:^|[\s\[{,:])\*([\w.-]+)')
SECRETS = {'Secret', 'SealedSecret', 'ExternalSecret'}
MIGRATOR = '_migrator'
try:
    PERFIL = json.loads((repo / 'dmpf.json').read_text(encoding='utf-8')).get('composeProfile') or 'dmpf'
except (OSError, ValueError, AttributeError):
    PERFIL = 'dmpf'


def literal(value):
    return DEFAULT.sub(lambda m: m.group(1), value)


def sem_comentario(linha):
    return re.sub(r'(^|\s)#.*$', '', linha).rstrip()


def empilhar(pilha, recuo, chave, valor):
    ancora = None
    if valor.startswith('&'):
        ancora, _, valor = valor[1:].partition(' ')
        valor = valor.strip()
    pilha.append({'recuo': recuo, 'chave': chave, 'ancora': ancora, 'item': False, 'aberto': not valor, 'nome': None, 'valor': valor})
    return recuo if ESCALAR.match(valor) else None


def estrutura(texto):
    pilha, docs, linhas, escalar = [], [{}], [], None
    for bruta in texto.splitlines():
        limpa = bruta.rstrip()
        conteudo = limpa.lstrip(' ')
        recuo = len(limpa) - len(conteudo)
        if escalar is not None:
            if not conteudo or recuo > escalar:
                linhas.append((len(docs) - 1, tuple(pilha), True))
                continue
            escalar = None
        conteudo = sem_comentario(conteudo)
        if not conteudo:
            linhas.append((len(docs) - 1, tuple(pilha), False))
            continue
        if recuo == 0 and conteudo.startswith('---'):
            docs.append({})
            pilha = []
            linhas.append((len(docs) - 1, (), False))
            continue
        if conteudo == '-' or conteudo.startswith('- '):
            while pilha and pilha[-1]['recuo'] > recuo:
                pilha.pop()
            if pilha and pilha[-1]['recuo'] == recuo and (pilha[-1]['item'] or not pilha[-1]['aberto']):
                pilha.pop()
            item = {'recuo': recuo, 'chave': None, 'ancora': None, 'item': True, 'aberto': False, 'nome': None, 'valor': ''}
            pilha.append(item)
            resto = conteudo[1:].lstrip()
            achado = ITEM.match(resto) if resto else None
            if ESCALAR.match(resto):
                escalar = recuo
            elif achado:
                chave, valor = achado.group(1).strip('\'"'), (achado.group(2) or '').strip()
                if chave == 'name':
                    item['nome'] = valor.strip('\'"')
                escalar = empilhar(pilha, recuo + len(conteudo) - len(resto), chave, valor)
            else:
                item['valor'] = resto
        else:
            while pilha and pilha[-1]['recuo'] >= recuo:
                pilha.pop()
            achado = ITEM.match(conteudo)
            if achado:
                chave, valor = achado.group(1).strip('\'"'), (achado.group(2) or '').strip()
                if pilha and pilha[-1]['item'] and chave == 'name':
                    pilha[-1]['nome'] = valor.strip('\'"')
                escalar = empilhar(pilha, recuo, chave, valor)
                if recuo == 0:
                    docs[-1].setdefault('topo', set()).add(chave)
                    if chave == 'kind':
                        docs[-1]['kind'] = valor.strip('\'"')
                if chave == 'name' and len(pilha) == 2 and pilha[0]['chave'] == 'metadata' and pilha[0]['recuo'] == 0:
                    docs[-1]['name'] = valor.strip('\'"')
        linhas.append((len(docs) - 1, tuple(dict(n) for n in pilha), False))
    return linhas, docs


def chave_de(linha, pilha):
    if pilha and pilha[-1]['chave'] == 'value':
        for no in reversed(pilha[:-1]):
            if no['item']:
                return no['nome']
    if pilha and pilha[-1]['chave'] and re.fullmatch(r'[A-Z_][A-Z0-9_]*', pilha[-1]['chave']):
        return pilha[-1]['chave']
    achada = CHAVE_DA_LINHA.match(linha)
    return achada.group(1) if achada else None


def servicos_api(app):
    return {f'{app}-api', f'{PERFIL}-{app}-api'}


def yaml_de(arquivo):
    return arquivo.suffix in ('.yml', '.yaml') or arquivo.name.endswith(('.yml.tmpl', '.yaml.tmpl'))


achados = []
arquivos = []
raizes = [repo / 'infra', repo / '.github/workflows'] + sorted((repo / APPS).glob('*/deploy'))
for base in raizes:
    if base.is_dir():
        arquivos += [p for p in sorted(base.rglob('*')) if p.is_file() and p.suffix != '.md']
for app in sorted(apps):
    manifesto = repo / APPS / app / 'deploy/infra.json'
    try:
        declarado = json.loads(manifesto.read_text(encoding='utf-8'))
    except (OSError, ValueError) as erro:
        achados.append(f'{manifesto.relative_to(repo)}: manifesto de infra ilegível ou ausente ({erro})')
        continue
    if declarado.get('app') != app or not declarado.get('database'):
        achados.append(f'{manifesto.relative_to(repo)}: o contexto {app} precisa declarar app "{app}" e o próprio database')

textos, estruturas, estendidos = {}, {}, set()
for arquivo in arquivos:
    try:
        textos[arquivo] = arquivo.read_text(encoding='utf-8')
    except (OSError, UnicodeDecodeError):
        continue
    if not yaml_de(arquivo):
        continue
    linhas, docs = estrutura(textos[arquivo])
    if arquivo.name in ('kustomization.yaml', 'kustomization.yml'):
        for doc in docs:
            doc.setdefault('kind', 'Kustomization')
    estruturas[arquivo] = (linhas, docs)
    for doc, pilha, escalar in linhas:
        if escalar or 'services' not in docs[doc].get('topo', set()) or not pilha:
            continue
        ultimo = pilha[-1]
        if ultimo['chave'] == 'service' and any(n['chave'] == 'extends' for n in pilha[:-1]):
            estendidos.add(ultimo['valor'].strip('\'"'))
        elif ultimo['chave'] == 'extends' and ultimo['valor']:
            alvo = re.search(r'service:\s*["\']?([\w.-]+)', ultimo['valor'])
            estendidos.add(alvo.group(1) if alvo else ultimo['valor'].strip('\'"'))


def fora_do_lugar(arquivo, numero, app, rotulo):
    if arquivo not in estruturas:
        return None
    linhas, docs = estruturas[arquivo]
    doc, pilha, _ = linhas[numero - 1]
    info = docs[doc]
    if 'kind' in info:
        secrets = {f'{app}-migrator'} if app else {f'{a}-migrator' for a in apps}
        if info['kind'] in SECRETS and info.get('name') in secrets:
            return None
        if info['kind'] == 'Kustomization' and any(n['chave'] == 'secretGenerator' for n in pilha) and any(
            n['item'] and n['nome'] in secrets for n in pilha
        ):
            return None
        return f'{rotulo} fora do Secret {" ou ".join(sorted(secrets))}, o único que só o Deployment do api lê'
    if 'services' not in info.get('topo', set()):
        return None
    topo = pilha[0]['chave'] if pilha and pilha[0]['recuo'] == 0 and not pilha[0]['item'] else None
    if topo != 'services' or len(pilha) < 2:
        return f'{rotulo} em {topo or "raiz"}, fora do serviço {app or "<app>"}-api: o que fica fora dele chega a relay e consumer'
    servico = pilha[1]['chave']
    donos = [app] if app else [a for a in apps if servico in servicos_api(a)]
    if not donos or servico not in servicos_api(donos[0]):
        return f'{rotulo} no serviço {servico}; só o serviço {app or "<app>"}-api a recebe'
    aliases = {a for linha in textos[arquivo].splitlines() for a in ALIAS.findall(sem_comentario(linha))}
    for no in pilha:
        if no['ancora'] and no['ancora'] in aliases:
            return f'{rotulo} sob a âncora &{no["ancora"]}, que *{no["ancora"]} replica em outro nó'
    if servico in estendidos:
        return f'{rotulo} no serviço {servico}, que outro serviço herda por extends'
    return None


for arquivo, texto in textos.items():
    nome = arquivo.relative_to(repo)
    linhas = estruturas[arquivo][0] if arquivo in estruturas else None
    for numero, linha in enumerate(texto.splitlines(), 1):
        pilha = linhas[numero - 1][1] if linhas else ()
        usos = {}
        for dsn in DSN.finditer(linha):
            user, db = literal(dsn.group(1)), literal(dsn.group(2))
            if (user, db) == (ADMIN, ADMIN) or (db in apps and user == db):
                continue
            chave = CHAVE.search(linha[:dsn.start()])
            chave = chave.group(1) if chave else chave_de(linha, pilha)
            if db in apps and user == db + MIGRATOR:
                if chave == 'PG_MIGRATE_DSN':
                    usos[db] = f'a role {user}'
                    continue
                achados.append(f'{nome}:{numero}: a role {user} só cabe no PG_MIGRATE_DSN, não em {chave or "DSN sem chave"}')
                continue
            achados.append(
                f'{nome}:{numero}: DSN com role {user} e banco {db} '
                f'(esperado o nome da app nos dois, a role <app>{MIGRATOR} no PG_MIGRATE_DSN do banco da app, ou {ADMIN}/{ADMIN})'
            )
        for kv in KV_USER.finditer(linha):
            user = literal(kv.group(1))
            if not user.endswith(MIGRATOR):
                continue
            app = user[: -len(MIGRATOR)]
            db = KV_DB.search(linha)
            db = literal(db.group(1)) if db else None
            if app not in apps or db != app:
                achados.append(f'{nome}:{numero}: DSN chave/valor com role {user} e banco {db or "ausente"} (esperado dbname={app}, o banco da app)')
                continue
            chave = chave_de(linha, pilha)
            if chave != 'PG_MIGRATE_DSN':
                achados.append(f'{nome}:{numero}: a role {user} só cabe no PG_MIGRATE_DSN, não em {chave or "DSN sem chave"}')
                continue
            usos[app] = f'a role {user}'
        pguser = [literal(m.group(1)) for m in PGUSER.finditer(linha)]
        if pilha and pilha[-1]['chave'] == 'value' and chave_de(linha, pilha) == 'PGUSER':
            pguser.append(literal(pilha[-1]['valor'].strip('\'"')))
        for user in pguser:
            if user.endswith(MIGRATOR):
                achados.append(f'{nome}:{numero}: a role {user} só cabe no PG_MIGRATE_DSN, não em PGUSER')
        if linhas and not usos and (MIGRATE_KEY.search(sem_comentario(linha)) or chave_de(linha, pilha) == 'PG_MIGRATE_DSN'):
            usos[None] = 'PG_MIGRATE_DSN'
        for app, rotulo in usos.items():
            motivo = fora_do_lugar(arquivo, numero, app, rotulo)
            if motivo:
                achados.append(f'{nome}:{numero}: {motivo}')
    if linhas:
        docs = estruturas[arquivo][1]
        for app in sorted(apps):
            token = re.compile(rf'(?<![\w.-]){re.escape(app)}-migrator(?![\w.-])')
            for numero, linha in enumerate(texto.splitlines(), 1):
                doc, pilha, _ = linhas[numero - 1]
                info = docs[doc]
                if 'kind' not in info or not token.search(sem_comentario(linha)) or info['kind'] in SECRETS:
                    continue
                if info['kind'] == 'Kustomization' and pilha and pilha[-1]['chave'] == 'name' and any(
                    n['chave'] == 'secretGenerator' for n in pilha
                ):
                    continue
                if info['kind'] == 'Deployment' and info.get('name') == f'{app}-api':
                    continue
                onde = f'{info["kind"]} {info.get("name", "")}'.rstrip()
                achados.append(f'{nome}:{numero}: o Secret {app}-migrator só cabe no Deployment {app}-api, não em {onde}')
    for db in POSTGRES_DB.findall(texto):
        if literal(db) != ADMIN:
            achados.append(f'{nome}: POSTGRES_DB={db} (o banco administrativo é {ADMIN})')
    if re.search(r'CREATE DATABASE', texto):
        declaradas = {app for laco in LOOP.findall(texto) for app in PAIR.findall(laco)}
        declaradas |= set(BLOCO.findall(texto))
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
  if [[ "$status" -eq 0 ]]; then
    aprovar "$rotulo"
    return
  fi
  while IFS= read -r linha; do
    [[ -n "$linha" ]] && falhar "$linha"
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

  if [[ "$achou" -eq 0 ]]; then
    if [[ "$PERMITIR_VAZIO" -eq 1 ]]; then
      echo "nenhum bounded context em $raiz ainda: nada a verificar"
      return 0
    fi
    echo "nenhum bounded context encontrado em $raiz" >&2
    return 2
  fi

  while IFS= read -r app; do
    printf '\n== %s (borda) ==\n' "$app"
    verificar_borda "$raiz/$app" "$app"
  done < <(bordas "$raiz")

  printf '\n== nomes no DDL ==\n'
  for ctx in "${ctxs[@]}"; do
    [[ -f "$raiz/$ctx/provider/schema.sql" ]] || continue
    saida="$(verificar_ddl contexto "$raiz/$ctx/provider/schema.sql")"
    status=$?
    relatar "$ctx: DDL nos nomes canônicos" "$saida" "$status"
    saida="$(verificar_grants "$raiz/$ctx")"
    status=$?
    relatar "$ctx: toda tabela do schema.sql com grant no provider.Grants" "$saida" "$status"
    saida="$(verificar_uniques "$raiz/$ctx")"
    status=$?
    relatar "$ctx: toda UNIQUE do schema.sql declarada no provider e em provider.Uniques" "$saida" "$status"
  done

  printf '\n== leitura de plataforma, projeção e alcance no código do contexto ==\n'
  for ctx in "${ctxs[@]}"; do
    saida="$(verificar_plataforma "$raiz/$ctx" "$ctx")"
    status=$?
    relatar "$ctx: leitura de plataforma e projeção só em provider/platform_*.go" "$saida" "$status"
  done

  local -a kernel=()
  local ddl="$repo/$KERNEL_DDL"
  [[ "$KERNEL_DDL" == /* ]] && ddl="$KERNEL_DDL"
  mapfile -t kernel < <(find "$ddl" -maxdepth 1 -name '*.sql' 2>/dev/null | sort)
  if [[ "$KERNEL_DDL" = none ]]; then
    aprovar "kernel: nenhum contexto usa o postgres do kernel, sem DDL a conferir"
  elif [[ "${#kernel[@]}" -gt 0 ]]; then
    saida="$(verificar_ddl kernel "${kernel[@]}")"
    status=$?
    relatar "kernel: DDL nos nomes canônicos" "$saida" "$status"
  else
    relatar "kernel: DDL nos nomes canônicos" "nenhum .sql do kernel em $ddl (DMPF_KERNEL_DDL)" 1
  fi

  printf '\n== banco e role em infra/, apps/backend/*/deploy e .github/workflows ==\n'
  saida="$(verificar_infra "$repo" "${ctxs[@]}")"
  status=$?
  relatar "banco e role com o nome da app (ou <app>_migrator no PG_MIGRATE_DSN), ou o par administrativo" "$saida" "$status"

  if [[ "$FALHAS" -gt 0 ]]; then
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
  [[ -d "$base/domain" ]] || { echo "$base não é um bounded context (falta domain/)" >&2; return 2; }
  printf '\n== %s ==\n' "$ctx"
  verificar_contexto "$base" "$ctx"
  if [[ -f "$base/provider/schema.sql" ]]; then
    saida="$(verificar_ddl contexto "$base/provider/schema.sql")"
    status=$?
    relatar "$ctx: DDL nos nomes canônicos" "$saida" "$status"
    saida="$(verificar_grants "$base")"
    status=$?
    relatar "$ctx: toda tabela do schema.sql com grant no provider.Grants" "$saida" "$status"
    saida="$(verificar_uniques "$base")"
    status=$?
    relatar "$ctx: toda UNIQUE do schema.sql declarada no provider e em provider.Uniques" "$saida" "$status"
  fi
  saida="$(verificar_plataforma "$base" "$ctx")"
  status=$?
  relatar "$ctx: leitura de plataforma e projeção só em provider/platform_*.go" "$saida" "$status"
  if [[ "$FALHAS" -gt 0 ]]; then
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
  tenant_id text  NOT NULL,
  probe_id  text  NOT NULL,
  snapshot  jsonb NOT NULL,
  label     text  GENERATED ALWAYS AS (snapshot->>'label') STORED NOT NULL,
  CONSTRAINT probes_pkey PRIMARY KEY (tenant_id, probe_id),
  CONSTRAINT probes_tenant_id_label_key UNIQUE (tenant_id, label)
);
CREATE INDEX IF NOT EXISTS probes_probe_id_idx ON probes (probe_id);
SQL
  cat > "$ctx/provider/schema.go" <<'GO'
package provider

var Grants = []postgres.Grant{postgres.Mutable("probes")}

var Uniques = []postgres.UniqueSet{probeTable.Uniques()}
GO
  cat > "$ctx/provider/probe_repository.go" <<'GO'
package provider

var probeTable = withUniqueLabel(postgres.SnapshotTable("probes", "probe_id", encode, decode, withID))

func withUniqueLabel(table postgres.Table[domain.ProbeID, domain.ProbeSnapshot]) postgres.Table[domain.ProbeID, domain.ProbeSnapshot] {
	table.Unique = []postgres.Unique{{
		Name:      "probes_tenant_id_label_key",
		Columns:   []string{"tenant_id", "label"},
		Rejection: domain.RejectProbeDuplicateLabel,
	}}
	return table
}

func NewProbeRepository(tx *postgres.Tx) ports.Repository[domain.ProbeID, domain.ProbeSnapshot] {
	return probeTable.Repository(tx)
}
GO
  cat > "$ctx/project.json" <<'JSON'
{"targets":{"serve-api":{},"serve-relay":{},"test-distributed":{}}}
JSON
  cat > "$ctx/dmpf-units.json" <<'JSON'
{"units":[{"id":"probe/app"},{"id":"probe/appkit"},{"id":"probe/distkit"}]}
JSON

  mkdir -p "$ctx/deploy"
  cat > "$ctx/deploy/infra.json" <<'JSON'
{"schema":"dmpf/infra@1","app":"probe","database":{"passwordEnv":"PROBE_PG_PASSWORD","local":"probe-local","dev":"probe-dev"}}
JSON
  cat > "$ctx/deploy/compose.yml" <<'YAML'
x-env: &env
  PG_DSN: postgres://probe:${PROBE_PG_PASSWORD:-probe-local}@postgres:5432/probe?sslmode=disable

services:
  probe-api:
    environment:
      <<: *env
  probe-relay:
    environment:
      <<: *env
YAML

  mkdir -p "$bff"/{app/api,app/rpc,cmd,deploy}
  cat > "$bff/deploy/infra.json" <<'JSON'
{"schema":"dmpf/infra@1","app":"bff"}
JSON
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

variar() {
  local repo="$1" tipo="$2" ctx="$1/$APPS/probe"
  case "$tipo" in
    com-migrador)
      printf '{"schema":"dmpf/infra@1","app":"probe","database":{"passwordEnv":"PROBE_PG_PASSWORD","local":"probe-local","dev":"probe-dev","migrator":{"passwordEnv":"PROBE_PG_MIGRATOR_PASSWORD","local":"probe-migrator-local","dev":"probe-migrator-dev"}}}\n' > "$ctx/deploy/infra.json"
      sed -i '/^  probe-api:$/,/^  probe-relay:$/ s|^      <<: \*env$|&\n      PG_MIGRATE_DSN: postgres://probe_migrator:${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local}@postgres:5432/probe?sslmode=disable|' \
        "$ctx/deploy/compose.yml"
      mkdir -p "$ctx/deploy/k8s/base" "$ctx/deploy/k8s/overlays/dev"
      cat > "$ctx/deploy/k8s/overlays/dev/kustomization.yaml" <<'YAML'
secretGenerator:
  - name: probe
    literals:
      - PG_DSN=postgres://probe:probe-dev@postgres-0.postgres:5432/probe?sslmode=disable
  - name: probe-migrator
    literals:
      - PG_MIGRATE_DSN=postgres://probe_migrator:probe-migrator-dev@postgres-0.postgres:5432/probe?sslmode=disable
YAML
      local papel
      for papel in api relay; do
        cat > "$ctx/deploy/k8s/base/deployment-$papel.yaml" <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: probe-$papel
spec:
  template:
    spec:
      containers:
        - name: $papel
          envFrom:
            - secretRef:
                name: probe
YAML
      done
      printf '            - secretRef:\n                name: probe-migrator\n' >> "$ctx/deploy/k8s/base/deployment-api.yaml"
      sed -i 's/^    command: .*$/    command: set -e; psql -c "CREATE DATABASE \\"probe\\" OWNER \\"probe_migrator\\""/' \
        "$repo/infra/local/compose.yml"
      ;;
    com-migrador-chave-valor)
      variar "$repo" com-migrador
      sed -i 's|^      PG_MIGRATE_DSN: .*$|      PG_MIGRATE_DSN: "host=postgres user=probe_migrator password=${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local} dbname=probe"|' \
        "$ctx/deploy/compose.yml"
      ;;
    dois-contextos)
      variar "$repo" com-migrador
      cp -r "$ctx" "$repo/$APPS/other"
      find "$repo/$APPS/other/deploy" -type f -exec sed -i 's/probe/other/g; s/PROBE/OTHER/g' {} +
      sed -i 's/^    command: .*$/&; psql -c "CREATE DATABASE \\"other\\" OWNER \\"other_migrator\\""/' "$repo/infra/local/compose.yml"
      ;;
    var-com-func-literal)
      cat > "$ctx/provider/probe_repository.go" <<'GO'
package provider

var (
	probeTable = postgres.Table[domain.ProbeID, domain.ProbeSnapshot]{
		Name: "probes",
		Encode: func(s probeState, code string) probeState {
			s.Code = code
			return s
		},
		Unique: []postgres.Unique{{
			Name:      "probes_tenant_id_label_key",
			Columns:   []string{"tenant_id", "label"},
			Rejection: domain.RejectProbeDuplicateLabel,
		}},
	}
	probeCodes = map[string]string{"a": "b"}
)
GO
      ;;
    name-por-const)
      sed -i 's/Name:      "probes_tenant_id_label_key",/Name:      probeLabelKey,/' "$ctx/provider/probe_repository.go"
      printf '\nconst probeLabelKey = "probes_tenant_id_label_key"\n' >> "$ctx/provider/probe_repository.go"
      ;;
    name-por-bloco-const)
      sed -i 's/Name:      "probes_tenant_id_label_key",/Name:      probeLabelKey,/' "$ctx/provider/probe_repository.go"
      printf '\nconst (\n\tprobeOther = "x"\n\tprobeLabelKey string = "probes_tenant_id_label_key"\n)\n' >> "$ctx/provider/probe_repository.go"
      ;;
    unique-em-comentario-sql)
      printf -- '-- labels are unique per tenant\n/* a unique label\n   per tenant */\n' >> "$ctx/provider/schema.sql"
      ;;
    unique-em-literal-sql)
      printf "COMMENT ON TABLE probes IS 'labels are unique; it''s per tenant';\n" >> "$ctx/provider/schema.sql"
      ;;
    unique-em-dollar-quote)
      printf 'COMMENT ON TABLE probes IS $$labels are unique$$;\n' >> "$ctx/provider/schema.sql"
      ;;
    unique-em-comentario-aninhado)
      printf '/* outer /* inner */ unique still comment */\n' >> "$ctx/provider/schema.sql"
      ;;
    unique-em-identificador-citado)
      sed -i 's/CONSTRAINT probes_tenant_id_label_key UNIQUE/CONSTRAINT "probes_tenant_id_label_key" UNIQUE/' "$ctx/provider/schema.sql"
      printf 'COMMENT ON COLUMN probes."unique" IS NULL;\n' >> "$ctx/provider/schema.sql"
      ;;
    unique-em-e-string)
      printf '%s\n' "COMMENT ON TABLE probes IS E'it\\'s unique';" >> "$ctx/provider/schema.sql"
      ;;
    constraint-por-alter)
      cat >> "$repo/$KERNEL_DDL/outbox.sql" <<'SQL'
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'outbox_id_check') THEN
    ALTER TABLE outbox ADD CONSTRAINT outbox_id_check CHECK (id > 0) NOT VALID;
  END IF;
END
$$;
SQL
      printf 'ALTER TABLE outbox VALIDATE CONSTRAINT outbox_id_check;\n' > "$repo/$KERNEL_DDL/outbox_validate.sql"
      ;;
    dolar-solto-nao-ascii)
      printf 'SELECT $%s ;\n' "$(printf 'é%.0s' $(seq 40))" >> "$ctx/provider/schema.sql"
      ;;
    unique-em-identificador-com-cifrao)
      printf 'COMMENT ON COLUMN probes.unique$flag IS NULL;\nCOMMENT ON COLUMN probes.flag$unique IS NULL;\n' >> "$ctx/provider/schema.sql"
      ;;
    particionada-sem-unique)
      printf 'CREATE TABLE IF NOT EXISTS probe_events (\n  tenant_id text NOT NULL,\n  at timestamptz NOT NULL,\n  CONSTRAINT probe_events_pkey PRIMARY KEY (tenant_id, at)\n) PARTITION BY RANGE (at);\n' >> "$ctx/provider/schema.sql"
      sed -i 's/postgres.Mutable("probes")}/postgres.Mutable("probes"), postgres.Mutable("probe_events")}/' "$ctx/provider/schema.go"
      ;;
    coluna-chamada-inherit)
      printf 'ALTER TABLE probes ADD COLUMN inherit text;\n' >> "$ctx/provider/schema.sql"
      ;;
    plataforma-em-platform-e-teste)
      local alvo
      for alvo in platform_probe_reader.go probe_reader_test.go; do
        printf 'package provider\n\nvar _ interface{ PlatformReader() }\nvar _ interface{ Projector() }\n' > "$ctx/provider/$alvo"
      done
      printf 'var _ = ports.WithPlatformReach\n' >> "$ctx/provider/probe_reader_test.go"
      ;;
    projetor-como-tipo-no-application)
      printf 'package application\n\ntype Resources struct {\n\tProjection ports.Projector[ID, S]\n\tReader     ports.PlatformReader[ID, S]\n}\n' \
        > "$ctx/application/probe_resources.go"
      ;;
    plataforma-nos-kits)
      local kit
      for kit in appkit distkit; do
        printf 'package %s\n\nvar _ interface{ PlatformReader() }\nvar _ = ports.WithPlatformReach\n' "$kit" > "$ctx/$kit/probe_platform.go"
      done
      ;;
    url-em-string-go)
      sed -i 's|^var probeTable = |var (\n\tdocs = fmt.Sprintf("https://example.com/%s", "x")\n\tsep  = '"'/'"'\n\tprobeTable = |; s|^\(\tprobeTable = .*\)$|\1\n)|' \
        "$ctx/provider/probe_repository.go"
      ;;
    *)
      printf 'variar: tipo desconhecido: %s\n' "$tipo" >&2
      return 2
      ;;
  esac
  return $?
}

migrar_no_relay() {
  local repo="$1" linha="$2"
  sed -i "/^  probe-relay:\$/,\$ s|^      <<: \\*env\$|&\\n$linha|" "$repo/$APPS/probe/deploy/compose.yml"
  return $?
}

sabotar() {
  local repo="$1" tipo="$2" ctx="$1/$APPS/probe"
  case "$tipo" in
    sem-binario) trash "$ctx/cmd/main.go" ;;
    leitor-por-interface-local) printf 'package provider\n\nvar _ interface{ PlatformReader() }\n' > "$ctx/provider/probe_reader.go" ;;
    projetor-por-interface-local) printf 'package provider\n\nvar _ interface{ Projector() }\n' > "$ctx/provider/probe_projection.go" ;;
    alcance-no-provider) printf 'package provider\n\nvar _ = ports.WithPlatformReach\n' > "$ctx/provider/probe_reach.go" ;;
    alcance-em-platform) printf 'package provider\n\nvar _ = ports.WithPlatformReach\n' > "$ctx/provider/platform_probe_reach.go" ;;
    alcance-verificado-em-rpc) printf 'package rpc\n\nvar _ = kernelhttp.WithVerifiedPlatformReach\n' > "$ctx/app/rpc/probe_reach.go" ;;
    alcance-verificado-em-platform) printf 'package provider\n\nvar _ = kernelhttp.WithVerifiedPlatformReach\n' > "$ctx/provider/platform_probe_reach.go" ;;
    leitor-por-interface-em-app) printf 'package app\n\nvar _ interface{ PlatformReader() }\n' > "$ctx/app/probe_reader.go" ;;
    projetor-por-interface-em-rpc) printf 'package rpc\n\nvar _ interface{ Projector() }\n' > "$ctx/app/rpc/probe_projection.go" ;;
    leitor-por-interface-em-cmd) printf 'package main\n\ntype leitor interface{ PlatformReader() }\n' > "$ctx/cmd/probe_reader.go" ;;
    projetor-em-subpacote-do-provider)
      mkdir -p "$ctx/provider/sub"
      printf 'package sub\n\nvar _ interface{ Projector() }\n' > "$ctx/provider/sub/platform_probe.go"
      ;;
    projetor-por-valor-de-metodo) printf 'package application\n\nvar _ = probes.Projector\n' > "$ctx/application/probe_projection.go" ;;
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
    bff-sem-manifesto) trash "$repo/$APPS/bff/deploy/infra.json" ;;
    tabela-prefixada) sed -i 's/probes/dmpf_probes/g' "$ctx/provider/schema.sql" ;;
    tabela-example) printf 'CREATE TABLE IF NOT EXISTS dmpf_example_x (id bigint);\n' >> "$ctx/provider/schema.sql" ;;
    indice-example) sed -i 's/probes_probe_id_idx/probes_example_idx/' "$ctx/provider/schema.sql" ;;
    tabela-do-kernel) printf 'CREATE TABLE IF NOT EXISTS outbox (id bigint);\n' >> "$ctx/provider/schema.sql" ;;
    indice-fora) sed -i 's/probes_probe_id_idx/idx_probe/' "$ctx/provider/schema.sql" ;;
    constraint-fora) sed -i 's/probes_pkey/pk_probes/' "$ctx/provider/schema.sql" ;;
    constraint-fora-por-alter) printf 'ALTER TABLE outbox ADD CONSTRAINT ck_outbox CHECK (id > 0);\n' > "$repo/$KERNEL_DDL/outbox_check.sql" ;;
    kernel-prefixado) sed -i 's/outbox/dmpf_outbox/g' "$repo/$KERNEL_DDL/outbox.sql" ;;
    dsn-compartilhado) sed -i 's#@postgres:5432/probe#@postgres:5432/dmpf#' "$repo/infra/local/compose.yml" ;;
    postgres-db) sed -i 's/POSTGRES_DB: postgres/POSTGRES_DB: dmpf/' "$repo/infra/local/compose.yml" ;;
    init-incompleto) sed -i 's/for pair in probe:/for pair in other:/' "$repo/infra/local/compose.yml" ;;
    ci-compartilhado) sed -i 's#localhost:5432/postgres#localhost:5432/dmpf#' "$repo/.github/workflows/ci.yml" ;;
    manifesto-ausente) trash "$ctx/deploy/infra.json" ;;
    manifesto-sem-banco) printf '{"schema":"dmpf/infra@1","app":"probe"}\n' > "$ctx/deploy/infra.json" ;;
    dsn-no-deploy) sed -i 's#@postgres:5432/probe#@postgres:5432/dmpf#' "$ctx/deploy/compose.yml" ;;
    migrador-de-outra-app) variar "$repo" dois-contextos && sed -i 's/probe_migrator:/other_migrator:/' "$ctx/deploy/compose.yml" ;;
    migrador-em-banco-alheio) variar "$repo" dois-contextos && sed -i '/PG_MIGRATE_DSN/s#5432/probe#5432/other#' "$ctx/deploy/compose.yml" ;;
    migrador-sem-app) variar "$repo" com-migrador && sed -i '/PG_MIGRATE_DSN/s#probe_migrator:#ghost_migrator:#; /PG_MIGRATE_DSN/s#5432/probe#5432/ghost#' "$ctx/deploy/compose.yml" ;;
    bloco-pela-role-do-migrador) variar "$repo" com-migrador && sed -i 's/CREATE DATABASE \\"probe\\"/CREATE DATABASE \\"probe_migrator\\"/' "$repo/infra/local/compose.yml" ;;
    migrador-na-ancora-x-env)
      variar "$repo" com-migrador
      sed -i '/^x-env: &env$/a\  PG_MIGRATE_DSN: postgres://probe_migrator:${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local}@postgres:5432/probe?sslmode=disable' \
        "$ctx/deploy/compose.yml"
      ;;
    migrador-no-relay)
      variar "$repo" com-migrador
      migrar_no_relay "$repo" '      PG_MIGRATE_DSN: postgres://probe_migrator:${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local}@postgres:5432/probe?sslmode=disable'
      ;;
    migrador-sob-ancora-do-api)
      variar "$repo" com-migrador
      sed -i '/^  probe-api:$/,/^  probe-relay:$/ s|^    environment:$|    environment: \&api-env|' "$ctx/deploy/compose.yml"
      sed -i '/^  probe-relay:$/,$ { /^      <<: \*env$/d; s|^    environment:$|    environment: *api-env| }' "$ctx/deploy/compose.yml"
      ;;
    migrador-herdado-por-extends)
      variar "$repo" com-migrador
      sed -i '/^  probe-relay:$/a\    extends:\n      service: probe-api' "$ctx/deploy/compose.yml"
      ;;
    migrador-dsn-chave-valor)
      variar "$repo" com-migrador
      migrar_no_relay "$repo" '      PG_DSN: "host=postgres user=probe_migrator password=${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local} dbname=probe"'
      ;;
    pguser-do-migrador) variar "$repo" com-migrador && migrar_no_relay "$repo" '      PGUSER: probe_migrator' ;;
    secret-do-migrador-no-relay)
      variar "$repo" com-migrador
      printf '            - secretRef:\n                name: probe-migrator\n' >> "$ctx/deploy/k8s/base/deployment-relay.yaml"
      ;;
    migrador-fora-do-secret)
      variar "$repo" com-migrador
      sed -i '/^  - name: probe-migrator$/,$d' "$ctx/deploy/k8s/overlays/dev/kustomization.yaml"
      printf '      - PG_MIGRATE_DSN=postgres://probe_migrator:probe-migrator-dev@postgres-0.postgres:5432/probe?sslmode=disable\n' \
        >> "$ctx/deploy/k8s/overlays/dev/kustomization.yaml"
      ;;
    migrador-no-pg-dsn)
      variar "$repo" com-migrador
      sed -i '1i x-env: \&env\n  PG_DSN: postgres://probe_migrator:${PROBE_PG_MIGRATOR_PASSWORD:-probe-migrator-local}@postgres:5432/probe?sslmode=disable' \
        "$ctx/deploy/compose.yml"
      ;;
    tabela-sem-grant) printf 'CREATE TABLE IF NOT EXISTS widgets (id bigint);\n' >> "$ctx/provider/schema.sql" ;;
    unique-sem-declaracao) sed -i '/Name:      "probes_tenant_id_label_key",/d' "$ctx/provider/probe_repository.go" ;;
    table-fora-dos-uniques) sed -i 's/{probeTable.Uniques()}/{}/' "$ctx/provider/schema.go" ;;
    unique-anonima-de-tabela) sed -i 's/CONSTRAINT probes_tenant_id_label_key UNIQUE/UNIQUE/' "$ctx/provider/schema.sql" ;;
    unique-de-coluna) sed -i 's/^  snapshot  jsonb NOT NULL,$/&\n  code      text  NOT NULL UNIQUE,/' "$ctx/provider/schema.sql" ;;
    unique-por-indice) printf 'CREATE UNIQUE INDEX IF NOT EXISTS probes_tenant_id_snapshot_idx ON probes (tenant_id, snapshot);\n' >> "$ctx/provider/schema.sql" ;;
    alter-add-unique) printf 'ALTER TABLE probe_events ADD UNIQUE (tenant_id, label);\n' >> "$ctx/provider/schema.sql" ;;
    unique-name-nao-resolvivel) sed -i 's/Name:      "probes_tenant_id_label_key",/Name:      probeKey("label"),/' "$ctx/provider/probe_repository.go" ;;
    unique-em-tabela-particionada) sed -i 's/^);$/) PARTITION BY LIST (tenant_id);/' "$ctx/provider/schema.sql" ;;
    unique-em-tabela-com-heranca) sed -i 's/^);$/) INHERITS (probe_base);/' "$ctx/provider/schema.sql" ;;
    herda-por-alter-com-asterisco) printf 'ALTER TABLE probes * INHERIT probe_base;\n' >> "$ctx/provider/schema.sql" ;;
    herda-por-alter-com-asterisco-entre-comentarios)
      printf 'ALTER TABLE probes /* a */ * -- b\nINHERIT probe_base;\n' >> "$ctx/provider/schema.sql"
      ;;
    unique-em-particao)
      sed -i 's/^CREATE TABLE IF NOT EXISTS probes ($/CREATE TABLE IF NOT EXISTS probes PARTITION OF probe_history (/; s/^);$/) FOR VALUES IN ('"'a'"');/' \
        "$ctx/provider/schema.sql"
      ;;
    unique-apos-dollar-quote)
      sabotar "$repo" unique-de-coluna
      sed -i "1i COMMENT ON TABLE probes IS \$\$it's per tenant\$\$;" "$ctx/provider/schema.sql"
      ;;
    unique-apos-dollar-tag)
      sabotar "$repo" unique-de-coluna
      sed -i "1i CREATE FUNCTION probe_tag() RETURNS text LANGUAGE sql AS \$\$ SELECT 'a' || \$x\$it's\$x\$ \$\$;" "$ctx/provider/schema.sql"
      ;;
    unique-apos-e-string)
      sabotar "$repo" unique-de-coluna
      sed -i "1i COMMENT ON TABLE probes IS E'it\\\\'s per tenant';" "$ctx/provider/schema.sql"
      ;;
    unique-apos-tag-nao-ascii)
      sabotar "$repo" unique-de-coluna
      sed -i "1i COMMENT ON TABLE probes IS \$€\$it's per tenant\$€\$;" "$ctx/provider/schema.sql"
      ;;
    unique-apos-identificador-nao-ascii)
      sabotar "$repo" unique-de-coluna
      sed -i "1i COMMENT ON COLUMN probes.label€\$\$ IS NULL;" "$ctx/provider/schema.sql"
      ;;
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

  local -a variantes=(com-migrador com-migrador-chave-valor dois-contextos var-com-func-literal name-por-const name-por-bloco-const
    unique-em-comentario-sql unique-em-literal-sql url-em-string-go unique-em-dollar-quote unique-em-comentario-aninhado
    unique-em-identificador-citado unique-em-e-string unique-em-identificador-com-cifrao dolar-solto-nao-ascii particionada-sem-unique
    coluna-chamada-inherit plataforma-em-platform-e-teste projetor-como-tipo-no-application plataforma-nos-kits constraint-por-alter)
  for variante in "${variantes[@]}"; do
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    variar "$tmp" "$variante"
    if ( FALHAS=0; fase_estrutural "$tmp" >/dev/null 2>&1 ); then
      aprovar "aceitou: $variante"
    else
      falhar "a variante '$variante' deveria passar"
      ( FALHAS=0; fase_estrutural "$tmp" 2>&1 | grep FALHA ) >&2
      status=1
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done

  local -a sabotagens=(
    sem-binario sem-appkit sem-distkit sem-rpc sem-config sem-wiring sem-schema
    com-http solto sem-target sem-unidade bff-sem-api bff-com-provider bff-sem-manifesto
    tabela-prefixada tabela-example indice-example tabela-do-kernel indice-fora constraint-fora constraint-fora-por-alter kernel-prefixado
    dsn-compartilhado postgres-db init-incompleto ci-compartilhado
    manifesto-ausente manifesto-sem-banco dsn-no-deploy
    migrador-de-outra-app migrador-em-banco-alheio migrador-sem-app bloco-pela-role-do-migrador
    migrador-no-pg-dsn tabela-sem-grant unique-sem-declaracao table-fora-dos-uniques
    unique-anonima-de-tabela unique-de-coluna unique-por-indice unique-name-nao-resolvivel alter-add-unique
    unique-em-tabela-particionada unique-em-tabela-com-heranca unique-em-particao
    herda-por-alter-com-asterisco herda-por-alter-com-asterisco-entre-comentarios
    unique-apos-dollar-quote unique-apos-dollar-tag unique-apos-e-string unique-apos-tag-nao-ascii unique-apos-identificador-nao-ascii
    migrador-na-ancora-x-env migrador-no-relay migrador-sob-ancora-do-api migrador-herdado-por-extends
    migrador-dsn-chave-valor pguser-do-migrador secret-do-migrador-no-relay migrador-fora-do-secret
    leitor-por-interface-local projetor-por-interface-local alcance-no-provider alcance-em-platform alcance-verificado-em-rpc alcance-verificado-em-platform
    leitor-por-interface-em-app projetor-por-interface-em-rpc leitor-por-interface-em-cmd
    projetor-em-subpacote-do-provider projetor-por-valor-de-metodo
  )
  local -A motivos=(
    [bloco-pela-role-do-migrador]="cria banco e role para ['probe_migrator']"
    [migrador-na-ancora-x-env]='em x-env, fora do serviço probe-api'
    [migrador-no-relay]='no serviço probe-relay; só o serviço probe-api'
    [migrador-sob-ancora-do-api]='sob a âncora &api-env'
    [migrador-herdado-por-extends]='no serviço probe-api, que outro serviço herda por extends'
    [migrador-dsn-chave-valor]='a role probe_migrator só cabe no PG_MIGRATE_DSN, não em PG_DSN'
    [pguser-do-migrador]='a role probe_migrator só cabe no PG_MIGRATE_DSN, não em PGUSER'
    [secret-do-migrador-no-relay]='o Secret probe-migrator só cabe no Deployment probe-api, não em Deployment probe-relay'
    [migrador-fora-do-secret]='fora do Secret probe-migrator'
    [constraint-fora-por-alter]='constraint ck_outbox fora de <tabela>_<colunas>_{pkey,key,check,fkey}'
    [unique-sem-declaracao]='constraint probes_tenant_id_label_key UNIQUE sem postgres.Unique de mesmo Name'
    [table-fora-dos-uniques]='probeTable declara Unique e falta em provider.Uniques'
    [unique-anonima-de-tabela]='UNIQUE sem nome na tabela probes'
    [unique-de-coluna]='UNIQUE sem nome na tabela probes'
    [unique-apos-dollar-quote]='UNIQUE sem nome na tabela probes'
    [unique-apos-dollar-tag]='UNIQUE sem nome na tabela probes'
    [unique-apos-e-string]='UNIQUE sem nome na tabela probes'
    [unique-apos-tag-nao-ascii]='UNIQUE sem nome na tabela probes'
    [unique-apos-identificador-nao-ascii]='UNIQUE sem nome na tabela probes'
    [unique-por-indice]='CREATE UNIQUE INDEX probes_tenant_id_snapshot_idx em probes'
    [alter-add-unique]='UNIQUE sem nome na tabela probe_events'
    [unique-em-tabela-particionada]='tabela probes declara UNIQUE e usa PARTITION BY'
    [unique-em-tabela-com-heranca]='tabela probes declara UNIQUE e usa INHERITS'
    [herda-por-alter-com-asterisco]='tabela probes declara UNIQUE e usa INHERIT,'
    [herda-por-alter-com-asterisco-entre-comentarios]='tabela probes declara UNIQUE e usa INHERIT,'
    [unique-em-particao]='tabela probes declara UNIQUE e usa PARTITION OF'
    [unique-name-nao-resolvivel]='o Name precisa ser literal ou const resolvível no provider'
    [leitor-por-interface-local]='provider/probe_reader.go:3 usa PlatformReader fora de provider/platform_*.go'
    [projetor-por-interface-local]='provider/probe_projection.go:3 usa Projector fora de provider/platform_*.go'
    [alcance-no-provider]='provider/probe_reach.go:3 usa WithPlatformReach, que só o kernel de borda monta'
    [alcance-em-platform]='provider/platform_probe_reach.go:3 usa WithPlatformReach, que só o kernel de borda monta'
    [alcance-verificado-em-rpc]='app/rpc/probe_reach.go:3 usa WithVerifiedPlatformReach, que só a borda REST chama'
    [alcance-verificado-em-platform]='provider/platform_probe_reach.go:3 usa WithVerifiedPlatformReach, que só a borda REST chama'
    [leitor-por-interface-em-app]='app/probe_reader.go:3 usa PlatformReader fora de provider/platform_*.go'
    [projetor-por-interface-em-rpc]='app/rpc/probe_projection.go:3 usa Projector fora de provider/platform_*.go'
    [leitor-por-interface-em-cmd]='cmd/probe_reader.go:3 usa PlatformReader fora de provider/platform_*.go'
    [projetor-em-subpacote-do-provider]='provider/sub/platform_probe.go:3 usa Projector fora de provider/platform_*.go'
    [projetor-por-valor-de-metodo]='application/probe_projection.go:3 usa Projector fora de provider/platform_*.go'
  )
  local saida
  for sabotagem in "${sabotagens[@]}"; do
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    sabotar "$tmp" "$sabotagem"
    if saida="$( ( FALHAS=0; fase_estrutural "$tmp" ) 2>&1 >/dev/null )"; then
      falhar "a sabotagem '$sabotagem' passou pelo gate"
      status=1
    elif [[ -n "${motivos[$sabotagem]:-}" ]] && ! grep -qF -- "${motivos[$sabotagem]}" <<<"$saida"; then
      falhar "a sabotagem '$sabotagem' reprovou por outro motivo: $(grep -m 3 FALHA <<<"$saida" | tr -s ' ')"
      status=1
    else
      aprovar "reprovou: $sabotagem"
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done

  if [[ "$status" -ne 0 ]]; then
    printf '\nAutoteste do gate: REPROVADO.\n' >&2
    return 1
  fi
  printf '\nAutoteste do gate: OK — cada vetor negativo foi recusado.\n'
  return 0
}

uso() {
  cat <<'TXT'
uso: dmpf-context-check.sh [--phase structural|self-test] [--root <dir>] [--allow-empty]

  structural  (default) verifica todo bounded context de apps/backend, a borda,
              os nomes do DDL, o deploy/infra.json de cada contexto e banco e
              role em infra/, apps/backend/*/deploy e .github/workflows
  self-test   prova o gate contra fixture sintética, um vetor por sabotagem
  --root      raiz do repositório a verificar (default: a do git)
  --context   verifica só o layout e o DDL de um contexto, sem a infra
  --allow-empty  aprova o workspace que ainda não tem bounded context
TXT
}

FASE="structural"
REPO="$ROOT"
CONTEXTO=""
PERMITIR_VAZIO=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --phase) shift; FASE="${1:-}"; [[ -n "$FASE" ]] || { uso >&2; exit 2; } ;;
    --phase=*) FASE="${1#--phase=}" ;;
    --root) shift; REPO="${1:-}"; [[ -n "$REPO" ]] || { uso >&2; exit 2; } ;;
    --root=*) REPO="${1#--root=}" ;;
    --context) shift; CONTEXTO="${1:-}"; [[ -n "$CONTEXTO" ]] || { uso >&2; exit 2; } ;;
    --context=*) CONTEXTO="${1#--context=}" ;;
    --allow-empty) PERMITIR_VAZIO=1 ;;
    -h|--help) uso; exit 0 ;;
    *) printf 'argumento desconhecido: %s\n' "$1" >&2; uso >&2; exit 2 ;;
  esac
  shift
done

case "$FASE" in
  structural)
    if [[ -n "$CONTEXTO" ]]; then fase_contexto "$CONTEXTO"; else fase_estrutural "$REPO"; fi
    ;;
  self-test) fase_self_test ;;
  *) printf 'fase desconhecida: %s\n' "$FASE" >&2; uso >&2; exit 2 ;;
esac
