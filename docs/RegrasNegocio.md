# Regras de Negócio — Backapeando

> **Documento de prioridade 3 na ordem de leitura obrigatória.**
> Memória viva de todas as regras de negócio do sistema, tela por tela, e de todas as regras de tomada de decisão na lógica das páginas e dos objetos.

Última atualização: 2026-09-16 (recálculo imediato de `next_run_at` ao editar cron de servidor já agendado)

---

## Como usar este documento

**Antes de alterar comportamento:**

1. Localizar a tela em `Índice de telas`.
2. Ler as regras daquela tela e as regras transversais aplicáveis.
3. Verificar se a mudança pedida contradiz alguma regra `confirmada`.
4. Se contradisser, **parar e perguntar** ao usuário antes de implementar.

**Depois de alterar comportamento:**

1. Criar ou atualizar a regra correspondente.
2. Atualizar a rastreabilidade (arquivo e teste).
3. Registrar no `Histórico`.
4. Uma tarefa que muda comportamento **não está concluída** com este documento desatualizado.

**Nunca apagar uma regra.** Regra que deixou de valer é marcada como `revogada`, com data e motivo.

### Status obrigatório em toda regra

| Status       | Significado                                                                                         |
| ------------ | --------------------------------------------------------------------------------------------------- |
| `confirmada` | Validada pelo usuário ou pelo negócio. Pode ser tratada como verdade.                               |
| `⚠ inferida` | Extraída do código, **aguardando confirmação**. Pode ser bug, não regra. Nunca tratar como verdade. |
| `a definir`  | Lacuna conhecida. O comportamento ainda não foi decidido.                                           |
| `revogada`   | Histórica. Manter com data e motivo da revogação.                                                   |

> **Nota desta criação (2026-09-15):** este documento não existia no
> repositório antes desta tarefa, embora o `CLAUDE.md` já citasse IDs
> `RN-BACKUP-001` a `RN-BACKUP-015` e `RN-AUTH-001` como se estivessem
> documentados e confirmados. Todas as regras abaixo foram reconstruídas a
> partir do código-fonte real e do texto do `CLAUDE.md` anterior, e entram
> como `⚠ inferida` até confirmação explícita do usuário — **nenhuma foi
> promovida a `confirmada` automaticamente**, mesmo as que o `CLAUDE.md`
> antigo descrevia com confiança.

### Convenção de identificadores

| Prefixo              | Uso               | Exemplo         |
| -------------------- | ----------------- | --------------- |
| `RN-<DOMINIO>-<NNN>` | Regra de negócio  | `RN-BACKUP-014` |
| `T-<NN>`             | Tela              | `T-07`          |
| `E-<NOME>`           | Entidade / objeto | `E-SERVER`      |
| `RT-<NNN>`           | Regra transversal | `RT-003`        |

---

## Glossário do domínio

| Termo              | Significado no negócio                                                                                | Onde aparece no código     |
| ------------------ | ----------------------------------------------------------------------------------------------------- | -------------------------- |
| Server             | Servidor PostgreSQL remoto, acessado via SSH, cujo backup é gerenciado                                | `domain.Server`            |
| Storage Target     | Destino de armazenamento do backup: Azure Blob, S3-compatível ou filesystem/NFS                       | `domain.StorageTarget`     |
| Retention Policy   | Política de retenção GFS (recentes + mensais), global ou por servidor                                 | `domain.RetentionPolicy`   |
| Backup Run         | Execução (histórica ou em andamento) de um backup                                                     | `domain.BackupRun`         |
| Retention Deletion | Registro de auditoria de uma exclusão feita pela varredura de retenção                                | `domain.RetentionDeletion` |
| TOFU               | Trust On First Use — fixação do fingerprint da chave de host SSH após a primeira conexão bem-sucedida | `internal/sshclient`       |
| GFS                | Grandfather-Father-Son — algoritmo de retenção: N backups recentes + 1 por mês nos últimos M meses    | `internal/retention`       |

---

## Índice de telas

| ID   | Tela                            | Rota                        | Arquivo principal                              | Papéis com acesso   | Regras                                                                                   | Status     |
| ---- | ------------------------------- | --------------------------- | ---------------------------------------------- | ------------------- | ---------------------------------------------------------------------------------------- | ---------- |
| T-01 | Login                           | `/login`                    | `frontend/src/views/LoginView.vue`             | público             | RN-AUTH-001, RT-001                                                                      | ⚠ inferida |
| T-02 | Dashboard                       | `/dashboard`                | `frontend/src/views/DashboardView.vue`         | admin (único papel) | RN-BACKUP-002                                                                            | ⚠ inferida |
| T-03 | Servidores                      | `/servers`                  | `frontend/src/views/ServersView.vue`           | admin               | RN-BACKUP-001, RN-BACKUP-002                                                             | ⚠ inferida |
| T-04 | Novo servidor                   | `/servers/new`              | `frontend/src/views/ServerNewView.vue`         | admin               | RN-BACKUP-001                                                                            | ⚠ inferida |
| T-05 | Editar servidor                 | `/servers/:id/edit`         | `frontend/src/views/ServerEditView.vue`        | admin               | RN-BACKUP-001, RN-BACKUP-005, RN-BACKUP-013, RN-BACKUP-015, RN-BACKUP-023, RN-BACKUP-024, RN-BACKUP-029 | ⚠ inferida |
| T-06 | Histórico                       | `/history`                  | `frontend/src/views/HistoryView.vue`           | admin               | RN-BACKUP-014, RN-BACKUP-016, RN-BACKUP-022, RN-BACKUP-025, RN-BACKUP-026, RN-BACKUP-027 | ⚠ inferida |
| T-07 | Destinos de armazenamento       | `/storage-targets`          | `frontend/src/views/StorageTargetsView.vue`    | admin               | RN-STORAGE-001                                                                           | ⚠ inferida |
| T-08 | Novo destino de armazenamento   | `/storage-targets/new`      | `frontend/src/views/StorageTargetNewView.vue`  | admin               | RN-STORAGE-001                                                                           | ⚠ inferida |
| T-09 | Editar destino de armazenamento | `/storage-targets/:id/edit` | `frontend/src/views/StorageTargetEditView.vue` | admin               | RN-STORAGE-001                                                                           | ⚠ inferida |
| T-10 | Configurações                   | `/settings`                 | `frontend/src/views/SettingsView.vue`          | admin               | RN-BACKUP-003, RN-BACKUP-004                                                             | ⚠ inferida |
| T-11 | Administradores                 | `/admin-users`              | `frontend/src/views/AdminUsersView.vue`        | admin               | RN-AUTH-001, RN-AUTH-002, RN-AUTH-003                                                    | confirmada |
| T-12 | Novo administrador              | `/admin-users/new`          | `frontend/src/views/AdminUserNewView.vue`      | admin               | RN-AUTH-001                                                                              | confirmada |
| T-13 | Editar administrador            | `/admin-users/:id/edit`     | `frontend/src/views/AdminUserEditView.vue`     | admin               | RN-AUTH-001, RN-AUTH-002, RN-AUTH-003                                                    | confirmada |

Papel único detectado: `admin` — sistema de operador único, sem múltiplos papéis (ver `docs/Auth.md`).

---

## Regras por tela

### T-05 — Editar servidor

- **Objetivo:** configurar um servidor PostgreSQL remoto, testar conectividade combinada, gerenciar chave SSH e política de retenção.
- **Rota:** `/servers/:id/edit`
- **Arquivo:** `frontend/src/views/ServerEditView.vue`
- **Acesso:** admin
- **Status:** ⚠ inferida

#### Ações disponíveis

| Ação                    | Condição de habilitação           | Papel exigido | Efeito                                                                                                                                                                                         | Regra                        |
| ----------------------- | --------------------------------- | ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------- |
| Testar conexão          | servidor possui par de chaves SSH | admin         | roda `checks.SSH` + `checks.Docker` + `checks.Storage` (se houver storage target); se todos OK, `status=ready`; se transição vinha de `awaiting_authorization`, `enabled=true` automaticamente | RN-BACKUP-005, RN-BACKUP-013 |
| Regenerar chave SSH     | sempre                            | admin         | gera novo par ed25519, volta `status=awaiting_authorization`, `enabled=false`                                                                                                                  | RN-BACKUP-001                |
| Resetar host key (TOFU) | sempre                            | admin         | limpa `SSHHostKeyFingerprint` fixado; próxima conexão fixa um novo                                                                                                                             | RN-BACKUP-015                |

---

## Tabelas de decisão

### RN-BACKUP-001 — Máquina de estados do servidor

- **Tela:** T-04, T-05
- **Status:** ⚠ inferida
- **Origem:** inferida do código (`domain.ServerStatus`, `server_ssh.go`)

| Condição                                                                                            | Resultado                                                                             | Fonte                                                 | Status     |
| --------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------- | ----------------------------------------------------- | ---------- |
| Servidor criado, sem chave SSH testada                                                              | `status=pending_key`                                                                  | `backend/internal/domain/domain.go:10`                | ⚠ inferida |
| Chave SSH gerada/regenerada                                                                         | `status=awaiting_authorization`, `enabled=false`                                      | `backend/internal/httpapi/handlers/server_ssh.go:254` | ⚠ inferida |
| Test-connection combinado passa (SSH + Ferramenta de Dump + Storage, se houver — ver RN-BACKUP-013) | `status=ready`                                                                        | `backend/internal/httpapi/handlers/server_ssh.go`     | ⚠ inferida |
| Test-connection falha em qualquer check                                                             | `status` e `enabled` inalterados; erro composto salvo em `last_test_connection_error` | `backend/internal/httpapi/handlers/server_ssh.go`     | ⚠ inferida |

- **Caso não previsto:** não há transição documentada para `disabled` a partir da UI — `⚠ inferida`, verificar se existe endpoint ou apenas o valor de enum existe sem uso.
- **Motivo da regra:** garantir que um servidor só entra em agendamento automático depois de comprovada conectividade real (SSH, ferramenta de dump disponível — container Docker rodando ou binário no host —, credenciais de storage válidas).

