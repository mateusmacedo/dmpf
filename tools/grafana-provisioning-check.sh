#!/usr/bin/env bash
# O Grafana expande `${…}` como variável de ambiente ao provisionar e só `$$` vira `$`
# literal (https://grafana.com/docs/grafana/latest/datasources/loki/configure/); um
# `${__value.raw}` sem escape chega ao datasource como url vazia.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

GRAFANA="infra/observability/grafana"
AUDITORIA="github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
FALHAS=0

falhar() {
  printf '  FALHA  %s\n' "$1" >&2
  FALHAS=$((FALHAS + 1))
}

aprovar() { printf '  ok     %s\n' "$1"; }

sem_escape() {
  grep -nP '(?<!\$)(?:\$\$)*\$\{' "$1"
}

TRACES_TO_LOGS="$(cat <<'AWK'
function aparar(s) { sub(/^[ \t]+/, "", s); sub(/[ \t\r]+$/, "", s); return s }
function valor(s,   q) {
  sub(/^[^:]*:/, "", s); s = aparar(s); q = substr(s, 1, 1)
  if ((q == "'" || q == "\"") && length(s) > 1 && substr(s, length(s)) == q) return substr(s, 2, length(s) - 2)
  sub(/[ \t]+#.*$/, "", s)
  return s
}
function fechar() { if (dentro) printf "%d\t%s\t%s\n", inicio, (custom == "" ? "-" : custom), query; dentro = 0 }
/^[ \t]*#/ || /^[ \t\r]*$/ { next }
{
  recuo = match($0, /[^ ]/) - 1
  if (dentro && recuo <= recuo_bloco) fechar()
  if ($0 ~ /^[ \t]*tracesToLogsV2:[ \t\r]*$/) { dentro = 1; inicio = NR; recuo_bloco = recuo; custom = ""; query = ""; next }
  if (dentro && $0 ~ /^[ \t]*customQuery:/) custom = valor($0)
  if (dentro && $0 ~ /^[ \t]*query:/) query = valor($0)
}
END { fechar() }
AWK
)"

DASHCHECK="$(cat <<'PY'
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import urllib.parse
import urllib.request

MODE, REPO, GRAF = sys.argv[1], sys.argv[2], sys.argv[3]
BASE = os.path.join(REPO, GRAF)
DASH = os.path.join(BASE, "dashboards")
MOUNT = "/var/lib/grafana/dashboards"
LOCAL_ONLY = ("carga/", "infra/postgres.json", "infra/containers.json")
DS_UIDS = {"prometheus", "loki", "tempo"}
OTLP = ("dmpf_", "http_", "rpc_", "messaging_", "db_client_", "go_", "otel_sdk_")
INTERVALO_FONTE = ((OTLP, 60), (("container_",), 60))
IMAGE = "prom/prometheus:v3.14.0"
RATE = re.compile(r"\b(rate|increase|irate)\(\s*([A-Za-z_:][A-Za-z0-9_:]*)\s*(\{[^}]*\})?\s*\[([^\]]+)\]")
RARO = re.compile(r"\b(?:rate|increase|irate)\(\s*(dmpf_dependency_\w+_total|dmpf_admission_rejections_total)\b")
SELECTOR = re.compile(r"([A-Za-z_:][A-Za-z0-9_:]*)?\s*\{([^{}]*)\}")
VOCAB = (
    (re.compile(r"dmpf_service_\w*"), "nome antigo dmpf_service_* (RF-D2)"),
    (re.compile(r"dmpf_otel_\w*"), "nome antigo dmpf_otel_* (RF-D7)"),
    (re.compile(r"breaker_state_ratio"), "breaker com _ratio (RF-C5)"),
    (re.compile(r'span_name\s*=~\s*"HTTP'), 'filtro span_name=~"HTTP .*" (RF-C5)'),
)
falhas, oks = [], []


def falha(tag, msg):
    falhas.append(f"[{tag}] {msg}")


def segundos(texto):
    m = re.fullmatch(r"(\d+)(ms|s|m|h|d)", (texto or "").strip())
    return int(m[1]) * {"ms": 0.001, "s": 1, "m": 60, "h": 3600, "d": 86400}[m[2]] if m else None


def fonte(metrica):
    for prefixos, seg in INTERVALO_FONTE:
        if metrica.startswith(prefixos):
            return seg
    return 15


def intervalo_do_datasource():
    caminho = os.path.join(BASE, "datasources.yaml")
    texto = open(caminho, encoding="utf-8").read() if os.path.isfile(caminho) else ""
    for bloco in re.split(r"(?m)^\s*-\s*name:", texto)[1:]:
        if re.search(r"(?m)^\s*uid:\s*['\"]?prometheus['\"]?\s*$", bloco):
            m = re.search(r"(?m)^\s*timeInterval:\s*['\"]?(\d+(?:ms|s|m|h))", bloco)
            if m:
                return segundos(m[1])
    return 15


