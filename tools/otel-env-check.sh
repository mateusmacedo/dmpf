#!/usr/bin/env bash
# Sem OTEL_EXPORTER_OTLP_PROTOCOL o autoexport usa http/protobuf (autoexport@v0.72.0/metrics.go:52,119-121),
# e o Collector só recebe OTLP/gRPC em :4317; no gRPC, o esquema http:// do endpoint desliga o TLS.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "fora de um repositório git" >&2; exit 2; }
cd "$ROOT" || exit 2

APPS="apps/backend"
TEMPLATES="tools/dmpf-plugin/templates/bounded-context/deploy"
APP_BASE="infra/local/compose/app-base.yml"
ALLOY="infra/observability/alloy"
FALHAS=0

falhar() {
  printf '  FALHA  %s\n' "$1" >&2
  FALHAS=$((FALHAS + 1))
}

aprovar() { printf '  ok     %s\n' "$1"; }

EXIGIDAS=(OTEL_EXPORTER_OTLP_PROTOCOL OTEL_EXPORTER_OTLP_ENDPOINT OTEL_TRACES_SAMPLER_ARG OTEL_GO_X_OBSERVABILITY
  OTEL_BSP_EXPORT_TIMEOUT OTEL_BLRP_EXPORT_TIMEOUT)

EXTRATOR="$(cat <<'AWK'
function aparar(s) { sub(/^[ \t]+/, "", s); sub(/[ \t\r]+$/, "", s); return s }
function sem_aspas(v,   q, resto, i) {
  v = aparar(v); q = substr(v, 1, 1)
  if (q == "'" || q == "\"") {
    resto = substr(v, 2); i = index(resto, q)
    return i > 0 ? substr(resto, 1, i - 1) : resto
  }
  sub(/[ \t]+#.*$/, "", v)
  return v
}
function emitir(n, tipo, chave, valor) { coberto[n SUBSEP chave] = 1; printf "%d\t%s\t%s\t%s\n", n, tipo, chave, valor }
function mencoes(s,   t) {
  sub(/[ \t]#.*$/, "", s)
  while (match(s, /[A-Za-z0-9_]+/)) {
    t = substr(s, RSTART, RLENGTH); s = substr(s, RSTART + RLENGTH)
    if (t ~ /^OT(EL|LP)_/ || t ~ /^(TRACE_SAMPLE_RATE|SERVICE|SERVICE_VERSION|INSTANCE_ID)$/) visto[NR SUBSEP t] = 1
  }
}
function referencias(s,   r) {
  while (match(s, /\$\{[A-Z][A-Z0-9_]*|\$\([A-Z][A-Z0-9_]*\)|\$[A-Z][A-Z0-9_]*/)) {
    r = substr(s, RSTART, RLENGTH); gsub(/[${}()]/, "", r)
    emitir(NR, "ref", r, "")
    s = substr(s, RSTART + RLENGTH)
  }
}
/^[ \t]*#/ || /^[ \t\r]*$/ { next }
{ mencoes($0) }
formato == "env" {
  if (match($0, /^[ \t]*(export[ \t]+)?[A-Za-z_][A-Za-z0-9_]*=/)) {
    chave = substr($0, RSTART, RLENGTH); valor = substr($0, RSTART + RLENGTH)
    sub(/^[ \t]*(export[ \t]+)?/, "", chave); sub(/=$/, "", chave)
    emitir(NR, "decl", chave, sem_aspas(valor)); referencias(valor)
  }
  next
}
{
  linha = $0; recuo = match(linha, /[^ ]/) - 1
  if (pendente != "") {
    if (recuo > recuo_pendente && linha ~ /^[ \t]*value:/) {
      valor = linha; sub(/^[ \t]*value:/, "", valor)
      emitir(n_pendente, "decl", pendente, sem_aspas(valor)); pendente = ""
      referencias(linha); next
    }
    if (recuo <= recuo_pendente) { emitir(n_pendente, "indireta", pendente, ""); pendente = "" }
  }
  if (linha ~ /^[ \t]*-[ \t]+value:/) {
    valor_item = linha; sub(/^[ \t]*-[ \t]+value:/, "", valor_item); valor_item = sem_aspas(valor_item)
    recuo_item = recuo; ha_valor_item = 1
    referencias(linha); next
  }
  if (ha_valor_item && recuo <= recuo_item) ha_valor_item = 0
  if (linha ~ /^[ \t]*(-[ \t]+)?name:[ \t]*['"]?[A-Z][A-Z0-9_]*['"]?[ \t\r]*$/) {
    chave = linha; sub(/^[ \t]*(-[ \t]+)?name:/, "", chave); chave = sem_aspas(chave)
    if (ha_valor_item && linha !~ /^[ \t]*-/ && recuo == recuo_item + 2) {
      emitir(NR, "decl", chave, valor_item); ha_valor_item = 0; next
    }
    pendente = chave; n_pendente = NR; recuo_pendente = (linha ~ /^[ \t]*-/) ? recuo : recuo - 2
    next
  }
  if (match(linha, /^[ \t]*-[ \t]+/)) {
    item = aparar(substr(linha, RSTART + RLENGTH)); q = substr(item, 1, 1)
    if (q == "'" || q == "\"") item = sem_aspas(item)
    if (match(item, /^[A-Z][A-Z0-9_]*=/)) emitir(NR, "decl", substr(item, 1, RLENGTH - 1), substr(item, RLENGTH + 1))
  } else if (match(linha, /^[ \t]*['"]?[A-Z][A-Z0-9_]*['"]?[ \t]*:([ \t]|$)/)) {
    chave = substr(linha, RSTART, RLENGTH); valor = substr(linha, RSTART + RLENGTH)
    sub(/:[ \t]*$/, "", chave); chave = aparar(chave); gsub(/['"]/, "", chave)
    emitir(NR, "decl", chave, sem_aspas(valor))
  }
  referencias(linha)
}
END {
  if (pendente != "") emitir(n_pendente, "indireta", pendente, "")
  for (k in visto) if (!(k in coberto)) { split(k, p, SUBSEP); emitir(p[1], "solta", p[2], "") }
}
AWK
)"

ALLOY_DROP="$(cat <<'AWK'
/^[ \t]*\/\// { next }
{
  linha = $0; nua = linha
  gsub(/"([^"\\]|\\.)*"/, "\"\"", nua)
  abre = gsub(/\{/, "{", nua); fecha = gsub(/\}/, "}", nua)
  if (prof == 0 && linha ~ /^[ \t]*[a-z_.]+[ \t]+"[^"]*"[ \t]*\{/) {
    tipo = linha; sub(/^[ \t]*/, "", tipo); sub(/[ \t].*$/, "", tipo)
    nome = linha; sub(/^[^"]*"/, "", nome); sub(/".*$/, "", nome)
  }
  if (tipo == "discovery.relabel" && !regra && linha ~ /^[ \t]*rule[ \t]*\{/) { regra = 1; prof_regra = prof; fonte = ""; rx = ""; acao = "" }
  if (regra) {
    if (linha ~ /^[ \t]*source_labels[ \t]*=/) { fonte = linha; sub(/^[^=]*=/, "", fonte); gsub(/[ \t\r]/, "", fonte) }
    if (linha ~ /^[ \t]*regex[ \t]*=/) { rx = linha; sub(/^[^=]*=[ \t]*"/, "", rx); sub(/"[ \t\r]*$/, "", rx) }
    if (linha ~ /^[ \t]*action[ \t]*=/) { acao = linha; sub(/^[^=]*=/, "", acao); gsub(/[ \t\r"]/, "", acao) }
  }
  if (tipo ~ /^loki\.source\./ && prof == 1 && linha ~ /^[ \t]*targets[ \t]*=/) {
    alvo = linha; sub(/^[^=]*=/, "", alvo); gsub(/[ \t\r]/, "", alvo)
    print "fonte\t" NR "\t" tipo " \"" nome "\"\t" alvo
  }
  prof += abre - fecha
  if (regra && prof <= prof_regra) {
    if (acao == "drop" && rx != "" && fonte == "[\"" rotulo "\"]") print "drop\t" NR "\t" nome "\t" rx
    regra = 0
  }
  if (prof <= 0) { prof = 0; tipo = "" }
}
AWK
)"

formato_de() {
  local nome="${1##*/}"
  nome="${nome%__tmpl__}"
  case "$nome" in
    .env* | env.*) echo env ;;
    *.yml | *.yaml | *.yaml.tmpl) echo yaml ;;
    *) echo outro ;;
  esac
}

legada() {
  case "$1" in
    TRACE_SAMPLE_RATE | SERVICE | SERVICE_VERSION | INSTANCE_ID) return 0 ;;
    OTLP_*_PORT) return 1 ;;
    OTLP_*) return 0 ;;
  esac
  return 1
}

regra_de() {
  case "$1" in
    OTEL_LOGS_EXPORTER) echo logs-exporter ;;
    OTEL_EXPORTER_OTLP_PROTOCOL | OTEL_EXPORTER_OTLP_TRACES_PROTOCOL | OTEL_EXPORTER_OTLP_METRICS_PROTOCOL | \
      OTEL_EXPORTER_OTLP_LOGS_PROTOCOL) echo protocolo ;;
    OTEL_EXPORTER_OTLP_ENDPOINT | OTEL_EXPORTER_OTLP_TRACES_ENDPOINT | OTEL_EXPORTER_OTLP_METRICS_ENDPOINT | \
      OTEL_EXPORTER_OTLP_LOGS_ENDPOINT) echo endpoint ;;
    OTEL_EXPORTER_OTLP_CERTIFICATE | OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE | OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE | \
      OTEL_EXPORTER_OTLP_LOGS_CERTIFICATE) echo certificado ;;
    OTEL_TRACES_SAMPLER_ARG) echo sampler-arg ;;
    OTEL_GO_X_OBSERVABILITY) echo go-x-observability ;;
    OTEL_BSP_EXPORT_TIMEOUT | OTEL_BLRP_EXPORT_TIMEOUT) echo export-timeout ;;
    *) return 1 ;;
  esac
}