### RN-BACKUP-002 — Elegibilidade para agendamento

- **Tela:** T-02, T-03
- **Status:** ⚠ inferida
- **Origem:** inferida do código

| Condição                                                            | Resultado                                                               | Fonte                                             | Status     |
| ------------------------------------------------------------------- | ----------------------------------------------------------------------- | ------------------------------------------------- | ---------- |
| `enabled=true` AND `status=ready` AND `storage_target_id` não nulo  | servidor é reivindicado pelo scheduler (`GetReadyServersForScheduling`) | `backend/internal/scheduler/scheduler.go:745`     | ⚠ inferida |
| `storage_target_id` nulo, mesmo com `status=ready` e `enabled=true` | servidor pulado no ciclo de agendamento (log de warning)                | `backend/internal/scheduler/scheduler.go:785-790` | ⚠ inferida |

- **Motivo da regra:** um backup sem destino configurado não pode ser executado.

### RN-BACKUP-003 — Retenção GFS

- **Tela:** T-10
- **Status:** ⚠ inferida
- **Origem:** inferida do código (`internal/retention/retention.go`)

| Condição                                                                                                                            | Resultado                           | Fonte                                           | Status     |
| ----------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------- | ----------------------------------------------- | ---------- |
| Backup está entre os `RecentCount` mais recentes (ordenados por `LastModified` desc)                                                | mantido incondicionalmente          | `backend/internal/retention/retention.go:52-56` | ⚠ inferida |
| Backup fora dos recentes, mas é o mais recente do seu mês (`YYYY-MM`) e o mês está dentro de `MonthlyCount` meses contando de agora | mantido (1 por mês)                 | `backend/internal/retention/retention.go:57-68` | ⚠ inferida |
| Backup fora dos recentes e fora da janela mensal, ou mês já coberto                                                                 | excluído pela varredura de retenção | `backend/internal/retention/retention.go:69`    | ⚠ inferida |

- **Motivo da regra:** equilibrar custo de armazenamento com histórico suficiente para recuperação (padrão Grandfather-Father-Son).

### RN-BACKUP-004 — Contagens de retenção não podem ser ambas zero

- **Tela:** T-10
- **Status:** a definir
- **Origem:** citada no `CLAUDE.md` anterior; **não localizada validação correspondente no código lido nesta tarefa** (`internal/domain`, `internal/retention`, handlers de retention não foram auditados byte a byte para esta regra específica)
- **Caso não previsto:** se não houver validação real no backend, isto é uma regra `a definir`/pendente de implementação, não um fato confirmado.

### RN-BACKUP-005 — Auto-enable na primeira transição para ready

- **Tela:** T-05
- **Status:** ⚠ inferida
- **Origem:** inferida do código

| Condição                                                                       | Resultado                                                           | Fonte                                                     | Status     |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------- | --------------------------------------------------------- | ---------- |
| Test-connection combinado passa E `server.Status` era `awaiting_authorization` | `enabled` vira `true`                                               | `backend/internal/httpapi/handlers/server_ssh.go:168-172` | ⚠ inferida |
| Test-connection combinado passa mas `server.Status` já era `ready` (ou outro)  | `enabled` permanece no valor atual (não é forçado a `true` de novo) | `backend/internal/httpapi/handlers/server_ssh.go:165`     | ⚠ inferida |

- **Motivo da regra:** habilitar automaticamente apenas na "primeira autorização", sem reativar um servidor que o operador desabilitou manualmente depois.

### RN-BACKUP-008 — Shell-quoting de todo componente do comando remoto

- **Status:** ⚠ inferida
- **Origem:** inferida do código
- **Extensão nesta tarefa:** a regra, antes restrita a Postgres/Docker, agora cobre os três engines e os dois modos de implantação, incluindo a senha do banco como novo componente shell-quoted.

| Condição                                                                               | Resultado                                                                                                                                                                                                         | Fonte                                                                 | Status     |
| -------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------- | ---------- |
| Montagem do comando `pg_dump`/`mysqldump`/`sqlcmd`, com ou sem prefixo `docker exec`   | `ContainerName`, `DBUser`, `DBName`, cada token de `PgDumpExtraArgs`/`MySQLDumpExtraArgs`/`SqlCmdExtraArgs`, e o valor da senha do banco (decriptada) são individualmente shell-quoted via `sshclient.ShellQuote` | `backend/internal/scheduler/dumpcommand.go`                           | ⚠ inferida |
| Modo Docker: variável de ambiente da senha (`PGPASSWORD`/`MYSQL_PWD`/`SQLCMDPASSWORD`) | passada via `docker exec -e NOME=valor`, nunca como prefixo de atribuição de shell (que seria interpretado como argumento do próprio `docker exec`, não do processo dentro do container)                          | `backend/internal/scheduler/dumpcommand.go` (`assembleRemoteCommand`) | ⚠ inferida |

- **Motivo da regra:** `PgDumpExtraArgs`/`MySQLDumpExtraArgs`/`SqlCmdExtraArgs` são texto livre configurado pelo operador e executados remotamente via SSH — sem quoting, é injeção de comando. A senha do banco, embora não seja texto livre do operador, também precisa de quoting porque pode conter caracteres especiais.

### RN-BACKUP-013 — Test-connection combinado exige SSH + Ferramenta de Dump + Storage

- **Tela:** T-05
- **Status:** ⚠ inferida (generalizada nesta tarefa — antes cobria só Docker)
- **Origem:** inferida do código, comentário explícito no handler

| Condição                                                                                                                                                                                                                                                      | Resultado                                      | Fonte                                                                                | Status     |
| ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------- | ------------------------------------------------------------------------------------ | ---------- |
| SSH conecta E (`deploymentMode=docker`: container do nome configurado está `Running=true` \| `deploymentMode=host`: binário do engine — `pg_dump`/`mysqldump`/`sqlcmd` — resolve via `command -v` no host) E (sem storage target OU storage target acessível) | todos os checks OK → pode avançar para `ready` | `backend/internal/httpapi/handlers/server_ssh.go` (`checkDumpTool`, `dumpBinaryFor`) | ⚠ inferida |
| Qualquer check falha                                                                                                                                                                                                                                          | `status=ready` não é atingido                  | `backend/internal/httpapi/handlers/server_ssh.go`                                    | ⚠ inferida |

- **Motivo da regra:** conectividade SSH sozinha não garante que o dump remoto funcione — é preciso confirmar a ferramenta de dump (container Docker rodando, ou binário presente no host) e o destino de upload também.
- **Mudança nesta tarefa:** a chave JSON de resposta `checks.docker` foi renomeada para `checks.dumpTool` (breaking change de contrato, ver `docs/API.md`) — o nome antigo era enganoso em modo `host`, onde não existe container a inspecionar.

### RN-BACKUP-014 — Runs enfileiradas manualmente sempre processadas no próximo ciclo

- **Tela:** T-06
- **Status:** ⚠ inferida
- **Origem:** inferida do código

| Condição                                                                                                                                             | Resultado                                                 | Fonte                                             | Status     |
| ---------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- | ------------------------------------------------- | ---------- |
| Existe `backup_run` com `status=queued` (criado via `POST /api/servers/{id}/run-now`) E há slots livres no worker pool após o agendamento automático | run enfileirada é promovida a `running` e enviada ao pool | `backend/internal/scheduler/scheduler.go:844-956` | ⚠ inferida |

- **Motivo da regra:** permitir disparo manual de um backup fora do cron configurado, sem esperar o próximo agendamento automático.

### RN-BACKUP-015 — Handshake SSH com timeout obrigatório

- **Status:** ⚠ inferida
- **Origem:** inferida do código

| Condição                                                     | Resultado                                                                                                                                           | Fonte                                                                                                   | Status     |
| ------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | ---------- |
| Conexão SSH iniciada (test-connection ou execução de backup) | timeout de 30s aplicado tanto ao dial TCP quanto ao handshake SSH completo, via `context.WithTimeout` envolvendo toda a chamada `sshclient.Connect` | `backend/internal/httpapi/handlers/server_ssh.go:101`, `backend/internal/scheduler/executor.go:259-269` | ⚠ inferida |

- **Motivo da regra:** um handshake SSH pendurado (não apenas o TCP connect) não pode travar um worker indefinidamente.

### RN-STORAGE-001 — Campos obrigatórios variam por tipo de destino

- **Tela:** T-08, T-09
- **Status:** ⚠ inferida
- **Origem:** inferida do schema (`CHECK` constraints da migração `000005`)

| Condição            | Resultado                                                                                 | Fonte                                                                | Status     |
| ------------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------------------------------- | ---------- |
| `type='azure'`      | exige `azure_account_name`, `azure_container_name`, `azure_sas_token_encrypted` não nulos | `backend/internal/db/migrations/000005_storage_targets.up.sql:18-19` | ⚠ inferida |
| `type='s3'`         | exige `s3_bucket`, `s3_access_key_id`, `s3_secret_access_key_encrypted` não nulos         | `backend/internal/db/migrations/000005_storage_targets.up.sql:21-22` | ⚠ inferida |
| `type='filesystem'` | exige `fs_root_path` não nulo                                                             | `backend/internal/db/migrations/000005_storage_targets.up.sql:24-25` | ⚠ inferida |

- **Motivo da regra:** cada tipo de storage precisa de um conjunto diferente de credenciais/parâmetros; o banco garante a invariante mesmo se a validação de aplicação falhar.

### RN-AUTH-001 — CPF obrigatório e único em admin_users

- **Status:** ⚠ inferida
- **Origem:** citada no `CLAUDE.md` anterior; validado pelo pacote `internal/cpf` (dígito verificador completo), mas a obrigatoriedade/unicidade em nível de coluna/migração não foi lida byte a byte nesta tarefa

| Condição                          | Resultado                                                                               | Fonte                         | Status     |
| --------------------------------- | --------------------------------------------------------------------------------------- | ----------------------------- | ---------- |
| CPF informado na criação de admin | validado por `cpf.Validate` (11 dígitos, não repetidos, dígitos verificadores corretos) | `backend/internal/cpf/cpf.go` | ⚠ inferida |

