---
name: skill-nestjs-patterns
description: |
  Use esta skill ao trabalhar com NestJS: modules, DI, decorators, pipes,
  guards, interceptors e exception filters.
model: sonnet
---

# NestJS — padrões

## Objetivo

Reunir padrões de NestJS para modules, dependency injection, controllers, pipes, guards, interceptors e exception filters.

## Quando usar

- Ao criar ou modificar modules e providers.
- Ao configurar DI customizada.
- Ao implementar controllers com validação.
- Ao adicionar guards de autenticação/autorização.
- Ao criar interceptors ou exception filters.

## Module structure

### Feature module

```typescript
@Module({
  imports: [TypeOrmModule.forFeature([DocumentEntity])],
  controllers: [DocumentController],
  providers: [DocumentService, DocumentRepository],
  exports: [DocumentService],
})
export class DocumentModule {}
```

### Shared module

```typescript
@Module({
  providers: [RedisService, CacheService],
  exports: [RedisService, CacheService],
})
export class SharedModule {}
```

Prefira exportar apenas o necessário — não exponha providers internos que outros modules não precisam.

### Regras de imports

| Regra | Motivo |
|-------|--------|
| Feature module importa SharedModule | Reutilizar serviços comuns |
| Evitar imports circulares | Usar `forwardRef()` apenas como último recurso |
| `forFeature()` no feature module | Registrar entities/repos por módulo |
| `forRoot()` apenas no AppModule | Config global (DB, cache, queue) |

## Dependency injection

### Provider básico

```typescript
@Injectable()
export class DocumentService {
  constructor(
    private readonly documentRepo: DocumentRepository,
    private readonly cacheService: CacheService,
  ) {}
}
```

### Custom providers

```typescript
// useFactory: quando precisa de lógica na criação
{
  provide: 'OPENAI_CLIENT',
  useFactory: (configService: ConfigService) => {
    return new OpenAI({ apiKey: configService.get('OPENAI_API_KEY') })
  },
  inject: [ConfigService],
}

// useValue: para constantes
{ provide: 'APP_VERSION', useValue: '2.0.0' }

// useClass: para trocar implementação
{ provide: DocumentRepository, useClass: TypeOrmDocumentRepository }
```

### Injetar com token

```typescript
@Injectable()
export class AiService {
  constructor(
    @Inject('OPENAI_CLIENT') private readonly openai: OpenAI,
  ) {}
}
```

Services com muitas dependências (como heurística, 5+) tendem a acumular responsabilidades — considere dividir.

## Controllers

### Estrutura básica

```typescript
@Controller('documents')
export class DocumentController {
  constructor(private readonly documentService: DocumentService) {}

  @Get()
  findAll(@Query() query: PaginationDto): Promise<PaginatedResult<DocumentDto>> {
    return this.documentService.findAll(query)
  }

  @Get(':id')
  findOne(@Param('id', ParseUUIDPipe) id: string): Promise<DocumentDto> {
    return this.documentService.findOne(id)
  }

  @Post()
  @UseGuards(AuthGuard)
  create(@Body() dto: CreateDocumentDto): Promise<DocumentDto> {
    return this.documentService.create(dto)
  }
}
```

Em geral evite `@Res()` diretamente — usá-lo desabilita interceptors e serialização. Prefira retornar o valor e deixar o framework cuidar.

## Pipes (validação)

### ValidationPipe global

```typescript
// main.ts
app.useGlobalPipes(
  new ValidationPipe({
    whitelist: true,
    forbidNonWhitelisted: true,
    transform: true,
  }),
)
```

### DTOs com class-validator

```typescript
import { IsString, IsUUID, IsOptional, MaxLength } from 'class-validator'

export class CreateDocumentDto {
  @IsString()
  @MaxLength(255)
  title: string

  @IsUUID()
  templateId: string

  @IsOptional()
  @IsString()
  description?: string
}
```

### Custom pipe

```typescript
@Injectable()
export class ParseDatePipe implements PipeTransform<string, Date> {
  transform(value: string): Date {
    const date = new Date(value)
    if (isNaN(date.getTime())) {
      throw new BadRequestException('Data inválida')
    }
    return date
  }
}
```

## Guards

### AuthGuard (JWT)

```typescript
@Injectable()
export class AuthGuard implements CanActivate {
  constructor(private readonly jwtService: JwtService) {}

  async canActivate(context: ExecutionContext): Promise<boolean> {
    const request = context.switchToHttp().getRequest()
    const token = this.extractToken(request)

    if (!token) throw new UnauthorizedException()

    try {
      request.user = await this.jwtService.verifyAsync(token)
      return true
    } catch {
      throw new UnauthorizedException()
    }
  }

  private extractToken(req: Request): string | null {
    return req.cookies?.token
      ?? req.headers.authorization?.replace('Bearer ', '')
      ?? null
  }
}
```

