#!/usr/bin/env bash
# Prova que o depguard (VETORES_*, quatro blocos e o provider de contexto) e o forbidigo (SIMBOLOS, só
# `domain`; VETORES_PLATAFORMA, a regra de plataforma do IDN-14 nos contextos) reprovam o que devem. Módulos vêm do `dmpf-units.json`, não de lista
# fixa: módulo novo entra no gate sozinho. Os fixtures .go são movidos para /tmp
# em vez de apagados — o runner não tem utilitário de lixeira.
set -uo pipefail

ROOT="${DMPF_WORKSPACE_ROOT:-$(git rev-parse --show-toplevel)}" || { echo "fora de um repositorio git" >&2; exit 1; }
cd "$ROOT" || exit 1
APPS="${DMPF_APPS_DIR:-apps/backend}"
DESCARTE="$(mktemp -d)" || { echo "falha ao criar diretorio temporario" >&2; exit 1; }
TRABALHO="$DESCARTE/tarefas"
mkdir "$TRABALHO" || { echo "falha ao criar diretorio temporario" >&2; exit 1; }
VAGAS="${DMPF_GATE_JOBS:-$(nproc 2>/dev/null || echo 2)}"

FIXTURE=""
FIXTURES=()
clausula_de_package() {
  local f
  for f in "$1"/*.go; do
    case "$f" in *_test.go) continue ;; esac
    awk '/^package /{print; exit}' "$f" 2>/dev/null
    return
  done
}

retirar_fixture() {
  local status=$? f
  for f in "$FIXTURE" "${FIXTURES[@]}"; do
    if [[ -n "$f" ]] && [[ -e "$f" ]]; then
      mv -f "$f" "$DESCARTE/descartado-$(date +%s%N)-$(basename "$f")" \
        || echo "AVISO: não consegui retirar $f do módulo" >&2
    fi
  done
  FIXTURE=""
  FIXTURES=()
  return $status
}

# pacote|capability|origem. `dentro` = está na `deny`; `fora` = não está, e prova
# que a regra é fechada (`list-mode: strict`), não uma denylist. Um array por
# bloco porque as `allow` divergem: `time` passa em `domain` e cai em `port`/
# `application`; `log` cai em `domain`/`port` e passa em `application`
# (`capability.go:47`) — o vetor no bloco errado inverteria o resultado.
VETORES_DOMAIN=(
  "net/http|io.network|dentro"
  "os|io.filesystem|dentro"
  "encoding/json|wire.codec|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
  "encoding/xml|wire.codec|fora"
)

VETORES_PORT=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
)

VETORES_APPLICATION=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "syscall|io.*|fora"
)

# `encoding/json` NAO entra como vetor: e wire.codec, a capability que so este
# bloco tem, e portanto esta na `allow`. `log` entra porque observability fica
# de fora de `contract`, como de `domain` e `port`.
VETORES_CONTRACT=(
  "time|io.clock|dentro"
  "net/http|io.network|dentro"
  "log|observability|dentro"
  "syscall|io.*|fora"
)

# Caminhos que `linters.exclusions.rules` tira do depguard, espelhados aqui como
# os VETORES_* espelham as `deny`. Exigir reprovacao neles seria exigir o que a
# configuracao deliberadamente nao faz; usa-los como alvo de fixture faria o
# gate reportar falha onde nao existe.
# pacote|motivo. Provider de contexto (`apps/**/provider`): a regra
# `context-provider` é uma denylist (`list-mode: lax`), então só há vetores
# `dentro`. Todos precisam cair: sem o `pgxpool` negado, `pool.Query` volta a
# existir para o contexto e o isolamento passa a depender de convenção (IDN-14).
VETORES_CONTEXT_PROVIDER=(
  "github.com/jackc/pgx/v5|driver"
  "github.com/jackc/pgx/v5/pgxpool|pool do driver"
  "database/sql|driver generico"
)

FORA_DO_DEPGUARD=(
  "contracts/gen/"
  "contract/gen/"
)

fora_do_depguard() {
  local alvo="$1/"
  local excluido
  for excluido in "${FORA_DO_DEPGUARD[@]}"; do
    case "$alvo" in
      *"$excluido"*) return 0 ;;
    esac
  done
  return 1
}

# Modulo de contrato cujos `include` sao todos codigo gerado: o depguard exclui o
# gerado, entao nao ha onde provar a politica local.
so_codigo_gerado() {
  local inc
  while IFS= read -r inc; do
    fora_do_depguard "$inc" || return 1
  done < <(includes_do_modulo "$1" "$2")
}

# Diretorios dos `include` do manifesto, relativos a raiz do repositorio.
includes_do_modulo() {
  local manifesto="$1" module_dir="$2" module_path rel inc
  module_path="$(awk '/^module /{print $2; exit}' "$module_dir/go.mod" 2>/dev/null)"
  while IFS= read -r inc; do
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    [[ -z "$rel" ]] && continue
    echo "$module_dir/$rel"
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) for (const i of u.include ?? []) console.log(i);
  ' "$ROOT/$manifesto" 2>/dev/null)
}

# import|corpo|familia. O import é permitido pelo depguard; só o símbolo cai.
# Uma família por vetor: remover um padrão do .golangci.yml reprova aqui. Só
# `domain`: o `exclusions.rules` do .golangci.yml restringe o forbidigo a
# `/domain/`.
SIMBOLOS=(
  'time|var _ = time.Now()|io.clock'
  'fmt|func init() { fmt.Println("x") }|io.stdout'
  '|func init() { println("x") }|builtin'
  'fmt|var s string; func init() { _, _ = fmt.Scanln(&s) }|io.stdin'
  'fmt|var _ = fmt.Errorf("x")|erro nao tipado'
  '|func init() { panic("x") }|lancamento'
)

# bloco|import|corpo|rotulo|marca. Regra de plataforma do IDN-14 (ADR-068): o `forbidigo` resolve
# o `pkg` pelo tipo, então o import vai com alias para provar que o alias não escapa. A marca é a
# mensagem da regra no .golangci.yml; reprovar por outro motivo não prova a regra.
KERNEL="github.com/mateusmacedo/dmpf/libs/backend/go"
VETORES_PLATAFORMA=(
  "provider|gatepostgres \"$KERNEL/postgres\"|var _ = gatepostgres.Table[string, struct{}]{}.PlatformReader|leitura de plataforma fora de platform_*.go|IDN-14 plataforma \\(tabela\\)"
  "provider|gatepostgres \"$KERNEL/postgres\"|var _ = gatepostgres.Table[string, struct{}]{}.Projector|projeção fora de platform_*.go|IDN-14 plataforma \\(tabela\\)"
  "provider|gatememory \"$KERNEL/memory\"|var _ = gatememory.Table[string, struct{}]{}.PlatformReader|leitura de plataforma em memória fora de platform_*.go|IDN-14 plataforma \\(tabela\\)"
  "provider|gatememory \"$KERNEL/memory\"|var _ = gatememory.Table[string, struct{}]{}.Projector|projeção em memória fora de platform_*.go|IDN-14 plataforma \\(tabela\\)"
  "application|gateports \"$KERNEL/ports\"|var _ = gateports.WithPlatformReach|alcance montado no application|IDN-14 plataforma \\(alcance\\)"
  "app|gateports \"$KERNEL/ports\"|var _ = gateports.WithPlatformReach|alcance montado em app/|IDN-14 plataforma \\(alcance\\)"
  "rpc|gateports \"$KERNEL/ports\"|var _ = gateports.WithPlatformReach|alcance montado em app/rpc|IDN-14 plataforma \\(alcance\\)"
  "kit|gateports \"$KERNEL/ports\"|var _ = gateports.WithPlatformReach|alcance montado num testkit/ aninhado em app/|IDN-14 plataforma \\(alcance\\)"
  "rpc|gatehttp \"$KERNEL/http\"|var _ = gatehttp.WithVerifiedPlatformReach|alcance verificado montado em app/rpc|IDN-14 plataforma \\(alcance REST\\)"
  "provider|gatehttp \"$KERNEL/http\"|var _ = gatehttp.WithVerifiedPlatformReach|alcance verificado montado no provider|IDN-14 plataforma \\(alcance REST\\)"
)

dir_do_bloco() {
  local manifesto="$1" module_dir="$2" module_path="$3" bloco="$4" sufixo="$5" inc rel
  while IFS= read -r inc; do
    case "$inc" in *"$sufixo") ;; *) continue ;; esac
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    [[ -n "$rel" ]] && { echo "$module_dir/$rel"; return; }
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) if (u.block === process.argv[2]) for (const i of u.include ?? []) console.log(i);
  ' "$ROOT/$manifesto" "$bloco" 2>/dev/null)
}

# O lint é o comando do target `lint` lido do grafo (versão e args do nx.json), sem o custo do
# `nx run` por vetor. O grafo também diz quem importa quem: um fixture vivo num módulo quebra a
# carga de qualquer lint que o alcance, então só correm juntos módulos sem relação de dependência.
if ! pnpm --silent nx graph --file="$TRABALHO/grafo.json" >/dev/null 2>&1; then
  echo "FALHA: nao consegui gerar o grafo do Nx" >&2
  exit 1
fi
declare -A LINT_DIR LINT_EXE LINT_ARGS RELACIONADOS
while IFS=$'\x1f' read -r nome raiz exe args relacionados; do
  LINT_DIR[$nome]="$raiz"
  LINT_EXE[$nome]="$exe"
  LINT_ARGS[$nome]="$args"
  RELACIONADOS[$nome]=" $relacionados "
done < <(node -e '
  const g = require(process.argv[1]).graph;
  const nomes = Object.keys(g.nodes);
  const diretas = {};
  const reversas = {};
  for (const n of nomes) {
    diretas[n] = (g.dependencies[n] ?? []).map((d) => d.target).filter((t) => g.nodes[t]);
    for (const t of diretas[n]) (reversas[t] ??= []).push(n);
  }
  const alcance = (n, arestas) => {
    const vistos = new Set();
    const pilha = [n];
    while (pilha.length) for (const b of arestas[pilha.pop()] ?? []) if (!vistos.has(b)) vistos.add(b) && pilha.push(b);
    return vistos;
  };
  for (const n of nomes) {
    const raiz = g.nodes[n].data.root;
    const rel = new Set([...alcance(n, diretas), ...alcance(n, reversas)]);
    for (const m of nomes) {
      const r = g.nodes[m].data.root;
      if (r.startsWith(raiz + "/") || raiz.startsWith(r + "/")) rel.add(m);
    }
    rel.delete(n);
    const lint = g.nodes[n].data.targets?.lint;
    let exe = "";
    let args = [];
    if (lint?.executor === "@nx-go/nx-go:lint") {
      exe = lint.options?.linter ?? "go";
      args = [...(lint.options?.linter == null ? ["fmt"] : []), ...(lint.options?.args ?? []), "./..."];
    }
    console.log([n, raiz, exe, args.join("\x1e"), [...rel].join(" ")].join("\x1f"));
  }
' "$TRABALHO/grafo.json")

lintar() {
  local projeto="$1" args
  if [[ -z "${LINT_EXE[$projeto]:-}" ]]; then
    pnpm nx run "$projeto":lint --skip-nx-cache
    return
  fi
  IFS=$'\x1e' read -r -a args <<<"${LINT_ARGS[$projeto]}"
  (cd "$ROOT/${LINT_DIR[$projeto]}" && "${LINT_EXE[$projeto]}" "${args[@]}")
}

total_falhas=0
modulos=0
modulos_domain=0
modulos_port=0
modulos_application=0
modulos_contract=0
fora_de_alcance=0
providers_de_contexto=0
contextos_plataforma=0

SEQ=0
TAREFAS=()
declare -A TAREFA_PROJETO TAREFA_FUNCAO TAREFA_CONTEXTO PID_DE

anotar() {
  printf '%s\n' "$1" > "$TRABALHO/$SEQ.saida"
  printf '%s\n' "${2:-0}" > "$TRABALHO/$SEQ.fim"
  SEQ=$((SEQ + 1))
}

registrar() {
  local funcao="$1" projeto="$2"
  shift 2
  TAREFA_FUNCAO[$SEQ]="$funcao"
  TAREFA_PROJETO[$SEQ]="$projeto"
  TAREFA_CONTEXTO[$SEQ]="$(declare -p "$@")"
  TAREFAS+=("$SEQ")
  SEQ=$((SEQ + 1))
}

provar_bloco() {
  local vetor pkg resto cap origem simbolo imp corpo familia linha inc inc_bloco rel sub_dir_candidato sub_dir sub_clause esperado rotulo saida status module_path
  if [[ "$fixture_dir" = "$module_dir" ]]; then
    echo "== $project ($module_dir) [bloco $bloco]"
  else
    echo "== $project ($module_dir) [bloco $bloco, fixture em ${fixture_dir#"$module_dir/"}]"
  fi

  for vetor in "${vetores[@]}"; do
    pkg="${vetor%%|*}"
    resto="${vetor#*|}"
    cap="${resto%%|*}"
    origem="${resto##*|}"

    FIXTURE="$(mktemp "$fixture_dir/zz_gate_XXXXXX.go")" || {
      echo "FALHA  $project: nao consegui criar o fixture"
      falhas=$((falhas + 1))
      break
    }
    printf '%s\n\nimport _ "%s"\n' "$pkg_clause" "$pkg" > "$FIXTURE"

    saida="$(lintar "$project" 2>&1)"
    status=$?
    retirar_fixture

    if [[ "$status" -eq 0 ]]; then
      echo "  FALHA  $pkg ($cap, $origem): o lint passou, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -qi "depguard" <<<"$saida"; then
      echo "  FALHA  $pkg ($cap, $origem): reprovou por outro motivo que nao o depguard"
      falhas=$((falhas + 1))
    else
      echo "  ok     $pkg ($cap, $origem)"
    fi
  done

  if [[ "$bloco" = "domain" ]]; then
    for simbolo in "${SIMBOLOS[@]}"; do
      imp="${simbolo%%|*}"
      resto="${simbolo#*|}"
      corpo="${resto%%|*}"
      familia="${resto##*|}"

      FIXTURE="$(mktemp "$fixture_dir/zz_gate_XXXXXX.go")" || {
        echo "FALHA  $project: nao consegui criar o fixture"
        falhas=$((falhas + 1))
        break
      }
      if [[ -n "$imp" ]]; then
        printf '%s\n\nimport "%s"\n\n%s\n' "$pkg_clause" "$imp" "$corpo" > "$FIXTURE"
      else
        printf '%s\n\n%s\n' "$pkg_clause" "$corpo" > "$FIXTURE"
      fi

      saida="$(lintar "$project" 2>&1)"
      status=$?
      retirar_fixture

      if [[ "$status" -eq 0 ]]; then
        echo "  FALHA  $corpo ($familia): o lint passou, mas deveria reprovar"
        falhas=$((falhas + 1))
      elif ! grep -qi "forbidigo" <<<"$saida"; then
        echo "  FALHA  $corpo ($familia): reprovou por outro motivo que nao o forbidigo"
        falhas=$((falhas + 1))
      else
        echo "  ok     $corpo ($familia, forbidigo)"
      fi
    done
  fi

  # Um fixture por `include` fora da raiz prova que o glob chega ao subpackage.
  # Em `domain` o fixture e de simbolo (forbidigo, restrito a `/domain/`); nos
  # demais, de package. Subpackage de bloco sem politica local (`provider`,
  # `app`) e declarado fora do gate: exigir reprovacao ali seria exigir o que
  # `linters.exclusions.rules` deliberadamente nao faz.
  module_path="$(awk '/^module /{print $2; exit}' "$module_dir/go.mod" 2>/dev/null)"
  while IFS= read -r linha; do
    inc="${linha%%|*}"
    inc_bloco="${linha##*|}"
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    [[ -z "$rel" ]] && continue
    sub_dir_candidato="$module_dir/$rel"

    case "$inc_bloco" in
      domain|port|application|contract) ;;
      *)
        echo "  --     $rel: unidade $inc_bloco, sem politica local; coberta pelo verificador do KRN-02"
        continue
        ;;
    esac

    if fora_do_depguard "$sub_dir_candidato"; then
      echo "  --     $rel: excluida do depguard em exclusions.rules; coberta pelo verificador do KRN-02"
      continue
    fi

    sub_dir="$module_dir/$rel"
    sub_clause="$(clausula_de_package "$sub_dir")"
    if [[ -z "$sub_clause" ]]; then
      echo "  FALHA  $rel: nenhum .go com clausula de package no include"
      falhas=$((falhas + 1))
      continue
    fi

    FIXTURE="$(mktemp "$sub_dir/zz_gate_XXXXXX.go")" || {
      echo "FALHA  $project: nao consegui criar o fixture em $rel"
      falhas=$((falhas + 1))
      break
    }
    if [[ "$bloco" = "domain" ]]; then
      printf '%s\n\nimport "time"\n\nvar _ = time.Now()\n' "$sub_clause" > "$FIXTURE"
      esperado="forbidigo"
      rotulo="time.Now() (io.clock, forbidigo, subpackage)"
    else
      printf '%s\n\nimport _ "time"\n' "$sub_clause" > "$FIXTURE"
      esperado="depguard"
      rotulo="import time (io.clock, depguard, subpackage)"
    fi

    saida="$(lintar "$project" 2>&1)"
    status=$?
    retirar_fixture

    if [[ "$status" -eq 0 ]]; then
      echo "  FALHA  $rel: o fixture passou no subpackage, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -qi "$esperado" <<<"$saida"; then
      echo "  FALHA  $rel: reprovou por outro motivo que nao o $esperado"
      falhas=$((falhas + 1))
    else
      echo "  ok     $rel: $rotulo"
    fi
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) for (const i of u.include ?? []) console.log(i + "|" + u.block);
  ' "$ROOT/$manifesto" 2>/dev/null)

  # Vetor positivo: a árvore limpa precisa passar, senão o gate só sabe dizer não.
  if lintar "$project" >/dev/null 2>&1; then
    echo "  ok     arvore limpa: aprovada"
  else
    echo "  FALHA  arvore limpa: reprovada, mas deveria passar"
    falhas=$((falhas + 1))
  fi
}

provar_contexto() {
  local entrada sub_dir sub_clause rel vetor pkg motivo saida status bloco imp corpo rotulo marca alvo clausula arquivo positivo molde f i
  local -a positivos rotulos
  for entrada in "${provedores[@]}"; do
    IFS='|' read -r sub_dir sub_clause rel <<<"$entrada"
    echo "== $project ($sub_dir) [provider de contexto]"

    for vetor in "${VETORES_CONTEXT_PROVIDER[@]}"; do
      pkg="${vetor%%|*}"
      motivo="${vetor#*|}"
      FIXTURE="$(mktemp "$sub_dir/zz_gate_XXXXXX.go")" || {
        echo "FALHA  $project: nao consegui criar o fixture em $rel"
        falhas=$((falhas + 1))
        break
      }
      printf '%s\n\nimport _ "%s"\n' "$sub_clause" "$pkg" > "$FIXTURE"

      saida="$(lintar "$project" 2>&1)"
      status=$?
      retirar_fixture

      if [[ "$status" -eq 0 ]]; then
        echo "  FALHA  $pkg ($motivo): o lint passou, mas deveria reprovar"
        falhas=$((falhas + 1))
      elif ! grep -q "context-provider" <<<"$saida"; then
        echo "  FALHA  $pkg ($motivo): reprovou por outro motivo que nao a regra context-provider"
        falhas=$((falhas + 1))
      else
        echo "  ok     $pkg ($motivo)"
      fi
    done

    if lintar "$project" >/dev/null 2>&1; then
      echo "  ok     arvore limpa: aprovada"
    else
      echo "  FALHA  arvore limpa: reprovada, mas deveria passar"
      falhas=$((falhas + 1))
    fi
  done

  [[ "$plataforma" = true ]] || return 0
  echo "== $project [plataforma IDN-14]"

  for vetor in "${VETORES_PLATAFORMA[@]}"; do
    IFS='|' read -r bloco imp corpo rotulo marca <<<"$vetor"
    case "$bloco" in
      provider) alvo="$provider_dir" clausula="$provider_clause" ;;
      app) alvo="$app_dir" clausula="$app_clause" ;;
      rpc) alvo="$rpc_dir" clausula="$rpc_clause" ;;
      kit) alvo="$app_dir" clausula="package testkit" ;;
      *) alvo="$application_dir" clausula="$application_clause" ;;
    esac
    if [[ "$bloco" = kit ]]; then
      FIXTURE="$(mktemp -d "$alvo/zz_gate_XXXXXX")" && mkdir "$FIXTURE/testkit" && arquivo="$FIXTURE/testkit/zz_gate.go"
    else
      FIXTURE="$(mktemp "$alvo/zz_gate_XXXXXX.go")" && arquivo="$FIXTURE"
    fi || {
      echo "FALHA  $project: não consegui criar o fixture em $alvo"
      falhas=$((falhas + 1))
      break
    }
    printf '%s\n\nimport %s\n\n%s\n' "$clausula" "$imp" "$corpo" > "$arquivo"

    saida="$(lintar "$project" 2>&1)"
    status=$?
    retirar_fixture

    if [[ "$status" -eq 0 ]]; then
      echo "  FALHA  $rotulo: o lint passou, mas deveria reprovar"
      falhas=$((falhas + 1))
    elif ! grep -q "(forbidigo)" <<<"$saida" || ! grep -qE "$marca" <<<"$saida"; then
      echo "  FALHA  $rotulo: reprovou por outro motivo que não a regra de plataforma"
      falhas=$((falhas + 1))
    else
      echo "  ok     $rotulo"
    fi
  done

  positivos=(
    "$provider_dir/platform_gate_XXXXXX.go|$provider_clause|uso permitido em platform_*.go|import gatepostgres \"$KERNEL/postgres\"|var _ = gatepostgres.Table[string, struct{}]{}.PlatformReader
var _ = gatepostgres.Table[string, struct{}]{}.Projector"
    "$provider_dir/zz_gate_XXXXXX.go|$provider_clause|símbolo de domain fora de domain|import \"time\"|var _ = time.Now()"
    "$app_dir/zz_gate_XXXXXX_test.go|$app_clause|alcance montado em _test.go|import gateports \"$KERNEL/ports\"|var _ = gateports.WithPlatformReach"
  )
  rotulos=()
  for positivo in "${positivos[@]}"; do
    IFS='|' read -r -d '' molde clausula rotulo imp corpo <<<"$positivo"
    corpo="${corpo%$'\n'}"
    f="$(mktemp "$molde")" || { echo "FALHA  $project: não consegui criar o fixture $molde"; falhas=$((falhas + 1)); continue; }
    FIXTURES+=("$f")
    rotulos+=("$rotulo")
    printf '%s\n\n%s\n\n%s\n' "$clausula" "$imp" "$corpo" > "$f"
  done
  if [[ "${#FIXTURES[@]}" -eq "${#positivos[@]}" ]]; then
    saida="$(lintar "$project" 2>&1)"
    status=$?
    for i in "${!FIXTURES[@]}"; do
      if [[ "$status" -ne 0 ]] && grep -qF "$(basename "${FIXTURES[$i]}")" <<<"$saida"; then
        echo "  FALHA  ${rotulos[$i]}: reprovou, mas deveria passar"
        falhas=$((falhas + 1))
      else
        echo "  ok     ${rotulos[$i]}"
      fi
    done
    if [[ "$status" -ne 0 ]] && ! grep -qF "zz_gate_" <<<"$saida" && ! grep -qF "platform_gate_" <<<"$saida"; then
      echo "  FALHA  positivos de plataforma: o lint reprovou sem nomear um fixture"
      falhas=$((falhas + 1))
    fi
  fi
  retirar_fixture
}

# Espelha o alcance do depguard, que seleciona por diretorio (`**/domain/**`,
# `**/ports/**`, `**/application/**`, `**/contracts/**`, `**/contract/**` e as formas com hifen).
# Modulo fora desses caminhos nao e coberto pelo depguard — o verificador do
# KRN-02 o cobre — e provar o gate nele seria provar o que nao existe. O array
# vem do CAMINHO, nao do manifesto: e o caminho que decide a regra aplicada.
bloco_do_caminho() {
  case "/$1/" in
    *-domain/* | */domain/*)           echo "domain" ;;
    *-ports/* | */ports/*)             echo "port" ;;
    *-application/* | */application/*) echo "application" ;;
    *-contracts/* | */contracts/* | */contract/*) echo "contract" ;;
    *)               echo "" ;;
  esac
}

while IFS= read -r manifesto; do
  module_dir="$(dirname "$manifesto")"

  # Exclusoes fechadas de RFC 10.3. Modulo sintetico sob testdata/ existe para
  # o verificador reprovar; trata-lo como producao faria o gate falhar pela
  # violacao que a fixture demonstra.
  case "/$module_dir/" in
    */testdata/*|*/vendor/*) continue ;;
  esac

  # só os blocos que o .golangci.yml conhece; um modulo que declare apenas
  # `provider`, `contract` ou `app` nao tem politica local a provar
  node -e '
    const m = require(process.argv[1]);
    const alvos = new Set(["domain", "port", "application", "contract"]);
    process.exit((m.units ?? []).some((u) => alvos.has(u.block)) ? 0 : 1);
  ' "$ROOT/$manifesto" 2>/dev/null || continue

  bloco="$(bloco_do_caminho "$module_dir")"
  if [[ -z "$bloco" ]]; then
    anotar "-- $module_dir: fora do alcance do depguard (**/domain/**, **/ports/**, **/application/**, **/contracts/**); coberto pelo verificador do KRN-02"
    fora_de_alcance=$((fora_de_alcance + 1))
    continue
  fi

  case "$bloco" in
    domain)      vetores=("${VETORES_DOMAIN[@]}") ;;
    port)        vetores=("${VETORES_PORT[@]}") ;;
    application) vetores=("${VETORES_APPLICATION[@]}") ;;
    contract)    vetores=("${VETORES_CONTRACT[@]}") ;;
  esac

  project="$(node -p 'require(process.argv[1]).name' "$ROOT/$module_dir/project.json" 2>/dev/null)"
  if [[ -z "$project" ]]; then
    anotar "FALHA  $module_dir: nao consegui ler o nome do projeto no project.json" 1
    continue
  fi

  # O fixture de package precisa de um diretorio com .go, e a clausula vem do
  # proprio codigo: fixa-la aqui quebraria o fixture em modulo com outro nome de
  # package. A raiz e a primeira escolha; quando ela nao tem codigo, vale o
  # primeiro `include` que tenha — o contracts declara tres unidades e
  # nenhum .go na raiz.
  fixture_dir=""
  pkg_clause=""
  for candidato in "$module_dir" $(includes_do_modulo "$manifesto" "$module_dir"); do
    fora_do_depguard "$candidato" && continue
    clausula="$(clausula_de_package "$candidato")"
    if [[ -n "$clausula" ]]; then
      fixture_dir="$candidato"
      pkg_clause="$clausula"
      break
    fi
  done
  if [[ -z "$pkg_clause" ]] && so_codigo_gerado "$manifesto" "$module_dir"; then
    anotar "-- $module_dir: so codigo gerado, fora do depguard por exclusao; coberto pelo verificador do KRN-02"
    fora_de_alcance=$((fora_de_alcance + 1))
    continue
  fi
  if [[ -z "$pkg_clause" ]]; then
    anotar "FALHA  $module_dir: nenhum .go com clausula de package dentro do alcance do depguard" 1
    continue
  fi

  modulos=$((modulos + 1))
  case "$bloco" in
    domain)      modulos_domain=$((modulos_domain + 1)) ;;
    port)        modulos_port=$((modulos_port + 1)) ;;
    application) modulos_application=$((modulos_application + 1)) ;;
    contract)    modulos_contract=$((modulos_contract + 1)) ;;
  esac
  registrar provar_bloco "$project" project module_dir manifesto bloco vetores fixture_dir pkg_clause
done < <(git ls-files '*dmpf-units.json')

# O laço acima pula os módulos de contexto (o caminho do módulo não é de bloco),
# então o provider de cada app é exercitado aqui, pelo include do manifesto.
while IFS= read -r manifesto; do
  module_dir="$(dirname "$manifesto")"
  case "$module_dir/" in
    "$APPS"/*) ;;
    *) continue ;;
  esac
  module_path="$(awk '/^module /{print $2; exit}' "$module_dir/go.mod" 2>/dev/null)"
  project="$(node -p 'require(process.argv[1]).name' "$ROOT/$module_dir/project.json" 2>/dev/null)"

  provedores=()
  while IFS= read -r inc; do
    rel="${inc#"$module_path"}"
    rel="${rel#/}"
    case "/$rel/" in
      */provider/) ;;
      *) continue ;;
    esac
    sub_dir="$module_dir/$rel"
    sub_clause="$(clausula_de_package "$sub_dir")"
    if [[ -z "$project" ]] || [[ -z "$sub_clause" ]]; then
      anotar "FALHA  $sub_dir: sem projeto Nx ou sem .go com clausula de package" 1
      continue
    fi
    providers_de_contexto=$((providers_de_contexto + 1))
    provedores+=("$sub_dir|$sub_clause|$rel")
  done < <(node -e '
    const m = require(process.argv[1]);
    for (const u of m.units ?? []) if (u.block === "provider") for (const i of u.include ?? []) console.log(i);
  ' "$ROOT/$manifesto" 2>/dev/null)

  plataforma=false
  provider_dir="$(dir_do_bloco "$manifesto" "$module_dir" "$module_path" provider /provider)"
  application_dir="$(dir_do_bloco "$manifesto" "$module_dir" "$module_path" application /application)"
  app_dir="$(dir_do_bloco "$manifesto" "$module_dir" "$module_path" app /app)"
  rpc_dir="$(dir_do_bloco "$manifesto" "$module_dir" "$module_path" app /app/rpc)"
  provider_clause=""
  application_clause=""
  app_clause=""
  rpc_clause=""
  if [[ -n "$provider_dir" ]]; then
    # A cláusula sai antes de criar fixture: um fixture vazio pode ser o primeiro .go do diretório.
    provider_clause="$(clausula_de_package "$provider_dir")"
    application_clause="$(clausula_de_package "$application_dir")"
    app_clause="$(clausula_de_package "$app_dir")"
    rpc_clause="$(clausula_de_package "$rpc_dir")"
    if [[ -z "$project" ]] || [[ -z "$provider_clause" ]] || [[ -z "$application_clause" ]] || [[ -z "$app_clause" ]] || [[ -z "$rpc_clause" ]]; then
      anotar "FALHA  $module_dir: sem projeto Nx, application, app ou app/rpc para provar a regra de plataforma" 1
    else
      contextos_plataforma=$((contextos_plataforma + 1))
      plataforma=true
    fi
  fi

  [[ "${#provedores[@]}" -gt 0 ]] || [[ "$plataforma" = true ]] || continue
  registrar provar_contexto "$project" project provedores plataforma provider_dir application_dir app_dir rpc_dir \
    provider_clause application_clause app_clause rpc_clause
done < <(git ls-files '*dmpf-units.json')

lancar() {
  local id="$1"
  (
    falhas=0
    trap 'retirar_fixture; printf "%s\n" "$falhas" > "$TRABALHO/$id.fim.tmp" && mv -f "$TRABALHO/$id.fim.tmp" "$TRABALHO/$id.fim"' EXIT
    trap 'exit 143' INT TERM
    eval "${TAREFA_CONTEXTO[$id]}"
    "${TAREFA_FUNCAO[$id]}"
  ) > "$TRABALHO/$id.saida" 2>&1 &
  PID_DE[$id]=$!
}

CORRENDO=()
conflita() {
  local projeto="$1" id
  for id in "${CORRENDO[@]}"; do
    [[ -n "${RELACIONADOS[$projeto]+x}" ]] || return 0
    [[ "${TAREFA_PROJETO[$id]}" = "$projeto" ]] && return 0
    [[ "${RELACIONADOS[$projeto]}" == *" ${TAREFA_PROJETO[$id]} "* ]] && return 0
  done
  return 1
}

atualizar_correndo() {
  local id vivos=()
  for id in "${CORRENDO[@]}"; do
    [[ -e "$TRABALHO/$id.fim" ]] && continue
    if kill -0 "${PID_DE[$id]}" 2>/dev/null; then
      vivos+=("$id")
      continue
    fi
    echo "  FALHA  ${TAREFA_PROJETO[$id]}: a prova terminou sem relatar o resultado" >> "$TRABALHO/$id.saida"
    printf '1\n' > "$TRABALHO/$id.fim"
  done
  CORRENDO=("${vivos[@]}")
}

IMPRESSO=0
imprimir_prontos() {
  while [[ "$IMPRESSO" -lt "$SEQ" ]] && [[ -e "$TRABALHO/$IMPRESSO.fim" ]]; do
    cat "$TRABALHO/$IMPRESSO.saida"
    total_falhas=$((total_falhas + $(cat "$TRABALHO/$IMPRESSO.fim")))
    IMPRESSO=$((IMPRESSO + 1))
  done
}

encerrar() {
  local pids
  pids="$(jobs -pr)"
  if [[ -n "$pids" ]]; then
    # shellcheck disable=SC2086
    kill -TERM $pids 2>/dev/null
    wait
  fi
}
trap encerrar EXIT
trap 'exit 130' INT TERM

PENDENTES=("${TAREFAS[@]}")
while [[ "${#PENDENTES[@]}" -gt 0 ]] || [[ "${#CORRENDO[@]}" -gt 0 ]]; do
  restantes=()
  for id in "${PENDENTES[@]}"; do
    if [[ "${#CORRENDO[@]}" -lt "$VAGAS" ]] && ! conflita "${TAREFA_PROJETO[$id]}"; then
      lancar "$id"
      CORRENDO+=("$id")
    else
      restantes+=("$id")
    fi
  done
  PENDENTES=("${restantes[@]}")
  imprimir_prontos
  [[ "${#CORRENDO[@]}" -gt 0 ]] && wait -n
  atualizar_correndo
done
imprimir_prontos

if [[ "$providers_de_contexto" -eq 0 ]]; then
  echo "FALHA: nenhum provider de contexto encontrado; a regra context-provider nao foi exercitada." >&2
  exit 1
fi

if [[ "$contextos_plataforma" -eq 0 ]]; then
  echo "FALHA: nenhum contexto com provider, application e app encontrado; a regra de plataforma (IDN-14) não foi exercitada." >&2
  exit 1
fi

if [[ "$modulos" -eq 0 ]]; then
  echo "FALHA: nenhum modulo com bloco domain, port, application ou contract encontrado; o gate nao exercitou nada." >&2
  exit 1
fi

if [[ "$total_falhas" -gt 0 ]]; then
  echo
  echo "$total_falhas verificacao(oes) falharam: o gate nao esta protegendo os blocos domain, port, application e contract."
  exit 1
fi

echo
echo "Gate de dependencia: $modulos modulo(s) — domain: $modulos_domain, port: $modulos_port, application: $modulos_application, contract: $modulos_contract."
echo "Vetores de package por bloco: domain ${#VETORES_DOMAIN[@]}, port ${#VETORES_PORT[@]}, application ${#VETORES_APPLICATION[@]}, contract ${#VETORES_CONTRACT[@]}; ${#SIMBOLOS[@]} de símbolo em domain; 1 positivo por módulo; provider de contexto ${#VETORES_CONTEXT_PROVIDER[@]} em $providers_de_contexto package(s); plataforma (IDN-14) ${#VETORES_PLATAFORMA[@]} negativo(s) e 3 positivo(s) em $contextos_plataforma contexto(s). Todos conformes."
if [[ "$fora_de_alcance" -gt 0 ]]; then
  # Declarado, nunca silencioso: um gate que esconde o proprio alcance passa a
  # informar cobertura que nao tem.
  echo "$fora_de_alcance modulo(s) fora do alcance do depguard, cobertos pelo verificador do KRN-02."
fi
