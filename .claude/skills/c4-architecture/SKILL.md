---
name: c4-architecture
description: Gera documentação de arquitetura usando diagramas do modelo C4 em sintaxe Mermaid. Use quando pedirem para criar diagramas de arquitetura, documentar a arquitetura do sistema, visualizar a estrutura do software, criar diagramas C4 ou gerar diagramas de contexto/container/componente/deployment. Gatilhos incluem "diagrama de arquitetura", "diagrama C4", "contexto do sistema", "diagrama de container", "diagrama de componentes", "diagrama de deployment", "documentar arquitetura", "visualizar arquitetura".
---

# Documentação de Arquitetura C4

Gera documentação de arquitetura de software usando diagramas do modelo C4 em sintaxe Mermaid.

## Fluxo de trabalho

1. **Entenda o escopo** - Determine quais níveis C4 são necessários com base no público
2. **Analise o codebase** - Explore o sistema para identificar componentes, containers e relacionamentos
3. **Gere os diagramas** - Crie diagramas C4 em Mermaid nos níveis de abstração apropriados
4. **Documente** - Escreva os diagramas em arquivos markdown com contexto explicativo

## Níveis de diagrama C4

Selecione o nível apropriado com base na necessidade de documentação:

| Nível | Tipo de Diagrama | Público | Mostra | Quando Criar |
|-------|-------------|----------|-------|----------------|
| 1 | **C4Context** | Todos | Sistema + atores externos | Sempre (obrigatório) |
| 2 | **C4Container** | Técnicos | Apps, bancos de dados, serviços | Sempre (obrigatório) |
| 3 | **C4Component** | Desenvolvedores | Componentes internos | Apenas se agregar valor |
| 4 | **C4Deployment** | DevOps | Nós de infraestrutura | Para sistemas em produção |
| - | **C4Dynamic** | Técnicos | Fluxos de requisição (numerados) | Para workflows complexos |

**Insight principal:** "Diagramas de Contexto + Container são suficientes para a maioria das equipes de desenvolvimento de software." Só crie diagramas de Componente/Código quando eles agregarem valor de verdade.

## Exemplos rápidos

### Contexto do Sistema (Nível 1)
```mermaid
C4Context
  title Contexto do Sistema - Workout Tracker

  Person(user, "Usuário", "Rastreia treinos e exercícios")
  System(app, "Workout Tracker", "PWA em Vue para acompanhar treinos de força e CrossFit")
  System_Ext(browser, "Navegador Web", "Armazena dados no IndexedDB")

  Rel(user, app, "Usa")
  Rel(app, browser, "Persiste dados em", "IndexedDB")
```

### Diagrama de Container (Nível 2)
```mermaid
C4Container
  title Diagrama de Container - Workout Tracker

  Person(user, "Usuário", "Rastreia treinos")

  Container_Boundary(app, "PWA Workout Tracker") {
    Container(spa, "SPA", "Vue 3, TypeScript", "Aplicação single-page")
    Container(pinia, "Gerenciamento de Estado", "Pinia", "Gerencia o estado da aplicação")
    ContainerDb(indexeddb, "IndexedDB", "Dexie", "Armazenamento local de treinos")
  }

  Rel(user, spa, "Usa")
  Rel(spa, pinia, "Lê/escreve estado")
  Rel(pinia, indexeddb, "Persiste", "Dexie ORM")
```

### Diagrama de Componentes (Nível 3)
```mermaid
C4Component
  title Diagrama de Componentes - Feature de Treino

  Container(views, "Views", "Páginas do Vue Router")

  Container_Boundary(workout, "Feature de Treino") {
    Component(useWorkout, "useWorkout", "Composable", "Estado de execução do treino")
    Component(useTimer, "useTimer", "Composable", "Máquina de estados do timer")
    Component(workoutRepo, "WorkoutRepository", "Dexie", "Persistência de treinos")
  }

  Rel(views, useWorkout, "Usa")
  Rel(useWorkout, useTimer, "Controla")
  Rel(useWorkout, workoutRepo, "Salva em")
```