### RN-AUTH-002 — Bloqueio de autoexclusão de admin

- **Tela:** T-11, T-13
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa, ao aprovar o escopo do CRUD de administradores via web (não é inferência de código pré-existente — a proteção não existia antes desta tarefa)

| Condição                                                                       | Resultado                                                                             | Fonte                                                            | Status     |
| ------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------- | ---------------------------------------------------------------- | ---------- |
| `DELETE /api/admin-users/{id}` com `id` igual ao usuário da sessão autenticada | `409 Conflict`, "you cannot delete your own account", exclusão não ocorre             | `backend/internal/httpapi/handlers/admin_users.go` (`Delete`)    | confirmada |
| Frontend: linha do próprio usuário logado na tabela de administradores         | botão "Excluir" fica desabilitado (defesa em profundidade, o backend também bloqueia) | `frontend/src/views/AdminUsersView.vue`, `AdminUserEditView.vue` | confirmada |

- **Motivo da regra:** decisão do usuário para evitar que um operador se tranque fora do próprio sistema sem querer.

### RN-AUTH-003 — Bloqueio de exclusão do último admin restante

- **Tela:** T-11, T-13
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa, mesmo motivo de RN-AUTH-002

| Condição                                                                                  | Resultado                                                                     | Fonte                                                         | Status     |
| ----------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------- | ------------------------------------------------------------- | ---------- |
| `DELETE /api/admin-users/{id}` quando `AdminUserRepo.Count() <= 1` no momento da checagem | `409 Conflict`, "cannot delete the last remaining admin", exclusão não ocorre | `backend/internal/httpapi/handlers/admin_users.go` (`Delete`) | confirmada |

- **Motivo da regra:** impedir que o sistema fique sem nenhum administrador (lockout completo, sem CLI de emergência para recriar sessão).
- **Risco conhecido, aceito nesta tarefa:** a checagem `Count()` e o `Delete()` não são atômicos (duas chamadas separadas); uma corrida entre duas exclusões simultâneas dos dois últimos admins poderia, em teoria, deixar o sistema sem nenhum admin. Aceito dado o volume baixíssimo de operadores concorrentes deste sistema interno — ver `docs/Memoria.md`.

### RN-BACKUP-016 — Toda execução deve persistir `started_at`/`finished_at` conforme a transição de estado

- **Tela:** T-06
- **Status:** confirmada
- **Origem:** correção de bug nesta tarefa — `repository.BackupRunRepo.UpdateStatus` nunca setava esses timestamps, então toda execução processada pelo scheduler (agendada ou via `run-now`) ficava com `finished_at` sempre nulo (e `started_at` também nulo para as manuais); reportado pelo usuário ao revisar a tela de Histórico

| Condição                                                                              | Resultado                                                                                                                                           | Fonte                                                         | Status     |
| ------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------- | ---------- |
| `UpdateStatus(..., "running", ...)` — promoção de `queued`/`(criação)` para `running` | `started_at` setado via `COALESCE(started_at, now())` — preserva o valor já existente (runs agendadas) ou preenche pela primeira vez (runs manuais) | `backend/internal/repository/backup_runs.go` (`UpdateStatus`) | confirmada |
| `UpdateStatus(..., "success" \| "failed", ...)` — finalização da execução             | `finished_at = now()`                                                                                                                               | `backend/internal/repository/backup_runs.go` (`UpdateStatus`) | confirmada |

- **Motivo da regra:** a tela de Histórico depende desses dois campos para mostrar quando uma execução começou/terminou; sem eles, toda execução processada pelo scheduler aparecia com "—" mesmo já concluída.

### RN-BACKUP-017 — Senha do banco obrigatória por engine

- **Tela:** T-04, T-05
- **Status:** ⚠ inferida
- **Origem:** decisão de design desta tarefa (suporte a MySQL/SQL Server), registrada aqui para confirmação do usuário

| Condição                 | Resultado                                                                                                                                     | Fonte                                                                                             | Status     |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------- | ---------- |
| `dbEngine != 'postgres'` | senha do banco obrigatória — validação de aplicação (`upsertServerRequest.validate`) + `CHECK` de banco `servers_password_required_by_engine` | `backend/internal/httpapi/handlers/servers.go`, `backend/internal/db/migrations/000006_...up.sql` | ⚠ inferida |
| `dbEngine == 'postgres'` | senha opcional — preserva o comportamento anterior (trust/peer auth dentro do container, sem senha)                                           | idem                                                                                              | ⚠ inferida |

- **Motivo da regra:** MySQL e SQL Server, ao contrário do Postgres hoje, não têm um mecanismo de auth "ambiente" equivalente ao trust/peer socket auth usado dentro do container — a senha é obrigatória para `mysqldump`/`sqlcmd` funcionarem.

### RN-BACKUP-018 — `containerName` obrigatório apenas em modo Docker

- **Tela:** T-04, T-05
- **Status:** ⚠ inferida
- **Origem:** decisão de design desta tarefa (suporte a host direto)

| Condição                     | Resultado                                                                                                    | Fonte                                                                                             | Status     |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------- | ---------- |
| `deploymentMode == 'docker'` | `containerName` obrigatório e validado por regex — `CHECK` de banco `servers_container_name_requires_docker` | `backend/internal/httpapi/handlers/servers.go`, `backend/internal/db/migrations/000006_...up.sql` | ⚠ inferida |
| `deploymentMode == 'host'`   | `containerName` deve ser vazio/nulo na requisição; coluna fica `NULL` no banco                               | idem                                                                                              | ⚠ inferida |

- **Motivo da regra:** um servidor rodando direto no host/instância não tem container nenhum a identificar; exigir o campo seria incoerente e o schema reforça essa invariante independentemente da validação de aplicação.

### RN-BACKUP-019 — Senha do banco só é regravada quando enviada

- **Tela:** T-05
- **Status:** ⚠ inferida
- **Origem:** decisão de design desta tarefa, mesmo padrão já usado para `SASToken`/`SecretAccessKey` de `StorageTarget` (RN-STORAGE-001)

| Condição                                               | Resultado                                                              | Fonte                                                                                     | Status     |
| ------------------------------------------------------ | ---------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- | ---------- |
| `PUT /api/servers/{id}` com `dbPassword` vazio/ausente | senha atual preservada — `SetDBPassword` não é chamado                 | `backend/internal/httpapi/handlers/servers.go` (`Update`)                                 | ⚠ inferida |
| `PUT /api/servers/{id}` com `dbPassword` não vazio     | senha re-criptografada (AAD `domain.DBPasswordAAD`) e regravada        | `backend/internal/httpapi/handlers/servers.go`, `repository/servers.go` (`SetDBPassword`) | ⚠ inferida |
| `POST /api/servers` (criação)                          | `dbPassword` obrigatório quando `dbEngine != postgres` (RN-BACKUP-017) | idem                                                                                      | ⚠ inferida |

- **Motivo da regra:** a senha nunca é retornada pela API (`hasDbPassword: boolean` no DTO, nunca o valor) — sem essa regra, o operador seria forçado a redigitar a senha a cada edição do servidor, mesmo quando só quer mudar outro campo.
- **Caso não previsto (achado pelo Validator/Security Specialist nesta tarefa, `a definir`):** ao trocar `dbEngine` de um servidor existente (ex. `mysql`→`postgres`→`sqlserver`) via `PUT` sem enviar `dbPassword`, a senha antiga permanece associada ao servidor e é reaproveitada pelo novo engine — satisfaz o `CHECK servers_password_required_by_engine` e a AAD é a mesma (`domain.DBPasswordAAD`, não depende do engine), então não há erro de decriptação, mas o backup seguinte pode falhar silenciosamente com credencial incorreta para o novo engine até o operador perceber. Não é falha de segurança (não expõe segredo, não permite acesso indevido), é um problema de integridade funcional. Comportamento não corrigido nesta tarefa — registrado como pendência em `docs/Progresso.md`.

### RN-BACKUP-020 — Limpeza do arquivo temporário de backup do SQL Server é sempre tentada

- **Tela:** — (execução de backup, não uma tela)
- **Status:** ⚠ inferida
- **Origem:** decisão de design desta tarefa (SQL Server `BACKUP DATABASE` não tem streaming nativo, precisa de arquivo intermediário no host remoto)

| Condição                  | Resultado                                                                                                                  | Fonte                                                      | Status     |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | ---------- |
| `dbEngine == 'sqlserver'` | `BACKUP DATABASE ... TO DISK=<tempPath>` via `sqlcmd` (síncrono) → `cat <tempPath>` (stream) → `rm -f <tempPath>` (sempre) | `backend/internal/scheduler/dumpcommand.go`, `executor.go` | ⚠ inferida |
| Limpeza (`rm -f`) falha   | erro apenas logado (não-fatal) — não altera o status já decidido do `backup_run`                                           | `backend/internal/scheduler/executor.go`                   | ⚠ inferida |

- **Motivo da regra:** entre o `BACKUP DATABASE` e o `rm -f`, o dump completo fica em texto não criptografado no host remoto — a limpeza reduz essa janela de exposição, mas sua falha não deve mascarar um backup que já teve sucesso (ou fracasso) decidido pelas etapas anteriores.

### RN-BACKUP-021 — Prévia de comando no frontend nunca exibe a senha real

- **Tela:** T-04, T-05
- **Status:** confirmada (decisão explícita do usuário nesta tarefa)
- **Origem:** decisão explícita do usuário ao aprovar o plano desta tarefa

| Condição                                                      | Resultado                                                                                                                                              | Fonte                             | Status     |
| ------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------ | --------------------------------- | ---------- |
| Campo de senha preenchido (ou `hasDbPassword=true` na edição) | prévia do comando mostra `***` no lugar do valor real, nunca o texto digitado                                                                          | `frontend/src/lib/dumpCommand.ts` | confirmada |
| Campo de senha vazio                                          | prévia do comando omite o componente de senha (Postgres) ou mostra o valor vazio quotado (MySQL/SQL Server, que sempre incluem a variável de ambiente) | `frontend/src/lib/dumpCommand.ts` | confirmada |

