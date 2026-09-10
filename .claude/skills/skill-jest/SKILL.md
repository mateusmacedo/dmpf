---
name: skill-jest
description: >
  Use esta skill quando o usuário pedir para "configurar Jest", "mock centralizado",
  "teste de use case", "teste de controller", "setup de teste", "jest.config",
  ou mencionar Jest, jest-mock-extended, pg-mem, matchers, mocks de módulo,
  timers fake, async testing.
  Cobre setup do Jest em backend (ts-jest, node), APIs e matchers, testes de use cases,
  controllers e repositórios (pg-mem), mock patterns (jest-mock-extended, manual mocks,
  module mocking) e async testing.
  Para testes E2E de API, ver uso direto de supertest.
model: opus
---

# Jest (backend)

## Objetivo

Testes com Jest no backend: setup, APIs, testes de use cases/controllers/repositórios e padrões de mock.

## Quando usar

- Ao configurar Jest em um projeto backend.
- Ao testar use cases, controllers ou repositórios.
- Ao criar mocks para dependências (repos, serviços externos, filas).

## Setup

### Configuração base

```typescript
// jest.config.ts
export default {
  preset: 'ts-jest',
  testEnvironment: 'node',
  moduleNameMapper: { '@/(.*)': '<rootDir>/src/$1' },
  clearMocks: true,
  collectCoverageFrom: [
    'src/**/*.ts',
    '!src/**/*.d.ts',
    '!src/**/index.ts',
  ],
}
```

Variações comuns:

- Projetos com testes em pasta separada (`tests/`) mantêm um `jest.config.ts` na raiz.
- Projetos NestJS costumam colocar testes colocalizados (`*.spec.ts`) e usar `@nestjs/testing` (`TestingModule`).

### Setup files

```typescript
// jest.setup.ts
process.env.TZ = 'UTC'           // timezone fixa para testes com datas
jest.setTimeout(10000)           // timeout maior para integração (opcional)
```

---

## Scripts (package.json)

```json
{
  "scripts": {
    "test": "jest",
    "test:watch": "jest --watch",
    "test:cov": "jest --coverage",
    "test:ci": "jest --ci --coverage --reporters=default"
  }
}
```

---

## APIs principais

### Estrutura de teste

```typescript
describe('[use-case] CreateUser', () => {
  let sut: CreateUserUseCase
  let userRepo: MockProxy<UserRepository>

  beforeEach(() => {
    userRepo = mock<UserRepository>()
    sut = setupCreateUser({ userRepo })
  })

  it('should create user with valid data', async () => {
    userRepo.save.mockResolvedValue(makeUser())
    const result = await sut.execute(makeCreateUserInput())
    expect(result).toEqual(expect.objectContaining({ id: expect.any(String) }))
  })
})
```

### Matchers comuns

```typescript
// Igualdade
expect(value).toBe(expected)           // estrita (===)
expect(value).toEqual(expected)        // profunda
expect(value).toStrictEqual(expected)  // profunda + verifica undefined

// Truthiness
expect(value).toBeTruthy()
expect(value).toBeFalsy()
expect(value).toBeNull()
expect(value).toBeUndefined()
expect(value).toBeDefined()

// Números
expect(value).toBeGreaterThan(n)
expect(value).toBeLessThan(n)
expect(value).toBeCloseTo(n, decimals)

// Strings
expect(value).toMatch(/regex/)
expect(value).toContain('substring')

// Arrays/Objects
expect(array).toContain(item)
expect(array).toHaveLength(n)
expect(object).toHaveProperty('key', value)

// Exceções
expect(() => fn()).toThrow()
expect(() => fn()).toThrow('message')
expect(() => fn()).toThrow(ErrorClass)

// Assíncrono
await expect(promise).resolves.toBe(value)
await expect(promise).rejects.toThrow()
```

### Mocks básicos

```typescript
// Função mock
const fn = jest.fn()
const fnWithReturn = jest.fn().mockReturnValue(42)
const asyncFn = jest.fn().mockResolvedValue(data)

// Verificações
expect(fn).toHaveBeenCalled()
expect(fn).toHaveBeenCalledWith(arg1, arg2)
expect(fn).toHaveBeenCalledTimes(n)
expect(fn).toHaveBeenLastCalledWith(arg)

// Spy
const spy = jest.spyOn(object, 'method')
spy.mockImplementation(() => 'mocked')

// Timers
jest.useFakeTimers()
jest.setSystemTime(new Date('2024-01-01'))
jest.advanceTimersByTime(1000)
jest.useRealTimers()
```