### Diagrama Dinâmico (Fluxo de Requisição)
```mermaid
C4Dynamic
  title Diagrama Dinâmico - Fluxo de Login do Usuário

  ContainerDb(db, "Banco de Dados", "PostgreSQL", "Credenciais de usuário")
  Container(spa, "Aplicação Single-Page", "React", "UI bancária")

  Container_Boundary(api, "Aplicação de API") {
    Component(signIn, "Controller de Login", "Express", "Endpoint de autenticação")
    Component(security, "Serviço de Segurança", "JWT", "Valida credenciais")
  }

  Rel(spa, signIn, "1. Envia credenciais", "JSON/HTTPS")
  Rel(signIn, security, "2. Valida")
  Rel(security, db, "3. Consulta usuário", "SQL")

  UpdateRelStyle(spa, signIn, $textColor="blue", $offsetY="-30")
```

### Diagrama de Deployment
```mermaid
C4Deployment
  title Diagrama de Deployment - Produção

  Deployment_Node(browser, "Navegador do Cliente", "Chrome/Firefox") {
    Container(spa, "SPA", "React", "Aplicação web")
  }

  Deployment_Node(aws, "AWS Cloud", "us-east-1") {
    Deployment_Node(ecs, "ECS Cluster", "Fargate") {
      Container(api, "Serviço de API", "Node.js", "API REST")
    }
    Deployment_Node(rds, "RDS", "db.r5.large") {
      ContainerDb(db, "Banco de Dados", "PostgreSQL", "Dados da aplicação")
    }
  }

  Rel(spa, api, "Chamadas de API", "HTTPS")
  Rel(api, db, "Lê/escreve", "JDBC")
```

## Sintaxe de elementos

### Pessoas e sistemas
```
Person(alias, "Rótulo", "Descrição")
Person_Ext(alias, "Rótulo", "Descrição")       # Pessoa externa
System(alias, "Rótulo", "Descrição")
System_Ext(alias, "Rótulo", "Descrição")       # Sistema externo
SystemDb(alias, "Rótulo", "Descrição")         # Sistema de banco de dados
SystemQueue(alias, "Rótulo", "Descrição")      # Sistema de fila
```

### Containers
```
Container(alias, "Rótulo", "Tecnologia", "Descrição")
Container_Ext(alias, "Rótulo", "Tecnologia", "Descrição")
ContainerDb(alias, "Rótulo", "Tecnologia", "Descrição")
ContainerQueue(alias, "Rótulo", "Tecnologia", "Descrição")
```

### Componentes
```
Component(alias, "Rótulo", "Tecnologia", "Descrição")
Component_Ext(alias, "Rótulo", "Tecnologia", "Descrição")
ComponentDb(alias, "Rótulo", "Tecnologia", "Descrição")
```

### Boundaries
```
Enterprise_Boundary(alias, "Rótulo") { ... }
System_Boundary(alias, "Rótulo") { ... }
Container_Boundary(alias, "Rótulo") { ... }
Boundary(alias, "Rótulo", "tipo") { ... }
```

### Relacionamentos
```
Rel(from, to, "Rótulo")
Rel(from, to, "Rótulo", "Tecnologia")
BiRel(from, to, "Rótulo")                        # Bidirecional
Rel_U(from, to, "Rótulo")                        # Para cima
Rel_D(from, to, "Rótulo")                        # Para baixo
Rel_L(from, to, "Rótulo")                        # Para a esquerda
Rel_R(from, to, "Rótulo")                        # Para a direita
```

### Nós de deployment
```
Deployment_Node(alias, "Rótulo", "Tipo", "Descrição") { ... }
Node(alias, "Rótulo", "Tipo", "Descrição") { ... }  # Forma abreviada
```

## Estilo e layout

### Configuração de layout
```
UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="1")
```
- `$c4ShapeInRow` - Número de formas por linha (padrão: 4)
- `$c4BoundaryInRow` - Número de boundaries por linha (padrão: 2)

### Estilo de elementos
```
UpdateElementStyle(alias, $fontColor="red", $bgColor="grey", $borderColor="red")
```

### Estilo de relacionamentos
```
UpdateRelStyle(from, to, $textColor="blue", $lineColor="blue", $offsetX="5", $offsetY="-10")
```
Use `$offsetX` e `$offsetY` para corrigir rótulos de relacionamento sobrepostos.

## Boas práticas

### Regras essenciais

1. **Todo elemento deve ter**: nome, tipo, tecnologia (quando aplicável) e descrição
2. **Use apenas setas unidirecionais** - Setas bidirecionais criam ambiguidade
3. **Rotule as setas com verbos de ação** - "Envia e-mail usando", "Lê de", não apenas "usa"
4. **Inclua rótulos de tecnologia** - "JSON/HTTPS", "JDBC", "gRPC"
5. **Mantenha menos de 20 elementos por diagrama** - Divida sistemas complexos em vários diagramas

