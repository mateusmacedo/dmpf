# Padrões Avançados de Arquitetura C4

Este guia cobre padrões avançados para documentar arquiteturas complexas, incluindo microsserviços, sistemas orientados a eventos, deployments e documentação de APIs.

## Arquitetura de Microsserviços

### Propriedade por Time Único

Quando um único time é dono de todos os microsserviços, modele-os como **containers** dentro de um único sistema:

```mermaid
C4Container
  title Plataforma de E-commerce - Time Único

  Person(customer, "Cliente", "Comprador online")

  System_Ext(payment, "Stripe", "Pagamentos")
  System_Ext(shipping, "FedEx", "Frete")

  System_Boundary(platform, "Plataforma de E-commerce") {
    Container(gateway, "API Gateway", "Kong", "Roteamento, autenticação, rate limiting")

    Container(orderSvc, "Serviço de Pedidos", "Node.js", "Processamento de pedidos")
    ContainerDb(orderDb, "BD de Pedidos", "PostgreSQL", "Pedidos")

    Container(productSvc, "Serviço de Produtos", "Go", "Catálogo de produtos")
    ContainerDb(productDb, "BD de Produtos", "MongoDB", "Produtos")

    Container(userSvc, "Serviço de Usuários", "Java", "Autenticação")
    ContainerDb(userDb, "BD de Usuários", "PostgreSQL", "Usuários")
    ContainerDb(cache, "Cache", "Redis", "Sessões")
  }

  Rel(customer, gateway, "Requisições de API", "HTTPS")
  Rel(gateway, orderSvc, "Roteia", "HTTP")
  Rel(gateway, productSvc, "Roteia", "HTTP")
  Rel(gateway, userSvc, "Roteia", "HTTP")

  Rel(orderSvc, orderDb, "Persiste", "SQL")
  Rel(productSvc, productDb, "Persiste", "MongoDB")
  Rel(userSvc, userDb, "Persiste", "SQL")
  Rel(userSvc, cache, "Cacheia", "Redis")

  Rel(orderSvc, payment, "Cobra", "REST")
  Rel(orderSvc, shipping, "Envia", "REST")
```

### Propriedade por Múltiplos Times

Quando times distintos são donos dos microsserviços, **promova cada um a um sistema de software**:

```mermaid
C4Context
  title Plataforma de E-commerce - Multi-Time

  Person(customer, "Cliente", "Comprador online")
  Person(admin, "Admin", "Gerente da loja")

  Enterprise_Boundary(company, "Acme Corp") {
    System(orderSystem, "Sistema de Pedidos", "Time Alpha - Ciclo de vida do pedido")
    System(productSystem, "Sistema de Produtos", "Time Beta - Gestão de catálogo")
    System(userSystem, "Sistema de Usuários", "Time Gamma - Identidade e autenticação")
    System(analyticsSystem, "Sistema de Analytics", "Time Delta - Business intelligence")
  }

  System_Ext(payment, "Stripe", "Processamento de pagamentos")
  System_Ext(warehouse, "Sistema de Armazém", "Parceiro de fulfillment")

  Rel(customer, orderSystem, "Faz pedidos")
  Rel(customer, productSystem, "Navega produtos")
  Rel(admin, productSystem, "Gerencia catálogo")
  Rel(admin, analyticsSystem, "Visualiza relatórios")

  Rel(orderSystem, userSystem, "Autentica")
  Rel(orderSystem, productSystem, "Verifica estoque")
  Rel(orderSystem, payment, "Processa pagamentos")
  Rel(orderSystem, warehouse, "Atende pedidos")
  Rel(analyticsSystem, orderSystem, "Agrega dados")
```

Cada time então cria o próprio diagrama de Container:

