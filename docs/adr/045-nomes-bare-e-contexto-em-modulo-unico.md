# ADR-045: Nomear projetos e packages pelo basename do diretório e fundir cada contexto de negócio em um módulo Go

## Status

Aceito — 2026-09-16. Supersede parcialmente o ADR-030 (a nomenclatura dos projetos e o addendum de 2026-09-10); mantém dele um módulo Go por lib do kernel, o BOM declarado e a segregação de stack no caminho. **Parcialmente supersedido pelo ADR-046 (2026-09-16)**: o módulo único por contexto vive em `apps/backend/<contexto>`, como app, e não em `libs/backend/go/<contexto>`; o verificador passa a tooling em `tools/dmpf-conformance`, sob a mesma exceção do plugin. Nome bare, alias pelo papel e um package por bloco permanecem.

## Contexto

Os 17 projetos Go do workspace carregavam três marcas no nome, todas redundantes com a posição na árvore:

- o prefixo de produto `dmpf-` (`dmpf-domain`) e, nas apps de referência, `dmpf-reference-` (`dmpf-reference-bff-go`);
- o prefixo de papel `provider-` (`dmpf-provider-grpc`);
- o sufixo de stack `-go` no nome do projeto Nx e do pacote npm (`dmpf-domain-go`, `@mateusmacedo/dmpf-domain-go`).

O ADR-030 justificou o sufixo pela futura contraparte TypeScript do kernel, que teria os mesmos nomes conceituais, e o addendum de 2026-09-10 fixou o contexto de negócio como cinco módulos irmãos em pasta própria (`bookings/{domain,ports,application,provider,app}`), cada um com `go.mod`, `project.json`, `package.json` e manifesto.

Na prática, o prefixo e o sufixo atrapalhavam mais do que distinguiam: `libs/backend/go/` já declara o scope e a stack; `dmpf` é o nome do repositório e do module path raiz (`github.com/mateusmacedo/dmpf`), não do módulo; e o `@nx-go/nx-go` infere o nome do projeto pelo basename do diretório (`create-nodes-v2.js:32-34`), de modo que o nome explícito divergia do inferido em todo módulo. Os cinco módulos do contexto, por sua vez, exigiam cinco `go.mod` workspace-only com `replace` cruzado no `go.work` para um código que é uma lib só.

Só a tag `dmpf@0.1.0` existe no repositório; nenhum projeto tem tag própria de release, então renomear não quebra continuidade de versionamento.

## Decisão

**Nome é o basename do diretório.** Para todo projeto Nx, pacote npm privado e package Go raiz: `libs/backend/go/grpc` é o projeto `grpc`, o pacote `@mateusmacedo/grpc` e o `package grpc`; `apps/backend/bff` é o projeto `bff` com binário em `cmd/bff`. Nenhum prefixo de produto ou papel, nenhum sufixo de stack. A regra alcança os binários do kernel (`cmd/conformance`, `cmd/bom`, `cmd/modsync`, `cmd/evidence`) e os identificadores lógicos que eram nome de módulo — `bounded_context` `kernel`, `conformance` e `bff`; unidades `kernel/domain`, `conformance/cmd-modsync`.

**Um contexto de negócio é um módulo Go.** `libs/backend/go/<contexto>` tem um `go.mod`, um `project.json`, um `package.json` e um `dmpf-units.json` com uma unidade por bloco; cada bloco é um package (`bookings/domain`, `bookings/ports`, `bookings/application`, `bookings/provider`, `bookings/app`). O verificador de conformidade não exige módulo por bloco — decide sobre o grafo de imports e o manifesto —, e o `depguard` seleciona por diretório (`**/domain/**`, `**/ports/**`, `**/application/**`, `**/contracts/**`), então os gates continuam mordendo dentro do módulo único. A tag `layer:` é a do bloco mais alto do módulo (`layer:apps` para um contexto completo), porque um módulo roda em um só estágio do CI e esse é o primeiro em que todas as suas dependências estão de pé. O `test-race` declara `dependsOn` sobre o do `postgres` do kernel, porque os dois harnesses truncam as mesmas tabelas do mesmo banco de job.

**Colisão de nome resolve-se no import, pelo papel, sem concatenar.** Um package bare colide com libs externas homônimas (`grpc` com `google.golang.org/grpc`, `http` com `net/http`, `sqs` com o SDK da AWS) e o kernel colide com o contexto (`domain`, `application`, `ports`). Quem importa os dois no mesmo arquivo dá alias ao **nosso** import, escolhido pelo papel que o package cumpre ali: o provider de transporte do kernel é `provider` (ou `kernel`, quando `provider` já é identificador local no arquivo); o domínio do kernel visto do contexto é `kernel`; a aplicação do kernel é `usecase`; as portas do kernel são `port`, o nome do bloco na RFC. A lib externa fica com o nome de sempre. Nomes concatenados (`providergrpc`, `kerneldomain`, `googlegrpc`) não são admitidos em package nem em alias.