---

## Testes de use cases / services

### Com jest-mock-extended

```typescript
import { mock, MockProxy } from 'jest-mock-extended'

describe('[use-case] CreateUser', () => {
  let sut: CreateUserUseCase
  let userRepo: MockProxy<UserRepository>
  let hasher: MockProxy<PasswordHasher>

  beforeEach(() => {
    userRepo = mock<UserRepository>()
    hasher = mock<PasswordHasher>()
    sut = new CreateUserUseCase({ userRepo, hasher })
  })

  it('should create user with valid data', async () => {
    hasher.hash.mockResolvedValue('hashed_pw')
    userRepo.save.mockResolvedValue(makeUser())

    const result = await sut.execute(makeCreateUserInput())

    expect(userRepo.save).toHaveBeenCalledWith(
      expect.objectContaining({ email: 'john@test.com' })
    )
    expect(result.id).toBeDefined()
  })

  it('should throw when email already exists', async () => {
    userRepo.findByEmail.mockResolvedValue(makeUser())

    await expect(sut.execute(makeCreateUserInput()))
      .rejects.toThrow(EmailAlreadyExistsError)
  })
})
```

`mock<Interface>()` cria um mock tipado que implementa a interface inteira, com cada método já sendo `jest.fn()`.

---

## Testes de controllers

```typescript
describe('[controller] UserController', () => {
  let sut: UserController
  let createUser: MockProxy<CreateUserUseCase>

  beforeEach(() => {
    createUser = mock<CreateUserUseCase>()
    sut = new UserController({ createUser })
  })

  it('should return 201 on successful creation', async () => {
    createUser.execute.mockResolvedValue(makeUser())
    const req = makeFakeRequest({ body: makeCreateUserInput() })
    const res = makeFakeResponse()

    await sut.create(req, res)

    expect(res.status).toHaveBeenCalledWith(201)
    expect(res.json).toHaveBeenCalledWith(
      expect.objectContaining({ id: expect.any(String) })
    )
  })

  it('should return 400 on validation error', async () => {
    createUser.execute.mockRejectedValue(new ValidationError('Email inválido'))
    const req = makeFakeRequest({ body: {} })
    const res = makeFakeResponse()

    await sut.create(req, res)

    expect(res.status).toHaveBeenCalledWith(400)
  })
})
```

Em NestJS, prefira `Test.createTestingModule()` em vez de instanciar o controller manualmente:

```typescript
const module = await Test.createTestingModule({
  controllers: [UserController],
  providers: [
    { provide: CreateUserUseCase, useValue: mock<CreateUserUseCase>() },
  ],
}).compile()

const sut = module.get(UserController)
```

---

## Testes de repositório com pg-mem

### Setup (banco in-memory)

```typescript
import { newDb } from 'pg-mem'
import { DataSource } from 'typeorm'

let dataSource: DataSource

beforeAll(async () => {
  const db = newDb({ autoCreateForeignKeyIndices: true })
  dataSource = await db.adapters.createTypeormDataSource({
    type: 'postgres',
    entities: [UserEntity],
    synchronize: true,
  })
  await dataSource.initialize()
})

afterAll(async () => {
  await dataSource.destroy()
})

afterEach(async () => {
  await dataSource.getRepository(UserEntity).clear()
})
```

### Teste do repositório

```typescript
describe('[repo] TypeOrmUserRepo', () => {
  let sut: TypeOrmUserRepo

  beforeEach(() => {
    sut = new TypeOrmUserRepo(dataSource)
  })

  it('should save and find user by email', async () => {
    const user = makeUser({ email: 'test@example.com' })
    await sut.save(user)

    const found = await sut.findByEmail('test@example.com')

    expect(found).toEqual(expect.objectContaining({ email: 'test@example.com' }))
  })

  it('should return null when user not found', async () => {
    const found = await sut.findByEmail('nonexistent@example.com')
    expect(found).toBeNull()
  })
})
```

---

## Padrões de mock no backend

### jest-mock-extended — interfaces e contratos

```typescript
import { mock, MockProxy } from 'jest-mock-extended'

const repo: MockProxy<UserRepository> = mock<UserRepository>()
repo.save.mockResolvedValue(makeUser())
```

### Mocks manuais para serviços externos