### Diretrizes de clareza

1. **Comece pelo Nível 1** - Diagramas de contexto ajudam a delimitar o escopo do sistema
2. **Um diagrama por arquivo** - Mantenha cada diagrama focado em um único nível de abstração
3. **Aliases significativos** - Use aliases descritivos (por exemplo, `orderService` em vez de `s1`)
4. **Descrições concisas** - Mantenha as descrições com menos de 50 caracteres quando possível
5. **Sempre inclua um título** - "Diagrama de Contexto do Sistema para [Nome do Sistema]"

### O que evitar

Consulte [references/common-mistakes.md](references/common-mistakes.md) para anti-padrões detalhados:
- Confundir containers (deployáveis) com componentes (não deployáveis)
- Modelar bibliotecas compartilhadas como containers
- Representar message brokers como um único container em vez de tópicos individuais
- Adicionar níveis de abstração indefinidos como "subcomponentes"
- Remover rótulos de tipo para "simplificar" os diagramas

## Diretrizes para microsserviços

### Propriedade por uma única equipe
Modele cada microsserviço como um **container** (ou grupo de containers):
```mermaid
C4Container
  title Microsserviços - Equipe Única

  System_Boundary(platform, "Plataforma de E-commerce") {
    Container(orderApi, "Serviço de Pedidos", "Spring Boot", "Processamento de pedidos")
    ContainerDb(orderDb, "Banco de Pedidos", "PostgreSQL", "Dados de pedidos")
    Container(inventoryApi, "Serviço de Estoque", "Node.js", "Gestão de estoque")
    ContainerDb(inventoryDb, "Banco de Estoque", "MongoDB", "Dados de estoque")
  }
```

### Propriedade por múltiplas equipes
Promova microsserviços a **sistemas de software** quando forem de propriedade de equipes separadas:
```mermaid
C4Context
  title Microsserviços - Múltiplas Equipes

  Person(customer, "Cliente", "Faz pedidos")
  System(orderSystem, "Sistema de Pedidos", "Equipe Alpha")
  System(inventorySystem, "Sistema de Estoque", "Equipe Beta")
  System(paymentSystem, "Sistema de Pagamento", "Equipe Gamma")

  Rel(customer, orderSystem, "Faz pedidos")
  Rel(orderSystem, inventorySystem, "Verifica estoque")
  Rel(orderSystem, paymentSystem, "Processa pagamento")
```

### Arquitetura orientada a eventos
Represente tópicos/filas individuais como containers, NÃO uma única caixa "Kafka":
```mermaid
C4Container
  title Arquitetura Orientada a Eventos

  Container(orderService, "Serviço de Pedidos", "Java", "Cria pedidos")
  Container(stockService, "Serviço de Estoque", "Java", "Gerencia o estoque")
  ContainerQueue(orderTopic, "order.created", "Kafka", "Eventos de pedido")
  ContainerQueue(stockTopic, "stock.reserved", "Kafka", "Eventos de estoque")

  Rel(orderService, orderTopic, "Publica em")
  Rel(stockService, orderTopic, "Assina")
  Rel(stockService, stockTopic, "Publica em")
  Rel(orderService, stockTopic, "Assina")
```

## Local de saída

Escreva a documentação de arquitetura em `docs/architecture/` seguindo a convenção de nomes:
- `c4-context.md` - Diagrama de contexto do sistema
- `c4-containers.md` - Diagrama de container
- `c4-components-{feature}.md` - Diagramas de componente por feature
- `c4-deployment.md` - Diagrama de deployment
- `c4-dynamic-{flow}.md` - Diagramas dinâmicos para fluxos específicos

## Nível de detalhe por público

| Público | Diagramas Recomendados |
|----------|---------------------|
| Executivos | Apenas Contexto do Sistema |
| Product Managers | Contexto + Container |
| Arquitetos | Contexto + Container + Componentes-chave |
| Desenvolvedores | Todos os níveis conforme necessário |
| DevOps | Container + Deployment |

## Referências

- [references/c4-syntax.md](references/c4-syntax.md) - Sintaxe completa do C4 em Mermaid
- [references/common-mistakes.md](references/common-mistakes.md) - Anti-padrões a evitar
- [references/advanced-patterns.md](references/advanced-patterns.md) - Microsserviços, orientação a eventos, deployment
