# ADR-060: O DMPF tem consumidor externo

## Status

Aceito — 2026-10-07. Implementa SPEC-P846PH1P. Supersede em parte o [ADR-059](./059-remover-o-bom-e-a-release-do-produto.md) e o [ADR-046](./046-libs-somente-kernel-de-reuso.md), nos pontos listados na Decisão.

## Contexto

O ADR-059 removeu o BOM e a release do produto porque não havia consumidor externo. Essa premissa venceu: os projetos criados a partir do repositório `nx-base-template` usam o DMPF fora deste monorepo e são esse consumidor.

O spike de 06/10/2026, com o kernel em `v1.0.0-rc.1`, mostrou o que já funciona por versão e o que não funciona:

- `go get` e build das libs do kernel e `go run` de `dmpf-conformance/cmd/{conformance,modsync,infrasync}@v1.0.0-rc.1` funcionam fora do platform.
- O `conformance` reprova quem recebe o kernel por tag: `DMPF-E001` com o kernel como dependência externa, `DMPF-E002` quando declarado `pure`. O inventário só enxerga unidades locais, a `DMPF-D002` exige `shared_kernel_units` designado à mão e os perfis de build ficam num caminho fixo do platform.
- O `@mateusmacedo/dmpf-plugin` é `private: true`, e o generator `bounded-context` fixa o prefixo de módulo, o escopo npm, o diretório `apps/backend`, o `go run ./tools/...`, targets que dependem de `bff` e `testkit`, um Dockerfile que copia `libs/` e `tools/` e versões literais de ferramentas.
- Scripts de gate, infra, ativos de IA e CI só existem dentro do platform, e nenhum job prova que um repositório externo consome o DMPF.

## Decisão

O DMPF passa a ter consumidor externo, e tudo o que o consumidor usa chega **por versão**: o kernel e o `dmpf-conformance` por tag no proxy Go, o plugin Nx por pacote no GitHub Packages. O consumidor não tem `replace` para módulos do kernel, caminho relativo ao platform nem arquivo copiado à mão.

**Âncora única de versão no plugin.** O `@mateusmacedo/dmpf-plugin` carrega um `versions.json` com a tag do kernel, a versão do `dmpf-conformance`, a versão Go (diretiva e imagem), `buf`, `protoc-gen-go`, `golangci-lint`, `govulncheck` e o `workflowRef`. Generators, executors e migrations leem desse arquivo, e nenhum template fixa versão. Uma versão do plugin fixa todas as outras, e o consumidor não escolhe versões avulsas. O arquivo é versionado, preenchido no fluxo de release com a versão-alvo explícita e o SHA do commit-fonte, e entra no commit de versão do `nx release`.

**Layout em `dmpf.json`.** A raiz do workspace declara o diretório de apps, a borda pública, o trust domain SPIFFE, o registry de imagens, o módulo Buf, o escopo npm, o prefixo de módulo e o `tooling.mode`. O platform usa `local`, com as ferramentas por `go run ./tools/...`, para testar mudança sem tag; o consumidor usa `version`, com `go run <módulo>@<versão>`.

**Conformance em modo consumidor.** As unidades de módulos fora do workspace que publicam `dmpf-units.json` entram no universo como shared kernel de leitura, lidas dos pacotes do `go list -deps -json` (`Module.Dir`, `Module.Version`, `Module.Main`): servem de destino de aresta, não são verificadas e satisfazem a `DMPF-D002` sem `shared_kernel_units`. A designação manual continua valendo para o kernel local. Manifesto inválido ou módulo ausente do cache sai com exit 2 e `NAO VERIFICADO`, nunca com exit 0. Os perfis de build vão embutidos por `go:embed`; o `modsync` aceita tag de pré-release, com a regra "maior estável; pré-release só sem estável", e o `infrasync` lê o layout do `dmpf.json`.

**Publicação no GitHub Packages.** O plugin perde o `private` e o release group `npm` publica em `https://npm.pkg.github.com`, no escopo `@mateusmacedo`. A publicação é encadeada no `nx-release.yml` por `needs`, porque a tag empurrada com o `GITHUB_TOKEN` não dispara outro workflow. O dist-tag deriva da versão: pré-release vai para `next`, estável para `latest`. O token entra só por variável de ambiente ou secret. Os módulos Go e o `dmpf-conformance` continuam com `package.json` privado: chegam pelo proxy Go, não pelo registry npm.

**Generators, executors e migrations.** O `init` é idempotente e escreve `dmpf.json`, `go.work`, `nx.json`, `.golangci.yml`, o esqueleto estático da infra, o chamador de CI e os ativos de IA. O `modulePrefix` é opcional, com prompt e o default reservado `example.com/change-me`, que o `bounded-context` recusa; uma segunda execução re-renderiza sem `--force` os arquivos que o próprio `init` gerou e ninguém editou. Os arquivos de infra gerados continuam pertencendo ao `infrasync`. O `bounded-context` lê `dmpf.json` e `versions.json`, sem prefixo, escopo ou diretório fixos e sem depender de projetos do platform. Os scripts `dmpf-context-check`, `dmpf-gate-check`, `buf-gate`, `buf`, `go-tidy`, `test-env` e `test-infra` viram executors do plugin; o platform usa os mesmos executors, e a cópia em `tools/` sai. As migrations editam a `Tree` e devolvem os comandos externos em `nextSteps`, porque uma migration não roda comando depois do flush.