def efetivo(painel, alvo):
    passo = segundos(alvo.get("interval")) or TIME_INTERVAL
    intervalo = max(segundos(painel.get("interval")) or 0, passo)
    return max(intervalo + passo, 4 * passo), intervalo


def local_only(rel):
    return any(rel == x or (x.endswith("/") and rel.startswith(x)) for x in LOCAL_ONLY)


def paineis(lista):
    for p in lista or []:
        yield p
        yield from paineis(p.get("panels"))


def carregar():
    arquivos = []
    if not os.path.isdir(DASH):
        falha("json", f"{GRAF}/dashboards ausente")
        return arquivos
    for raiz, _, nomes in os.walk(DASH):
        for nome in sorted(nomes):
            if nome.endswith(".json"):
                caminho = os.path.join(raiz, nome)
                arquivos.append((os.path.relpath(caminho, DASH).replace(os.sep, "/"), caminho))
    return sorted(arquivos)


def ds_ok(ds, onde):
    if not isinstance(ds, dict) or ds.get("uid") not in DS_UIDS or ds.get("type") != ds.get("uid"):
        falha("datasource", f"{onde}: datasource deve ser {{type, uid}} com uid em {sorted(DS_UIDS)}, veio {json.dumps(ds)}")


def vocabulario(expr, onde):
    for rx, motivo in VOCAB:
        if rx.search(expr):
            falha("vocabulario", f"{onde}: {motivo}")
    for m in SELECTOR.finditer(expr):
        if re.search(r"(^|[\s,])service\s*(=~|!~|!=|=)", m[2]) and not (m[1] or "").startswith("traces_"):
            falha("vocabulario", f"{onde}: label service fora das séries traces_* do Tempo; nas séries OTLP é service_name (RF-D3)")
    if "dmpf_tenant_id" in expr and "dmpf_admission_rejections_total" not in expr:
        falha("vocabulario", f"{onde}: dmpf_tenant_id só em dmpf_admission_rejections_total (MET-07, RF-D3)")


def janela(expr, painel, alvo, onde):
    for m in RATE.finditer(expr):
        metrica, win = m[2], m[4].strip()
        seg = fonte(metrica)
        minimo = 2 * seg
        if win in ("$__rate_interval", "${__rate_interval}", "$__interval"):
            taxa, intervalo = efetivo(painel, alvo)
            valor = intervalo if win == "$__interval" else taxa
            if valor < minimo:
                falha("janela", f"{onde}: {metrica} chega a cada {seg}s; o {win} efetivo do painel é {valor:g}s e "
                                f"precisa de pelo menos {minimo}s (2 × {seg}s): o Grafana resolve o $__rate_interval "
                                f"como max(interval do painel + timeInterval de {TIME_INTERVAL:g}s, 4 × timeInterval)")
        elif not win.startswith("$__range"):
            janela_seg = segundos(win)
            if janela_seg is not None and janela_seg < minimo:
                falha("janela", f"{onde}: janela [{win}] menor que {minimo}s (2 × {seg}s, o intervalo da fonte) "
                                f"sobre {metrica}")


def raro(expr, onde):
    if RARO.search(expr) and not (re.search(r"\bunless\b", expr) and re.search(r"\boffset\b", expr)):
        falha("evento-raro", f"{onde}: contador de evento raro sem o termo de série nova "
                             f"(c unless c offset <janela>): o evento que cria a série some do increase")


def substituir(expr, rate="2m", rng="1h", iv="30s"):
    e = expr.replace("${__rate_interval}", rate).replace("$__rate_interval", rate)
    e = re.sub(r"\$__range\b", rng, e).replace("$__interval", iv)
    e = re.sub(r'(?<![=!~])="\$\{?[A-Za-z_]\w*(:[^}]*)?\}?"', '=~".+"', e)
    e = re.sub(r"\$\{([A-Za-z_]\w*)(:[^}]*)?\}", ".+", e)
    return re.sub(r"\$([A-Za-z_]\w*)", ".+", e)