### Role-based guard

```typescript
@Injectable()
export class EditorGuard implements CanActivate {
  canActivate(context: ExecutionContext): boolean {
    const { user } = context.switchToHttp().getRequest()
    return user?.role === 'editor' || user?.role === 'admin'
  }
}

// Uso
@Post()
@UseGuards(AuthGuard, EditorGuard)
create(@Body() dto: CreateDocumentDto) { /* ... */ }
```

## Interceptors

### Logging

```typescript
@Injectable()
export class LoggingInterceptor implements NestInterceptor {
  intercept(context: ExecutionContext, next: CallHandler): Observable<unknown> {
    const req = context.switchToHttp().getRequest()
    const now = Date.now()

    return next.handle().pipe(
      tap(() => {
        logger.info(`${req.method} ${req.url} - ${Date.now() - now}ms`)
      }),
    )
  }
}
```

### Transform response

```typescript
@Injectable()
export class WrapResponseInterceptor implements NestInterceptor {
  intercept(_ctx: ExecutionContext, next: CallHandler) {
    return next.handle().pipe(
      map((data) => ({ success: true, data })),
    )
  }
}
```

## Exception filters

### Filter global

```typescript
@Catch()
export class AllExceptionsFilter implements ExceptionFilter {
  catch(exception: unknown, host: ArgumentsHost) {
    const ctx = host.switchToHttp()
    const response = ctx.getResponse()

    const status = exception instanceof HttpException
      ? exception.getStatus()
      : HttpStatus.INTERNAL_SERVER_ERROR

    const message = exception instanceof HttpException
      ? exception.message
      : 'Erro interno do servidor'

    response.status(status).json({
      success: false,
      error: { status, message },
    })
  }
}
```

### Exceções built-in mais comuns

| Exceção | Status | Quando usar |
|---------|--------|-------------|
| `BadRequestException` | 400 | Validação falhou |
| `UnauthorizedException` | 401 | Token inválido/ausente |
| `ForbiddenException` | 403 | Sem permissão |
| `NotFoundException` | 404 | Recurso não encontrado |
| `ConflictException` | 409 | Duplicidade |
| `UnprocessableEntityException` | 422 | Regra de negócio violada |

## Anti-patterns

| Anti-pattern | Preferir |
|--------------|----------|
| Service com muitas dependências | Dividir em services menores |
| `@Res()` no controller | Retornar valor, deixar o framework serializar |
| Dependência circular | Extrair para module compartilhado |
| Lógica de negócio no controller | Mover para service |
| Guard acessando banco diretamente | Guard chama service que acessa banco |
| `forwardRef()` excessivo | Redesenhar as dependências |

## Recursos comuns (referência)

Pacotes frequentemente usados no ecossistema NestJS:

- `@nestjs/swagger` — documentação automática de API.
- `@nestjs/throttler` — rate limiting por rota.
- `@nestjs/websockets` — WebSockets (Socket.IO, etc.).
- ConfigModule com validação via Joi/Zod.
- SWC como alternativa ao `tsc` para builds mais rápidos.
- ESLint 9 com flat config.

### Pipeline module (exemplo)

Registro de steps via token:

```typescript
// step-providers.ts
export const STEP_PROVIDERS = 'STEP_PROVIDERS'

// pipeline.module.ts
@Module({
  providers: [
    PipelineEngine,
    {
      provide: STEP_PROVIDERS,
      useFactory: (...steps: PipelineStep[]) => steps,
      inject: [ExtractStep, GenerateStep, ReviewStep],
    },
  ],
})
export class PipelineModule {}
```

Steps implementam uma interface comum:

```typescript
export interface PipelineStep {
  name: string
  execute(ctx: PipelineCtx): Promise<PipelineCtx>
}
```

## Checklist

- [ ] Module com `imports`, `providers`, `exports` bem definidos.
- [ ] Services com quantidade razoável de dependências injetadas.
- [ ] Controllers sem lógica de negócio (delegam para services).
- [ ] DTOs validados com class-validator + ValidationPipe global.
- [ ] Guards para autenticação e autorização.
- [ ] Evitar `@Res()` diretamente.
- [ ] Exception filter global para padronizar erros.
- [ ] Sem dependências circulares (redesenhar quando preciso).
- [ ] `forRoot()` no AppModule, `forFeature()` nos feature modules.