em_hmg() {
  case "$1" in
    */k8s/overlays/hmg/*) return 0 ;;
  esac
  return 1
}

certificado_de() { printf '%s' "${1%_ENDPOINT}_CERTIFICATE"; }

valor_efetivo() {
  local valor="$1"
  if [[ $valor =~ ^\$\{[A-Za-z_][A-Za-z0-9_]*:?-(.*)\}$ ]]; then
    valor="${BASH_REMATCH[1]}"
  fi
  case "$valor" in
    *'${'* | *'$('*) return 1 ;;
  esac
  printf '%s' "$valor"
}

checar_valor() {
  local onde="$1" chave="$2" valor="$3" certificados="${4:- }"
  case "$(regra_de "$chave")" in
    logs-exporter)
      [ "$valor" = otlp ] ||
        falhar "[logs-exporter] $onde: OTEL_LOGS_EXPORTER=$valor; o log das apps sai só por OTLP (otlp)" ;;
    protocolo)
      [ "$valor" = grpc ] ||
        falhar "[protocolo] $onde: $chave=$valor; o Collector só recebe OTLP/gRPC (grpc)" ;;
    endpoint)
      if [[ $valor == https://* ]]; then
        if ! [[ $valor =~ ^https://[^/:@]+:4317/?$ ]]; then
          falhar "[endpoint] $onde: $chave=$valor fora de https://<host>:4317, a porta OTLP/gRPC do Collector"
        elif [[ $certificados != *" OTEL_EXPORTER_OTLP_CERTIFICATE "* && $certificados != *" $(certificado_de "$chave") "* ]]; then
          falhar "[endpoint] $onde: $chave=$valor sem OTEL_EXPORTER_OTLP_CERTIFICATE no mesmo arquivo; sem a CA interna o exporter verifica o Collector pelo pool do sistema"
        fi
      elif [[ $valor != http://* ]]; then
        falhar "[endpoint] $onde: $chave=$valor sem esquema http:// nem https://; sem esquema o exporter gRPC liga TLS sem a CA interna"
      elif ! [[ $valor =~ ^http://[^/:@]+:4317/?$ ]]; then
        falhar "[endpoint] $onde: $chave=$valor fora de http://<host>:4317, a porta OTLP/gRPC do Collector"
      elif em_hmg "$onde"; then
        falhar "[endpoint] $onde: $chave=$valor em texto claro no overlay hmg, que exporta por https:// com OTEL_EXPORTER_OTLP_CERTIFICATE"
      fi ;;
    certificado)
      [[ $valor == /* ]] ||
        falhar "[certificado] $onde: $chave=$valor não é caminho absoluto; o exporter lê a CA do arquivo montado no pod" ;;
    sampler-arg)
      [ "$valor" = 1.0 ] ||
        falhar "[sampler-arg] $onde: OTEL_TRACES_SAMPLER_ARG=$valor; a cabeça grava tudo e a taxa é da cauda (1.0)" ;;
    go-x-observability)
      [ "$valor" = true ] ||
        falhar "[go-x-observability] $onde: OTEL_GO_X_OBSERVABILITY=$valor; sem true o SDK não emite otel.sdk.processor.span.processed" ;;
    export-timeout)
      [ "$valor" = 3000 ] ||
        falhar "[export-timeout] $onde: $chave=$valor; o encerramento divide 10 s entre os sinais (~3,3 s cada), então 3000 ms" ;;
  esac
}

conferir_arquivo() {
  local repo="$1" arquivo="$2" rel formato registros n tipo chave valor efetivo certificados=" " antes="$FALHAS"
  rel="${arquivo#"$repo"/}"
  formato="$(formato_de "$arquivo")"
  [ "$formato" = outro ] && return 0
  if ! registros="$(awk -v formato="$formato" "$EXTRATOR" "$arquivo" | sort -s -k1,1n)"; then
    falhar "[leitura] $rel: o awk não conseguiu extrair as envs"
    return 0
  fi
  while IFS=$'\t' read -r n tipo chave valor; do
    [ "$tipo" = decl ] && [ "$(regra_de "$chave")" = certificado ] && certificados+="$chave "
  done <<<"$registros"
  while IFS=$'\t' read -r n tipo chave valor; do
    [ -n "$chave" ] || continue
    if [ "$tipo" = solta ]; then
      if legada "$chave"; then
        falhar "[env-legada] $rel:$n: $chave numa forma que o gate não lê (flow ou bloco); a env saiu com a RF-E1"
      elif regra_de "$chave" >/dev/null; then
        falhar "[$(regra_de "$chave")] $rel:$n: $chave numa forma que o gate não lê (flow ou bloco); declare chave e valor em linha própria"
      fi
      continue
    fi
    if legada "$chave"; then
      if [ "$tipo" = ref ]; then
        falhar "[env-legada] $rel:$n: referência a $chave, env que saiu com a RF-E1"
      else
        falhar "[env-legada] $rel:$n: $chave saiu com a RF-E1; a configuração é pelas OTEL_* canônicas"
      fi
      continue
    fi
    [ "$tipo" = ref ] && continue
    regra_de "$chave" >/dev/null || continue
    DECLARADAS["$rel|$chave"]=1
    DECLARADAS["${rel%/*}/|$chave"]=1
    if [ "$tipo" = indireta ]; then
      falhar "[$(regra_de "$chave")] $rel:$n: $chave sem value literal para conferir (valueFrom)"
      continue
    fi
    if ! efetivo="$(valor_efetivo "$valor")"; then
      falhar "[$(regra_de "$chave")] $rel:$n: $chave=$valor interpolado sem default literal para conferir"
      continue
    fi
    checar_valor "$rel:$n" "$chave" "$efetivo" "$certificados"
  done <<<"$registros"
  [ "$FALHAS" -eq "$antes" ] && aprovar "$rel"
  return 0
}

motivo_ausente() {
  case "$1" in
    OTEL_EXPORTER_OTLP_PROTOCOL) echo "o default do autoexport é http/protobuf" ;;
    OTEL_EXPORTER_OTLP_ENDPOINT) echo "o default localhost:4317 não tem esquema http:// e liga TLS" ;;
    OTEL_TRACES_SAMPLER_ARG) echo "os manifestos declaram 1.0 (RF-E7)" ;;
    OTEL_GO_X_OBSERVABILITY) echo "sem a flag o SDK não emite otel.sdk.processor.span.processed" ;;
    OTEL_BSP_EXPORT_TIMEOUT | OTEL_BLRP_EXPORT_TIMEOUT) echo "o default do SDK é 30000 ms, acima da fatia de cada sinal no encerramento" ;;
  esac
}

conferir_presenca() {
  local repo="$1" rel="$2" chave antes="$FALHAS"
  if [ ! -f "$repo/$rel" ]; then
    falhar "[fonte] $rel ausente; é a origem das OTEL_* de um modo de implantação"
    return 0
  fi
  for chave in "${EXIGIDAS[@]}"; do
    [ -n "${DECLARADAS["$rel|$chave"]:-}" ] ||
      falhar "[$(regra_de "$chave")] $rel: $chave ausente; $(motivo_ausente "$chave")"
  done
  [ "$FALHAS" -eq "$antes" ] && aprovar "$rel: declara ${EXIGIDAS[*]}"
  return 0
}

conferir_tls_hmg() {
  local dir="$1" chave antes="$FALHAS"
  for chave in OTEL_EXPORTER_OTLP_ENDPOINT OTEL_EXPORTER_OTLP_CERTIFICATE; do
    [ -n "${DECLARADAS["$dir/|$chave"]:-}" ] ||
      falhar "[endpoint] $dir: $chave ausente; sem ele o overlay hmg herda o http:// da base e exporta em texto claro"
  done
  [ "$FALHAS" -eq "$antes" ] && aprovar "$dir: exporta por https:// com OTEL_EXPORTER_OTLP_CERTIFICATE"
  return 0
}

servicos_compose() {
  awk '/^services:/ { dentro = 1; next } /^[A-Za-z]/ { dentro = 0 }
    dentro && /^  [A-Za-z0-9._-]+:/ { n = $1; sub(/:$/, "", n); print n }' "$1"
}

nome_kubernetes() {
  sed -n 's/^[[:space:]]*app\.kubernetes\.io\/name:[[:space:]]*\([A-Za-z0-9._-]*\)[[:space:]]*$/\1/p' "$1" | head -n 1
}

descartada() {
  local app="$1" rx
  while IFS= read -r rx; do
    [ -n "$rx" ] && printf '%s\n' "$app" | grep -Eq -- "^(${rx})\$" && return 0
  done <<<"$2"
  return 1
}

conferir_alloy() {
  local repo="$1" rel="$2" rotulo="$3" regras tipo n comp valor fonte alvo relabel app
  shift 3
  local -a fontes=() faltando
  local -A descarte=()
  if [ ! -f "$repo/$rel" ]; then
    falhar "[alloy] $rel ausente"
    return 0
  fi
  if [ "$#" -eq 0 ]; then
    falhar "[alloy] $rel: nenhum nome de app para conferir o descarte por $rotulo"
    return 0
  fi
  if ! regras="$(awk -v rotulo="$rotulo" "$ALLOY_DROP" "$repo/$rel")"; then
    falhar "[alloy] $rel: o awk não conseguiu ler as rules"
    return 0
  fi
  while IFS=$'\t' read -r tipo n comp valor; do
    case "$tipo" in
      drop) descarte["$comp"]+="${valor//\\\\/\\}"$'\n' ;;
      fonte) fontes+=("$n"$'\t'"$comp"$'\t'"$valor") ;;
    esac
  done <<<"$regras"
  [ "${#fontes[@]}" -gt 0 ] || falhar "[alloy] $rel: nenhum loki.source com targets para conferir o descarte das apps"
  for fonte in ${fontes[@]+"${fontes[@]}"}; do
    IFS=$'\t' read -r n comp alvo <<<"$fonte"
    if ! [[ $alvo =~ ^discovery\.relabel\.([A-Za-z0-9_]+)\.output$ ]]; then
      falhar "[alloy] $rel:$n: $comp lê $alvo, sem passar pelo discovery.relabel que descarta as apps"
      continue
    fi
    relabel="${BASH_REMATCH[1]}"
    faltando=()
    for app in "$@"; do
      descartada "$app" "${descarte[$relabel]:-}" || faltando+=("$app")
    done
    if [ "${#faltando[@]}" -eq 0 ]; then
      aprovar "$rel: $comp lê $alvo, cuja rule drop por $rotulo descarta $*"
    else
      falhar "[alloy] $rel: nenhuma rule drop de discovery.relabel.$relabel por $rotulo descarta ${faltando[*]}; o stdout das apps duplicaria o log OTLP"
    fi
  done
  return 0
}

fase_estrutural() {
  local repo="${1:-$ROOT}" deploy arquivo nome
  local -A DECLARADAS=()
  local -a deploys=() arquivos=() servicos=() nomes=()
  for deploy in "$repo/$APPS"/*/deploy; do
    [ -d "$deploy" ] && deploys+=("$deploy")
  done
  if [ "${#deploys[@]}" -eq 0 ]; then
    echo "nenhum manifesto de app em $repo/$APPS/*/deploy" >&2
    return 2
  fi

  for deploy in "${deploys[@]}"; do
    [ -f "$deploy/.env.example" ] && arquivos+=("$deploy/.env.example")
    [ -f "$deploy/compose.yml" ] && arquivos+=("$deploy/compose.yml")
    if [ -d "$deploy/k8s" ]; then
      while IFS= read -r arquivo; do arquivos+=("$arquivo"); done < <(
        find "$deploy/k8s" -type f \( -name '*.yaml' -o -name '*.yml' -o -name '*.yaml.tmpl' \) | sort
      )
    fi
    if [ -f "$deploy/compose.yml" ]; then
      while IFS= read -r nome; do servicos+=("$nome"); done < <(servicos_compose "$deploy/compose.yml")
    fi
    if [ -f "$deploy/k8s/base/kustomization.yaml" ]; then
      nome="$(nome_kubernetes "$deploy/k8s/base/kustomization.yaml")"
      [ -n "$nome" ] && nomes+=("$nome")
    fi
  done
  for arquivo in "$repo/$APP_BASE" "$repo/infra/local/.env.example" "$repo/infra/local/env.platform.example"; do
    [ -f "$arquivo" ] && arquivos+=("$arquivo")
  done
  if [ -d "$repo/$TEMPLATES" ]; then
    while IFS= read -r arquivo; do arquivos+=("$arquivo"); done < <(find "$repo/$TEMPLATES" -type f -name '*__tmpl__' | sort)
  fi

  printf '\n== OTEL_* e envs legadas nos manifestos das apps e nos templates ==\n'
  for arquivo in "${arquivos[@]}"; do
    conferir_arquivo "$repo" "$arquivo"
  done

  printf '\n== OTEL_* exigidas na origem de cada modo de implantação ==\n'
  for deploy in "${deploys[@]}"; do
    conferir_presenca "$repo" "${deploy#"$repo"/}/.env.example"
    conferir_presenca "$repo" "${deploy#"$repo"/}/k8s/base/configmap.yaml"
  done
  conferir_presenca "$repo" "$APP_BASE"
  conferir_presenca "$repo" "$TEMPLATES/.env.example__tmpl__"
  conferir_presenca "$repo" "$TEMPLATES/k8s/base/configmap.yaml__tmpl__"

  printf '\n== TLS do OTLP no overlay hmg ==\n'
  for deploy in "${deploys[@]}" "$repo/$TEMPLATES"; do
    [ -d "$deploy/k8s/overlays/hmg" ] && conferir_tls_hmg "${deploy#"$repo"/}/k8s/overlays/hmg"
  done

  printf '\n== descarte das apps no Alloy ==\n'
  conferir_alloy "$repo" "$ALLOY/alloy-docker.alloy" __meta_docker_container_label_com_docker_compose_service \
    ${servicos[@]+"${servicos[@]}"}
  conferir_alloy "$repo" "$ALLOY/alloy-kubernetes.alloy" __meta_kubernetes_pod_label_app_kubernetes_io_name \
    ${nomes[@]+"${nomes[@]}"}

  if [ "$FALHAS" -gt 0 ]; then
    printf '\nGate de env OTel: REPROVADO com %d achado(s).\n' "$FALHAS" >&2
    return 1
  fi
  printf '\nGate de env OTel: OK — OTEL_* canônicas, OTLP com TLS em hmg, nenhuma env legada e as apps fora do Alloy.\n'
  return 0
}