def estrutura(arquivos):
    uids, titulos, exprs = {}, {}, []
    for rel, caminho in arquivos:
        if "/" not in rel:
            falha("paridade", f"dashboards/{rel} está na raiz: nenhum provider lê a raiz")
        elif rel.count("/") > 1:
            falha("paridade", f"dashboards/{rel}: só um nível de pasta (o ConfigMap do Kubernetes é plano)")
        texto = open(caminho, encoding="utf-8").read()
        try:
            d = json.loads(texto)
        except json.JSONDecodeError as e:
            falha("json", f"dashboards/{rel}: JSON inválido ({e})")
            continue
        uid, titulo = d.get("uid"), d.get("title")
        if not isinstance(uid, str) or not uid:
            falha("json", f"dashboards/{rel}: sem uid")
        elif uid in uids:
            falha("json", f"uid {uid} repetido em dashboards/{rel} e dashboards/{uids[uid]}")
        else:
            uids[uid] = rel
        if titulo in titulos:
            falha("json", f"título {titulo!r} repetido em dashboards/{rel} e dashboards/{titulos[titulo]}")
        titulos[titulo] = rel
        if d.get("id") is not None:
            falha("json", f"dashboards/{rel}: id deve ser nulo ou ausente; o Grafana atribui")
        if "__inputs" in d or "${DS_" in texto:
            falha("datasource", f"dashboards/{rel}: __inputs ou ${{DS_*}} de dashboard importado")
        for v in d.get("templating", {}).get("list", []):
            q = v.get("query")
            q = q.get("query") if isinstance(q, dict) else q
            if v.get("type") == "query":
                ds_ok(v.get("datasource"), f"dashboards/{rel} variável {v.get('name')}")
            if isinstance(q, str):
                vocabulario(q, f"dashboards/{rel} variável {v.get('name')}")
        n = 0
        for p in paineis(d.get("panels")):
            if p.get("type") == "row":
                continue
            n += 1
            onde = f"dashboards/{rel} painel {p.get('id')} {p.get('title')!r}"
            alvos = p.get("targets") or []
            if alvos:
                ds_ok(p.get("datasource"), onde)
            for t in alvos:
                ds_ok(t.get("datasource"), f"{onde} alvo {t.get('refId')}")
                if (t.get("datasource") or {}).get("uid") == "prometheus" and t.get("expr"):
                    e = t["expr"]
                    vocabulario(e, f"{onde} alvo {t.get('refId')}")
                    janela(e, p, t, f"{onde} alvo {t.get('refId')}")
                    raro(e, f"{onde} alvo {t.get('refId')}")
                    exprs.append((f"{onde} alvo {t.get('refId')}", e))
        oks.append(f"dashboards/{rel}: uid {uid}, {n} painéis")
    return exprs


def paridade(arquivos):
    pastas = sorted({rel.split("/")[0] for rel, _ in arquivos if "/" in rel})
    ler = lambda nome: open(os.path.join(BASE, nome), encoding="utf-8").read() if os.path.isfile(os.path.join(BASE, nome)) else ""
    provider, implant, kust = ler("dashboards.yaml"), ler("deployment.yaml"), ler("kustomization.yaml")
    destinos = {}
    for bloco in re.split(r"(?m)^\s*-\s*name:", provider)[1:]:
        caminho = re.search(r"(?m)^\s*path:\s*['\"]?([^'\"\s]+)", bloco)
        fuid = re.search(r"(?m)^\s*folderUid:\s*['\"]?([^'\"\s#]+)", bloco)
        if not caminho:
            continue
        alvo = caminho[1].rstrip("/")
        if alvo == MOUNT:
            falha("paridade", f"dashboards.yaml: provider em {MOUNT} lê as subpastas e duplica os uid")
            continue
        pasta = alvo[len(MOUNT) + 1:] if alvo.startswith(MOUNT + "/") else alvo
        if not fuid:
            falha("paridade", f"dashboards.yaml: provider de {alvo} sem folderUid fixo")
        destinos[pasta] = fuid[1] if fuid else ""
    montagens = {m[2]: m[1] for m in re.finditer(r"-\s*name:\s*(\S+)\s*\n\s*mountPath:\s*(\S+)", implant)}
    cms = {m[1]: m[2] for m in re.finditer(r"-\s*name:\s*(\S+)\s*\n\s*configMap:\s*\n\s*name:\s*(\S+)", implant)}
    geradores, atual = {}, None
    for linha in kust.splitlines():
        m = re.match(r"\s*-\s*name:\s*(\S+)", linha)
        if m:
            atual = m[1]
            geradores.setdefault(atual, [])
            continue
        m = re.match(r"\s*-\s*(dashboards/\S+\.json)\s*$", linha)
        if m and atual:
            geradores[atual].append(m[1])
    for pasta in pastas:
        antes = len(falhas)
        if pasta not in destinos:
            falha("paridade", f"dashboards/{pasta}/ sem provider em dashboards.yaml")
        esperado = sorted(f"dashboards/{rel}" for rel, _ in arquivos if rel.startswith(pasta + "/") and not local_only(rel))
        nome = f"grafana-dashboards-{pasta}"
        volume = montagens.get(f"{MOUNT}/{pasta}")
        if not volume:
            falha("paridade", f"deployment.yaml sem volumeMount em {MOUNT}/{pasta}")
        elif esperado and cms.get(volume) != nome:
            falha("paridade", f"deployment.yaml: {MOUNT}/{pasta} monta o volume {volume} "
                              f"({cms.get(volume) or 'sem ConfigMap'}), não o ConfigMap {nome}")
        elif not esperado and volume in cms:
            falha("paridade", f"deployment.yaml: {MOUNT}/{pasta} é só do Compose e monta o ConfigMap {cms[volume]}; "
                              f"use emptyDir")
        listado = sorted(geradores.pop(nome, []))
        if listado != esperado:
            falha("paridade", f"kustomization.yaml: ConfigMap {nome} lista {listado}, a pasta pede {esperado} "
                              f"(só do Compose: {', '.join(LOCAL_ONLY)})")
        if len(falhas) == antes:
            oks.append(f"pasta {pasta}: folderUid {destinos[pasta]}, {len(esperado)} arquivo(s) no Kubernetes")
    for nome, lista in geradores.items():
        if lista:
            falha("paridade", f"kustomization.yaml: ConfigMap {nome} lista {lista} fora de uma pasta de dashboards")
    for pasta in destinos:
        if pasta not in pastas:
            falha("paridade", f"dashboards.yaml: provider aponta {MOUNT}/{pasta}, pasta inexistente no repositório")


