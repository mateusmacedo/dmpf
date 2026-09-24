# ADR-052: A identidade do workload é verificada no gRPC por mTLS e no Kafka por SASL

## Status

Aceito — 2026-09-23. Implementa [SPEC-9B6SHEH8](../specs/SPEC-9B6SHEH8-contexto-execucao-identidade-tenant.md).

## Contexto

O code review da spec encontrou três falhas que se somavam:

1. A permissão do sujeito não era checada em ponto algum. O BFF resolvia as permissões e não as conferia; os contextos só recebiam chamada sem sujeito, porque `CTX-12` impede o sujeito de atravessar o fan-out, e `application.Permitted` deixava passar toda cadeia sem sujeito.
2. O gRPC interno lia `x-tenant-id` da metadata sem autenticar quem chamava. O servidor tinha TLS só de servidor; qualquer processo que alcançasse a porta escolhia o tenant.
3. O consumer declarava `TransportVerified` a partir de "TLS ligado", mas o Kafka só verificava o broker. Quem pudesse publicar no tópico forjava o `source` e o `tenantid` do envelope.

`IDN-03` define fronteira confiável como aquela em que a identidade do workload do outro lado é verificada; `IDN-04` exige, no consumo, a integridade do envelope e a confiança da fronteira de transporte; `IDN-18` autoriza a cadeia sem sujeito pela identidade do workload que a executa.

## Decisão

**O sujeito é autorizado na borda, e o workload é autenticado em cada salto interno.**

1. **Borda.** Cada rota declara a permissão que exige (`http.Route.Permission`), e `ResolveIdentity` nega com 403 o sujeito que não a tem, antes do fan-out. `ValidateEdge` recusa na partida a rota que exige sujeito e não declara permissão (`IDN-16`, `IDN-17`).
2. **gRPC.** Com TLS ativo, o servidor exige e verifica o certificado do cliente contra uma CA declarada (`DMPF_GRPC_CLIENT_CA_FILE`) e só serve os workloads da allowlist (`DMPF_GRPC_TRUSTED_CLIENTS`), comparada com a URI SAN ou o DNS SAN do certificado — nunca com o CN, que não tem semântica de nome e que a CA pode emitir livre. O interceptor que confere isso, unário e de stream, roda antes da admissão e do contexto, então `x-tenant-id` só é lido de peer verificado. A identidade do BFF é `spiffe://dmpf/bff`. A partida recusa TLS sem CA de cliente ou sem allowlist, e o `ServerConfig` recusa TLS que não exija e verifique o certificado do cliente.
3. **Kafka.** O cliente se autentica por SASL SCRAM-SHA-256/512 ou por certificado de cliente; a partida recusa TLS sem autenticação de cliente. `TransportVerified` só vale com TLS **e** cliente autenticado. A ligação entre o principal e o `source` que o consumer admite é a **ACL do broker por principal**: só o principal de `orders` produz em `orders.events`. A ACL é da plataforma e é **pré-requisito** de todo ambiente com a fronteira verificada: autorização ligada no broker, `orders` com write e describe em `orders.events` e `orders.events.dlq`, `reservations` com write e describe em `reservations.events`, `reservations.events.dlq` e `orders.events.dlq`, e read e describe em `orders.events` e no grupo `reservations`. Sem ela, `TransportVerified` afirma uma ligação que o broker não impõe. O critério de fronteira é daqui.
4. **Opt-out de desenvolvimento.** `DMPF_GRPC_INSECURE` e `DMPF_KAFKA_INSECURE` continuam existindo como escolha explícita e registrada no log. No compose local, o gRPC roda com mTLS de uma CA de desenvolvimento (`pki-init`) e o Kafka exige SASL no listener interno, sem TLS no broker, com a autorização ligada e as mesmas ACLs gravadas pelo `redpanda-init`; a fronteira do consumer fica `development-only`.

## Alternativas descartadas

| Alternativa | Por que foi rejeitada |
| ----------- | --------------------- |
| Propagar o sujeito e as permissões aos contextos | Viola `CTX-12`: o contexto chamado passaria a autorizar com a identidade que o chamador afirma, o problema do delegado confuso |
| Confiar na rede interna | `IDN-02`: herança de confiança de canal não autentica sujeito nem workload |
| Filtrar o `source` sem autenticar o produtor | O `source` é preenchido por quem publica; sem principal autenticado o filtro é declarativo |
| Certificados públicos (Let's Encrypt) no mTLS interno | O Let's Encrypt deixou de emitir o uso de autenticação de cliente em 2026, não emite URI SAN e não valida nomes internos. Fica para a borda pública, em momento próprio |

## Consequências

**Positivas:**

- Um token válido sem a permissão da rota é recusado na borda, e um processo que alcance a porta de um contexto não consegue escolher tenant.
- O consumer só chama verificada a fronteira que o transporte provou.

**Negativas:**

- Cada ambiente precisa de uma CA de clientes, do certificado do BFF e de um principal SASL por processo, entregues por Secret.
- A sonda gRPC do kubelet não apresenta certificado; com mTLS ativo os `api` usam sonda de socket.
- O listener externo do Redpanda local continua sem autenticação, para os testes do host, e o principal dele (o usuário vazio) é superusuário. Ele não é usado pela topologia, mas outro container da rede o alcança: é só para desenvolvimento.
- Em `hmg` o Kafka é gerenciado fora do repositório; a ACL fica registrada como pré-requisito no `secrets.example.yaml.tmpl` do overlay, sem enforcement daqui.