```mermaid
C4Container
  title Sistema de Pedidos - Time Alpha

  System_Ext(productSystem, "Sistema de Produtos", "Verificações de estoque")
  System_Ext(userSystem, "Sistema de Usuários", "Autenticação")
  System_Ext(payment, "Stripe", "Pagamentos")

  Container_Boundary(orderSystem, "Sistema de Pedidos") {
    Container(orderApi, "API de Pedidos", "Spring Boot", "Endpoints REST")
    Container(orderWorker, "Worker de Pedidos", "Spring Boot", "Processamento assíncrono")
    ContainerDb(orderDb, "BD de Pedidos", "PostgreSQL", "Dados de pedidos")
    ContainerQueue(orderQueue, "Fila de Pedidos", "RabbitMQ", "Fila de processamento")
  }

  Rel(orderApi, orderDb, "Lê/grava", "JDBC")
  Rel(orderApi, orderQueue, "Publica", "AMQP")
  Rel(orderWorker, orderQueue, "Consome", "AMQP")
  Rel(orderWorker, orderDb, "Atualiza", "JDBC")

  Rel(orderApi, userSystem, "Valida tokens", "REST")
  Rel(orderApi, productSystem, "Reserva estoque", "REST")
  Rel(orderWorker, payment, "Cobra", "REST")
```

## Arquitetura Orientada a Eventos

### Exibindo Tópicos Individuais

Sempre modele tópicos/filas de mensagens como containers separados:

```mermaid
C4Container
  title Processamento de Pedidos - Orientado a Eventos

  Container(orderSvc, "Serviço de Pedidos", "Java", "Cria pedidos")
  Container(inventorySvc, "Serviço de Estoque", "Go", "Gerencia estoque")
  Container(paymentSvc, "Serviço de Pagamentos", "Node.js", "Processa pagamentos")
  Container(shippingSvc, "Serviço de Envio", "Python", "Cria envios")
  Container(notificationSvc, "Serviço de Notificações", "Python", "Envia alertas")

  ContainerQueue(orderCreated, "order.created", "Kafka", "Eventos de novo pedido")
  ContainerQueue(stockReserved, "inventory.reserved", "Kafka", "Eventos de reserva de estoque")
  ContainerQueue(paymentComplete, "payment.completed", "Kafka", "Eventos de pagamento")
  ContainerQueue(orderShipped, "order.shipped", "Kafka", "Eventos de envio")

  Rel(orderSvc, orderCreated, "Publica", "Avro")

  Rel(inventorySvc, orderCreated, "Consome", "Avro")
  Rel(inventorySvc, stockReserved, "Publica", "Avro")

  Rel(paymentSvc, stockReserved, "Consome", "Avro")
  Rel(paymentSvc, paymentComplete, "Publica", "Avro")

  Rel(shippingSvc, paymentComplete, "Consome", "Avro")
  Rel(shippingSvc, orderShipped, "Publica", "Avro")

  Rel(notificationSvc, orderCreated, "Consome", "Avro")
  Rel(notificationSvc, paymentComplete, "Consome", "Avro")
  Rel(notificationSvc, orderShipped, "Consome", "Avro")

  Rel(orderSvc, paymentComplete, "Consome", "Avro")
  Rel(orderSvc, orderShipped, "Consome", "Avro")

  UpdateLayoutConfig($c4ShapeInRow="4")
```

### Fluxo de Eventos com Diagrama Dinâmico

Use diagramas dinâmicos para mostrar a sequência de eventos:

```mermaid
C4Dynamic
  title Fluxo de Processamento de Pedidos

  Container(orderSvc, "Serviço de Pedidos", "Java")
  Container(inventorySvc, "Serviço de Estoque", "Go")
  Container(paymentSvc, "Serviço de Pagamentos", "Node.js")
  Container(shippingSvc, "Serviço de Envio", "Python")

  ContainerQueue(orderCreated, "order.created", "Kafka")
  ContainerQueue(stockReserved, "inventory.reserved", "Kafka")
  ContainerQueue(paymentComplete, "payment.completed", "Kafka")

  Rel(orderSvc, orderCreated, "1. Publica pedido", "Avro")
  Rel(inventorySvc, orderCreated, "2. Consome pedido", "Avro")
  Rel(inventorySvc, stockReserved, "3. Publica reserva", "Avro")
  Rel(paymentSvc, stockReserved, "4. Consome reserva", "Avro")
  Rel(paymentSvc, paymentComplete, "5. Publica pagamento", "Avro")
  Rel(shippingSvc, paymentComplete, "6. Consome pagamento", "Avro")
```

