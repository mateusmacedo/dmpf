---
description: Preenche template de Pull Request com base no diff da branch atual versus uma branch base (padrão: origin/develop). USE WHEN o usuário pedir para preencher descrição de PR automaticamente, houver um template de PR a ser completado, ou for necessário classificar tipo de PR com base em commits e arquivos alterados.
allowed-tools: Bash
---

# Preencher Template de PR a partir do Diff

Gere o conteúdo de descrição de PR com base em evidências reais do Git, sem inventar informações.

## Entradas esperadas

$ARGUMENTS

- `template`: texto do template de PR fornecido pelo usuário
- `baseRef` (opcional): branch base de comparação (default: `origin/develop`)
- `headRef` (opcional): referência de comparação superior (default: `HEAD`)
- `ticketsLinks` (opcional): links adicionais fornecidos pelo usuário

## Saída esperada

Markdown com o template completo, incluindo:

1. Descrição objetiva do que foi feito
2. Seção de tickets/documentações (com placeholders quando não houver dados)
3. Classificação do tipo de PR com checkboxes coerentes
4. Checklist com marcação conservadora (somente itens com evidência)

---

## Workflow

1. Coletar contexto Git
   - Levantar commits entre base e head
   - Levantar arquivos alterados com status (`A`, `M`, `D`, etc.)
   - Levantar estatísticas de alteração (`insertions/deletions`)

2. Consolidar resumo técnico
   - Agrupar mudanças por domínio (CI, build, libs, docs, testes)
   - Extrair intenção dos commits
   - Gerar bullets curtos e verificáveis

3. Classificar tipo de PR
   - Aplicar heurísticas por caminhos e mensagens de commit
   - Marcar múltiplos tipos quando fizer sentido

4. Preencher template
   - Escrever descrição no padrão solicitado
   - Preservar estrutura e ordem dos blocos do template
   - Não remover seções fornecidas pelo usuário

5. Validar consistência
   - Não inventar ticket, critério de aceite ou teste executado
   - Garantir coerência entre descrição, tipos marcados e checklist

---

## Heurísticas de classificação

Marcar `🤖 Build` quando houver mudanças em:

- `package.json`, lockfiles, `.npmrc`
- `nx.json`, `project.json`, `tsconfig*`
- scripts/configurações de build/release

Marcar `🔁 CI` quando houver mudanças em:

- `.github/workflows/*`
- arquivos de pipeline/automação

Marcar `🥓 Parte de uma feature` quando:

- as mudanças habilitam infraestrutura/processo, mas não entregam uma feature final de produto

Marcar `🍕 Feature Completa` quando:

- houver entrega funcional final para usuário/negócio

Marcar `🐛 Bug Fix` quando:

- o diff e os commits indicarem correção de defeito

Marcar `📝 Atualização de documentação` quando:

- alterações forem predominantemente em arquivos de documentação

Marcar `✅ Testes` quando:

- houver criação/alteração clara de testes

> Regra geral: se houver dúvida, não marcar.

---

## Regras para checklist

- `🚩 O título do PR está aderente ao padrão`
  - Não marcar automaticamente sem validação do título final

- `🔬 Eu revisei minhas próprias alterações`
  - Marcar apenas se houver confirmação explícita no contexto

- `🧪 Eu testei minhas alterações`
  - Marcar somente com evidência de execução real de testes

- `📝 Eu validei os critérios de aceite`
  - Não marcar sem critérios definidos e verificados

- `🛠️ Foram realizados os testes cruzados`
  - Não marcar sem evidência explícita

---

## Regras de qualidade

- Escrever em PT-BR claro e objetivo
- Evitar linguagem genérica sem rastreabilidade no diff
- Manter texto pronto para colar no corpo do PR
- Não incluir comandos ou logs desnecessários no resultado final

## Anti-padrões (proibido)

- Inventar tickets, links, aprovações ou testes
- Marcar checkboxes por suposição
- Omitir mudanças relevantes do diff
- Reescrever template em outro formato sem solicitação
