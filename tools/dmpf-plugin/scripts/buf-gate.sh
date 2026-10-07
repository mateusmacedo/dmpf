#!/usr/bin/env bash
# Gates Buf de um módulo de contrato: lint | pins | generate-check | breaking (+ warmup).
# Uso: buf-gate.sh <subcomando> <diretório com buf.yaml e buf.gen.yaml> [--project <projeto Nx>]
# Fail-closed (BUF-12): condição que o gate não consegue avaliar reprova; não há bypass.
# Temporários ficam em /tmp (como em dmpf-gate-check.sh), sem utilitário de lixeira.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)" || { echo "REPROVADO: fora de um repositorio git" >&2; exit 2; }
cd "$ROOT" || exit 2

uso() { echo "uso: buf-gate.sh lint | pins | generate-check | breaking | warmup <diretorio-do-modulo> [--project <nome>]" >&2; exit 2; }

SUB="${1:-}"
MOD="${2:-}"
[ -n "$SUB" ] && [ -n "$MOD" ] || uso
shift 2
PROJECT=""
while [ $# -gt 0 ]; do
  case "$1" in
    --project) PROJECT="${2:-}"; [ -n "$PROJECT" ] || uso; shift 2 ;;
    *) uso ;;
  esac
done
MOD="${MOD%/}"
[ -d "$MOD" ] || { echo "REPROVADO: diretorio do modulo inexistente: $MOD" >&2; exit 2; }
[ -n "$PROJECT" ] || PROJECT="$(basename "$MOD")"

BUF_YAML="$MOD/buf.yaml"
BUF_GEN="$MOD/buf.gen.yaml"
BUF_SH="$(dirname "$(realpath "$0")")/buf.sh"
VERSIONS="$(dirname "$(realpath "$0")")/../versions.json"
buf_pin() { node -p 'require(process.argv[1]).buf' "$VERSIONS"; }

buf() { bash "$BUF_SH" "$@"; }
reprovar() { echo "REPROVADO: $*" >&2; exit 1; }
aviso() { echo "AVISO: $*" >&2; }

# Caminhos dos módulos com name, os publicados, de um buf.yaml lido da entrada padrão.
publicados() {
  awk '
    function fecha() { if (p != "" && n) print p; p = ""; n = 0 }
    /^[[:space:]]*-[[:space:]]*path:/ { fecha(); p = $0; sub(/^[[:space:]]*-[[:space:]]*path:[[:space:]]*/, "", p); sub(/[[:space:]]*$/, "", p); next }
    /^[[:space:]]+name:/ { if (p != "") n = 1; next }
    /^[^[:space:]]/ { fecha() }
    END { fecha() }
  '
}

modulos() { publicados <"$BUF_YAML"; }

# O buf.yaml de BUF_YAML só com a entrada do módulo informado em modules[].
so_modulo() { # modulo
  awk -v alvo="$1" '
    /^[[:space:]]*-[[:space:]]*path:/ { v = $0; sub(/^[[:space:]]*-[[:space:]]*path:[[:space:]]*/, "", v); sub(/[[:space:]]*$/, "", v); dentro = 1; manter = (v == alvo) }
    dentro && /^[^[:space:]-]/ { dentro = 0 }
    !dentro || manter { print }
  ' "$BUF_YAML"
}

# Diretório do gerado, relativo à raiz, a partir do primeiro `out` do buf.gen.yaml.
gen_dir() {
  local out
  out="$(grep -E '^\s*out:' "$BUF_GEN" | head -1 | sed -E 's/^\s*out:\s*//; s/\s*$//')"
  [ -n "$out" ] || reprovar "$BUF_GEN sem 'out' de plugin"
  realpath -m --relative-to="$ROOT" "$ROOT/$MOD/$out"
}

# Módulo Go dono do gerado: o go.mod mais próximo subindo a partir do diretório gerado.
lib_dir() {
  local d
  d="$(gen_dir)"
  while [ "$d" != "." ] && [ "$d" != "/" ]; do
    [ -f "$d/go.mod" ] && { echo "$d"; return; }
    d="$(dirname "$d")"
  done
  reprovar "nenhum go.mod acima de $(gen_dir)"
}

gate_lint() {
  local lib
  lib="$(lib_dir)"
  buf format --diff --exit-code "$MOD" || reprovar "buf format encontrou diferencas em $MOD"
  buf lint "$MOD" || reprovar "buf lint (STANDARD) reprovou"
  # P0-3 é structurally reviewable: a vedação vale para qualquer texto do artefato.
  if grep -rIliE 'exactly[ -]once' "$MOD" "$lib" 2>/dev/null | grep -q .; then
    grep -rIliE 'exactly[ -]once' "$MOD" "$lib" >&2
    reprovar "artefato declara ou sugere exactly-once (RFC 2.3, P0-3)"
  fi
  echo "lint: OK"
}

