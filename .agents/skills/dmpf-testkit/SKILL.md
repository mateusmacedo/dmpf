---
name: testkit
description: >-
  Implementa, revisa e executa testes do kernel DMPF Go com o testkit deste
  workspace. Use para UPRs e agregados, services transacionais, conformidade de
  UoW/Inbox/Outbox, golden fixtures, consumer adapters, redelivery distribuído ou
  fitness arquitetural. Não use para testes Go sem relação com o DMPF.
---

# DMPF testkit

Use o menor kit capaz de provar a regra alterada e avance pela pirâmide somente
quando a mudança atravessar uma fronteira. O resultado precisa demonstrar o
efeito observável e o diagnóstico normativo, não apenas exercitar um helper.

## Confirme a API vigente

O código no disco é a autoridade. Antes de escrever ou revisar um teste:

1. Leia `libs/backend/go/testkit/README.md` e o `doc.go` do package escolhido.
2. Inspecione as declarações exportadas e pelo menos um uso real em `*_test.go`.
3. Procure consumidores fora do módulo para entender a adaptação do candidato.

Use buscas focadas, por exemplo:

```bash
rg -n '^(type|func|const|var) [A-Z]' libs/backend/go/testkit/<package> --glob '*.go' --glob '!**/*_test.go'
rg -n 'testkit/<package>' libs/backend/go --glob '*_test.go'
```

Consulte [references/package-routing.md](references/package-routing.md) para
escolher o package e localizar os exemplos canônicos.

## Invariantes do instrumento

- `domainkit` e `golden` devolvem veredictos por valor e não importam
  `testing`. Faça a ponte em teste externo com `tb.Require` ou
  `tb.RequireReport`.
- O `domainkit` recebe UPR, projeções e clone por valor. Não introduza banco,
  broker, relógio, gerador de ID ou duplo de infraestrutura no teste de domínio.
- Uma rejeição de domínio não muda o estado nem produz evento. Sempre prove
  também o determinismo com `ReadTwice` quando testar uma UPR.
- O `serviceskit` observa posições no ledger. Estado e outbox ficam entre o
  mesmo `begin` e `commit`; `publish` direto pelo service é defeito.
- Cada execução de `providerkit` deve receber um candidato limpo. Mantenha
  `Skipped` explícito para cláusulas impossíveis de injetar; não o trate como
  aprovação silenciosa.
- Use `clock`, `ids` e `stable` para controlar tempo, identidades e ordem. Não
  prove regra normativa com `time.Sleep`, `time.Now`, aleatoriedade ou ordem de
  mapa/goroutine.
- O `golden` precisa reportar separadamente semântica (`DMPF-R001`), hash
  (`DMPF-R002`) e identidade de bytes (`DMPF-R003`) nas direções aplicáveis.
  Equivalência semântica não substitui bytes canônicos.
- Use `appkit` para o fluxo em um processo até os efeitos persistidos e o gesto
  de broker. Prove que `Ack` ocorre depois do commit.
- Use `distkit` somente quando broker, redelivery e isolamento entre processos
  forem parte do comportamento. Duas goroutines não substituem os processos OS
  do vetor distribuído, e o controle ingênuo deve reprovar com `DMPF-R004`.
- A fitness function reutiliza `DMPF-D001`, `DMPF-D002` e `DMPF-E001..E004` do
  `conformance`; não crie uma taxonomia paralela nem incorpore o trust model
  do baseline ao teste da suíte.

## Forma do teste

1. Identifique a regra DMPF, o bloco alterado e o efeito observável.
2. Escolha o kit pela fronteira, não pela conveniência do harness.
3. Adapte o código real à interface corrente do kit. Não substitua a integração
   que está sendo certificada por um helper ou fake que contorne o caminho real.
4. Cubra o vetor positivo e um red control que falharia se o oráculo estivesse
   invertido, ausente ou permissivo.
5. Assevere o veredicto e os efeitos finais. Quando a ordem for normativa,
   verifique também o ponto de sincronização ou a posição no ledger.
6. Preserve build tags e condições de infraestrutura dos testes vizinhos.

Ao refatorar um símbolo do kit, busque todos os usos no workspace antes de
alterá-lo. Ao adicionar uma dependência test-only entre módulos Go, confirme o
`go.mod`, o `go.work`, o grafo Nx e as regras do `dmpf-units.json`; não use
atalhos de import ou paths para mascarar a aresta.

## Validação

Rode primeiro o alvo estreito do módulo alterado e depois a cadeia relevante:

```bash
pnpm nx run testkit:fmt-check
pnpm nx run testkit:vet
pnpm nx run testkit:lint
pnpm nx run testkit:build
pnpm nx run testkit:test-race
pnpm nx run testkit:govulncheck
```

`test-race` inclui a build tag `integration`; sem `DMPF_PG_DSN`, os casos de
infraestrutura fazem skip local e falham com `CI` definido. Para a prova
distribuída, use Postgres e Redpanda reais:

```bash
DMPF_PG_DSN='postgres://app:app@localhost:5432/app?sslmode=disable' \
DMPF_KAFKA_BROKERS=localhost:9092 \
pnpm nx run testkit:test-distributed
```

Inclua na validação os módulos consumidores modificados. Não afirme que uma
integração passou quando ela apenas foi pulada por ausência de infraestrutura.