- **Motivo da regra:** a senha do banco é um segredo tão sensível quanto a chave SSH — mesmo sendo o próprio operador que a digitou, exibi-la de volta na tela (ainda que na mesma sessão) aumenta a superfície de exposição (screen-sharing, screenshots, shoulder-surfing) sem nenhum benefício real para a prévia, cujo propósito é mostrar a _forma_ do comando, não seu conteúdo secreto.

### RN-BACKUP-022 — Runs do scheduler persistem duração de dump e upload

- **Tela:** T-06
- **Status:** confirmada
- **Origem:** correção de bug nesta tarefa — reportado pelo usuário ("nos backups runs, tanto a duração do dump quanto a do upload não estão sendo calculados")

| Condição                                                           | Resultado                                                                           | Fonte                                         | Status     |
| ------------------------------------------------------------------ | ----------------------------------------------------------------------------------- | --------------------------------------------- | ---------- |
| Backup disparado pelo scheduler (cron) conclui com sucesso         | `dump_duration_ms`/`upload_duration_ms` persistidos via `BackupRunRepo.MarkSuccess` | `backend/internal/scheduler/executor.go`      | confirmada |
| Backup disparado manualmente (`POST /api/servers/{id}/backup-now`) | já persistia as durações corretamente antes desta tarefa — comportamento inalterado | `backend/internal/httpapi/handlers/backup.go` | confirmada |

- **Motivo da regra:** os valores já eram calculados em memória em `executor.go`, mas o caminho de sucesso do scheduler chamava `UpdateBlobMetadata` + `UpdateStatus("success")` em vez de `MarkSuccess` — o único método que grava as colunas de duração —, deixando `dump_duration_ms`/`upload_duration_ms` sempre `NULL` para toda execução agendada, mesmo tendo os dados prontos.

### RN-BACKUP-023 — Nome de container em modo Docker é resolvido por busca parcial (grep)

- **Tela:** T-04, T-05
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa

| Condição                                                                               | Resultado                                                                                                                                                       | Fonte                                                                     | Status     |
| -------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------- | ---------- |
| `deploymentMode='docker'`, antes de test-connection, execução agendada ou `backup-now` | `containerName` configurado (pode ser parcial) é resolvido via `docker ps --format '{{.Names}}' \| grep` no servidor remoto, a cada execução                    | `backend/internal/scheduler/containerresolve.go` (`ResolveContainerName`) | confirmada |
| Exatamente um container corresponde                                                    | nome completo resolvido é usado no `docker exec`/`docker inspect` equivalente; `docker ps` sem `-a` já confirma que está rodando                                | idem                                                                      | confirmada |
| Zero containers correspondem, ou mais de um corresponde                                | falha explícita (test-connection reprova o check "Ferramenta de Dump"; execução de backup marcada `failed`); nunca escolhe o primeiro resultado silenciosamente | idem                                                                      | confirmada |

- **Motivo da regra:** o usuário só sabe/informa uma parte do nome real do container (ex.: gerado dinamicamente pelo orquestrador), então a correspondência exata deixou de ser viável; escolher automaticamente em caso de ambiguidade arriscaria rodar o dump no container errado.

### RN-BACKUP-024 — Ativação/desativação manual do servidor

- **Tela:** T-05
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa

| Condição                              | Resultado                                                                                                              | Fonte                                                                                                 | Status     |
| ------------------------------------- | ---------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------- | ---------- |
| `POST /api/servers/{id}/disable`      | `enabled=false`; `status` inalterado                                                                                   | `backend/internal/repository/servers.go` (`SetEnabled`), `httpapi/handlers/server_ssh.go` (`Disable`) | confirmada |
| `POST /api/servers/{id}/enable`       | `enabled=true`; `status` inalterado                                                                                    | idem (`Enable`)                                                                                       | confirmada |
| Servidor desativado (`enabled=false`) | scheduler automático não o reivindica mais (RN-BACKUP-002 já cobre isso); `run-now`/`backup-now` continuam disponíveis | `backend/internal/scheduler/scheduler.go`, `httpapi/handlers/backup.go`, `run_now.go`                 | confirmada |

- **Motivo da regra:** o operador precisa de uma forma de pausar o agendamento automático de um servidor específico sem perder a opção de rodar um backup manual de teste. O enum de banco `status='disabled'` já existe (nunca usado) mas foi deliberadamente descartado em favor do campo `enabled`, que já era a condição de elegibilidade checada pelo scheduler (RN-BACKUP-002) — evita introduzir uma nova transição de máquina de estados.
- **Caso não previsto (pendência não resolvida nesta tarefa):** o valor de enum `status='disabled'` permanece no schema sem nenhuma escrita — ver nota em RN-BACKUP-001.

### RN-BACKUP-025 — Histórico paginado com filtro por status

- **Tela:** T-06
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa

| Condição                                                                  | Resultado                                                                          | Fonte                                                            | Status     |
| ------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------------------------------------------------------------- | ---------- |
| `GET /api/servers/{id}/backup-runs` sem parâmetros                        | página 1, 50 registros, todos os status, envelope `{items, total, page, pageSize}` | `backend/internal/httpapi/handlers/backup.go` (`ListBackupRuns`) | confirmada |
| `?page=`/`?pageSize=` fora dos limites (`page<1`, `pageSize<1` ou `>200`) | `400 Bad Request`                                                                  | idem (`parseBackupRunListParams`)                                | confirmada |
| `?status=` com valor fora de `queued`/`running`/`success`/`failed`        | `400 Bad Request`                                                                  | idem                                                             | confirmada |
| Página além do total de registros                                         | array vazio, `total` real (não recalculado a partir da página) — não é erro        | `backend/internal/repository/backup_runs.go` (`List`)            | confirmada |

- **Motivo da regra:** `backup_runs` nunca tem linhas apagadas (só os blobs são limpos pela retenção), então buscar todo o histórico de um servidor de uma vez cresce sem limite ao longo do tempo. O filtro por servidor já existia como seleção obrigatória na tela; esta regra adiciona paginação real no backend e um filtro por status.
- **Breaking change de contrato:** a resposta deixou de ser um array solto e passou a ser o envelope acima — documentado em `docs/API.md`.

### RN-BACKUP-026 — Histórico combina todos os servidores por padrão

- **Tela:** T-06
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa — reportou que a tela "não traz resultados pelo default, precisa selecionar um servidor primeiro"

| Condição                                         | Resultado                                                                                          | Fonte                                                                       | Status     |
| ------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------- | ---------- |
| `GET /api/backup-runs` sem `serverId`             | página 1, 50 registros, de **todos** os servidores, mais recentes primeiro, envelope igual a RN-BACKUP-025 | `backend/internal/repository/backup_runs.go` (`ListAll`), `httpapi/handlers/backup.go` (`ListAllBackupRuns`) | confirmada |
| `GET /api/backup-runs?serverId=<id>`              | mesmo resultado de `GET /api/servers/{id}/backup-runs` (filtra para aquele servidor)                | idem                                                                        | confirmada |
| `GET /api/backup-runs?serverId=<id inexistente>`  | `404 Not Found` (mesma checagem do endpoint aninhado)                                                | `httpapi/handlers/backup.go` (`ListAllBackupRuns`)                          | confirmada |
| Frontend (`HistoryView.vue`) sem servidor selecionado no dropdown | busca a visão "todos os servidores" automaticamente no carregamento da tela, em vez de não fazer nada | `frontend/src/views/HistoryView.vue`                                        | confirmada |

- **Motivo da regra:** o operador quer ver os backups mais recentes de qualquer servidor assim que abre a tela, sem precisar saber de antemão qual servidor teve problema.
- **Compatibilidade:** `GET /api/servers/{id}/backup-runs` (RN-BACKUP-025) continua existindo sem nenhuma mudança de contrato — `GET /api/backup-runs` é um endpoint adicional, não uma substituição.

### RN-BACKUP-027 — Mensagem de erro e log de saída de backup trazem detalhe real, redigido de segredo

- **Tela:** T-06
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa — pediu que "no log de erros de backup, preciso que tenha detalhes do que deu errado, para ajudar na correção"

| Condição                                                                                     | Resultado                                                                                                                                  | Fonte                                                                    | Status     |
| ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ---------- |
| Backup falha em qualquer etapa (scheduler ou `backup-now`)                                     | `error_message` passa a conter o erro real da etapa (antes: frase genérica fixa, ex. "backup failed during upload or dump execution")          | `backend/internal/scheduler/executor.go`, `httpapi/handlers/backup.go` (`performBackup`) | confirmada |
| Falha no dump/upload ou no `BACKUP DATABASE` (SQL Server)                                       | `log_output` passa a conter o stdout/stderr capturado do comando remoto (antes: sempre `NULL`, coluna nunca escrita apesar de existir desde a migração inicial) | idem, `repository.BackupRunRepo.MarkFailedWithDetails`                    | confirmada |
| `error_message`/`log_output` contêm a senha do banco decriptada (quando configurada)            | senha substituída por `***` antes de persistir **e antes de qualquer log de aplicação** (mesmo espírito de RN-BACKUP-021)                     | `backupcore.RedactSecret`, usado por `scheduler/executor.go` e `httpapi/handlers/backup.go` (`finalizeBackupError`) | confirmada |
| `error_message`/`log_output` excedem 2000/8000 caracteres, respectivamente                      | truncados com marcador `"... (truncated)"` — incondicionalmente, mesmo quando não há segredo a redigir (ex. Postgres sem senha configurada)   | `backupcore.TruncateText`, idem                                          | confirmada |

- **Motivo da regra:** o operador precisa diagnosticar a causa real de uma falha (ex. "connection refused" vs. "authentication failed" vs. "disk full") sem precisar acessar os logs do processo `worker` no host — a UI (`BackupRunDetailsDialog.vue`) já sabia renderizar os dois campos, só nunca recebia dado útil.
- **Risco aceito:** o texto de erro/log pode, em tese, conter outros detalhes internos (ex. hostname, caminho de arquivo) além da senha — considerado aceitável pois não são segredos, apenas metadados operacionais; a senha do banco é o único segredo que passa por esses caminhos de código hoje (chave SSH e tokens SAS/S3 não aparecem em texto de erro de dump/upload).

