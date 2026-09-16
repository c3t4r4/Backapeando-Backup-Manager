# Organograma — Backapeando

## Objetivo

Mapa visual vivo da lógica do projeto: como os módulos se relacionam, como o fluxo de negócio principal atravessa o sistema, como as telas se conectam e quais integrações externas existem.

Este documento é complementar a `docs/Arquitetura.md`, não um substituto. Sempre que a lógica do projeto mudar de um jeito que tornaria um destes diagramas enganoso, este arquivo precisa ser atualizado — ver `## Regras de alteração`.

## Visão geral de módulos

```mermaid
flowchart TD
    Frontend["frontend/ (Vue 3 SPA)"] --> API["backend/cmd/api (HTTP)"]
    API --> Auth["internal/auth (sessão, CSRF, Argon2id, rate limit)"]
    API --> Repos["internal/repository (pgx)"]
    Repos --> DB[("PostgreSQL")]
    Worker["backend/cmd/worker (Scheduler)"] --> Repos
    Worker --> SSH["internal/sshclient (TOFU)"]
    Worker --> Storage["internal/storage (Azure/S3/filesystem)"]
    Worker --> Retention["internal/retention (GFS puro)"]
    API -.-> Redis[("Redis — rate limit")]
```

## Fluxo de negócio principal

Fluxo de um backup agendado, do disparo à retenção. `docs/RegrasNegocio.md` é a fonte da verdade textual das regras por trás de cada passo.

```mermaid
flowchart LR
    Cron(["Ciclo de polling do worker"]) --> Claim{"Servidor elegível?<br/>enabled=true, status=ready,<br/>storage_target_id != null"}
    Claim -->|"Sim (SKIP LOCKED)"| Create["Cria backup_run (running)"]
    Claim -->|"Não"| Fim1(["Aguarda próximo ciclo"])
    Create --> SSHConn["SSH dial + TOFU"]
    SSHConn -->|"falha"| Failed(["backup_run = failed"])
    SSHConn -->|"sucesso"| Dump["pg_dump/mysqldump/sqlcmd (docker exec opcional, stream)"]
    Dump --> Upload["Upload para storage target"]
    Upload -->|"falha"| Failed
    Upload -->|"sucesso"| Success(["backup_run = success"])
    Success --> Sweep["Varredura de retenção GFS (não-fatal)"]
    Sweep --> Fim2(["Ciclo concluído"])
```

## Navegação de telas

Rota → tela, espelhando o índice de telas de `docs/RegrasNegocio.md`.

```mermaid
flowchart TD
    Login["/login → Login"] --> Dashboard["/dashboard → Dashboard"]
    Dashboard --> Servers["/servers → Servidores"]
    Dashboard --> History["/history → Histórico"]
    Dashboard --> StorageTargets["/storage-targets → Destinos de armazenamento"]
    Dashboard --> Settings["/settings → Configurações"]
    Dashboard --> AdminUsers["/admin-users → Administradores"]
    Servers --> ServerNew["/servers/new → Novo servidor"]
    Servers --> ServerEdit["/servers/:id/edit → Editar servidor"]
    StorageTargets --> StorageNew["/storage-targets/new → Novo destino"]
    StorageTargets --> StorageEdit["/storage-targets/:id/edit → Editar destino"]
    AdminUsers --> AdminUserNew["/admin-users/new → Novo administrador"]
    AdminUsers --> AdminUserEdit["/admin-users/:id/edit → Editar administrador"]
```

## Integrações externas

```mermaid
flowchart LR
    Sistema["Backapeando (worker)"] --> Azure["Azure Blob Storage"]
    Sistema --> S3["S3 / compatível"]
    Sistema --> FS["Filesystem / NFS local"]
    Sistema --> SSHRemoto["Servidor de banco remoto — Postgres/MySQL/SQL Server (via SSH)"]
    Sistema -.-> Redis["Redis (rate limit de login)"]
```

## Regras de alteração

Gatilho de atualização deste documento: módulo, camada, fluxo de negócio, tela, integração externa ou relação entre componentes mudou de forma que algum diagrama acima fique desatualizado.

Antes de alterar:

1. Consultar este arquivo.
2. Consultar `docs/Arquitetura.md` e `docs/RegrasNegocio.md` para confirmar se a mudança é real e não apenas de implementação interna.
3. Implementar a mudança.
4. Atualizar o(s) diagrama(s) afetado(s) — adicionar, remover ou renomear nós conforme a realidade nova.
5. Remover nó ou seção que deixou de existir; não deixar diagrama descrevendo algo que já foi removido do projeto.
6. Atualizar o histórico.

## Histórico

| Data | Área | Alteração | Motivo |
| --- | --- | --- | --- |
| 2026-09-15 | Todos os diagramas | Documento criado do zero a partir do código real, ausente antes desta tarefa | `/init-project --update` |
| 2026-09-15 | Navegação de telas | Adicionados nós `/admin-users`, `/admin-users/new`, `/admin-users/:id/edit` | CRUD de usuário administrador solicitado pelo usuário |
| 2026-09-15 | Fluxo de negócio principal, Integrações externas | Nó de dump generalizado de "docker exec pg_dump" para "pg_dump/mysqldump/sqlcmd (docker exec opcional)"; nó de integração generalizado de "Servidor PostgreSQL remoto" para "Servidor de banco remoto — Postgres/MySQL/SQL Server" | Suporte a MySQL/SQL Server e a bancos rodando direto no host, sem Docker |