### Padrão CQRS

```mermaid
C4Container
  title Arquitetura CQRS

  Person(user, "Usuário", "Usuário da aplicação")

  Container_Boundary(app, "Aplicação") {
    Container(commandApi, "API de Comandos", "Java", "Operações de escrita")
    Container(queryApi, "API de Consultas", "Node.js", "Operações de leitura")

    ContainerDb(writeDb, "BD de Escrita", "PostgreSQL", "Fonte da verdade")
    ContainerDb(readDb, "BD de Leitura", "Elasticsearch", "Otimizado para consultas")

    ContainerQueue(events, "Eventos de Domínio", "Kafka", "Mudanças de estado")
    Container(projector, "Projetor", "Java", "Atualiza o modelo de leitura")
  }

  Rel(user, commandApi, "Comandos", "HTTPS")
  Rel(user, queryApi, "Consultas", "HTTPS")

  Rel(commandApi, writeDb, "Grava", "JDBC")
  Rel(commandApi, events, "Publica", "Avro")

  Rel(projector, events, "Consome", "Avro")
  Rel(projector, readDb, "Atualiza", "REST")

  Rel(queryApi, readDb, "Consulta", "REST")
```

## Padrões de Deployment

### Deployment de Produção na AWS

```mermaid
C4Deployment
  title Produção - AWS us-east-1

  Deployment_Node(route53, "Route 53", "DNS") {
    Container(dns, "DNS", "AWS", "api.example.com")
  }

  Deployment_Node(cloudfront, "CloudFront", "CDN") {
    Container(cdn, "CDN", "AWS", "Cache de assets estáticos")
  }

  Deployment_Node(vpc, "VPC", "10.0.0.0/16") {

    Deployment_Node(public, "Sub-redes Públicas", "Multi-AZ") {
      Deployment_Node(alb, "ALB", "LB de Aplicação") {
        Container(lb, "Load Balancer", "AWS ALB", "Terminação TLS, roteamento")
      }
    }

    Deployment_Node(private, "Sub-redes Privadas", "Multi-AZ") {

      Deployment_Node(ecs, "Cluster ECS", "Fargate") {
        Container(api1, "API", "Node.js", "Instância 1")
        Container(api2, "API", "Node.js", "Instância 2")
        Container(worker1, "Worker", "Python", "Instância 1")
      }

      Deployment_Node(rds, "RDS", "db.r5.xlarge") {
        ContainerDb(primary, "Primário", "PostgreSQL 14", "Multi-AZ")
      }

      Deployment_Node(elasticache, "ElastiCache", "cache.r5.large") {
        ContainerDb(redis, "Redis", "Redis 7", "Modo cluster")
      }
    }
  }

  Rel(dns, cdn, "Roteia para", "HTTPS")
  Rel(cdn, lb, "Encaminha", "HTTPS")
  Rel(lb, api1, "Roteia", "HTTP")
  Rel(lb, api2, "Roteia", "HTTP")
  Rel(api1, primary, "Consulta", "JDBC")
  Rel(api2, primary, "Consulta", "JDBC")
  Rel(api1, redis, "Cacheia", "Redis")
  Rel(worker1, primary, "Processa", "JDBC")
```

### Deployment em Kubernetes