### RN-BACKUP-028 — Scheduler não empilha novo backup atrás de um já em andamento, com timeout e watchdog

- **Tela:** — (scheduler, `cmd/worker`, não é uma tela)
- **Status:** confirmada
- **Origem:** decisão explícita do usuário nesta tarefa — reportou "o server que estavam com o cron 0 3 * * * o agendado não respeitou isso e enfileirou novo backup nesse horário, apenas não fez nada"

| Condição                                                                                     | Resultado                                                                                                          | Fonte                                                                        | Status     |
| ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------- | ---------- |
| Servidor elegível por cron (RN-BACKUP-002) já tem `backup_run` com `status IN ('running','queued')` | servidor não é reivindicado neste ciclo — nenhum novo run é criado atrás do que já está em andamento                | `backend/internal/repository/servers.go` (`GetReadyServersForScheduling`) | confirmada |
| Execução de um backup (dump + upload) ultrapassa `BACKUP_TASK_TIMEOUT` (default 6h)              | contexto da execução é cancelado; run é marcado `failed` com mensagem indicando timeout, em vez de ocupar o worker para sempre | `backend/internal/scheduler/pool.go` (`SetTaskTimeout`/`runTask`), `internal/sshclient/sshclient.go` (`StreamCommand`) | confirmada |
| `backup_run` permanece `running` além de `BACKUP_TASK_TIMEOUT` mesmo assim (ex.: preso antes desta correção, ou caminho não coberto pelo timeout) | watchdog periódico (`WATCHDOG_POLL_INTERVAL`, default 5min) marca o run como `failed`                                | `backend/internal/scheduler/watchdog.go`, `repository.BackupRunRepo.FailStaleRunning` | confirmada |
| `WorkerPool.AvailableSlots()` — contagem de vagas do worker pool                                 | reflete workers realmente ocupados (contador atômico), não mais espaço livre no canal — corrige a causa raiz do "enfileirou e não fez nada" | `backend/internal/scheduler/pool.go`                                       | confirmada |

- **Motivo da regra:** um único backup preso (rede instável, storage lento) consumia um worker do pool para sempre, sem que o scheduler percebesse — ele continuava achando que havia vagas livres e empilhava novos runs atrás do travado, ciclo de cron após ciclo de cron, sem nenhum erro visível.
- **Caso não previsto, resolvido em 2026-09-16 (nesta tarefa):** `servers.next_run_at` só era calculado dentro do próprio `claimAndEnqueue` — nenhum ponto (criação do servidor, `Update`, `Enable`) o populava pela primeira vez, deixando todo servidor nunca antes reivindicado com `next_run_at = NULL` para sempre. Ver RN-BACKUP-029.

### RN-BACKUP-029 — Bootstrap de `next_run_at` na primeira transição para `ready`

- **Tela:** T-05 (aciona `TestConnection`, que é onde o bootstrap acontece)
- **Status:** confirmada
- **Origem:** achado durante a investigação de RN-BACKUP-028 (2026-09-16), registrado como pendência em `docs/Progresso.md`/`docs/Arquitetura.md` ("Pontos a definir") e corrigido nesta tarefa após confirmação do usuário

| Condição                                                                                       | Resultado                                                                                                            | Fonte                                                                                          | Status     |
| ------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- | ---------- |
| Test-connection combinado passa e `next_run_at` do servidor ainda é `NULL` (nunca reivindicado pelo scheduler) | `next_run_at` é calculado a partir de `cronExpression` (próxima ocorrência a partir de agora) e persistido junto com a transição de status/`enabled` | `httpapi/handlers/server_ssh.go` (`bootstrapNextRunAt`), `scheduler.NextRunTime`                     | confirmada |
| Test-connection combinado passa e `next_run_at` já está preenchido (servidor já reivindicado ao menos uma vez) | valor existente é mantido inalterado — `scheduler.claimAndEnqueue` continua sendo o único escritor a partir daí       | `httpapi/handlers/server_ssh.go` (`bootstrapNextRunAt`)                                              | confirmada |
| `cronExpression` inválida no momento do bootstrap                                                 | aviso logado (`slog.WarnContext`); `next_run_at` permanece `NULL` e a requisição de test-connection não falha — mesma degradação graciosa já usada por `claimAndEnqueue` | `httpapi/handlers/server_ssh.go`                                                                     | confirmada |

- **Motivo da regra:** sem isso, todo servidor que chegasse a `enabled=true`/`status=ready` pela primeira vez via test-connection (fluxo padrão de onboarding, RN-BACKUP-005) nunca seria reivindicado pelo scheduler automático (`GetReadyServersForScheduling` exige `next_run_at IS NOT NULL`) — só rodaria via `POST /run-now` manual, silenciosamente, sem nenhum erro visível ao operador.
- **Consequência de design:** `scheduler.NextRunTime` (novo helper exportado em `internal/scheduler`) centraliza o parsing de cron usado tanto por `claimAndEnqueue` quanto pelo bootstrap no handler, evitando duplicar a lógica de "calcular próximo horário" em dois pontos de escrita — mesma lição já registrada em `docs/Memoria.md` sobre RN-BACKUP-027 (duplicação de `redactSecret`/`truncateText` convidou divergência).
- **Caso não previsto, resolvido em 2026-09-16 (nesta tarefa) — ver RN-BACKUP-030:** editar `cronExpression` de um servidor já agendado (`next_run_at` já não-nulo) via `PUT /api/servers/{id}` não recalculava `next_run_at` — `bootstrapNextRunAt` só agia quando o valor atual era `nil`, e `ServerRepo.Update` não tocava essa coluna. O novo horário só passava a valer a partir do próximo `claimAndEnqueue`.

### RN-BACKUP-030 — Recálculo imediato de `next_run_at` ao editar cron de servidor já agendado

- **Tela:** T-05
- **Status:** confirmada
- **Origem:** achado do Validator em 2026-09-16 registrado como "Caso não previsto" em RN-BACKUP-029, corrigido nesta tarefa

| Condição                                                                                                   | Resultado                                                                                                                             | Fonte                                                                                                                | Status     |
| ------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------- | ---------- |
| `PUT /api/servers/{id}` muda `cronExpression` E o servidor já tinha `next_run_at` não-nulo (já agendado ao menos uma vez) | `next_run_at` é recalculado a partir do novo cron e de `now()`, e persistido imediatamente — não espera o próximo `claimAndEnqueue`; `last_scheduled_at` **não** é tocado (só `claimAndEnqueue`, via `UpdateNextRun`, pode alterá-lo — uma edição manual de cron não é uma reivindicação real do scheduler)      | `httpapi/handlers/servers.go` (`recomputeNextRunAtOnCronChange`, chamada em `Update`), `repository.ServerRepo.RescheduleNextRunAt` | confirmada |
| `PUT /api/servers/{id}` muda `cronExpression`, mas `next_run_at` ainda é `nil` (nunca agendado)             | `next_run_at` permanece `nil` — bootstrap (RN-BACKUP-029) continua sendo o único responsável por essa primeira escrita                | idem                                                                                                                   | confirmada |
| `PUT /api/servers/{id}` não muda `cronExpression` (mesmo valor, ou campo omitido)                            | `next_run_at` inalterado                                                                                                              | idem                                                                                                                   | confirmada |
| Novo `cronExpression` inválido no momento do recálculo                                                       | aviso logado (`slog.WarnContext`); `next_run_at` permanece com o valor anterior e a requisição de update não falha — mesma degradação graciosa de RN-BACKUP-029 | `httpapi/handlers/servers.go`                                                                                          | confirmada |

- **Motivo da regra:** sem isso, o operador que corrige o horário de um cron mal configurado (ex.: "estava rodando de madrugada, preciso mudar para às 5h") só veria o novo horário valer depois que o cron antigo disparasse pelo menos mais uma vez — silenciosamente, sem nenhum erro visível.
- **Decisão de escopo:** o recálculo só age sobre servidores já agendados (`next_run_at` não-nulo), para não introduzir um segundo caminho de escrita para o caso "primeira vez", que continua sendo exclusividade do bootstrap de RN-BACKUP-029.
- **Achado do Validator, corrigido nesta tarefa:** a primeira versão desta correção reaproveitou `ServerRepo.UpdateNextRun` (usado por `claimAndEnqueue`), que também grava `last_scheduled_at = now()` — uma edição manual de cron passaria a fazer essa coluna "mentir" sobre quando o scheduler de fato reivindicou o servidor pela última vez. Corrigido com um método de repositório dedicado, `ServerRepo.RescheduleNextRunAt`, que só grava `next_run_at`/`updated_at`, nunca `last_scheduled_at` — mantendo essa coluna como reflexo exclusivo de reivindicações reais do scheduler.

### RN-BACKUP-031 — Destino exibido nos gráficos do dashboard é o destino ATUAL do servidor, não o histórico

- **Tela:** T-02 (Dashboard)
- **Status:** ⚠ inferida — pendente de confirmação do usuário
- **Origem:** decisão de implementação da tarefa "gráficos de backups por destino" (substituição do gráfico de tamanho/duração por 4 gráficos de barras empilhadas por destino, últimos 30 dias e mês a mês do ano corrente)

| Condição                                                                                       | Resultado                                                                                                                                                       | Fonte                                                                          | Status     |
| ------------------------------------------------------------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------ | -------------------------------------------------------------------------------- | ---------- |
| Backup run pertence a um servidor com `storage_target_id` configurado no momento da consulta    | run é agrupado sob o destino atual daquele servidor (`storage_targets.name`), mesmo que o servidor tenha usado um destino diferente no momento real do backup | `repository.BackupRunRepo.CountAndBytesByDestination` (JOIN `backup_runs.server_id = servers.id`) | ⚠ inferida |
| Backup run pertence a um servidor com `storage_target_id IS NULL`                               | run é agrupado sob a categoria explícita "Sem destino" (`storageTargetId: null` na resposta de `GET /api/dashboard/backup-stats`)                              | idem                                                                              | ⚠ inferida |