montar_fixture() {
  local raiz="$1" app="$1/$APPS/orders/deploy" tpl="$1/$TEMPLATES"
  mkdir -p "$app/k8s/base" "$app/k8s/overlays/dev" "$app/k8s/overlays/hmg" "$raiz/infra/local/compose" "$raiz/$ALLOY" \
    "$tpl/k8s/base" "$tpl/k8s/overlays/dev" "$tpl/k8s/overlays/hmg"

  cat > "$app/.env.example" <<'ENV'
# Processo do orders no host, contra a infra local.
OTEL_SERVICE_NAME=orders
OTEL_RESOURCE_ATTRIBUTES=service.version=local,service.instance.id=orders-local,deployment.environment.name=local
PG_DSN=postgres://orders:orders-local@localhost:5432/orders?sslmode=disable
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_TRACES_SAMPLER_ARG=1.0
OTEL_LOGS_EXPORTER=otlp
OTEL_PROPAGATORS=tracecontext
OTEL_GO_X_OBSERVABILITY=true
OTEL_BSP_EXPORT_TIMEOUT=3000
OTEL_BLRP_EXPORT_TIMEOUT=3000
LOG_LEVEL=debug
ENV

  cat > "$app/compose.yml" <<'YAML'
x-image: &image
  image: ${ORDERS_IMAGE:-orders:local}

x-env: &env
  OTEL_SERVICE_NAME: orders
  PG_DSN: postgres://orders:${ORDERS_PG_PASSWORD:-orders-local}@postgres:5432/orders?sslmode=disable

services:
  dmpf-orders-api:
    <<: *image
    extends:
      file: ../../../../infra/local/compose/app-base.yml
      service: dmpf-app
    environment:
      <<: *env
      OTEL_RESOURCE_ATTRIBUTES: ${OTEL_RESOURCE_ATTRIBUTES:-service.version=local,deployment.environment.name=local},service.instance.id=orders-api,dmpf.process.role=api

  dmpf-orders-relay:
    <<: *image
    extends:
      file: ../../../../infra/local/compose/app-base.yml
      service: dmpf-app
    command: ['--role', 'relay']
    environment:
      <<: *env
      OTEL_RESOURCE_ATTRIBUTES: ${OTEL_RESOURCE_ATTRIBUTES:-service.version=local,deployment.environment.name=local},service.instance.id=orders-relay,dmpf.process.role=relay
YAML

  cat > "$app/k8s/base/kustomization.yaml" <<'YAML'
labels:
  - includeSelectors: true
    pairs:
      app.kubernetes.io/name: orders
      app.kubernetes.io/part-of: dmpf
resources:
  - configmap.yaml
  - deployment-api.yaml
YAML

  cat > "$app/k8s/base/configmap.yaml" <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders
data:
  GRPC_ADDR: ':9090'
  OTEL_SERVICE_NAME: orders
  OTEL_EXPORTER_OTLP_PROTOCOL: grpc
  OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4317
  OTEL_TRACES_SAMPLER_ARG: '1.0'
  OTEL_LOGS_EXPORTER: otlp
  OTEL_PROPAGATORS: tracecontext
  OTEL_GO_X_OBSERVABILITY: 'true'
  OTEL_BSP_EXPORT_TIMEOUT: '3000'
  OTEL_BLRP_EXPORT_TIMEOUT: '3000'
YAML

  cat > "$app/k8s/base/deployment-api.yaml" <<'YAML'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: orders-api
spec:
  template:
    spec:
      containers:
        - name: api
          args: ['--role', 'api']
          envFrom:
            - configMapRef:
                name: orders
          env:
            - name: K8S_POD_NAME
              valueFrom:
                fieldRef:
                  fieldPath: metadata.name
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: $(OTEL_RESOURCE_ATTRIBUTES),service.instance.id=$(K8S_POD_NAME),dmpf.process.role=api
YAML

  cat > "$app/k8s/overlays/dev/configmap-orders-patch.yaml" <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders
data:
  KAFKA_INSECURE: 'true'
  OTEL_RESOURCE_ATTRIBUTES: service.version=dev,deployment.environment.name=dev
  OTEL_EXPORTER_OTLP_METRICS_PROTOCOL: grpc
  OTEL_EXPORTER_OTLP_METRICS_ENDPOINT: http://otel-collector:4317
YAML

  cat > "$app/k8s/overlays/hmg/configmap-orders-patch.yaml" <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: orders
data:
  KAFKA_INSECURE: 'false'
  OTEL_RESOURCE_ATTRIBUTES: service.version=hmg,deployment.environment.name=hmg
  OTEL_EXPORTER_OTLP_ENDPOINT: https://otel-collector:4317
  OTEL_EXPORTER_OTLP_CERTIFICATE: /etc/dmpf/otel/ca.crt
YAML

  cat > "$raiz/$APP_BASE" <<'YAML'
services:
  dmpf-app:
    profiles: [dmpf]
    environment:
      OTEL_EXPORTER_OTLP_PROTOCOL: grpc
      OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4317
      OTEL_TRACES_SAMPLER_ARG: '1.0'
      OTEL_LOGS_EXPORTER: otlp
      OTEL_PROPAGATORS: tracecontext
      OTEL_GO_X_OBSERVABILITY: 'true'
      OTEL_BSP_EXPORT_TIMEOUT: '3000'
      OTEL_BLRP_EXPORT_TIMEOUT: '3000'
      LOG_LEVEL: debug
YAML

  cat > "$raiz/infra/local/.env.example" <<'ENV'
# Porta do Collector publicada no host; nenhuma app a lê
OTLP_GRPC_PORT=4317
OTEL_RESOURCE_ATTRIBUTES=service.version=local,deployment.environment.name=local
ENV

  cat > "$tpl/.env.example__tmpl__" <<'ENV'
#   set -a; . apps/backend/<%- name %>/deploy/.env; set +a; pnpm nx run <%- name %>:serve-api
OTEL_SERVICE_NAME=<%- name %>
GRPC_ADDR=127.0.0.1:<%- grpcPort %>
KAFKA_<%- envName %>_TOPIC=<%- name %>.events
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
OTEL_TRACES_SAMPLER_ARG=1.0
OTEL_LOGS_EXPORTER=otlp
OTEL_GO_X_OBSERVABILITY=true
OTEL_BSP_EXPORT_TIMEOUT=3000
OTEL_BLRP_EXPORT_TIMEOUT=3000
ENV

  cat > "$tpl/compose.yml__tmpl__" <<'YAML'
x-env: &env
  OTEL_SERVICE_NAME: <%- name %>

services:
  dmpf-<%- name %>-api:
    extends:
      file: ../../../../infra/local/compose/app-base.yml
      service: dmpf-app
    environment:
      <<: *env
      OTEL_RESOURCE_ATTRIBUTES: ${OTEL_RESOURCE_ATTRIBUTES:-service.version=local,deployment.environment.name=local},service.instance.id=<%- name %>-api,dmpf.process.role=api
      KAFKA_<%- envName %>_TOPIC: ${KAFKA_<%- envName %>_TOPIC:-<%- name %>.events}
YAML

  cat > "$tpl/k8s/base/configmap.yaml__tmpl__" <<'YAML'
apiVersion: v1
kind: ConfigMap
metadata:
  name: <%- name %>
data:
  KAFKA_<%- envName %>_TOPIC: <%- name %>.events
  OTEL_SERVICE_NAME: <%- name %>
  OTEL_EXPORTER_OTLP_PROTOCOL: grpc
  OTEL_EXPORTER_OTLP_ENDPOINT: http://otel-collector:4317
  OTEL_TRACES_SAMPLER_ARG: '1.0'
  OTEL_LOGS_EXPORTER: otlp
  OTEL_PROPAGATORS: tracecontext
  OTEL_GO_X_OBSERVABILITY: 'true'
  OTEL_BSP_EXPORT_TIMEOUT: '3000'
  OTEL_BLRP_EXPORT_TIMEOUT: '3000'
YAML

  cat > "$tpl/k8s/base/deployment-api.yaml__tmpl__" <<'YAML'
spec:
  template:
    spec:
      containers:
        - name: api
          env:
            - name: OTEL_RESOURCE_ATTRIBUTES
              value: $(OTEL_RESOURCE_ATTRIBUTES),service.instance.id=$(K8S_POD_NAME),dmpf.process.role=api
YAML

  cat > "$tpl/k8s/overlays/dev/configmap-__name__-patch.yaml__tmpl__" <<'YAML'
data:
  GRPC_INSECURE: 'true'
  OTEL_RESOURCE_ATTRIBUTES: service.version=dev,deployment.environment.name=dev
YAML

  cat > "$tpl/k8s/overlays/hmg/configmap-__name__-patch.yaml__tmpl__" <<'YAML'
data:
  GRPC_TLS_CERT_FILE: /etc/dmpf/grpc/tls.crt
  OTEL_RESOURCE_ATTRIBUTES: service.version=hmg,deployment.environment.name=hmg
  OTEL_EXPORTER_OTLP_ENDPOINT: https://otel-collector:4317
  OTEL_EXPORTER_OTLP_CERTIFICATE: /etc/dmpf/otel/ca.crt
YAML

  printf '{ "app": "<%%- name %%>", "image": { "env": "<%%- envName %%>_IMAGE" } }\n' > "$tpl/infra.json__tmpl__"

  cat > "$raiz/$ALLOY/alloy-docker.alloy" <<'ALLOY'
// Coleta de logs da infraestrutura no Compose.
discovery.docker "containers" {
  host = "unix:///var/run/docker.sock"
}

discovery.relabel "containers" {
  targets = discovery.docker.containers.targets

  rule {
    source_labels = ["__meta_docker_container_label_com_docker_compose_service"]
    regex         = "dmpf-.*"
    action        = "drop"
  }

  rule {
    source_labels = ["__meta_docker_container_label_com_docker_compose_service"]
    target_label  = "compose_service"
  }
}

loki.source.docker "containers" {
  host       = "unix:///var/run/docker.sock"
  targets    = discovery.relabel.containers.output
  forward_to = [loki.write.local.receiver]
}
ALLOY

  cat > "$raiz/$ALLOY/alloy-kubernetes.alloy" <<'ALLOY'
discovery.kubernetes "pods" {
  role = "pod"
}

discovery.relabel "pods" {
  targets = discovery.kubernetes.pods.targets

  rule {
    source_labels = ["__meta_kubernetes_pod_label_app_kubernetes_io_name"]
    regex         = "bff|orders"
    action        = "drop"
  }

  rule {
    source_labels = ["__meta_kubernetes_pod_label_app_kubernetes_io_name"]
    target_label  = "app"
  }
}

loki.source.kubernetes "pods" {
  targets    = discovery.relabel.pods.output
  forward_to = [loki.write.local.receiver]
}
ALLOY
}