```mermaid
C4Deployment
  title Produção - Kubernetes

  Deployment_Node(ingress, "Ingress Controller", "nginx") {
    Container(nginx, "Nginx", "nginx-ingress", "TLS, roteamento")
  }

  Deployment_Node(cluster, "Cluster Kubernetes", "EKS 1.28") {

    Deployment_Node(nsApp, "namespace app", "Aplicação") {

      Deployment_Node(apiDeploy, "api-deployment", "3 réplicas") {
        Container(api, "Pod da API", "Node.js 20", "API REST")
      }

      Deployment_Node(workerDeploy, "worker-deployment", "2 réplicas") {
        Container(worker, "Pod do Worker", "Python 3.11", "Jobs em background")
      }
    }

    Deployment_Node(nsData, "namespace data", "Bancos de dados") {

      Deployment_Node(pgStateful, "postgres-statefulset", "HA") {
        ContainerDb(pg, "PostgreSQL", "PostgreSQL 15", "Primário + Réplica")
      }

      Deployment_Node(redisStateful, "redis-statefulset", "Cluster") {
        ContainerDb(redis, "Redis", "Redis 7", "Cluster de 3 nós")
      }
    }
  }

  Rel(nginx, api, "Roteia /api/*", "HTTP")
  Rel(api, pg, "Consulta", "JDBC")
  Rel(api, redis, "Cacheia", "Redis")
  Rel(worker, pg, "Processa", "JDBC")
```

### Deployment Multi-Região

```mermaid
C4Deployment
  title Multi-Região Ativo-Ativo

  Deployment_Node(globalLB, "Load Balancer Global", "AWS Global Accelerator") {
    Container(glb, "GLB", "AWS", "Roteamento geográfico")
  }

  Deployment_Node(usEast, "US-East-1", "Região Primária") {
    Deployment_Node(usEcs, "Cluster ECS", "Fargate") {
      Container(usApi, "API", "Node.js", "Instâncias US")
    }
    Deployment_Node(usRds, "RDS", "Multi-AZ") {
      ContainerDb(usPrimary, "BD Primário", "PostgreSQL", "Líder de escrita")
    }
  }

  Deployment_Node(euWest, "EU-West-1", "Região Secundária") {
    Deployment_Node(euEcs, "Cluster ECS", "Fargate") {
      Container(euApi, "API", "Node.js", "Instâncias EU")
    }
    Deployment_Node(euRds, "RDS", "Réplica de Leitura") {
      ContainerDb(euReplica, "BD Réplica", "PostgreSQL", "Réplica de leitura")
    }
  }

  Rel(glb, usApi, "Tráfego US", "HTTPS")
  Rel(glb, euApi, "Tráfego EU", "HTTPS")
  Rel(usApi, usPrimary, "Lê/grava", "JDBC")
  Rel(euApi, euReplica, "Lê", "JDBC")
  Rel(euApi, usPrimary, "Grava", "JDBC")
  Rel(usPrimary, euReplica, "Replica", "Streaming")
```

## Padrões de Documentação de API

### Padrão de API Gateway

```mermaid
C4Container
  title Arquitetura de API Gateway

  Person(mobile, "Usuário Mobile", "Usuário de app iOS/Android")
  Person(web, "Usuário Web", "Usuário de navegador")
  Person(partner, "Parceiro", "Integração de terceiros")

  Container(mobileApp, "App Mobile", "React Native", "Cliente mobile nativo")
  Container(webApp, "App Web", "React", "Cliente SPA")

  Container_Boundary(apiPlatform, "Plataforma de API") {
    Container(gateway, "API Gateway", "Kong", "Autenticação, rate limit, roteamento")
    Container(bff, "BFF", "Node.js", "Backend for frontend")

    Container(userApi, "API de Usuários", "Java", "Gestão de usuários")
    Container(orderApi, "API de Pedidos", "Go", "Processamento de pedidos")
    Container(productApi, "API de Produtos", "Python", "Catálogo de produtos")
  }

  System_Ext(auth0, "Auth0", "Provedor de identidade")

  Rel(mobile, mobileApp, "Usa")
  Rel(web, webApp, "Usa")
  Rel(partner, gateway, "Chamadas de API", "REST/HTTPS")

  Rel(mobileApp, bff, "GraphQL", "HTTPS")
  Rel(webApp, bff, "GraphQL", "HTTPS")

  Rel(bff, gateway, "Chamadas REST", "HTTP")
  Rel(gateway, auth0, "Valida tokens", "HTTPS")

  Rel(gateway, userApi, "Roteia /users/*", "HTTP")
  Rel(gateway, orderApi, "Roteia /orders/*", "HTTP")
  Rel(gateway, productApi, "Roteia /products/*", "HTTP")
```

