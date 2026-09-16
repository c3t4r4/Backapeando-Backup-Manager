# API — Backapeando

> Consultar antes de alterar qualquer API. Atualizar após qualquer alteração aprovada.

## Informações gerais

- Tipo: REST
- Versão: sem versionamento explícito (sem prefixo `/v1`)
- Base URL de desenvolvimento: `http://localhost:8081` (proxied em `http://localhost:3000/api` via Nginx do container `web`)
- Base URL de produção: mesma origem do frontend, via proxy reverso (Traefik/Nginx, gerido fora deste repositório)
- Formato: JSON
- Versionamento: `a definir` — sem estratégia hoje

## Autenticação

- Estratégia: sessão HTTP em cookie (`SESSION_COOKIE_NAME`, padrão `backapeando_backup_session`), `HttpOnly+Secure+SameSite=Strict`
- Header ou mecanismo: cookie de sessão + `X-CSRF-Token` (double-submit) em toda requisição mutante
- Expiração: idle TTL configurável (`SESSION_IDLE_TTL`, padrão 12h), teto absoluto (`SESSION_ABSOLUTE_TTL`, padrão 7 dias)
- Refresh token: não — sessão desliza (idle) até o teto absoluto

Rotas isentas de autenticação/CSRF (`publicPaths` em `router.go`): `GET /healthz`, `GET /api/health`, `POST /api/auth/login`.

## Formato de erro

```json
{
  "error": "mensagem"
}
```

`⚠ inferida`: o formato exato varia por handler (`writeError`) — não confirmado se todos seguem exatamente esta forma; verificar `internal/httpapi/handlers` antes de assumir um contrato genérico.

## Resumo de endpoints

| Método | Rota | Descrição | Auth | Regras | Status |
| --- | --- | --- | --- | --- | --- |
| GET | `/healthz` | Health check (Docker/Traefik) | Não | — | Ativo |
| GET | `/api/health` | Health check (idêntico) | Não | — | Ativo |
| POST | `/api/auth/login` | Login (rate limit via Redis) | Não | RT-001 | Ativo |
| POST | `/api/auth/logout` | Logout, revoga sessão no servidor | Sim | — | Ativo |
| GET | `/api/auth/me` | Sessão atual | Sim | — | Ativo |
| GET | `/api/servers` | Lista servidores | Sim | — | Ativo |
| POST | `/api/servers` | Cria servidor | Sim | RN-BACKUP-001, RN-BACKUP-017, RN-BACKUP-018 | Ativo |
| GET | `/api/servers/{id}` | Detalhe do servidor | Sim | — | Ativo |
| PUT | `/api/servers/{id}` | Atualiza servidor | Sim | RN-BACKUP-017, RN-BACKUP-018, RN-BACKUP-019 | Ativo |
| DELETE | `/api/servers/{id}` | Remove servidor | Sim | — | Ativo |
| POST | `/api/servers/{id}/test-connection` | Test-connection combinado (SSH+Ferramenta de Dump+Storage) | Sim | RN-BACKUP-005, RN-BACKUP-013 | Ativo |
| POST | `/api/servers/{id}/regenerate-key` | Gera novo par de chaves SSH | Sim | RN-BACKUP-001 | Ativo |
| POST | `/api/servers/{id}/reset-host-key` | Limpa fingerprint TOFU fixado | Sim | RN-BACKUP-015 | Ativo |
| POST | `/api/servers/{id}/enable` | Ativa o servidor manualmente (`enabled=true`) | Sim | RN-BACKUP-024 | Ativo |
| POST | `/api/servers/{id}/disable` | Desativa o servidor manualmente (`enabled=false`); só impede o agendamento automático | Sim | RN-BACKUP-024 | Ativo |
| GET | `/api/servers/{id}/retention-policy` | Política de retenção do servidor | Sim | RN-BACKUP-003 | Ativo |
| PUT | `/api/servers/{id}/retention-policy` | Atualiza política do servidor | Sim | RN-BACKUP-003, RN-BACKUP-004 | Ativo |
| DELETE | `/api/servers/{id}/retention-policy` | Remove política específica (volta ao default global) | Sim | RN-BACKUP-003 | Ativo |
| POST | `/api/servers/{id}/backup-now` | Dispara backup síncrono | Sim | — | Ativo |
| POST | `/api/servers/{id}/run-now` | Enfileira backup manual (`status=queued`) | Sim | RN-BACKUP-014 | Ativo |
| GET | `/api/servers/{id}/backup-runs` | Histórico paginado de execuções do servidor | Sim | RN-BACKUP-025 | Ativo |
| GET | `/api/backup-runs` | Histórico paginado de execuções de todos os servidores (ou de um, via `?serverId=`) | Sim | RN-BACKUP-026 | Ativo |
| GET | `/api/storage-targets` | Lista destinos de armazenamento | Sim | — | Ativo |
| POST | `/api/storage-targets` | Cria destino | Sim | RN-STORAGE-001 | Ativo |
| GET | `/api/storage-targets/{id}` | Detalhe do destino | Sim | — | Ativo |
| PUT | `/api/storage-targets/{id}` | Atualiza destino | Sim | RN-STORAGE-001 | Ativo |
| DELETE | `/api/storage-targets/{id}` | Remove destino | Sim | — | Ativo |
| GET | `/api/retention-policy/default` | Política de retenção global | Sim | RN-BACKUP-003 | Ativo |
| PUT | `/api/retention-policy/default` | Atualiza política global | Sim | RN-BACKUP-003, RN-BACKUP-004 | Ativo |
| GET | `/api/dashboard/summary` | Resumo agregado para o dashboard | Sim | — | Ativo |
| GET | `/api/dashboard/backup-stats` | Backups e bytes por destino, diário (30 dias) e mensal (ano corrente) | Sim | RN-BACKUP-031 | Ativo |
| GET | `/api/admin-users` | Lista administradores (nunca inclui `password_hash`) | Sim | RN-AUTH-001 | Ativo |
| POST | `/api/admin-users` | Cria administrador (senha ≥ 12 caracteres) | Sim | RN-AUTH-001 | Ativo |
| GET | `/api/admin-users/{id}` | Detalhe do administrador | Sim | RN-AUTH-001 | Ativo |
| PUT | `/api/admin-users/{id}` | Atualiza e-mail/CPF; senha opcional (vazio mantém a atual) | Sim | RN-AUTH-001 | Ativo |
| DELETE | `/api/admin-users/{id}` | Remove administrador — `409` se autoexclusão ou último admin | Sim | RN-AUTH-002, RN-AUTH-003 | Ativo |