inserir_apos() {
  local arquivo="$1" ancora="$2" texto="$3"
  awk -v ancora="$ancora" -v texto="$texto" '{ print } !feito && index($0, ancora) { print texto; feito = 1 }' \
    "$arquivo" > "$arquivo.novo" && mv "$arquivo.novo" "$arquivo"
}

sabotar() {
  local raiz="$1" tipo="$2" app="$1/$APPS/orders/deploy" tpl="$1/$TEMPLATES"
  case "$tipo" in
    endpoint-sem-esquema) sed -i 's#ENDPOINT: http://otel-collector:4317#ENDPOINT: otel-collector:4317#' "$raiz/$APP_BASE" ;;
    endpoint-fora-da-porta) sed -i 's#http://otel-collector:4317#http://otel-collector:4318#' "$app/k8s/base/configmap.yaml" ;;
    endpoint-https-sem-certificado)
      sed -i 's#ENDPOINT: http://otel-collector:4317#ENDPOINT: https://otel-collector:4317#' "$app/k8s/base/configmap.yaml" ;;
    endpoint-https-fora-da-porta)
      sed -i 's#https://otel-collector:4317#https://otel-collector:4318#' "$app/k8s/overlays/hmg/configmap-orders-patch.yaml" ;;
    endpoint-http-em-hmg)
      sed -i 's#https://otel-collector:4317#http://otel-collector:4317#' "$app/k8s/overlays/hmg/configmap-orders-patch.yaml" ;;
    endpoint-hmg-herdado-da-base)
      sed -i '/OTEL_EXPORTER_OTLP_\(ENDPOINT\|CERTIFICATE\)/d' "$tpl/k8s/overlays/hmg/configmap-__name__-patch.yaml__tmpl__" ;;
    certificado-ausente-em-hmg) sed -i '/OTEL_EXPORTER_OTLP_CERTIFICATE/d' "$app/k8s/overlays/hmg/configmap-orders-patch.yaml" ;;
    certificado-relativo)
      sed -i 's#CERTIFICATE: /etc/dmpf/otel/ca.crt#CERTIFICATE: ca.crt#' "$app/k8s/overlays/hmg/configmap-orders-patch.yaml" ;;
    https-com-certificado-na-base)
      sed -i 's#ENDPOINT: http://otel-collector:4317#ENDPOINT: https://otel-collector:4317#' "$app/k8s/base/configmap.yaml"
      printf '  OTEL_EXPORTER_OTLP_CERTIFICATE: /etc/dmpf/otel/ca.crt\n' >> "$app/k8s/base/configmap.yaml" ;;
    https-de-metrics-com-certificado-do-sinal)
      sed -i 's#METRICS_ENDPOINT: http://#METRICS_ENDPOINT: https://#' "$app/k8s/overlays/dev/configmap-orders-patch.yaml"
      printf '  OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE: /etc/dmpf/otel/ca.crt\n' >> "$app/k8s/overlays/dev/configmap-orders-patch.yaml" ;;
    protocolo-http-com-4317) sed -i 's#PROTOCOL: grpc#PROTOCOL: http/protobuf#' "$app/k8s/base/configmap.yaml" ;;
    protocolo-ausente) sed -i '/^OTEL_EXPORTER_OTLP_PROTOCOL=/d' "$app/.env.example" ;;
    protocolo-interpolado) sed -i 's#PROTOCOL: grpc#PROTOCOL: ${OTEL_EXPORTER_OTLP_PROTOCOL:-http/protobuf}#' "$raiz/$APP_BASE" ;;
    protocolo-de-traces-http) printf '  OTEL_EXPORTER_OTLP_TRACES_PROTOCOL: http/protobuf\n' >> "$app/k8s/base/configmap.yaml" ;;
    protocolo-de-metrics-interpolado)
      inserir_apos "$raiz/$APP_BASE" 'OTEL_EXPORTER_OTLP_PROTOCOL: grpc' \
        '      OTEL_EXPORTER_OTLP_METRICS_PROTOCOL: ${OTEL_EXPORTER_OTLP_METRICS_PROTOCOL:-http/protobuf}' ;;
    protocolo-de-logs-no-deployment)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - name: OTEL_EXPORTER_OTLP_LOGS_PROTOCOL\n              value: http/protobuf" ;;
    endpoint-de-traces-sem-esquema) printf 'OTEL_EXPORTER_OTLP_TRACES_ENDPOINT=localhost:4317\n' >> "$app/.env.example" ;;
    endpoint-de-traces-sem-value)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - name: OTEL_EXPORTER_OTLP_TRACES_ENDPOINT\n              valueFrom:\n                configMapKeyRef:\n                  name: orders\n                  key: endpoint" ;;
    endpoint-de-metrics-fora-da-porta)
      sed -i 's#METRICS_ENDPOINT: http://otel-collector:4317#METRICS_ENDPOINT: http://otel-collector:4318#' \
        "$app/k8s/overlays/dev/configmap-orders-patch.yaml" ;;
    endpoint-de-logs-em-flow)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - { name: OTEL_EXPORTER_OTLP_LOGS_ENDPOINT, value: http://otel-collector:4317 }" ;;
    sampler-arg-0.1)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - name: OTEL_TRACES_SAMPLER_ARG\n              value: '0.1'" ;;
    sampler-arg-ausente) sed -i '/OTEL_TRACES_SAMPLER_ARG/d' "$tpl/k8s/base/configmap.yaml__tmpl__" ;;
    flag-ausente) sed -i '/OTEL_GO_X_OBSERVABILITY/d' "$raiz/$APP_BASE" ;;
    timeout-de-spans-ausente) sed -i '/OTEL_BSP_EXPORT_TIMEOUT/d' "$raiz/$APP_BASE" ;;
    timeout-de-logs-ausente) sed -i '/OTEL_BLRP_EXPORT_TIMEOUT/d' "$tpl/k8s/base/configmap.yaml__tmpl__" ;;
    timeout-de-spans-default) sed -i 's/^OTEL_BSP_EXPORT_TIMEOUT=3000$/OTEL_BSP_EXPORT_TIMEOUT=30000/' "$app/.env.example" ;;
    timeout-de-logs-em-segundos)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - name: OTEL_BLRP_EXPORT_TIMEOUT\n              value: 3s" ;;
    flag-false) sed -i 's/^OTEL_GO_X_OBSERVABILITY=true$/OTEL_GO_X_OBSERVABILITY=false/' "$tpl/.env.example__tmpl__" ;;
    logs-exporter-console) printf '  OTEL_LOGS_EXPORTER: console\n' >> "$app/k8s/overlays/dev/configmap-orders-patch.yaml" ;;
    env-legada) printf 'OTLP_ENDPOINT=localhost:4317\n' >> "$app/.env.example" ;;
    env-legada-da-familia-otlp) printf 'OTLP_HEADERS=authorization=x\n' >> "$raiz/infra/local/.env.example" ;;
    env-legada-no-deployment)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - name: INSTANCE_ID\n              valueFrom:\n                fieldRef:\n                  fieldPath: metadata.name" ;;
    env-legada-interpolada) sed -i 's/service\.version=local,deployment/service.version=${SERVICE_VERSION:-local},deployment/' "$tpl/compose.yml__tmpl__" ;;
    env-em-flow-no-deployment)
      inserir_apos "$app/k8s/base/deployment-api.yaml" 'fieldPath: metadata.name' \
        "            - { name: OTEL_LOGS_EXPORTER, value: console }" ;;
    env-legada-em-flow-no-compose)
      printf '\n  dmpf-orders-consumer:\n    <<: *image\n    environment: { OTEL_SERVICE_NAME: orders, SERVICE_VERSION: local }\n' >> "$app/compose.yml" ;;
    alloy-docker-sem-descarte) sed -i '/action *= "drop"/d' "$raiz/$ALLOY/alloy-docker.alloy" ;;
    alloy-kubernetes-sem-uma-app) sed -i 's/"bff|orders"/"bff"/' "$raiz/$ALLOY/alloy-kubernetes.alloy" ;;
    alloy-fonte-sem-relabel)
      sed -i 's/targets    = discovery\.relabel\.containers\.output/targets    = discovery.docker.containers.targets/' \
        "$raiz/$ALLOY/alloy-docker.alloy" ;;
    sem-manifestos) trash "$raiz/apps" ;;
  esac
}