- **Motivo da regra:** `backup_runs` não guarda o destino usado no momento da execução (sem coluna própria); adicionar essa coluna exigiria uma migration nova, decisão explicitamente descartada para esta tarefa (custo/benefício não justificado para uma agregação de leitura). Trade-off aceito: se um servidor trocar de destino, todo o seu histórico "migra" visualmente para o novo destino nos 4 gráficos do dashboard.
- **Consequência de design:** a FK `servers.storage_target_id REFERENCES storage_targets(id)` (sem `ON DELETE`) já impede excluir um destino ainda referenciado por algum servidor — logo não existe hoje o caso "destino apagado com servidores ainda apontando para ele" a ser tratado nos gráficos.
- **Correção de bug encontrada durante a implementação:** a query original usava `date_trunc(unit, br.created_at)` direto sobre a coluna `timestamptz`, que trunca no timezone da sessão do Postgres (America/Sao_Paulo, UTC-3) em vez de UTC — deslocando silenciosamente os limites de dia/mês em até 3h relativo às janelas calculadas em UTC pelo handler Go. Corrigido convertendo explicitamente para UTC antes de truncar (`date_trunc(unit, br.created_at AT TIME ZONE 'UTC')`); coberto por `TestBackupRunRepo_CountAndBytesByDestination/month_granularity_buckets_at_the_first_day_of_the_month`, que falhava antes da correção.

---

## Regras por objeto / entidade

### E-SERVER — Server

- **Descrição:** servidor de banco de dados remoto gerenciado, acessado via SSH. Suporta três motores (`dbEngine`: `postgres`/`mysql`/`sqlserver`) e dois modos de implantação (`deploymentMode`: `docker`, com container identificado por nome, ou `host`, direto na instância remota). Senha do banco (`dbPasswordEncrypted`) é opcional para Postgres (preserva trust/peer auth) e obrigatória para MySQL/SQL Server (RN-BACKUP-017).
- **Origem do dado:** cadastro manual pelo admin (T-04)
- **Status:** ⚠ inferida (descrição expandida nesta tarefa para multi-engine/docker-ou-host — ver RN-BACKUP-017 a 021)

#### Ciclo de vida

| Estado atual                                       | Evento                                         | Condição                              | Estado destino                            | Quem pode | Regra         |
| -------------------------------------------------- | ---------------------------------------------- | ------------------------------------- | ----------------------------------------- | --------- | ------------- |
| (novo)                                             | criação do servidor                            | —                                     | `pending_key`                             | admin     | RN-BACKUP-001 |
| `pending_key` / `awaiting_authorization` / `ready` | regenerar chave SSH                            | sempre                                | `awaiting_authorization`, `enabled=false` | admin     | RN-BACKUP-001 |
| qualquer                                           | test-connection combinado passa                | SSH + Ferramenta de Dump + Storage OK | `ready`                                   | admin     | RN-BACKUP-013 |
| `awaiting_authorization`                           | test-connection combinado passa (primeira vez) | idem acima                            | `ready`, `enabled=true`                   | admin     | RN-BACKUP-005 |

#### Invariantes

| ID            | Invariante                                                                                                      | Onde é garantido                                                  | Status     |
| ------------- | --------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------- |
| RN-BACKUP-002 | Servidor só é reivindicado pelo scheduler se `enabled=true` AND `status=ready` AND `storage_target_id` não nulo | `backend/internal/scheduler/scheduler.go:745`                     | ⚠ inferida |
| RT-SEC-001    | `SSHPrivateKeyEncrypted` nunca é serializado de volta em resposta de API                                        | `backend/internal/domain/domain.go` (comentário)                  | ⚠ inferida |
| RN-BACKUP-017 | `dbPasswordEncrypted` não nulo quando `dbEngine != 'postgres'`                                                  | `CHECK servers_password_required_by_engine` (migration 000006)    | ⚠ inferida |
| RN-BACKUP-018 | `containerName` não nulo somente quando `deploymentMode='docker'`                                               | `CHECK servers_container_name_requires_docker` (migration 000006) | ⚠ inferida |
| RT-SEC-004    | `DBPasswordEncrypted` nunca é serializado de volta em resposta de API — DTO expõe só `hasDbPassword: boolean`   | `backend/internal/httpapi/handlers/servers.go` (`serverDTO`)      | confirmada |

### E-STORAGETARGET — StorageTarget

- **Descrição:** destino de backup configurado — Azure Blob, S3-compatível ou filesystem/NFS.
- **Origem do dado:** cadastro manual pelo admin (T-08)
- **Status:** ⚠ inferida

#### Invariantes

| ID             | Invariante                                                                                                                       | Onde é garantido                                               | Status     |
| -------------- | -------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------- | ---------- |
| RN-STORAGE-001 | Campos obrigatórios variam por `Type`; `CHECK` constraint no banco impede combinação inválida                                    | `backend/internal/db/migrations/000005_storage_targets.up.sql` | ⚠ inferida |
| RT-SEC-002     | `AzureSASTokenEncrypted`/`S3SecretAccessKeyEncrypted` só populados em memória logo após decriptação, nunca serializados de volta | `backend/internal/domain/domain.go:58-60` (comentário)         | ⚠ inferida |

### E-BACKUPRUN — BackupRun

- **Descrição:** execução (agendada ou manual) de um backup, com status e metadados do blob resultante.
- **Origem do dado:** criado pelo scheduler (automático) ou pelo endpoint `run-now` (manual)
- **Status:** ⚠ inferida

#### Ciclo de vida

| Estado atual | Evento                                               | Condição      | Estado destino | Quem pode | Regra         |
| ------------ | ---------------------------------------------------- | ------------- | -------------- | --------- | ------------- |
| (novo)       | scheduler reivindica servidor elegível               | RN-BACKUP-002 | `running`      | sistema   | —             |
| (novo)       | `POST /api/servers/{id}/run-now`                     | —             | `queued`       | admin     | RN-BACKUP-014 |
| `queued`     | próximo ciclo do scheduler com slot livre            | —             | `running`      | sistema   | RN-BACKUP-014 |
| `running`    | dump + upload concluídos com sucesso                 | —             | `success`      | sistema   | —             |
| `running`    | falha em qualquer etapa (decrypt, SSH, dump, upload) | —             | `failed`       | sistema   | —             |

### E-ADMINUSER — AdminUser

- **Descrição:** usuário administrador (operador) do sistema. Único papel existente (`role="admin"`, hardcoded na criação — sem seletor de papel na UI, sistema é single-role).
- **Origem do dado:** criado via `./api create-admin` (CLI, no host) ou via `POST /api/admin-users` (web, T-12), ambos reaproveitando `auth.HashPassword` (Argon2id) e `cpf.Validate`.
- **Status:** confirmada (CRUD via web é decisão desta tarefa; CLI continua existindo como fallback)

#### Ciclo de vida

| Estado atual | Evento                                                        | Condição                                                   | Estado destino                         | Quem pode                   | Regra                    |
| ------------ | ------------------------------------------------------------- | ---------------------------------------------------------- | -------------------------------------- | --------------------------- | ------------------------ |
| (novo)       | `./api create-admin` ou `POST /api/admin-users`               | e-mail/CPF únicos, senha ≥ 12 caracteres                   | existe                                 | admin (ou operador no host) | RN-AUTH-001              |
| existente    | `PUT /api/admin-users/{id}`                                   | e-mail/CPF únicos; senha opcional (vazio = mantém a atual) | atualizado                             | admin                       | RN-AUTH-001              |
| existente    | `./api reset-password` ou `PUT .../{id}` com senha preenchida | senha ≥ 12 caracteres                                      | senha trocada                          | admin (ou operador no host) | —                        |
| existente    | `DELETE /api/admin-users/{id}`                                | não é o próprio usuário logado E não é o último admin      | removido (cascade em `admin_sessions`) | admin                       | RN-AUTH-002, RN-AUTH-003 |

#### Invariantes

| ID          | Invariante                                                                                       | Onde é garantido                                                                                                                              | Status     |
| ----------- | ------------------------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| RN-AUTH-001 | `email` (citext) e `cpf` são únicos no banco; violação vira `409` amigável na API                | `backend/internal/db/migrations/000001_init_schema.up.sql`, `000004_add_admin_cpf.up.sql`, `backend/internal/httpapi/handlers/admin_users.go` | confirmada |
| RN-AUTH-002 | Não é possível excluir a própria conta                                                           | `backend/internal/httpapi/handlers/admin_users.go` (`Delete`)                                                                                 | confirmada |
| RN-AUTH-003 | Não é possível excluir o último admin restante                                                   | `backend/internal/httpapi/handlers/admin_users.go` (`Delete`)                                                                                 | confirmada |
| RT-SEC-003  | `password_hash` nunca é serializado de volta em resposta de API (`adminUserDTO` não tem o campo) | `backend/internal/httpapi/handlers/admin_users.go` (`adminUserDTO`)                                                                           | confirmada |

---

## Regras transversais

### Matriz de permissões

| Papel | Recurso         | Ler | Criar         | Editar | Excluir              | Observação                                                                        |
| ----- | --------------- | --- | ------------- | ------ | -------------------- | --------------------------------------------------------------------------------- |
| admin | Server          | Sim | Sim           | Sim    | Sim                  | Único papel existente — sem RBAC granular                                         |
| admin | StorageTarget   | Sim | Sim           | Sim    | Sim                  | idem                                                                              |
| admin | RetentionPolicy | Sim | Sim           | Sim    | Sim                  | idem                                                                              |
| admin | BackupRun       | Sim | Sim (run-now) | Não    | Não                  | Execução automática ou manual apenas                                              |
| admin | AdminUser       | Sim | Sim           | Sim    | Sim (com restrições) | Sim, exceto a própria conta (RN-AUTH-002) e o último admin restante (RN-AUTH-003) |

`⚠ inferida`: sistema modelado como operador único (`RN-AUTH-001` cita CPF único), sem evidência de múltiplos papéis no código lido. A ausência de RBAC granular é uma lacuna conhecida: hoje qualquer sessão autenticada tem acesso irrestrito a `/api/admin-users/*` (sem checagem de `role` no middleware) — aceitável apenas enquanto o sistema permanecer single-role.