Fonte: `backend/internal/httpapi/router.go`. Path params (`{id}`) via `net/http` ServeMux (Go 1.22+).

`POST`/`PUT /api/admin-users*` retornam `409 Conflict` para e-mail ou CPF duplicado (`{"error": "email already registered"}` / `{"error": "cpf already registered"}`), mapeados a partir da violação de constraint única do Postgres (`admin_users_email_key`/`admin_users_cpf_unique`).

### `POST`/`PUT /api/servers` — payload estendido (multi-engine + Docker-ou-host)

**Breaking change de contrato:** `containerName` deixou de ser sempre obrigatório — agora é obrigatório apenas quando `deploymentMode='docker'` (RN-BACKUP-018) e deve ser omitido/vazio quando `deploymentMode='host'`.

Campos novos no request (`upsertServerRequest`):

| Campo                | Tipo                                    | Obrigatório                                                              | Observação                                                                 |
| --------------------- | ---------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| `dbEngine`            | `'postgres' \| 'mysql' \| 'sqlserver'`   | Sim                                                                          | Motor do banco de dados                                                       |
| `deploymentMode`      | `'docker' \| 'host'`                    | Sim                                                                          | Onde o banco roda                                                             |
| `containerName`       | `string`                                 | Sim quando `deploymentMode='docker'`; deve ser vazio quando `'host'`         | RN-BACKUP-018                                                                  |
| `dbPassword`          | `string` (write-only)                   | Sim na criação quando `dbEngine != 'postgres'` (RN-BACKUP-017); opcional na edição | Vazio no `PUT` = mantém a senha atual (RN-BACKUP-019); nunca retornado pela API |
| `mysqlDumpExtraArgs`  | `string`                                 | Não                                                                          | Argumentos extras do `mysqldump`                                              |
| `sqlCmdExtraArgs`     | `string`                                 | Não                                                                          | Argumentos extras do `sqlcmd`                                                 |

Campos novos na resposta (`serverDTO`):