fase_self_test() {
  local base tmp status=0 vetor sabotagem esperado saida codigo achados
  base="$(mktemp -d)" || return 2
  trap 'trash "$base" >/dev/null 2>&1 || true' RETURN

  montar_fixture "$base"
  printf '\n== vetor positivo ==\n'
  if ( FALHAS=0; fase_estrutural "$base" >/dev/null 2>&1 ); then
    aprovar "a fixture íntegra passa"
  else
    falhar "a fixture íntegra deveria passar"
    ( FALHAS=0; fase_estrutural "$base" 2>&1 | grep FALHA ) >&2
    status=1
  fi

  printf '\n== vetores negativos (sabotagem:motivo esperado) ==\n'
  local -a vetores=(
    endpoint-sem-esquema:endpoint
    endpoint-fora-da-porta:endpoint
    endpoint-https-sem-certificado:endpoint
    endpoint-https-fora-da-porta:endpoint
    endpoint-http-em-hmg:endpoint
    endpoint-hmg-herdado-da-base:endpoint
    certificado-ausente-em-hmg:endpoint
    certificado-relativo:certificado
    protocolo-http-com-4317:protocolo
    protocolo-ausente:protocolo
    protocolo-interpolado:protocolo
    protocolo-de-traces-http:protocolo
    protocolo-de-metrics-interpolado:protocolo
    protocolo-de-logs-no-deployment:protocolo
    endpoint-de-traces-sem-esquema:endpoint
    endpoint-de-traces-sem-value:endpoint
    endpoint-de-metrics-fora-da-porta:endpoint
    endpoint-de-logs-em-flow:endpoint
    sampler-arg-0.1:sampler-arg
    sampler-arg-ausente:sampler-arg
    flag-ausente:go-x-observability
    flag-false:go-x-observability
    timeout-de-spans-ausente:export-timeout
    timeout-de-logs-ausente:export-timeout
    timeout-de-spans-default:export-timeout
    timeout-de-logs-em-segundos:export-timeout
    logs-exporter-console:logs-exporter
    env-legada:env-legada
    env-legada-da-familia-otlp:env-legada
    env-legada-no-deployment:env-legada
    env-legada-interpolada:env-legada
    env-em-flow-no-deployment:logs-exporter
    env-legada-em-flow-no-compose:env-legada
    alloy-docker-sem-descarte:alloy
    alloy-kubernetes-sem-uma-app:alloy
    alloy-fonte-sem-relabel:alloy
    sem-manifestos:-
  )
  for vetor in "${vetores[@]}"; do
    sabotagem="${vetor%%:*}"
    esperado="${vetor#*:}"
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    sabotar "$tmp" "$sabotagem"
    saida="$(FALHAS=0; fase_estrutural "$tmp" 2>&1 >/dev/null)"
    codigo=$?
    achados="$(grep -F 'FALHA' <<<"$saida")"
    if [ "$codigo" -eq 0 ]; then
      falhar "a sabotagem '$sabotagem' passou pelo gate"
      status=1
    elif [ "$esperado" = "-" ]; then
      if [ "$codigo" -eq 2 ]; then
        aprovar "reprovou: $sabotagem (nada a conferir)"
      else
        falhar "a sabotagem '$sabotagem' deveria sair com 2, saiu com $codigo"
        status=1
      fi
    elif [ -n "$achados" ] && ! grep -qvF -- "FALHA  [$esperado]" <<<"$achados"; then
      aprovar "reprovou: $sabotagem [$esperado]"
    else
      falhar "a sabotagem '$sabotagem' reprovou por outro motivo (esperado [$esperado])"
      printf '%s\n' "${achados:-$saida}" >&2
      status=1
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done

  printf '\n== variações aceitas (o gate precisa aprovar) ==\n'
  for vetor in https-com-certificado-na-base https-de-metrics-com-certificado-do-sinal; do
    tmp="$(mktemp -d)" || return 2
    montar_fixture "$tmp"
    sabotar "$tmp" "$vetor"
    if saida="$(FALHAS=0; fase_estrutural "$tmp" 2>&1 >/dev/null)"; then
      aprovar "aprovou: $vetor"
    else
      falhar "a variação '$vetor' deveria passar pelo gate"
      grep -F 'FALHA' <<<"$saida" >&2
      status=1
    fi
    trash "$tmp" >/dev/null 2>&1 || true
  done

  if [ "$status" -ne 0 ]; then
    printf '\nAutoteste do gate: REPROVADO.\n' >&2
    return 1
  fi
  printf '\nAutoteste do gate: OK — cada vetor negativo foi recusado pelo motivo certo.\n'
  return 0
}

uso() {
  cat <<'TXT'
uso: tools/otel-env-check.sh [--self-test] [--root <dir>]

  (default)    confere as OTEL_* dos manifestos das apps e dos templates do generator
  --self-test  prova o gate contra fixture sintética, um vetor por sabotagem
  --root       raiz do repositório a verificar (default: a do git)
TXT
}

FASE="structural"
REPO="$ROOT"
while [ $# -gt 0 ]; do
  case "$1" in
    --self-test) FASE="self-test" ;;
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
esac
