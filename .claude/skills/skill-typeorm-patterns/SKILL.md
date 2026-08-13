---
name: skill-typeorm-patterns
description: |
  Use esta skill ao trabalhar com TypeORM: entities, repositories, migrations,
  query builder e relations.
model: sonnet
---

# TypeORM — padrões

## Objetivo

Padronizar o uso de TypeORM: entities, repository pattern, query builder, migrations e performance.

## Quando usar

- Ao criar ou modificar entities/tabelas.
- Ao implementar repositories na camada de infra.
- Ao escrever queries complexas com QueryBuilder.
- Ao criar ou revisar migrations.
- Ao investigar problemas de performance.

## Entities

### Estrutura base

```typescript
@Entity('users')
export class UserEntity {
  @PrimaryGeneratedColumn('uuid')
  id: string

  @Column({ type: 'varchar', length: 255 })
  name: string

  @Column({ type: 'varchar', unique: true })
  email: string

  @Column({ type: 'enum', enum: UserRole, default: UserRole.USER })
  role: UserRole

  @CreateDateColumn()
  createdAt: Date

  @UpdateDateColumn()
  updatedAt: Date
}
```

### Colunas JSONB

```typescript
@Column({ type: 'jsonb', default: {} })
metadata: Record<string, unknown>
```

### Relations

```typescript
@ManyToOne(() => OrganizationEntity, (org) => org.users)
@JoinColumn({ name: 'organization_id' })
organization: OrganizationEntity

@OneToMany(() => DocumentEntity, (doc) => doc.user)
documents: DocumentEntity[]
```

Prefira definir `@JoinColumn` explicitamente em `@ManyToOne` em vez de confiar em nomes gerados automaticamente.

### Colunas enum

```typescript
export enum DocumentStatus {
  DRAFT = 'draft',
  PROCESSING = 'processing',
  COMPLETED = 'completed',
  FAILED = 'failed',
}

@Column({ type: 'enum', enum: DocumentStatus })
status: DocumentStatus
```

## Repository pattern

### Interface no domínio

```typescript
export interface UserRepository {
  findById(id: string): Promise<User | null>
  findByEmail(email: string): Promise<User | null>
  save(user: User): Promise<User>
  delete(id: string): Promise<void>
}
```

### Implementação na infra

```typescript
export class TypeOrmUserRepository implements UserRepository {
  private readonly repo: Repository<UserEntity>

  constructor(dataSource: DataSource) {
    this.repo = dataSource.getRepository(UserEntity)
  }

  async findById(id: string): Promise<User | null> {
    const entity = await this.repo.findOne({ where: { id } })
    return entity ? this.toDomain(entity) : null
  }
}
```

O contrato do repositório vive no domínio; a implementação com TypeORM fica em infra. A camada de domínio, em geral, não deve importar TypeORM.

## Query Builder vs Find

| Cenário | Usar |
|---------|------|
| Busca simples por campo | `find()` / `findOne()` |
| Filtros combinados simples | `find({ where: { ... } })` |
| JOINs complexos | QueryBuilder |
| Subqueries | QueryBuilder |
| Agregações (COUNT, SUM) | QueryBuilder |
| Paginação com filtros | QueryBuilder |

### Exemplo

```typescript
const results = await this.repo
  .createQueryBuilder('doc')
  .leftJoinAndSelect('doc.user', 'user')
  .where('doc.status = :status', { status: 'completed' })
  .andWhere('doc.createdAt >= :since', { since })
  .orderBy('doc.createdAt', 'DESC')
  .skip(offset)
  .take(limit)
  .getMany()
```

Use parâmetros nomeados (`:status`) em vez de concatenar strings.

## Migrations

### Comandos CLI

```bash
# Gerar migration a partir de diferenças nas entities
npx typeorm migration:generate -d src/infra/database/data-source.ts src/infra/database/migrations/AddUserRole

# Criar migration vazia (scripts manuais)
npx typeorm migration:create src/infra/database/migrations/SeedDefaultRoles

# Executar migrations pendentes
npx typeorm migration:run -d src/infra/database/data-source.ts
```

### Práticas seguras

| Recomendação | Motivo |
|--------------|--------|
| Evitar `synchronize: true` em produção | Risco de perda de dados |
| Evitar dropar colunas sem backfill | Dados podem ser perdidos |
| Criar migration reversível (`up` + `down`) | Rollback seguro |
| Testar migration em staging antes de produção | Prevenir downtime |
| Nomear migrations com descrição clara | Rastreabilidade |

### Migration segura para remover coluna

```typescript
// 1) Primeira migration: tornar coluna nullable
public async up(queryRunner: QueryRunner): Promise<void> {
  await queryRunner.query(
    `ALTER TABLE "documents" ALTER COLUMN "legacy_field" DROP NOT NULL`
  )
}

// 2) Deploy e confirmação de que nenhum código usa a coluna
// 3) Segunda migration: remover a coluna
```

## Performance

### Selecionar colunas específicas

```typescript
// Seleciona apenas o necessário
const users = await repo.find({
  select: ['id', 'name', 'email'],
  where: { organizationId },
})

// Carrega todas as colunas (inclui JSONBs pesados)
const users = await repo.find({ where: { organizationId } })
```

### Evitar N+1

```typescript
// JOIN na query
const docs = await repo.find({
  where: { userId },
  relations: ['template', 'organization'],
})

// Fetch das relations em loop
for (const doc of docs) {
  doc.template = await templateRepo.findOne({ where: { id: doc.templateId } })
}
```

### Índices

```typescript
@Entity('documents')
@Index(['organizationId', 'status'])
@Index(['userId', 'createdAt'])
export class DocumentEntity {
  // ...
}
```

### Paginação

```typescript
const [items, total] = await repo.findAndCount({
  where: { organizationId },
  order: { createdAt: 'DESC' },
  skip: (page - 1) * limit,
  take: limit,
})
```

## Anti-patterns

| Anti-pattern | Preferir |
|--------------|----------|
| `synchronize: true` em produção | Migrations |
| SQL sem parametrização | `:param` ou `$1` |
| `save()` sem validação prévia | Validar com Zod/class-validator antes |
| Eager loading global | `relations` por query |
| Entity como DTO de resposta | Mapear para DTO/modelo de domínio |

## Variações comuns por projeto

- **Separar entity de domínio**: entities PG com prefixo (`PgUserEntity`) separadas das entidades de domínio, mais mapeamento explícito em repositórios.
- **Base class de repositório**: `PostgresRepository<T>` com métodos genéricos (`findById`, `save`, `delete`).
- **Multi-DB**: TypeORM para relacional e outra tecnologia (ex.: Mongoose) para documentos.
- **NestJS**: `@InjectRepository()` direto e entities servindo como modelo.
- **Colunas JSONB como configuração**: guardar definições de pipeline/steps em colunas JSONB.

Adapte ao padrão do projeto.

## Checklist

- [ ] Entity com decorators corretos (`@Entity`, `@Column`, `@PrimaryGeneratedColumn`).
- [ ] Relations com `@JoinColumn` explícito.
- [ ] Contrato de repositório no domínio, implementação na infra.
- [ ] Queries complexas com QueryBuilder e parâmetros nomeados.
- [ ] Migration reversível (up + down).
- [ ] Sem `synchronize: true` em produção.
- [ ] Select específico em tabelas com colunas pesadas.
- [ ] Índices para colunas de WHERE/ORDER BY frequentes.
- [ ] Paginação em listagens.