| Campo                | Tipo      | Observação                                                                 |
| --------------------- | ---------- | ----------------------------------------------------------------------------- |
| `dbEngine`            | `string`   | —                                                                              |
| `deploymentMode`      | `string`   | —                                                                              |
| `containerName`       | `string?`  | Ausente (`omitempty`) quando `deploymentMode='host'`                          |
| `hasDbPassword`       | `boolean`  | Nunca a senha em si — só indica se está configurada                          |
| `mysqlDumpExtraArgs`  | `string`   | —                                                                              |
| `sqlCmdExtraArgs`     | `string`   | —                                                                              |

`POST /api/servers/{id}/test-connection`: a chave `checks.docker` foi renomeada para `checks.dumpTool` (mesmo breaking change de contrato) — em modo `host` reflete um check de "binário de dump presente no `PATH` do host remoto" em vez de "container Docker rodando". Em modo `docker`, o check agora resolve `containerName` como padrão parcial via `docker ps | grep` (RN-BACKUP-023) em vez de exigir o nome exato.

`containerName` em modo `docker` (`POST`/`PUT /api/servers`): deixou de exigir o nome exato do container — é tratado como padrão parcial e resolvido via `docker ps --format '{{.Names}}' | grep` a cada execução (test-connection, scheduler e `backup-now`). Zero ou múltiplos containers correspondentes é erro explícito (RN-BACKUP-023).

### `GET /api/servers/{id}/backup-runs` — paginação e filtro por status (RN-BACKUP-025)

**Breaking change de contrato:** a resposta deixou de ser um array solto (`BackupRunDTO[]`) e passou a ser um envelope paginado. Primeiro endpoint do projeto a usar parâmetros de query string — convenção estabelecida aqui vale para endpoints futuros que precisem de paginação.

Parâmetros de query string (todos opcionais):

| Parâmetro  | Tipo                                            | Default | Observação                                                        |
| ---------- | ------------------------------------------------ | ------- | ------------------------------------------------------------------- |
| `page`     | inteiro ≥ 1                                      | `1`     | `400 Bad Request` se `<1` ou não numérico                          |
| `pageSize` | inteiro entre 1 e 200                            | `50`    | `400 Bad Request` fora do intervalo                                |
| `status`   | `queued` \| `running` \| `success` \| `failed`   | nenhum (todos os status) | `400 Bad Request` se o valor não for um dos quatro     |

Corpo da resposta:

```json
{
  "items": [ /* BackupRunDTO[] da página atual */ ],
  "total": 137,
  "page": 1,
  "pageSize": 50
}
```

`total` reflete o total de registros que casam com o filtro em todas as páginas, não `items.length` — inclusive quando `page` está além da última página existente (nesse caso `items` vem vazio, mas `total` continua o valor real, não `0`).

### `GET /api/backup-runs` — histórico combinando todos os servidores (RN-BACKUP-026)

Endpoint novo, não aninhado sob `/api/servers/{id}` — o servidor é opcional, não faz parte do path. Mesmos parâmetros de `GET /api/servers/{id}/backup-runs` (`page`, `pageSize`, `status`), mais:

| Parâmetro  | Tipo   | Default                    | Observação                                                                 |
| ---------- | ------ | -------------------------- | --------------------------------------------------------------------------- |
| `serverId` | string | nenhum (todos os servidores) | Quando informado, `404 Not Found` se o servidor não existir — mesma checagem do endpoint aninhado |

Sem nenhum parâmetro: página 1, 50 registros, de todos os servidores, mais recentes primeiro — é o que a tela de Histórico (T-06) agora chama por padrão, em vez de não buscar nada até um servidor ser selecionado. Mesmo envelope de resposta `{items, total, page, pageSize}` do endpoint aninhado; `serverId` de cada item já vem no `BackupRunDTO` (campo `serverId`, já existente).

`GET /api/servers/{id}/backup-runs` continua existindo sem nenhuma mudança de contrato — este é um endpoint adicional, não uma substituição.

### `error_message`/`log_output` de `BackupRunDTO` — detalhe real de falha (RN-BACKUP-027)

Antes, `error_message` era sempre uma frase genérica fixa por etapa (ex.: "backup failed during upload or dump execution"), e `log_output` nunca era preenchido (coluna sempre `null` apesar de existir desde a migração inicial e já ser renderizada pelo frontend). Agora:

- `error_message` contém o erro real da etapa que falhou (ex.: `"dump: command exited 1: pg_dump: error: connection to database failed"`), truncado a 2000 caracteres.
- `log_output` contém o stdout/stderr capturado do comando remoto quando disponível (dump, `BACKUP DATABASE` do SQL Server), truncado a 8000 caracteres.
- Ambos os campos têm a senha do banco (quando configurada) substituída por `***` antes de persistir — nunca vazam o valor real, mesmo que apareça no texto do erro/log.

Nenhuma mudança de schema JSON (os dois campos já existiam no DTO) — apenas o conteúdo passou a ser útil.

### `GET /api/dashboard/backup-stats` — backups e bytes por destino (RN-BACKUP-031)

Endpoint novo, consumido pelos 4 gráficos de barras empilhadas do Dashboard (T-02): contagem de backups e soma de `blob_size_bytes`, agrupados por destino de armazenamento, em duas janelas fixas — diária dos últimos 30 dias e mensal do ano corrente (sempre os 12 meses, meses futuros zero-preenchidos). Sem parâmetros de query.

```json
{
  "destinations": [
    { "storageTargetId": "uuid-1", "name": "Azure Produção" },
    { "storageTargetId": "uuid-2", "name": "S3 Backup Externo" },
    { "storageTargetId": null, "name": "Sem destino" }
  ],
  "dailyLast30Days": {
    "since": "2026-08-18T00:00:00Z",
    "until": "2026-09-17T00:00:00Z",
    "labels": ["2026-08-18", "...", "2026-09-16"],
    "countSeries": [{ "storageTargetId": "uuid-1", "data": [3, 2, 0] }],
    "bytesSeries": [{ "storageTargetId": "uuid-1", "data": [104857600, 0, 0] }]
  },
  "monthlyThisYear": {
    "year": 2026,
    "labels": ["2026-01", "...", "2026-12"],
    "countSeries": ["..."],
    "bytesSeries": ["..."]
  }
}
```

- `destinations` é a fonte única de ordem/legenda para as 4 séries: todo `storage_targets` (ordenado por `name`) mais uma entrada sintética final `{storageTargetId: null, name: "Sem destino"}`, sempre presente mesmo sem nenhum run sem destino.
- Toda série (`countSeries`/`bytesSeries`) tem exatamente `len(destinations)` entradas, na mesma ordem, com `data` do mesmo tamanho de `labels`, zero-preenchido — nunca é preciso casar arrays por busca.
- `monthlyThisYear.labels` sempre tem as 12 entradas de janeiro a dezembro do ano corrente, mesmo que meses futuros ainda não tenham dados (aparecem zerados).
- **Destino refletido é o ATUAL do servidor** (`servers.storage_target_id`), não o destino real em vigor no momento de cada execução passada — ver RN-BACKUP-031. Se um servidor trocar de destino, seu histórico "migra" visualmente para o novo destino nos 4 gráficos.

## Middlewares aplicados (ordem real)

CORS (mais externo) → Recover → Logging → SecurityHeaders → [RequireCSRF + RequireAuth, com bypass para rotas públicas] → mux — ver `docs/Arquitetura.md` e `router.go`.

## APIs externas

| Serviço | Base URL | Autenticação | Timeout | Retry | Observações |
| --- | --- | --- | --- | --- | --- |
| Azure Blob Storage | por `StorageTarget` (SAS URL) | SAS token | `a definir` | `a definir` | Usado apenas quando `StorageTarget.Type=azure` |
| S3 / compatível | por `StorageTarget` (`s3_endpoint`) | Access key + secret | `a definir` | `a definir` | `s3_use_path_style` suporta MinIO e afins |

## Regras de alteração

Antes de alterar:

1. Consultar este arquivo.
2. Consultar `docs/RegrasNegocio.md` se a alteração muda comportamento.
3. Procurar endpoint equivalente.
4. Avaliar compatibilidade.
5. Implementar.
6. Atualizar este arquivo.
7. Atualizar as regras de negócio afetadas.
8. Atualizar testes.
9. Executar Validator.
10. Executar Tester.
11. Executar Security Specialist.
12. Atualizar o histórico.

## Histórico