```typescript
// __mocks__/s3Client.ts
export const s3Client = {
  upload: jest.fn().mockResolvedValue({ Location: 'https://s3/file.pdf' }),
  getObject: jest.fn().mockResolvedValue({ Body: Buffer.from('content') }),
  deleteObject: jest.fn().mockResolvedValue({}),
}
```

### `jest.mock` para módulos

```typescript
// Mock de DataSource do TypeORM
jest.mock('../database', () => ({
  dataSource: {
    getRepository: jest.fn().mockReturnValue({
      find: jest.fn(),
      save: jest.fn(),
    }),
  },
}))

// Mock de Redis client
jest.mock('../redis', () => ({
  redis: {
    get: jest.fn(),
    set: jest.fn(),
    del: jest.fn(),
  },
}))

// Mock parcial (preserva implementações reais)
jest.mock('../utils', () => ({
  ...jest.requireActual('../utils'),
  sendEmail: jest.fn(),
}))
```

### Testes dependentes de tempo

```typescript
beforeEach(() => {
  jest.useFakeTimers()
  jest.setSystemTime(new Date('2024-06-15T10:00:00Z'))
})

afterEach(() => {
  jest.useRealTimers()
})

it('should set expiration 30 days from now', () => {
  const result = createToken()
  expect(result.expiresAt).toEqual(new Date('2024-07-15T10:00:00Z'))
})
```

---

## Factories (helpers de teste)

### Padrão make + override

```typescript
// tests/factories/makeUser.ts
export const makeUser = (overrides: Partial<User> = {}): User => ({
  id: 'any-id',
  name: 'John Doe',
  email: 'john@test.com',
  createdAt: new Date('2024-01-01'),
  ...overrides,
})

export const makeCreateUserInput = (
  overrides: Partial<CreateUserDto> = {},
): CreateUserDto => ({
  name: 'John Doe',
  email: 'john@test.com',
  password: 'StrongP@ss1',
  ...overrides,
})
```

Uma factory por entidade/DTO, com defaults válidos customizáveis via `overrides`.

---

## Async testing

### Handlers de job

```typescript
describe('[worker] SendEmailWorker', () => {
  let sut: SendEmailWorker
  let emailService: MockProxy<EmailService>

  beforeEach(() => {
    emailService = mock<EmailService>()
    sut = new SendEmailWorker({ emailService })
  })

  it('should send email with correct data', async () => {
    const job = makeJob({ data: { to: 'user@test.com', subject: 'Hello' } })

    await sut.process(job)

    expect(emailService.send).toHaveBeenCalledWith(
      expect.objectContaining({ to: 'user@test.com' })
    )
  })

  it('should throw on invalid email', async () => {
    const job = makeJob({ data: { to: '', subject: 'Hello' } })
    await expect(sut.process(job)).rejects.toThrow(ValidationError)
  })
})
```

### Transações

```typescript
it('should rollback on error', async () => {
  userRepo.save.mockRejectedValue(new Error('DB error'))

  await expect(sut.execute(input)).rejects.toThrow('DB error')

  // efeitos colaterais não devem ocorrer
  expect(emailService.send).not.toHaveBeenCalled()
})
```

---

## Nomenclatura sugerida

| Tipo de teste | describe | Exemplo de it |
| -------------- | ---------- | --------------- |
| Use case | `[use-case] CreateUser` | `should create user with valid data` |
| Controller | `[controller] UserController` | `should return 201 on success` |
| Repositório | `[repo] TypeOrmUserRepo` | `should find user by email` |
| Worker/Consumer | `[worker] SendEmailWorker` | `should send email with correct data` |
| Service | `[service] TokenService` | `should generate valid JWT` |
| Utilitário | `[util] formatDate` | `should format ISO to DD/MM/YYYY` |

---

## Checklist

- [ ] Jest configurado com `ts-jest` e `testEnvironment: 'node'`.
- [ ] Mocks limpos após cada teste (`clearMocks: true` ou `afterEach`).
- [ ] Use cases testados com jest-mock-extended para dependências.
- [ ] Controllers testados verificando status codes e response body.
- [ ] Factories (`make*`) criadas para entidades e DTOs comuns.
- [ ] Testes cobrem caso feliz e erros esperados.
- [ ] Integração de repositório preferencialmente com pg-mem (não banco real).
- [ ] Imports/aliases resolvidos no `moduleNameMapper`.