### Detalhe de Componentes da API

```mermaid
C4Component
  title API de Pedidos - Diagrama de Componentes

  Container(gateway, "API Gateway", "Kong")
  ContainerDb(db, "BD de Pedidos", "PostgreSQL")
  ContainerQueue(events, "Eventos de Pedido", "Kafka")
  System_Ext(payment, "Serviço de Pagamentos", "Stripe")

  Container_Boundary(orderApi, "API de Pedidos") {
    Component(controller, "Controller de Pedidos", "Spring MVC", "Endpoints REST")
    Component(validator, "Validador de Requisição", "Bean Validation", "Validação de entrada")
    Component(service, "Serviço de Pedidos", "Spring Service", "Lógica de negócio")
    Component(paymentClient, "Cliente de Pagamento", "Feign", "Integração com Stripe")
    Component(repository, "Repositório de Pedidos", "Spring Data JPA", "Acesso a dados")
    Component(publisher, "Publicador de Eventos", "Spring Kafka", "Publicação de eventos")
  }

  Rel(gateway, controller, "Requisições HTTP", "JSON")
  Rel(controller, validator, "Valida")
  Rel(controller, service, "Delega")
  Rel(service, paymentClient, "Cobra")
  Rel(service, repository, "Persiste")
  Rel(service, publisher, "Publica eventos")

  Rel(paymentClient, payment, "REST", "HTTPS")
  Rel(repository, db, "JDBC", "SQL")
  Rel(publisher, events, "Produz", "Avro")
```

## Padrões de Diagramas Complementares

### Fluxo de Autenticação (Dinâmico)

```mermaid
C4Dynamic
  title Fluxo OAuth2 Authorization Code

  Container(spa, "SPA", "React", "Aplicação web")
  Container(api, "API", "Node.js", "Resource server")
  System_Ext(auth0, "Auth0", "Servidor de autorização")
  ContainerDb(db, "BD de Usuários", "PostgreSQL", "Dados de usuário")

  Rel(spa, auth0, "1. Redireciona para /authorize")
  Rel(auth0, spa, "2. Redireciona com código de autorização")
  Rel(spa, api, "3. Troca código por tokens", "HTTPS")
  Rel(api, auth0, "4. POST /oauth/token", "HTTPS")
  Rel(api, spa, "5. Retorna tokens de access + refresh")
  Rel(spa, api, "6. Requisição de API com access token", "HTTPS")
  Rel(api, db, "7. Busca dados do usuário", "SQL")
```

### Fluxo de Tratamento de Erros

```mermaid
C4Dynamic
  title Tratamento de Erros - Circuit Breaker

  Container(api, "API", "Node.js")
  Container(circuitBreaker, "Circuit Breaker", "Resilience4j")
  System_Ext(payment, "Serviço de Pagamentos", "Stripe")
  ContainerDb(fallback, "Cache de Fallback", "Redis")

  Rel(api, circuitBreaker, "1. Solicita pagamento")
  Rel(circuitBreaker, payment, "2. Encaminha requisição", "HTTPS")
  Rel(payment, circuitBreaker, "3a. Resposta de sucesso")
  Rel(circuitBreaker, api, "4a. Retorna sucesso")

  Rel(payment, circuitBreaker, "3b. Timeout/Erro")
  Rel(circuitBreaker, fallback, "4b. Verifica resposta em cache")
  Rel(circuitBreaker, api, "5b. Retorna fallback ou erro")
```

## Integração com Architecture Decision Records

Vincule diagramas C4 a Architecture Decision Records (ADRs):

### Referência a ADR nos Diagramas