def promql(exprs):
    if not exprs:
        return
    tmp = tempfile.mkdtemp(prefix="dashcheck-")
    os.chmod(tmp, 0o755)
    regras = os.path.join(tmp, "rules.yaml")
    with open(regras, "w", encoding="utf-8") as f:
        json.dump({"groups": [{"name": "dashboards", "rules": [
            {"record": f"dashboard:expr_{i}", "expr": substituir(e)} for i, (_, e) in enumerate(exprs)]}]}, f)
    os.chmod(regras, 0o644)
    promtool = os.environ.get("PROMTOOL")
    if promtool:
        cmd = [promtool, "check", "rules", regras]
    elif shutil.which("docker"):
        cmd = ["docker", "run", "--rm", "-v", f"{tmp}:/r:ro", "--entrypoint", "promtool", IMAGE, "check", "rules",
               "/r/rules.yaml"]
    else:
        falha("promql", f"promtool indisponível: defina PROMTOOL ou instale o docker (imagem {IMAGE})")
        return
    r = subprocess.run(cmd, capture_output=True, text=True, timeout=600)
    subprocess.run(["trash", tmp], capture_output=True)
    if r.returncode == 0:
        oks.append(f"{len(exprs)} expressões PromQL aceitas pelo promtool")
        return
    saida = r.stdout + r.stderr
    erros = re.findall(r'"dashboard:expr_(\d+)":\s*(.*)', saida)
    for i, msg in erros:
        falha("promql", f"{exprs[int(i)][0]}: {msg.strip()}")
    if not erros:
        falha("promql", "promtool reprovou sem apontar a expressão: " + saida.strip()[-400:])


def live(arquivos, url, inicio, fim):
    from datetime import datetime
    t0 = datetime.fromisoformat(inicio.replace("Z", "+00:00")).timestamp()
    t1 = datetime.fromisoformat(fim.replace("Z", "+00:00")).timestamp()
    vazios = raros = total = 0
    for rel, caminho in arquivos:
        d = json.load(open(caminho, encoding="utf-8"))
        for p in paineis(d.get("panels")):
            alvos = [t for t in p.get("targets") or []
                     if (t.get("datasource") or {}).get("uid") == "prometheus" and t.get("expr")]
            if not alvos:
                continue
            total += 1
            series = 0
            for t in alvos:
                taxa, intervalo = efetivo(p, t)
                q = urllib.parse.urlencode({"query": substituir(t["expr"], f"{taxa:g}s", f"{int(t1 - t0)}s",
                                                                f"{intervalo:g}s"),
                                            "start": t0, "end": t1, "step": f"{intervalo:g}s"})
                resp = json.load(urllib.request.urlopen(f"{url}/api/v1/query_range?{q}", timeout=60))
                series += len(resp["data"]["result"])
            rotulo = (f"dashboards/{rel} painel {p['id']} {p['title']!r}: {series} série(s), "
                      f"$__rate_interval efetivo {taxa:g}s")
            if series:
                print(f"  ok     {rotulo}")
            elif "noValue" in p.get("fieldConfig", {}).get("defaults", {}):
                raros += 1
                print(f"  raro   {rotulo} (noValue declarado: evento que não ocorreu)")
            else:
                vazios += 1
                print(f"  VAZIO  {rotulo}")
    print(f"\n{total} painéis Prometheus: {total - vazios - raros} com dados, {raros} de evento raro sem ocorrência, "
          f"{vazios} vazios.")
    sys.exit(1 if vazios else 0)


TIME_INTERVAL = intervalo_do_datasource()
arquivos = carregar()
if MODE == "live":
    live(arquivos, sys.argv[4], sys.argv[5], sys.argv[6])
exprs = estrutura(arquivos)
paridade(arquivos)
if not any(f.startswith("[json]") for f in falhas):
    promql(exprs)
for o in oks:
    print(f"OK\t{o}")
for f in falhas:
    print(f"FALHA\t{f}")
PY
)"