| Data | Endpoint | Alteração | Compatibilidade | Motivo |
| --- | --- | --- | --- | --- |
| 2026-09-15 | todos | Documento criado do zero a partir de `router.go`, ausente antes desta tarefa | n/a | `/init-project --update` |
| 2026-09-15 | `/api/admin-users`, `/api/admin-users/{id}` (GET/POST/GET/PUT/DELETE) | Rotas novas — CRUD de administradores via web, antes só existia via CLI | Aditiva (novos endpoints, nenhum contrato existente mudou) | CRUD de usuário administrador solicitado pelo usuário |
| 2026-09-15 | `POST`/`PUT /api/servers`, `POST /api/servers/{id}/test-connection` | Payload ganha `dbEngine`, `deploymentMode`, `dbPassword`, `mysqlDumpExtraArgs`, `sqlCmdExtraArgs`, `hasDbPassword`; `containerName` deixa de ser sempre obrigatório; `checks.docker` renomeado para `checks.dumpTool` | **Breaking** (`containerName` obrigatório condicional, `checks.docker`→`checks.dumpTool`) | Suporte a MySQL/SQL Server e a rodar o banco direto no host (sem Docker) |
| 2026-09-15 | `GET /api/servers/{id}/backup-runs` | Resposta vira envelope paginado `{items, total, page, pageSize}` (era array solto); novos parâmetros de query `page`/`pageSize`/`status` | **Breaking** (formato da resposta muda) | Histórico crescia sem limite; usuário pediu paginação de 50/página e filtro por status |
| 2026-09-15 | `POST /api/servers/{id}/enable`, `POST /api/servers/{id}/disable` | Rotas novas — ativação/desativação manual do servidor via `enabled` | Aditiva | Usuário pediu opção de desativar um servidor manualmente |
| 2026-09-15 | `POST`/`PUT /api/servers`, `POST /api/servers/{id}/test-connection` | `containerName` em modo `docker` passa a ser resolvido como padrão parcial (`docker ps \| grep`) em vez de exigir o nome exato | Aditiva (mesmo campo, comportamento de resolução mais permissivo) | Usuário só sabe/informa parte do nome real do container |
| 2026-09-16 | `GET /api/backup-runs` | Endpoint novo — histórico combinando todos os servidores, mesmos parâmetros de paginação/status do endpoint aninhado, mais `serverId` opcional | Aditiva (endpoint novo, `GET /api/servers/{id}/backup-runs` inalterado) | Tela de Histórico não mostrava nada até um servidor ser selecionado; usuário pediu default de últimos 50 de todos os servidores |
| 2026-09-16 | `BackupRunDTO.errorMessage`/`.logOutput` (todos os endpoints que retornam `BackupRunDTO`) | Conteúdo passa a ser o erro real da etapa que falhou (antes: frase genérica fixa) e o stdout/stderr capturado do comando remoto (antes: sempre `null`), ambos redigidos da senha do banco e truncados | Aditiva (nenhuma mudança de schema JSON, só do conteúdo) | Log de erro de backup não trazia detalhe suficiente para diagnosticar a causa da falha |
| 2026-09-16 | `PUT /api/servers/{id}` | Quando `cronExpression` muda em um servidor já agendado (`nextRunAt` já preenchido), `nextRunAt` na resposta passa a refletir o novo horário imediatamente, em vez de manter o valor calculado a partir do cron anterior até o próximo ciclo do scheduler | Aditiva (nenhum campo novo de payload/resposta, só o valor de `nextRunAt` muda de comportamento) | Achado do Validator: editar cron de servidor já agendado não recalculava `nextRunAt` até a próxima claim (ver `docs/RegrasNegocio.md` RN-BACKUP-030) |
| 2026-09-16 | `GET /api/dashboard/backup-stats` (novo); `GET /api/dashboard/summary` perde o campo `recentRuns` | Endpoint novo com backups/bytes por destino (diário 30 dias + mensal ano corrente); `recentRuns` removido do summary (só alimentava o gráfico de linha antigo, agora substituído) | **Breaking** para `recentRuns` (campo removido de `/api/dashboard/summary`); aditiva para o novo endpoint | Dashboard trocou o gráfico de tamanho/duração por execução por 4 gráficos de barras empilhadas por destino, pedido pelo usuário (ver `docs/RegrasNegocio.md` RN-BACKUP-031) |