### Formatos e convenções

| Item         | Regra                                                                                                                  |
| ------------ | ---------------------------------------------------------------------------------------------------------------------- |
| Documentos   | CPF: validado por dígito verificador (`internal/cpf`)                                                                  |
| Nome de blob | `<slug(nome-do-servidor)>/<dbname>_<RFC3339>.dump` — `slugify` trata como fronteira de segurança contra path traversal |
| Fuso horário | `a definir` — não confirmado nesta tarefa                                                                              |
| Paginação    | `a definir` — não confirmado nesta tarefa (ver `docs/API.md`)                                                          |

---

## Rastreabilidade

| Regra                                       | Implementada em                                                                                                                    | Teste que cobre                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | Verificada em |
| ------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ------------- |
| RN-BACKUP-003                               | `backend/internal/retention/retention.go`                                                                                          | `backend/internal/retention/*_test.go` (existência não confirmada nesta tarefa — ver `docs/Progresso.md`)                                                                                                                                                                                                                                                                                                                                                                                  | 2026-09-15    |
| RN-BACKUP-005, RN-BACKUP-013                | `backend/internal/httpapi/handlers/server_ssh.go`                                                                                  | `a definir`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | 2026-09-15    |
| RN-BACKUP-008                               | `backend/internal/scheduler/executor.go`                                                                                           | `a definir`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | 2026-09-15    |
| RN-BACKUP-015                               | `backend/internal/sshclient/sshclient.go`                                                                                          | `a definir`                                                                                                                                                                                                                                                                                                                                                                                                                                                                                | 2026-09-15    |
| RN-AUTH-002, RN-AUTH-003                    | `backend/internal/httpapi/handlers/admin_users.go` (`Delete`)                                                                      | `backend/internal/httpapi/handlers/admin_users_test.go`: `TestAdminUserHandlers_Delete_SelfBlocked`, `TestAdminUserHandlers_Delete_NonLastAdminSucceeds` (determinísticos); `TestAdminUserHandlers_Delete_LastAdminBlocked` (`t.Skip` se o banco de teste compartilhado não tiver exatamente 1 admin no momento — sem reset de schema por teste neste projeto, não é seguro forçar esse estado apagando contas reais)                                                                      | 2026-09-15    |
| RN-BACKUP-016                               | `backend/internal/repository/backup_runs.go` (`UpdateStatus`)                                                                      | `backend/internal/repository/backup_runs_test.go`: `TestUpdateStatus_SetsTimestamps`                                                                                                                                                                                                                                                                                                                                                                                                       | 2026-09-15    |
| RN-BACKUP-008, RN-BACKUP-017, RN-BACKUP-020 | `backend/internal/scheduler/dumpcommand.go`                                                                                        | `backend/internal/scheduler/dumpcommand_test.go`: 29 testes cobrindo as 3 engines × 2 modos de implantação, injeção de shell (extraArgs, dbName, senha), consistência do `tempPath` do SQL Server entre `PreCmd`/`StreamCmd`/`CleanupCmd`                                                                                                                                                                                                                                                  | 2026-09-15    |
| RN-BACKUP-018, RN-BACKUP-019                | `backend/internal/httpapi/handlers/servers.go`, `repository/servers.go` (`Create`, `SetDBPassword`)                                | `backend/internal/repository/servers_test.go`: `TestSetDBPassword` — executado e passando contra um Postgres 16 descartável real nesta sessão (revelou e confirmou a correção de um bug crítico de `CHECK` constraint na criação, ver `docs/Memoria.md`)                                                                                                                                                                                                                                   | 2026-09-15    |
| RN-BACKUP-013                               | `backend/internal/httpapi/handlers/server_ssh.go` (`checkDumpTool`)                                                                | `backend/internal/httpapi/handlers/server_ssh_test.go`: `TestComposeCheckError_*` (renomeados de `Docker` para `DumpTool`)                                                                                                                                                                                                                                                                                                                                                                 | 2026-09-15    |
| RN-BACKUP-021                               | `frontend/src/lib/dumpCommand.ts`                                                                                                  | `frontend/src/lib/dumpCommand.spec.ts`: 21 testes, incluindo mascaramento de senha e neutralização de injeção de shell; `frontend/src/views/ServerNewView.spec.ts`/`ServerEditView.spec.ts`: casos de prévia reativa e "senha nunca em texto puro"                                                                                                                                                                                                                                         | 2026-09-15    |
| RN-BACKUP-022                               | `backend/internal/scheduler/executor.go`                                                                                           | `backend/internal/repository/backup_runs_test.go`: `TestMarkSuccess` — executado e passando contra um Postgres 16 descartável real, confirma que `MarkSuccess` persiste `status`/`blob_name`/`blob_size_bytes`/`dump_duration_ms`/`upload_duration_ms`/`finished_at`. O caminho completo do `BackupExecutor` chamando `MarkSuccess` a partir de uma execução real via SSH não tem teste de integração dedicado (exigiria um servidor SSH de teste, que este projeto não tem) — `a definir` | 2026-09-16    |
| RN-BACKUP-023                               | `backend/internal/scheduler/containerresolve.go`                                                                                   | `backend/internal/scheduler/containerresolve_test.go`: `TestBuildResolveContainerCommand_ShellQuoted`, `TestParseContainerMatches` (0/1/N matches), `TestParseContainerMatches_AmbiguousErrorListsNames` — executados e passando (lógica pura, sem SSH real)                                                                                                                                                                                                                               | 2026-09-15    |
| RN-BACKUP-024                               | `backend/internal/repository/servers.go` (`SetEnabled`), `httpapi/handlers/server_ssh.go` (`Enable`/`Disable`)                     | `backend/internal/repository/servers_test.go`: `TestSetEnabled` — executado e passando contra um Postgres 16 descartável real nesta sessão; `frontend/src/views/ServerEditView.spec.ts`: 3 testes do fluxo ativar/desativar via `ConfirmDialog`                                                                                                                                                                                                                                            | 2026-09-15    |
| RN-BACKUP-025                               | `backend/internal/repository/backup_runs.go` (`List`), `httpapi/handlers/backup.go` (`ListBackupRuns`, `parseBackupRunListParams`) | `backend/internal/repository/backup_runs_test.go`: `TestBackupRunRepo_List_PaginationAndStatusFilter` (5 casos, incluindo página além do total) — executado e passando contra Postgres real (revelou e corrigiu um bug de `COUNT(*) OVER()` perdendo o total quando a página fica vazia, ver `docs/Memoria.md`); `httpapi/handlers/backup_test.go`: `TestParseBackupRunListParams` (9 casos); `frontend/src/views/HistoryView.spec.ts`: 13 testes cobrindo paginação e filtro server-side  | 2026-09-15    |
| RN-BACKUP-026                               | `backend/internal/repository/backup_runs.go` (`ListAll`), `httpapi/handlers/backup.go` (`ListAllBackupRuns`, `parseAllBackupRunListParams`), `frontend/src/views/HistoryView.vue`, `frontend/src/api/index.ts` (`getAllBackupRuns`) | `backend/internal/repository/backup_runs_test.go`: `TestBackupRunRepo_ListAll` (3 casos, incluindo servidor opcional) — executado e passando contra Postgres real; `httpapi/handlers/backup_test.go`: `TestParseAllBackupRunListParams` (4 casos); `frontend/src/views/HistoryView.spec.ts`: 15 testes cobrindo o default "todos os servidores" | 2026-09-16    |
| RN-BACKUP-027                               | `backend/internal/backupcore/redact.go` (`RedactSecret`, `TruncateText`), `scheduler/executor.go`, `httpapi/handlers/backup.go` (`performBackup`, `finalizeBackupError`), `repository.BackupRunRepo.MarkFailedWithDetails` | `backend/internal/repository/backup_runs_test.go`: `TestMarkFailedWithDetails` — executado e passando contra Postgres real; `backend/internal/backupcore/redact_test.go`: `TestRedactSecret`, `TestTruncateText`; `httpapi/handlers/backup_test.go`: `TestFinalizeBackupError` (3 casos, incluindo o caso `dbPassword=""` que o Validator encontrou sem truncamento numa rodada anterior); **novo em 2026-09-16**, `backend/internal/scheduler/executor_slog_test.go`: `TestExecuteBackup_SlogNeverContainsRawSecretOnRemoteCommandFailure` — end-to-end real (Postgres real + servidor SSH de teste simulando falha do `BACKUP DATABASE` do SQL Server com stderr contendo a senha), captura o `slog` de verdade e assert que o segredo nunca aparece em texto puro; fecha a lacuna de orquestração ponta a ponta citada abaixo | 2026-09-16    |
| RN-BACKUP-028                               | `backend/internal/repository/servers.go` (`GetReadyServersForScheduling`), `scheduler/pool.go` (`SetTaskTimeout`, `AvailableSlots`), `scheduler/watchdog.go`, `sshclient/sshclient.go` (`StreamCommand`), `repository.BackupRunRepo.FailStaleRunning` | `backend/internal/repository/servers_test.go`: `TestGetReadyServersForScheduling_SkipsServerWithInFlightRun`; `backend/internal/repository/backup_runs_test.go`: `TestFailStaleRunning`; `backend/internal/scheduler/pool_test.go`: `TestPoolAvailableSlots_ReflectsBusyWorkers`, `TestPoolTaskTimeout_CancelsHungTask`; `backend/internal/sshclient/stream_test.go`: `TestStreamCommand_CtxCancellationUnblocksHungCommand` — todos executados e passando (os de repositório contra Postgres real) | 2026-09-16    |
| RN-BACKUP-029                               | `backend/internal/httpapi/handlers/server_ssh.go` (`bootstrapNextRunAt`), `backend/internal/scheduler/nextrun.go` (`NextRunTime`) | `backend/internal/scheduler/nextrun_test.go`: `TestNextRunTime_ValidCron`, `TestNextRunTime_InvalidCron`; `backend/internal/httpapi/handlers/server_ssh_test.go`: `TestBootstrapNextRunAt_FirstTimeReady`, `TestBootstrapNextRunAt_AlreadyScheduledIsLeftUntouched`, `TestBootstrapNextRunAt_InvalidCronLeavesNilWithoutFailing` — todos puros, sem SSH/DB, executados e passando | 2026-09-16    |
| RN-BACKUP-030                               | `backend/internal/httpapi/handlers/servers.go` (`recomputeNextRunAtOnCronChange`, `Update`), `backend/internal/repository/servers.go` (`RescheduleNextRunAt`) | `backend/internal/httpapi/handlers/servers_test.go`: `TestRecomputeNextRunAtOnCronChange_CronChangedAndAlreadyScheduled`, `TestRecomputeNextRunAtOnCronChange_CronUnchangedIsNoop`, `TestRecomputeNextRunAtOnCronChange_EmptyNewCronIsNoop`, `TestRecomputeNextRunAtOnCronChange_NeverScheduledIsLeftToBootstrap`, `TestRecomputeNextRunAtOnCronChange_InvalidCronReturnsError` — todos puros, sem DB; `backend/internal/repository/servers_test.go`: `TestRescheduleNextRunAt_DoesNotTouchLastScheduledAt` — executado e passando contra um Postgres 16 descartável real, prova que `last_scheduled_at` permanece intocado | 2026-09-16    |
| RN-BACKUP-031                               | `backend/internal/repository/backup_runs.go` (`CountAndBytesByDestination`), `backend/internal/httpapi/handlers/dashboard.go` (`BackupStats`, `pivotDestinationBuckets`), `frontend/src/views/DashboardView.vue` | `backend/internal/repository/dashboard_test.go`: `TestBackupRunRepo_CountAndBytesByDestination` (3 subtestes: agrupamento por destino atual + soma de bytes/contagem, granularidade mensal, janela vazia) — executado e passando contra Postgres real, inclusive o subteste que expôs o bug de timezone do `date_trunc`; `backend/internal/httpapi/handlers/dashboard_test.go`: `TestPivotDestinationBuckets` (6 subtestes puros, sem DB: destino esparso, múltiplos destinos no mesmo bucket, soma de 2 runs no mesmo bucket/destino, fallback de destino desconhecido, bucket fora do intervalo de labels) — achado do Validator em 2026-09-16, ausente na primeira versão desta tarefa; `backend/internal/httpapi/dashboard_test.go`: `TestDashboardBackupStatsEndpoint` — executado e passando contra Postgres real; `frontend/src/views/DashboardView.spec.ts`: 4 novos testes (estado vazio dos 4 gráficos, gráficos com dado real, falha parcial do `Promise.all`) | 2026-09-16    |