conferir_dashboards() {
  local repo="$1" saida status=0 tipo texto achou=0
  printf '\n== dashboards em %s/dashboards: identidade, datasource, paridade, PromQL, vocabulário, janela ==\n' "$GRAFANA"
  saida="$(python3 -c "$DASHCHECK" check "$repo" "$GRAFANA" 2>&1)" || status=$?
  while IFS=$'\t' read -r tipo texto; do
    case "$tipo" in
      OK) aprovar "$texto" ;;
      FALHA) falhar "$texto"; achou=1 ;;
    esac
  done <<<"$saida"
  if [ "$status" -ne 0 ] && [ "$achou" -eq 0 ]; then
    falhar "[dashboards] o verificador terminou com $status: $(tail -n 3 <<<"$saida")"
  fi
  return 0
}

conferir_trace_logs() {
  local repo="$1" rel="$GRAFANA/datasources.yaml" blocos inicio custom query achou=0 filtro
  filtro="\|[[:space:]]*scope_name[[:space:]]*!=[[:space:]]*\"${AUDITORIA//./\\.}\""
  printf '\n== trace→logs sem a auditoria em %s ==\n' "$rel"
  if [ ! -f "$repo/$rel" ]; then
    falhar "[trace-logs] $rel ausente; é onde o Tempo leva do trace aos logs"
    return 0
  fi
  if ! blocos="$(awk "$TRACES_TO_LOGS" "$repo/$rel")"; then
    falhar "[trace-logs] $rel: o awk não conseguiu ler o tracesToLogsV2"
    return 0
  fi
  while IFS=$'\t' read -r inicio custom query; do
    [ -n "$inicio" ] || continue
    achou=1
    if [ "$custom" != true ]; then
      falhar "[trace-logs] $rel:$inicio: tracesToLogsV2 sem customQuery: true; a consulta que o Grafana monta não exclui a auditoria"
    elif ! [[ $query =~ $filtro ]]; then
      falhar "[trace-logs] $rel:$inicio: a query do tracesToLogsV2 não exclui scope_name != \"$AUDITORIA\" (RF-A7)"
    else
      aprovar "$rel:$inicio: a query do tracesToLogsV2 exclui a auditoria por scope_name"
    fi
  done <<<"$blocos"
  [ "$achou" -eq 1 ] || falhar "[trace-logs] $rel: nenhum tracesToLogsV2 para conferir a exclusão da auditoria"
  return 0
}

