# Regras do Projeto

Catálogo centralizado de regras do sistema, organizadas por categoria.

Este é um scaffold de template: os catálogos nascem vazios de propósito. Projetos
derivados registram aqui as regras reais conforme surgem. Mantenha cada regra na
categoria correta e siga o formato descrito abaixo para que o catálogo permaneça
consultável.

## Categorias

| Categoria | Escopo |
| --- | --- |
| [Negócio](./business/README.md) | Regras de domínio e lógica de negócio |
| [Aplicação](./application/README.md) | Regras técnicas e qualidade de código |
| [Produto](./product/README.md) | Comportamento de UI, fluxos de usuário, UX |

## Formato

Cada regra segue o formato `REGRA-NNN` com prefixo por categoria:

| Prefixo | Categoria |
| --- | --- |
| `BIZ-` | Negócio |
| `APP-` | Aplicação |
| `PRD-` | Produto |

### Estrutura de uma regra

```markdown
### REGRA BIZ-001: Validação de documento obrigatória

**Contexto**: Quando e por que esta regra se aplica.

**Regra**: Descrição clara e objetiva do comportamento esperado.

**Exemplos**:
- Caso válido: documento com CPF preenchido é aceito
- Caso inválido: documento sem CPF é rejeitado com erro

**Exceções**: Situações onde a regra não se aplica (se houver).

**Origem**: De onde veio a decisão — reunião, incidente, pesquisa — com a data.
```

Os campos **Contexto**, **Regra** e **Origem** são obrigatórios. **Exemplos** e
**Exceções** entram quando esclarecem a aplicação da regra.

## Quando criar uma regra

Registre uma regra quando ela atender a pelo menos um destes critérios:

- Comportamento que precisa ser consistente em múltiplos módulos.
- Decisão que impacta múltiplos fluxos ou componentes.
- Restrição que desenvolvedores precisam conhecer antes de implementar.
- Lógica que já causou ou pode causar bugs por falta de documentação.

## Quando manter fora do catálogo

Prefira deixar de fora, para o catálogo focar no que agrega valor:

- Padrão de código que o linter já cobre — deixe o linter cuidar.
- Convenção óbvia da linguagem ou do framework.
- Comportamento restrito a um único arquivo ou função — documente no próprio código.
- Preferência estética sem impacto funcional.