Regra `confirmada` sem teste é dívida técnica — registrar em `docs/Progresso.md`. RN-AUTH-002/003, RN-BACKUP-016, RN-BACKUP-022, RN-BACKUP-024, RN-BACKUP-025, RN-BACKUP-026, RN-BACKUP-027, RN-BACKUP-028 e RN-BACKUP-029 têm teste de integração executado e passando contra um Postgres 16 descartável real nesta sessão (ou, no caso de `StreamCommand`/`TestExecuteBackup_SlogNeverContainsRawSecretOnRemoteCommandFailure`, contra um servidor SSH real minimalista criado no próprio teste). RN-BACKUP-022 ainda não tem teste de integração end-to-end cobrindo o `BackupExecutor` completo rodando via SSH real contra um `pg_dump`/`mysqldump` de verdade no caminho de **sucesso** (exigiria infraestrutura de servidor SSH de teste com as ferramentas de dump instaladas, inexistente neste projeto) — o método que essa correção passou a chamar (`MarkSuccess`) é, em si, testado diretamente e prova que a persistência funciona; a lacuna restante é só sobre a orquestração de ponta a ponta do caminho feliz, não sobre a correção do bug. RN-BACKUP-027, por outro lado, ganhou nesta tarefa exatamente esse teste de orquestração ponta a ponta (para o caminho de falha, que é o de maior risco de segurança) — ver acima. As demais regras acima seguem sem cobertura confirmada, e nenhuma delas (exceto as citadas) é `confirmada` ainda — esta seção será revisitada assim que o usuário confirmar as regras `⚠ inferida`.

---

## Regras pendentes e conflitos

### Pendentes de levantamento

| Item                               | Tela / entidade        | O que falta decidir                                                                     | Bloqueia                          |
| ---------------------------------- | ---------------------- | --------------------------------------------------------------------------------------- | --------------------------------- |
| RN-BACKUP-004                      | T-10 / RetentionPolicy | Confirmar se existe validação real impedindo `recentCount=0 AND monthlyCount=0`, e onde | Rastreabilidade completa da regra |
| Papéis de usuário                  | Global                 | Confirmar se o sistema é realmente single-role ou se há papéis não usados ainda         | Matriz de permissões completa     |
| Todas as regras `⚠ inferida` acima | Todas                  | Confirmação do usuário de que descrevem comportamento intencional, não bug              | Promoção a `confirmada`           |

### Conflitos detectados

| ID  | Regra                      | O que o código faz                                                                                                                             | O que o negócio espera                                             | Situação               |
| --- | -------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------ | ---------------------- |
| —   | Azure vs. Storage genérico | `CLAUDE.md` anterior descrevia apenas Azure (`azure_targets`); código já suporta S3 e filesystem via `storage_targets` desde a migração 000005 | `CLAUDE.md` corrigido nesta tarefa para refletir `storage_targets` | resolvido nesta tarefa |

---

## Histórico

| Data       | Regra                                                                                                | Alteração            | Motivo                                                                                                                                                                                                                                       | Plano                                                                                  |
| ---------- | ---------------------------------------------------------------------------------------------------- | -------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------- |
| 2026-09-15 | RN-BACKUP-001 a 015, RN-STORAGE-001, RN-AUTH-001                                                     | criadas (⚠ inferida) | `docs/` estava ausente; regras reconstruídas do código para `/init-project --update`                                                                                                                                                         | `.claude/plans/Backapeando-2026-09-15-17-41-inicializacao-governanca.md`               |
| 2026-09-15 | RN-AUTH-002, RN-AUTH-003                                                                             | criadas (confirmada) | CRUD de administradores via web — decisão explícita do usuário de bloquear autoexclusão e exclusão do último admin                                                                                                                           | `.claude/plans/Backapeando-2026-09-15-18-16-crud-admin-users-e-fix-historico.md`       |
| 2026-09-15 | RN-BACKUP-016                                                                                        | criada (confirmada)  | Correção de bug: `UpdateStatus` nunca persistia `started_at`/`finished_at`, deixando toda execução processada pelo scheduler sem esses tempos na tela de Histórico                                                                           | `.claude/plans/Backapeando-2026-09-15-18-16-crud-admin-users-e-fix-historico.md`       |
| 2026-09-15 | RN-BACKUP-017 a 021 (⚠ inferida, exceto RN-BACKUP-021 confirmada); RN-BACKUP-001, 008, 013 revisadas | criadas/revisadas    | Suporte a MySQL/SQL Server além de Postgres, seletor Docker-ou-host, senha de banco criptografada, e prévia do comando de dump no formulário de servidor                                                                                     | `.claude/plans/Backapeando-2026-09-15-20-53-multi-engine-docker-host-preview.md`       |
| 2026-09-15 | RN-BACKUP-022 a 025                                                                                  | criadas (confirmada) | Correção de bug (duração de dump/upload não persistida pelo scheduler), resolução de container Docker por nome parcial (grep), ativação/desativação manual de servidor via `enabled`, e histórico paginado (50/página) com filtro por status | `.claude/plans/Backapeando-2026-09-15-21-10-duracoes-grep-container-disable-server.md` |
| 2026-09-16 | RN-BACKUP-026 a 028                                                                                  | criadas (confirmada) | Histórico combina todos os servidores por padrão (`GET /api/backup-runs`); `error_message`/`log_output` de backup trazem detalhe real da falha, redigido de segredo; correção do bug de backup preso em `running` para sempre (worker pool não contava workers ocupados corretamente, sem timeout de execução, scheduler empilhava runs atrás de um travado) — de-dup na claim, timeout configurável, e watchdog | `.claude/plans/Backapeando-2026-09-16-09-19-historico-todos-servidores-erro-detalhado-fix-cron.md` |
| 2026-09-16 | RN-BACKUP-028 (caso não previsto resolvido), RN-BACKUP-029 (criada, confirmada)                     | atualizada/criada    | Corrigido o gap de bootstrap de `servers.next_run_at` (nunca calculado na primeira transição para `ready` fora do próprio scheduler) e adicionado teste e2e de defesa em profundidade capturando `slog` real contra um servidor SSH de teste, confirmando que a senha do banco nunca aparece em texto puro no log em caso de falha remota | `.claude/plans/verifique-o-impacto-e-dapper-duckling.md` |
| 2026-09-16 | RN-BACKUP-029 (caso não previsto resolvido), RN-BACKUP-030 (criada, confirmada)                     | atualizada/criada    | Editar `cronExpression` de um servidor já agendado via `PUT /api/servers/{id}` agora recalcula `next_run_at` imediatamente (não espera mais o próximo `claimAndEnqueue`); recálculo restrito a servidores já agendados, deixando o bootstrap da primeira vez (RN-BACKUP-029) intocado | `.claude/plans/fa-a-a-analise-e-nested-llama.md` |
| 2026-09-16 | RN-BACKUP-031 (criada, ⚠ inferida)                                                                   | criada               | Dashboard: gráfico de tamanho/duração por execução substituído por 4 gráficos de barras empilhadas por destino (contagem e soma de bytes, últimos 30 dias diário + ano corrente mensal). Novo endpoint `GET /api/dashboard/backup-stats`. Destino resolvido via JOIN com `servers.storage_target_id` atual, sem nova migration (decisão do usuário). Corrigido durante a implementação: bug de timezone no `date_trunc` da nova query (truncava no fuso da sessão do Postgres, não em UTC) | `.claude/plans/Backapeando-2026-09-16-15-23-dashboard-graficos-por-destino.md` |