**Ativos de IA pelo plugin.** As skills `dmpf-bounded-context` e `dmpf-testkit`, o agente `dmpf-context-author`, a rule `dmpf-bounded-context` e o comando `dmpf-new-context` têm fonte canônica no pacote. O `init` e as migrations escrevem em `.claude/` e `.agents/` do consumidor; o platform recebe pelo mesmo generator, e o CI reprova divergência.

**Workflow reutilizável por SHA.** O `dmpf-go-ci.yml` (`workflow_call`) roda a pirâmide Go por `layer:*`, os gates Buf, `modsync --check`, `infrasync --check`, `conformance` e o context-check. O `ci.yml` do platform o chama, e o chamador do consumidor fica fixado no `workflowRef` de `versions.json`. As actions internas vêm do commit do próprio workflow: cada job faz checkout de `job.workflow_repository` em `job.workflow_sha` e as usa por caminho local, porque um workflow reutilizável chamado de outro repositório resolve `uses: ./...` no repositório do chamador, e um `@<sha>` literal não consegue apontar para o commit que o contém.

**Prova de consumo por CI.** Em todo PR, o job `external-consumer` cria um workspace Nx vazio fora da árvore, instala o plugin por `pnpm pack` e recebe o kernel e o `dmpf-conformance` do PR por um proxy Go `file://`, em versão efêmera derivada do SHA, o que exercita o caminho por tag. Depois de cada release, o mesmo cenário roda contra o proxy público e o GitHub Packages e executa os `nextSteps` das migrations; a falha deixa o run vermelho, sem rollback automático. O teste de upgrade de uma versão X para Y começa na release seguinte à primeira publicada.

**Fica fora.** BOM e certificação de produto: nesse ponto o ADR-059 continua valendo. A prova de consumo é um job de CI, não um artefato de release, e não volta a tag do produto `dmpf@<semver>`.

Pontos supersedidos:

- **ADR-059:** a premissa de que não há consumidor externo, e com ela a negativa que condicionava a volta da certificação ao aparecimento de um consumidor. O consumidor apareceu e a certificação não volta; ficam a remoção do BOM, da evidência de release e da tag do produto.
- **ADR-046:** o plugin deixa de ser pacote npm privado. O `@mateusmacedo/dmpf-conformance` continua privado.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| --- | --- |
| Versões avulsas escolhidas pelo consumidor | Os templates do generator importam a API do kernel; combinação não testada quebra em silêncio quando essa API muda |
| npmjs.com com trusted publishing | O mantenedor decidiu manter código, publicação e registry no mesmo host |
| Marketplace de plugin Claude Code para os ativos de IA | Seria um segundo canal de versão, e a rule viraria skill, perdendo o carregamento por `paths:` |
| Kernel como submódulo no `go.work` do consumidor | Passa no `conformance` com o baseline ajustado à mão, mas reintroduz a cópia local |
| Reescrever os scripts em Go agora | Custo alto para esta entrega; empacotar como executors já deixa uma cópia só |
| Copiar scripts, infra e CI no template | Divergência garantida a cada mudança no platform |
| Overlay de `GOWORK` com o kernel do PR | Põe o kernel no workspace e não exercita a leitura por tag |
| Publicação disparada pelo push da tag | A tag empurrada com o `GITHUB_TOKEN` não dispara outro workflow |
| Workflow reutilizável referenciado por tag (`tools/...` ou `@scope/pkg@x`) | O suporte a `@` e `/` no ref não foi confirmado; o SHA é imutável |
| Platform também em modo `version` | Exigiria tag para testar qualquer mudança local nas ferramentas |
| Target `copy-assets` no build do plugin | Redeclara o que a inferência fornece (ADR-002); templates, scripts e `versions.json` ficam na raiz do pacote |
| `include` remoto da infra no compose | Os bind mounts relativos exigem os arquivos em disco |
| Comandos executados no corpo da migration | Rodariam antes do flush da `Tree` |

## Consequências

**Positivas:**

- Um projeto novo passa de copiar e adaptar o platform a `nx add`, `init` e `bounded-context`, com atualização por `nx migrate`.
- Scripts, infra, ativos de IA e `.golangci.yml` têm uma fonte só; o platform consome o mesmo que o consumidor, e o CI reprova divergência.
- O consumo por versão ganha gate: o job `external-consumer` em todo PR e a prova pós-release a cada release.
- O `conformance` aceita o kernel por tag sem edição manual do baseline.

**Negativas:**

- Todo consumidor precisa de token com `read:packages`, mesmo para pacote público, e cada repositório que roda Actions precisa de acesso de leitura ao pacote ou de um PAT.
- O lockstep impede subir o kernel sem subir o plugin.
- O platform roda as ferramentas em modo `local` e o consumidor em `version`; diferença entre os dois modos só aparece nos jobs de consumo.
- O upgrade de X para Y só tem cobertura a partir da segunda versão publicada.
- **Custo aceito:** o plugin vira API pública. Mudança em `dmpf.json`, `versions.json`, opções de generator ou executors passa a exigir migration, e os `project.json` do platform passam a depender do plugin construído.

## Referências

- SPEC-P846PH1P (DEVS-70).
- [ADR-002](./002-nx-task-configuration.md), [ADR-030](./030-granularidade-modulo-go-e-bom.md), [ADR-042](./042-shared-kernel.md), [ADR-046](./046-libs-somente-kernel-de-reuso.md), [ADR-047](./047-tags-de-modulo-go-e-consumo-fora-do-workspace.md), [ADR-053](./053-nomenclatura-e-isolamento-de-banco-e-padronizacao-de-contextos.md) e [ADR-059](./059-remover-o-bom-e-a-release-do-produto.md).
- `docs/guides/dmpf-manifesto.md` — schema do `dmpf-units.json`.