gate_pins() {
  local cli_pin plugin_pin plugin_ver runtime_ver deps_line lib outro outro_ver
  cli_pin="$(buf_pin)" || reprovar "$VERSIONS ilegivel (BUF-06)"
  [[ "$cli_pin" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || reprovar "$VERSIONS: buf $cli_pin nao e pin exato vX.Y.Z (BUF-06)"

  plugin_pin="$(grep -oE "protoc-gen-[a-z0-9-]+@[^][:space:]\"',]+" "$BUF_GEN" || true)"
  [ -n "$plugin_pin" ] || reprovar "$BUF_GEN sem plugin local pinado (BUF-06)"
  while IFS= read -r pin; do
    echo "$pin" | grep -qE '@v[0-9]+\.[0-9]+\.[0-9]+$' || reprovar "plugin sem pin exato: $pin (BUF-06)"
  done <<<"$plugin_pin"
  if grep -qiE 'buf\.build/[^[:space:]]+/[^[:space:]]+:(latest|v[0-9]+$)' "$BUF_GEN"; then
    reprovar "$BUF_GEN referencia plugin remoto sem pin exato (BUF-06)"
  fi

  lib="$(lib_dir)"
  plugin_ver="$(echo "$plugin_pin" | grep -oE 'protoc-gen-go@v[0-9]+\.[0-9]+\.[0-9]+' | sed 's/.*@//')"
  runtime_ver="$(grep -oE 'google\.golang\.org/protobuf v[0-9]+\.[0-9]+\.[0-9]+' "$lib/go.mod" | awk '{print $2}')"
  [ -n "$plugin_ver" ] || reprovar "protoc-gen-go sem versao em $BUF_GEN"
  [ -n "$runtime_ver" ] || reprovar "google.golang.org/protobuf ausente de $lib/go.mod"
  # O gerado exige runtime >= versão do plugin; igualar os dois é o único estado sem drift silencioso.
  [ "$plugin_ver" = "$runtime_ver" ] || reprovar "protoc-gen-go $plugin_ver difere de google.golang.org/protobuf $runtime_ver no go.mod"

  # Todos os módulos de contrato geram com o mesmo plugin: igualdade por módulo mais
  # igualdade entre módulos deixa um único runtime protobuf no workspace.
  while IFS= read -r outro; do
    [ -n "$outro" ] || continue
    outro_ver="$(grep -oE 'protoc-gen-go@v[0-9]+\.[0-9]+\.[0-9]+' "$outro" | head -1 | sed 's/.*@//')"
    [ "$outro_ver" = "$plugin_ver" ] || reprovar "protoc-gen-go diverge entre modulos: $BUF_GEN tem $plugin_ver, $outro tem ${outro_ver:-nenhum} (BUF-06)"
  done < <(git ls-files --cached --others --exclude-standard -- '*buf.gen.yaml' 'buf.gen.yaml' | sort -u)

  deps_line="$(grep -E '^deps:' "$BUF_YAML" || true)"
  [ -n "$deps_line" ] || reprovar "$BUF_YAML sem chave deps (BUF-02)"
  if ! echo "$deps_line" | grep -qE '^deps:\s*\[\s*\]\s*$'; then
    [ -f "$MOD/buf.lock" ] || reprovar "deps declaradas sem buf.lock versionado (BUF-02)"
  fi
  echo "pins: OK (buf $cli_pin, protoc-gen-go@$plugin_ver = protobuf $runtime_ver)"
}

gate_generate_check() {
  local t1 t2 gen
  gen="$(gen_dir)"
  t1="$(mktemp -d)" || reprovar "mktemp"
  t2="$(mktemp -d)" || reprovar "mktemp"
  (cd "$MOD" && buf generate -o "$t1/$MOD") || reprovar "buf generate (1) falhou"
  (cd "$MOD" && buf generate -o "$t2/$MOD") || reprovar "buf generate (2) falhou"
  diff -r "$t1" "$t2" >/dev/null || { diff -r "$t1" "$t2" | head -20 >&2; reprovar "duas geracoes consecutivas divergem (BUF-11)"; }
  [ -d "$t1/$gen" ] || reprovar "geracao nao produziu $gen"
  diff -r "$t1/$gen" "$gen" >/dev/null || { diff -r "$t1/$gen" "$gen" | head -40 >&2; reprovar "drift entre o gerado e o versionado em $gen (BUF-11, REP-02)"; }
  echo "generate-check: OK"
}

# Raízes de módulo Buf (<dir do buf.yaml>/<path>) rastreadas num commit.
raizes_em() { # ref
  local ref="$1" arquivo dir p
  while IFS= read -r arquivo; do
    [ -n "$arquivo" ] || continue
    dir="$(dirname "$arquivo")"
    while IFS= read -r p; do
      [ -n "$p" ] || continue
      if [ "$dir" = "." ]; then echo "$p"; else echo "$dir/$p"; fi
    done < <(git show "$ref:$arquivo" | publicados)
  done < <(git ls-tree -r --name-only "$ref" | grep -E '(^|/)buf\.yaml$')
}

# Diretórios de pacote (com .proto), relativos à raiz do módulo, num commit.
pacotes_em() { # ref raiz
  git ls-tree -r --name-only "$1" -- "$2/" | grep -E '\.proto$' | sed "s|^$2/||" | xargs -rn1 dirname | sort -u
}

gate_breaking() {
  local base modulo raiz lista indice against rel origem arquivo herdado origens raizes_head presente
  base="${NX_BASE:-}"
  [ -n "$base" ] || reprovar "baseline nao declarado: NX_BASE vazio (BUF-05)"
  git rev-parse --verify --quiet "${base}^{commit}" >/dev/null || reprovar "baseline irresolvivel: $base (BUF-05)"
  [ -f "$BUF_YAML" ] || reprovar "$BUF_YAML ausente: sem workspace nao ha modulo a verificar (BUF-01)"
  lista="$(modulos)"
  # Lista vazia deixaria o laço sem iterar e o gate sairia 0 sem olhar nada.
  [ -n "$lista" ] || reprovar "$BUF_YAML sem modulos declarados em 'modules[].path' com 'name' (BUF-01)"

  # A identidade de um .proto publicado é o pacote: indexa, por diretório relativo à
  # raiz do módulo (REP-01), onde cada pacote estava na base.
  indice="$(mktemp)" || reprovar "mktemp"
  while IFS= read -r raiz; do
    [ -n "$raiz" ] || continue
    pacotes_em "$base" "$raiz" | sed "s|$|	$raiz|" >>"$indice"
  done < <(raizes_em "$base")

  while IFS= read -r modulo; do
    raiz="$MOD/$modulo"
    against="$(mktemp -d)" || reprovar "mktemp"
    so_modulo "$modulo" >"$against/buf.yaml" || reprovar "copia de $BUF_YAML"
    mkdir -p "$against/$modulo"
    herdado=0
    origens=""
    while IFS= read -r rel; do
      [ -n "$rel" ] || continue
      origem="$(awk -F'\t' -v r="$rel" -v p="$raiz" '$1 == r { if ($2 == p) { print p; exit } o = o ? o : $2 } END { if (o) print o }' "$indice" | head -1)"
      [ -n "$origem" ] || continue
      herdado=1
      case " $origens " in *" $origem "*) ;; *) origens="${origens:+$origens }$origem" ;; esac
      mkdir -p "$against/$modulo/$rel"
      while IFS= read -r arquivo; do
        git show "$base:$arquivo" >"$against/$modulo/$rel/$(basename "$arquivo")" || reprovar "nao foi possivel extrair $arquivo de $base (BUF-05)"
      done < <(git ls-tree --name-only "$base" -- "$origem/$rel/" | grep -E '\.proto$')
    done < <(find "$raiz" -name '*.proto' -printf '%h\n' 2>/dev/null | sed "s|^$raiz/\{0,1\}||; s|^$|.|" | sort -u)

    if [ "$herdado" -eq 0 ]; then
      aviso "modulo $raiz em estado 'sem baseline': nenhum pacote dele publicado em $base, buf breaking dispensado (BUF-08)"
      continue
    fi
    buf breaking "$MOD" --against "$against" || reprovar "buf breaking (FILE) reprovou contra $base"
    echo "breaking: OK ($raiz contra $base; pacotes de $origens)"
  done <<<"$lista"

  # Nenhum pacote publicado some: cada pacote da base existe em HEAD em algum módulo,
  # conferido no que HEAD rastreia, não na árvore de trabalho.
  raizes_head="$(raizes_em HEAD)"
  while IFS=$'\t' read -r rel origem; do
    [ -n "$rel" ] || continue
    presente=0
    while IFS= read -r raiz; do
      [ -n "$raiz" ] || continue
      if git ls-tree --name-only HEAD -- "$raiz/$rel/" | grep -qE '\.proto$'; then presente=1; break; fi
    done <<<"$raizes_head"
    [ "$presente" -eq 1 ] || reprovar "pacote publicado em $base ausente de todos os modulos em HEAD: $rel (antes em $origem) (BUF-08)"
  done <"$indice"
}

# Compila a CLI e o plugin uma vez, antes das tasks paralelas: três `go run` a frio
# simultâneos estouraram o timeout de 30 min do job no gitea-runner (buf a frio custa ~2x o golangci-lint).
gate_warmup() {
  local plugin_pin
  buf --version >/dev/null || reprovar "CLI Buf indisponivel pelo pin de $VERSIONS"
  plugin_pin="$(grep -oE 'protoc-gen-go@v[0-9]+\.[0-9]+\.[0-9]+' "$BUF_GEN" | head -1)"
  [ -n "$plugin_pin" ] || reprovar "$BUF_GEN sem pin exato de protoc-gen-go (BUF-06)"
  go run "google.golang.org/protobuf/cmd/$plugin_pin" --version >/dev/null || reprovar "plugin $plugin_pin indisponivel"
  echo "warmup: OK (buf $(buf_pin), $plugin_pin)"
}

case "$SUB" in
  lint) gate_lint ;;
  pins) gate_pins ;;
  generate-check) gate_generate_check ;;
  breaking) gate_breaking ;;
  warmup) gate_warmup ;;
  *) uso ;;
esac
