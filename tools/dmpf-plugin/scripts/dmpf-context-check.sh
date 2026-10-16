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
# próprio banco no deploy/infra.json e um grant para cada tabela do schema.sql.
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
  exigir "$base/deploy/infra.json" "$app: o manifesto de infra em deploy/infra.json"
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
    [ -f "$raiz/$ctx/provider/schema.sql" ] || continue
    saida="$(verificar_ddl contexto "$raiz/$ctx/provider/schema.sql")"
    status=$?
    relatar "$ctx: DDL nos nomes canônicos" "$saida" "$status"
    saida="$(verificar_grants "$raiz/$ctx")"
    status=$?
    relatar "$ctx: toda tabela do schema.sql com grant no provider.Grants" "$saida" "$status"
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
    saida="$(verificar_grants "$base")"
    status=$?
    relatar "$ctx: toda tabela do schema.sql com grant no provider.Grants" "$saida" "$status"
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
  cat > "$ctx/provider/schema.go" <<'GO'
package provider

var Grants = []postgres.Grant{postgres.Mutable("probes")}
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

  local -a variantes=(com-migrador com-migrador-chave-valor dois-contextos)
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
    tabela-prefixada tabela-example indice-example tabela-do-kernel indice-fora constraint-fora kernel-prefixado
    dsn-compartilhado postgres-db init-incompleto ci-compartilhado
    manifesto-ausente manifesto-sem-banco dsn-no-deploy
    migrador-de-outra-app migrador-em-banco-alheio migrador-sem-app bloco-pela-role-do-migrador
    migrador-no-pg-dsn tabela-sem-grant
    migrador-na-ancora-x-env migrador-no-relay migrador-sob-ancora-do-api migrador-herdado-por-extends
    migrador-dsn-chave-valor pguser-do-migrador secret-do-migrador-no-relay migrador-fora-do-secret
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

  if [ "$status" -ne 0 ]; then
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
while [ $# -gt 0 ]; do
  case "$1" in
    --phase) shift; FASE="${1:-}"; [ -n "$FASE" ] || { uso >&2; exit 2; } ;;
    --phase=*) FASE="${1#--phase=}" ;;
    --root) shift; REPO="${1:-}"; [ -n "$REPO" ] || { uso >&2; exit 2; } ;;
    --root=*) REPO="${1#--root=}" ;;
    --context) shift; CONTEXTO="${1:-}"; [ -n "$CONTEXTO" ] || { uso >&2; exit 2; } ;;
    --context=*) CONTEXTO="${1#--context=}" ;;
    --allow-empty) PERMITIR_VAZIO=1 ;;
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