**O generator gera essa forma.** `bounded-context` emite um módulo em `<directory>/<name>` com os templates de módulo uma vez e `doc.go` por bloco (`package <bloco>`), une as dependências externas dos blocos no manifesto único, deriva a `layer:` do bloco mais alto e registra uma entrada no `go.work`. O harness (`tools/dmpf-harness-check.sh`) e a prova do generator (`tools/dmpf-generator-check.sh`) passam a esperar um `project.json` por contexto.

**Tooling não é lib nem app.** O plugin Nx `tools/dmpf-plugin` (`@mateusmacedo/dmpf-plugin`) mantém o nome: ele não pertence ao kernel nem a um contexto, e o prefixo é o que o distingue de outros plugins que o workspace venha a ter. O mesmo vale para `tools/dmpf-baseline` e os scripts `tools/dmpf-*.sh`.

**O vocabulário da RFC não é nome de projeto.** Ficam intactos o arquivo `dmpf-units.json` (o `metadata_container` que o verificador lê), o schema `dmpf/units@1`, os códigos `DMPF-D002`/`DMPF-B001`, as variáveis `DMPF_*`, os scripts `tools/dmpf-*.sh`, os workflows `dmpf-*.yml` e a release certificada `bom/dmpf/0.1.0.json` com a sua evidência, que registra o estado do commit `8e171d31` e não o estado corrente.

## Alternativas descartadas

- **Manter o sufixo `-go` para a futura contraparte TypeScript.** A contraparte, quando nascer, vive em `libs/<scope>/ts/<módulo>`; o Nx não colide porque o `project.json` fixa o nome, e o `@nx-go/nx-go` nunca infere fora de `go.mod`. Pagar o sufixo hoje por um problema que a árvore já resolve não compensa.
- **Nomear os providers pelo diretório composto (`providergrpc`).** Elimina a colisão com a lib externa, mas reintroduz a concatenação que se quis remover e deixa o package com nome diferente do papel que cumpre em cada arquivo.
- **Dar alias à lib externa (`googlegrpc "google.golang.org/grpc"`).** Espalha um nome inventado por 26 arquivos para preservar o nosso bare; o alias no nosso import, escolhido pelo papel local, lê melhor e mantém a lib externa como toda a comunidade a escreve.
- **Manter `bookings-<bloco>` como namespace do contexto.** É o mesmo prefixo colado de antes, só com outro dono; e a colisão que ele evitava desaparece quando o contexto é um módulo só.
- **Nome de projeto com barra (`bookings/domain`).** Resolveria o Nx, mas não o `package.json` do pnpm, que exige nome npm válido, e manteria cinco módulos para uma lib.

## Consequências

- `go.work` cai de 22 para 18 entradas `use`; os `replace` são regravados pelo `modsync --write`, que passa a ser o dono dos `require` entre irmãos.
- O baseline de unidades foi regravado (`--write-baseline`) porque ids e import paths mudaram; a mudança é normativa e vai em commit próprio (`DMPF-T002`).
- 26 arquivos ganharam alias no import do provider do kernel e 12 no import do kernel visto do `bookings`; nenhum código de produção mudou de comportamento.
- O `.golangci.yml` ganhou `**/contracts/**` no seletor do bloco `contract`; sem ele, o rename teria desligado esse `depguard` em silêncio.
- Um `project.json` de contexto que perdesse o campo `name` faria o `@nx-go/nx-go` inferir `bookings` do basename — a forma correta —, e não mais um nome composto.
- O BOM `bom/dmpf/0.1.0.json` acompanha a árvore pela máquina de estados de `BOM-07`, sem tocar na evidência: os `registry_ref.file` apontam para os paths novos; `google.golang.org/grpc@v1.83.1` passa a `rejeitada` e `v1.83.2` entra como `candidata` (o `go.mod` já estava em `v1.83.2` antes deste ADR); o produto `apps/backend/dmpf-reference`, que o ADR-044 substituiu, passa a `depreciada` com `successor` `apps/backend/bff`, e `bff`, `orders` e `reservations` entram como `candidata`. A evidência de `bom/evidence/0.1.0/` permanece como registro do commit `8e171d31`.