fase_estrutural() {
  local repo="${1:-$ROOT}" arquivo achados achou=0
  printf '\n== interpolação em %s/*.yaml ==\n' "$GRAFANA"
  for arquivo in "$repo/$GRAFANA"/*.yaml; do
    [ -f "$arquivo" ] || continue
    achou=1
    achados="$(sem_escape "$arquivo")"
    if [ -z "$achados" ]; then
      aprovar "${arquivo#"$repo"/}: todo \${ está escapado como \$\${"
      continue
    fi
    while IFS= read -r linha; do
      falhar "${arquivo#"$repo"/}:$linha (\${ sem \$ à esquerda vira variável de ambiente; use \$\${)"
    done <<<"$achados"
  done

  if [ "$achou" -eq 0 ]; then
    echo "nenhum arquivo de provisionamento em $repo/$GRAFANA" >&2
    return 2
  fi
  conferir_trace_logs "$repo"
  conferir_dashboards "$repo"
  if [ "$FALHAS" -gt 0 ]; then
    printf '\nGate de provisionamento: REPROVADO com %d achado(s).\n' "$FALHAS" >&2
    return 1
  fi
  printf '\nGate de provisionamento: OK — nenhum ${ seria expandido pelo Grafana, o trace→logs exclui a auditoria e os dashboards passam nas sete checagens.\n'
  return 0
}

montar_fixture() {
  local dir="$1/$GRAFANA"
  mkdir -p "$dir"
  cat > "$dir/datasources.yaml" <<'YAML'
datasources:
  - name: Prometheus
    uid: prometheus
    jsonData:
      timeInterval: 15s
  - name: Loki
    jsonData:
      derivedFields:
        - name: TraceID
          url: '$${__value.raw}'
        - name: correlation_id
          url: '{ span.dmpf.correlation_id = "$${__value.raw}" }'
        - name: literal
          url: 'custo em $$$${__value.raw}'
  - name: Tempo
    jsonData:
      tracesToLogsV2:
        customQuery: true
        query: '{$${__tags}} | trace_id="$${__trace.traceId}" | scope_name != "github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"'
YAML
  cat > "$dir/dashboards.yaml" <<'YAML'
providers:
  - name: dmpf-plataforma
    folder: Plataforma
    folderUid: dmpf-plataforma
    options:
      path: /var/lib/grafana/dashboards/plataforma
  - name: dmpf-carga
    folder: Carga
    folderUid: dmpf-carga
    options:
      path: /var/lib/grafana/dashboards/carga
YAML
  cat > "$dir/kustomization.yaml" <<'YAML'
configMapGenerator:
  - name: grafana-datasources
    files:
      - datasources.yaml
  - name: grafana-dashboards-plataforma
    files:
      - dashboards/plataforma/a.json
YAML
  cat > "$dir/deployment.yaml" <<'YAML'
spec:
  template:
    spec:
      containers:
        - name: grafana
          volumeMounts:
            - name: dashboards-plataforma
              mountPath: /var/lib/grafana/dashboards/plataforma
            - name: dashboards-carga
              mountPath: /var/lib/grafana/dashboards/carga
      volumes:
        - name: dashboards-plataforma
          configMap:
            name: grafana-dashboards-plataforma
        - name: dashboards-carga
          emptyDir: {}
YAML
  mkdir -p "$dir/dashboards/plataforma" "$dir/dashboards/carga"
  cat > "$dir/dashboards/plataforma/a.json" <<'JSON'
{
  "uid": "fx-a",
  "title": "Fixture A",
  "templating": {
    "list": [
      {
        "name": "service",
        "type": "query",
        "datasource": { "type": "prometheus", "uid": "prometheus" },
        "query": "label_values(target_info, service_name)"
      }
    ]
  },
  "panels": [
    {
      "id": 1,
      "type": "timeseries",
      "title": "OTLP",
      "interval": "2m",
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "sum by (service_name) (rate(dmpf_operation_duration_seconds_count{service_name=~\"$service\"}[$__rate_interval]))"
        }
      ]
    },
    {
      "id": 2,
      "type": "timeseries",
      "title": "Evento raro",
      "interval": "2m",
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "sum by (dmpf_dependency) (increase(dmpf_dependency_retries_total[$__rate_interval])) + sum by (dmpf_dependency) ((dmpf_dependency_retries_total unless dmpf_dependency_retries_total offset $__rate_interval) or dmpf_dependency_retries_total * 0)"
        }
      ]
    },
    {
      "id": 3,
      "type": "logs",
      "title": "Logs",
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        { "refId": "A", "datasource": { "type": "loki", "uid": "loki" }, "expr": "{service_name=~\"$service\"}" }
      ]
    },
    {
      "id": 4,
      "type": "timeseries",
      "title": "Spans",
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "datasource": { "type": "prometheus", "uid": "prometheus" },
          "expr": "sum by (span_name) (rate(traces_spanmetrics_calls_total{service=~\"$service\"}[$__rate_interval]))"
        }
      ]
    }
  ]
}
JSON
  cat > "$dir/dashboards/carga/load-bff.json" <<'JSON'
{
  "uid": "load-bff",
  "title": "Fixture carga",
  "panels": [
    {
      "id": 1,
      "type": "timeseries",
      "title": "VUs",
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        { "refId": "A", "datasource": { "type": "prometheus", "uid": "prometheus" }, "expr": "sum(k6_vus)" }
      ]
    }
  ]
}
JSON
}

editar_painel() {
  python3 - "$1" "$2" "$3" "$4" <<'PY'
import json, sys
caminho, painel, campo, valor = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
d = json.load(open(caminho))
p = d["panels"][painel]
if campo == "expr":
    p["targets"][0]["expr"] = valor
elif campo == "datasource":
    p["datasource"] = valor
elif campo == "sem-interval":
    p.pop("interval", None)
elif campo == "interval":
    p["interval"] = valor
json.dump(d, open(caminho, "w"))
PY
}

sabotar() {
  local dir="$1/$GRAFANA" tipo="$2"
  case "$tipo" in
    sem-escape) sed -i "s/url: '\$\${__value.raw}'/url: '\${__value.raw}'/" "$dir/datasources.yaml" ;;
    no-meio-da-linha) sed -i 's/= "\$\${__value.raw}"/= "${__value.raw}"/' "$dir/datasources.yaml" ;;
    escape-impar) sed -i 's/\$\$\$\${__value.raw}/$$${__value.raw}/' "$dir/datasources.yaml" ;;
    custom-query-sem-escape) sed -i 's/trace_id="\$\${__trace.traceId}"/trace_id="${__trace.traceId}"/' "$dir/datasources.yaml" ;;
    outro-arquivo) printf '      folder: ${GF_FOLDER}\n' >> "$dir/dashboards.yaml" ;;
    trace-logs-com-auditoria) sed -i 's# | scope_name != "[^"]*"##' "$dir/datasources.yaml" ;;
    trace-logs-outro-scope) sed -i 's#observability/audit"#observability/tracing"#' "$dir/datasources.yaml" ;;
    trace-logs-sem-custom-query) sed -i 's/customQuery: true/customQuery: false/' "$dir/datasources.yaml" ;;
    sem-traces-to-logs) sed -i '/tracesToLogsV2:/,$d' "$dir/datasources.yaml" ;;
    sem-datasources) trash "$dir/datasources.yaml" ;;
    json-invalido) printf '{"uid": ' > "$dir/dashboards/plataforma/a.json" ;;
    uid-repetido)
      cp "$dir/dashboards/plataforma/a.json" "$dir/dashboards/plataforma/b.json"
      sed -i 's#      - dashboards/plataforma/a.json#&\n      - dashboards/plataforma/b.json#' "$dir/kustomization.yaml" ;;
    datasource-por-nome) editar_painel "$dir/dashboards/plataforma/a.json" 0 datasource Prometheus ;;
    fora-do-configmap) sed -i '/dashboards\/plataforma\/a.json/d' "$dir/kustomization.yaml" ;;
    carga-no-configmap) sed -i 's#      - dashboards/plataforma/a.json#&\n      - dashboards/carga/load-bff.json#' "$dir/kustomization.yaml" ;;
    pasta-sem-provider)
      mkdir -p "$dir/dashboards/kernel"
      sed 's/"fx-a"/"fx-c"/; s/"Fixture A"/"Fixture C"/' "$dir/dashboards/plataforma/a.json" > "$dir/dashboards/kernel/c.json" ;;
    provider-na-raiz) sed -i 's#path: /var/lib/grafana/dashboards/carga#path: /var/lib/grafana/dashboards#' "$dir/dashboards.yaml" ;;
    promql-invalida) editar_painel "$dir/dashboards/plataforma/a.json" 0 expr 'sum by (le (rate(foo[2m]))' ;;
    nome-antigo) editar_painel "$dir/dashboards/plataforma/a.json" 0 expr 'sum(rate(dmpf_service_requests_total[2m]))' ;;
    label-service) editar_painel "$dir/dashboards/plataforma/a.json" 0 expr 'sum(rate(dmpf_operation_duration_seconds_count{service="bff"}[2m]))' ;;
    tenant-fora-da-admissao) editar_painel "$dir/dashboards/plataforma/a.json" 0 expr 'sum by (dmpf_tenant_id) (rate(dmpf_operation_duration_seconds_count[2m]))' ;;
    janela-curta) editar_painel "$dir/dashboards/plataforma/a.json" 0 expr 'sum(rate(dmpf_operation_duration_seconds_count[1m]))' ;;
    rate-interval-sem-interval) editar_painel "$dir/dashboards/plataforma/a.json" 0 sem-interval x ;;
    otlp-interval-30s) editar_painel "$dir/dashboards/plataforma/a.json" 0 interval 30s ;;
    time-interval-lido)
      sed -i 's/timeInterval: 15s/timeInterval: 1s/' "$dir/datasources.yaml"
      editar_painel "$dir/dashboards/plataforma/a.json" 0 interval 110s ;;
    volumes-trocados)
      sed -i 's#mountPath: /var/lib/grafana/dashboards/plataforma#TROCA#; s#mountPath: /var/lib/grafana/dashboards/carga#mountPath: /var/lib/grafana/dashboards/plataforma#; s#TROCA#mountPath: /var/lib/grafana/dashboards/carga#' "$dir/deployment.yaml" ;;
    raro-sem-serie-nova) editar_painel "$dir/dashboards/plataforma/a.json" 1 expr 'sum by (dmpf_dependency) (increase(dmpf_dependency_retries_total[$__rate_interval]))' ;;
    sem-arquivos) trash "$dir/datasources.yaml" "$dir/dashboards.yaml" ;;
  esac
}

MOTIVOS="$(cat <<'TXT'
sem-escape	url: '${__value.raw}' (${ sem $ à esquerda
no-meio-da-linha	= "${__value.raw}" }' (${ sem $ à esquerda
escape-impar	custo em $$${__value.raw}' (${ sem $ à esquerda
custom-query-sem-escape	trace_id="${__trace.traceId}"
outro-arquivo	dashboards.yaml:12:      folder: ${GF_FOLDER}
trace-logs-com-auditoria	a query do tracesToLogsV2 não exclui scope_name
trace-logs-outro-scope	a query do tracesToLogsV2 não exclui scope_name
trace-logs-sem-custom-query	tracesToLogsV2 sem customQuery: true
sem-traces-to-logs	nenhum tracesToLogsV2 para conferir
sem-datasources	datasources.yaml ausente
sem-arquivos	[paridade] dashboards/plataforma/ sem provider em dashboards.yaml
json-invalido	[json] dashboards/plataforma/a.json: JSON inválido
uid-repetido	[json] uid fx-a repetido
datasource-por-nome	[datasource] dashboards/plataforma/a.json painel 1 'OTLP': datasource deve ser
fora-do-configmap	ConfigMap grafana-dashboards-plataforma lista [], a pasta pede
carga-no-configmap	lista ['dashboards/carga/load-bff.json', 'dashboards/plataforma/a.json']
pasta-sem-provider	[paridade] dashboards/kernel/ sem provider em dashboards.yaml
provider-na-raiz	lê as subpastas e duplica os uid
volumes-trocados	/var/lib/grafana/dashboards/plataforma monta o volume dashboards-carga
promql-invalida	[promql] dashboards/plataforma/a.json painel 1 'OTLP' alvo A: could not parse expression
nome-antigo	nome antigo dmpf_service_*
label-service	label service fora das séries traces_*
tenant-fora-da-admissao	dmpf_tenant_id só em dmpf_admission_rejections_total
janela-curta	janela [1m] menor que 120s (2 × 60s, o intervalo da fonte)
rate-interval-sem-interval	painel 1 'OTLP' alvo A: dmpf_operation_duration_seconds_count chega a cada 60s; o $__rate_interval efetivo do painel é 60s
otlp-interval-30s	painel 1 'OTLP' alvo A: dmpf_operation_duration_seconds_count chega a cada 60s; o $__rate_interval efetivo do painel é 60s
time-interval-lido	o $__rate_interval efetivo do painel é 111s
raro-sem-serie-nova	[evento-raro] dashboards/plataforma/a.json painel 2
TXT
)"

fase_self_test() {
  local base tmp status=0 sabotagem motivo saida
  base="$(mktemp -d)" || return 2
  trap 'trash "$base" >/dev/null 2>&1 || true' RETURN

  montar_fixture "$base"
  printf '\n== vetor positivo ==\n'
  if ( FALHAS=0; fase_estrutural "$base" >/dev/null 2>&1 ); then
    aprovar "a fixture escapada passa"
  else
    falhar "a fixture escapada deveria passar"
    ( FALHAS=0; fase_estrutural "$base" 2>&1 | grep FALHA ) >&2
    status=1
  fi

  printf '\n== vetores negativos (cada um com a mensagem esperada) ==\n'
  while IFS=$'\t' read -r sabotagem motivo; do
    [ -n "$sabotagem" ] || continue
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    sabotar "$tmp" "$sabotagem"
    if saida="$( FALHAS=0; fase_estrutural "$tmp" 2>&1 )"; then
      falhar "a sabotagem '$sabotagem' passou pelo gate"
      status=1
    elif ! grep -qF -- "$motivo" <<<"$saida"; then
      falhar "a sabotagem '$sabotagem' reprovou sem a mensagem esperada: $motivo"
      status=1
    else
      aprovar "reprovou pelo motivo esperado: $sabotagem"
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done <<<"$MOTIVOS"

  if [ "$status" -ne 0 ]; then
    printf '\nAutoteste do gate: REPROVADO.\n' >&2
    return 1
  fi
  printf '\nAutoteste do gate: OK — cada vetor negativo foi recusado.\n'
  return 0
}

uso() {
  cat <<'TXT'
uso: tools/grafana-provisioning-check.sh [--self-test] [--root <dir>]
       tools/grafana-provisioning-check.sh --live <url-do-prometheus> --from <iso> --to <iso>

  (default)    reprova ${ sem $ à esquerda em infra/observability/grafana/*.yaml, o trace→logs
               do datasources.yaml que não exclui a auditoria por scope_name e, nos dashboards:
               JSON e uid, datasource por UID, paridade Compose ↔ Kubernetes (provider,
               volumeMount e ConfigMap por pasta), PromQL pelo promtool (PROMTOOL ou a imagem
               prom/prometheus:v3.14.0 no docker), vocabulário da spec, janela das séries OTLP e
               do cAdvisor, e contador de evento raro sem o termo de série nova
  --self-test  prova o gate contra fixture sintética, um vetor por sabotagem
  --live       roda cada consulta PromQL dos dashboards na janela contra um Prometheus no ar
               e lista os painéis vazios (fora do CI)
  --root       raiz do repositório a verificar (default: a do git)
TXT
}

FASE="structural"
REPO="$ROOT"
LIVE_URL="" LIVE_FROM="" LIVE_TO=""
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test) FASE="self-test" ;;
    --live) FASE="live"; shift; LIVE_URL="${1:-}" ;;
    --from) shift; LIVE_FROM="${1:-}" ;;
    --to) shift; LIVE_TO="${1:-}" ;;
    --root) shift; REPO="${1:-}"; [ -n "$REPO" ] || { uso >&2; exit 2; } ;;
    --root=*) REPO="${1#--root=}" ;;
    -h|--help) uso; exit 0 ;;
    *) printf 'argumento desconhecido: %s\n' "$1" >&2; uso >&2; exit 2 ;;
  esac
  shift
done

case "$FASE" in
  structural) fase_estrutural "$REPO" ;;
  self-test) fase_self_test ;;
  live)
    [ -n "$LIVE_URL" ] && [ -n "$LIVE_FROM" ] && [ -n "$LIVE_TO" ] || { uso >&2; exit 2; }
    python3 -c "$DASHCHECK" live "$REPO" "$GRAFANA" "$LIVE_URL" "$LIVE_FROM" "$LIVE_TO" ;;
esac