```mermaid
C4Container
  title Arquitetura do Sistema
  %% Ver ADR-001 para a seleção do API Gateway
  %% Ver ADR-002 para a escolha do banco de dados
  %% Ver ADR-003 para a abordagem orientada a eventos

  Container(gateway, "API Gateway", "Kong", "ADR-001: Escolhido pelo ecossistema de plugins")
  Container(api, "API de Pedidos", "Spring Boot", "Processamento de pedidos")
  ContainerDb(db, "BD de Pedidos", "PostgreSQL", "ADR-002: Conformidade ACID exigida")
  ContainerQueue(events, "Eventos", "Kafka", "ADR-003: Padrão event sourcing")

  Rel(gateway, api, "Roteia", "HTTP")
  Rel(api, db, "Persiste", "JDBC")
  Rel(api, events, "Publica", "Avro")
```

### Estrutura de Diretórios

Organize os diagramas C4 junto com os ADRs:

```text
docs/
├── architecture/
│   ├── c4-context.md
│   ├── c4-containers.md
│   ├── c4-components-order-api.md
│   ├── c4-deployment-production.md
│   └── c4-dynamic-auth-flow.md
└── decisions/
    ├── 001-api-gateway-selection.md
    ├── 002-database-selection.md
    ├── 003-event-driven-architecture.md
    └── template.md
```

## Diagrama de Panorama de Sistemas

Para visões em nível corporativo que exibem múltiplos sistemas:

```mermaid
C4Context
  title Panorama de Sistemas Corporativos

  Person(customer, "Cliente", "Cliente externo")
  Person(employee, "Funcionário", "Equipe interna")
  Person(partner, "Parceiro", "Parceiro de negócios")

  Enterprise_Boundary(enterprise, "Acme Corporation") {

    Boundary(customerFacing, "Voltado ao Cliente", "Externo") {
      System(ecommerce, "Plataforma de E-commerce", "Loja online")
      System(mobile, "App Mobile", "Experiência mobile do cliente")
      System(support, "Portal de Suporte", "Atendimento ao cliente")
    }

    Boundary(internal, "Sistemas Internos", "Operações") {
      System(erp, "Sistema ERP", "SAP - Finanças e operações")
      System(crm, "Sistema CRM", "Salesforce - Dados de clientes")
      System(analytics, "Plataforma de Analytics", "Business intelligence")
    }

    Boundary(integration, "Camada de Integração", "Middleware") {
      System(esb, "Hub de Integração", "MuleSoft - Gestão de APIs")
      System(etl, "Pipeline de Dados", "Airflow - Processamento de dados")
    }
  }

  System_Ext(payment, "Gateway de Pagamento", "Stripe")
  System_Ext(shipping, "Provedor de Frete", "FedEx")
  System_Ext(warehouse, "Sistema de Armazém", "Parceiro 3PL")

  Rel(customer, ecommerce, "Compra online")
  Rel(customer, mobile, "Usa o app")
  Rel(customer, support, "Busca ajuda")
  Rel(employee, erp, "Gerencia operações")
  Rel(employee, crm, "Gerencia clientes")
  Rel(partner, esb, "Integração via API")

  Rel(ecommerce, esb, "Chamadas de API")
  Rel(esb, erp, "Sincroniza pedidos")
  Rel(esb, crm, "Sincroniza clientes")
  Rel(esb, payment, "Processa pagamentos")
  Rel(esb, shipping, "Cria envios")
  Rel(etl, analytics, "Alimenta dados")
```

## Resumo de Boas Práticas

1. **Escolha a abstração com base na propriedade**: Time único = containers, Múltiplos times = sistemas
2. **Mostre tópicos de mensagem individuais**: Não use uma única caixa "Kafka" ou "RabbitMQ"
3. **Use diagramas de deployment para infraestrutura**: Mantenha os diagramas de container no plano lógico
4. **Crie diagramas dinâmicos para fluxos complexos**: Autenticação, pagamento, tratamento de erros
5. **Vincule aos ADRs**: Documente por que as decisões foram tomadas
6. **Use o panorama de sistemas para visões corporativas**: Mostre todos os sistemas e suas relações
7. **Mantenha os diagramas focados**: Uma preocupação por diagrama, divida quando ficar complexo
