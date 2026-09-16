# Infraestrutura — Backapeando

> Consultar antes de alterar Docker, Compose, deploy, redes, volumes ou variáveis de ambiente.
> **Este arquivo registra apenas nomes e finalidades. Nunca valores de secrets.**

## Ambientes

| Ambiente        | Finalidade | Onde roda                                         | Branch                          | Observações                                              |
| --------------- | ---------- | ------------------------------------------------- | ------------------------------- | -------------------------------------------------------- |
| Desenvolvimento | Uso local  | Docker Compose (`docker-compose.yml`)             | `main` (única branch existente) | Portas expostas ao host (`5432`, `8081`, `3000`, `6379`) |
| Homologação     | —          | `a definir` — sem stack/branch dedicada detectada | `a definir`                     | —                                                        |
| Produção        | Produção   | Docker Swarm (`docker-compose.prod.yml`)          | `main`                          | Traefik/Portainer geridos fora deste repositório         |

## Docker

| Item                      | Valor                                                                                 |
| ------------------------- | ------------------------------------------------------------------------------------- |
| Docker no desenvolvimento | Sim                                                                                   |
| Docker nos testes         | Parcial — testcontainers citado em `CLAUDE.md`; integração requer `DATABASE_URL` real |
| Docker no deploy          | Sim                                                                                   |
| Orquestração              | Compose (dev) / Swarm (prod)                                                          |
| Deploy via Portainer      | Gerido fora deste repositório                                                         |
| Proxy reverso             | Nginx (frontend, dev) / Traefik (prod, gerido fora deste repositório)                 |

### Imagens

| Imagem                                  | Origem                                              | Tag                        | Usuário do container           | Observações                                                                               |
| --------------------------------------- | --------------------------------------------------- | -------------------------- | ------------------------------ | ----------------------------------------------------------------------------------------- |
| `golang:1.26-alpine` → binário `api`    | build multi-stage (`backend/cmd/api/Dockerfile`)    | `alpine:latest` no runtime | `appuser` (uid 1000, não-root) | Healthcheck `wget /api/health`                                                            |
| `golang:1.26-alpine` → binário `worker` | build multi-stage (`backend/cmd/worker/Dockerfile`) | `alpine:latest` no runtime | `appuser` (uid 1000, não-root) | Healthcheck baseado em `pgrep`, sem porta exposta                                         |
| `node:20-alpine` → build Vue            | build multi-stage (`frontend/Dockerfile`)           | `nginx:alpine` no runtime  | `nginx` (imagem base)          | Healthcheck `wget /health` via `127.0.0.1` (não `localhost`, para evitar resolução `::1`) |
| `postgres:16-alpine`                    | Docker Hub                                          | `16-alpine`                | padrão da imagem               | —                                                                                         |
| `redis:7-alpine`                        | Docker Hub                                          | `7-alpine`                 | padrão da imagem               | Usado para rate limiting de login                                                         |

**Observação:** `alpine:latest` (runtime de `api` e `worker`) usa tag `latest`, não fixada — inconsistente com a regra "nunca `latest` em produção". Registrado como pendência.

### Serviços (docker-compose.yml — dev)

| Serviço    | Imagem/Build                    | Portas expostas | Depende de                              | Healthcheck                       |
| ---------- | ------------------------------- | --------------- | --------------------------------------- | --------------------------------- |
| `postgres` | `postgres:16-alpine`            | `5432:5432`     | —                                       | `pg_isready`                      |
| `api`      | `backend/cmd/api/Dockerfile`    | `8081:8081`     | `postgres` (healthy), `redis` (healthy) | `wget /api/health`                |
| `worker`   | `backend/cmd/worker/Dockerfile` | nenhuma         | `postgres` (healthy)                    | processo (`pgrep`, no Dockerfile) |
| `web`      | `frontend/Dockerfile`           | `3000:80`       | `api`                                   | `wget /health`                    |
| `redis`    | `redis:7-alpine`                | `6379:6379`     | —                                       | `redis-cli ping`                  |

### Serviços (docker-compose.prod.yml — Swarm)

Serviços detectados: `postgres`, `redis`, `api`, `worker`, `web`, `backapeando_backend`, `backapeando_ingress` (nomes exatos — não auditados campo a campo nesta tarefa; consultar o arquivo diretamente antes de alterar).

### Redes

| Rede              | Tipo        | Externa     | Serviços conectados                             |
| ----------------- | ----------- | ----------- | ----------------------------------------------- |
| `internal` (dev)  | bridge      | Não         | postgres, api, worker, web, redis               |
| Redes de produção | `a definir` | `a definir` | consultar `docker-compose.prod.yml` diretamente |

### Volumes

| Volume                       | Tipo    | Conteúdo                       | Persistente | Backup                                                                                                                   |
| ---------------------------- | ------- | ------------------------------ | ----------- | ------------------------------------------------------------------------------------------------------------------------ |
| `postgres_data` (dev e prod) | nomeado | Dados do Postgres              | Sim         | `a definir` — o próprio sistema faz backup de _outros_ bancos; backup do seu próprio Postgres operacional não confirmado |
| `traefik_data` (prod)        | nomeado | Certificados/estado do Traefik | Sim         | `a definir`                                                                                                              |

Regra: bind mount de diretório amplo do host é proibido em produção — nenhum detectado nesta auditoria.

## Secrets

**Somente nomes e origem. Nenhum valor real neste arquivo, em commits ou em logs.**

| Nome                             | Finalidade                                    | Origem                                                                    | Ambientes                |
| -------------------------------- | --------------------------------------------- | ------------------------------------------------------------------------- | ------------------------ |
| `DATABASE_URL`                   | Conexão Postgres                              | Variável de ambiente (dev) / Docker secret `database_url` (prod)          | dev, prod                |
| `MASTER_ENCRYPTION_KEY`          | Chave AES-256-GCM (chaves SSH, tokens SAS/S3) | Variável de ambiente (dev) / Docker secret `master_encryption_key` (prod) | dev, prod                |
| `MASTER_ENCRYPTION_KEY_PREVIOUS` | Rotação de chave AES                          | Variável de ambiente / `_FILE`                                            | prod (quando em rotação) |
| `postgres_password`              | Senha do Postgres em produção                 | Docker secret                                                             | prod                     |

- Rotação: suportada para `MASTER_ENCRYPTION_KEY` via `MASTER_ENCRYPTION_KEY_PREVIOUS`; política de cadência `a definir`.
- Convenção `_FILE`: toda variável sensível aceita `<NOME>_FILE` apontando para um arquivo (padrão Docker Swarm secrets), consumida por `internal/config.getEnvOrFile`.

### ⚠ Achado de segurança — chave de desenvolvimento versionada

`docker-compose.yml` (rastreado no Git, commit `4914aed`) define
`MASTER_ENCRYPTION_KEY` em texto plano (valor real omitido deste documento —
consultar o arquivo versionado diretamente se precisar do valor) para os
serviços `api` e `worker`. É uma chave apenas de
desenvolvimento local (comentário `# dev only` ao lado de outra variável no
mesmo arquivo), sem uso conhecido fora deste ambiente — mas está **versionada
em texto plano**, tecnicamente violando a regra global "secrets nunca devem
ser versionados". Se este valor já foi usado para proteger qualquer dado que
não seja puramente descartável de desenvolvimento, tratar como comprometido.
Ação corretiva sugerida (não aplicada nesta tarefa — infraestrutura exige
confirmação do usuário antes de qualquer mudança): mover para `.env` local
não versionado, com `.env.example` documentando apenas o nome da variável.

## Variáveis de ambiente

| Variável                                   | Finalidade                                   | Obrigatória                                  | Origem              | Valor padrão                                   |
| ------------------------------------------ | -------------------------------------------- | -------------------------------------------- | ------------------- | ---------------------------------------------- |
| `HTTP_ADDR`                                | Endereço de bind da API                      | Não                                          | env                 | `:8081`                                        |
| `DATABASE_URL` / `DATABASE_URL_FILE`       | Conexão Postgres                             | Sim                                          | env / Docker secret | sem padrão                                     |
| `MASTER_ENCRYPTION_KEY` / `_FILE`          | Chave AES-256-GCM (base64, 32 bytes)         | Sim                                          | env / Docker secret | sem padrão                                     |
| `MASTER_ENCRYPTION_KEY_PREVIOUS` / `_FILE` | Chave anterior (rotação)                     | Não                                          | env / Docker secret | vazio                                          |
| `SESSION_COOKIE_NAME`                      | Nome do cookie de sessão                     | Não                                          | env                 | `backapeando_backup_session`                   |
| `SESSION_IDLE_TTL`                         | TTL de inatividade da sessão                 | Não                                          | env                 | `12h`                                          |
| `SESSION_ABSOLUTE_TTL`                     | Teto absoluto da sessão                      | Não                                          | env                 | `168h` (7 dias)                                |
| `SECURE_COOKIES`                           | Flag `Secure` dos cookies                    | Sim (falha fast se ausente)                  | env                 | sem padrão — deve ser `true`/`false` explícito |
| `CORS_ALLOWED_ORIGIN`                      | Origem única permitida para CORS             | Não                                          | env                 | vazio (CORS desabilitado, same-origin)         |
| `REDIS_URL`                                | Conexão Redis (rate limit)                   | `a definir` (worker define mas não usa hoje) | env                 | `redis://redis:6379` (dev)                     |
| `MAX_CONCURRENT_BACKUPS`                   | Tamanho do worker pool                       | `a definir`                                  | env                 | `3` (dev)                                      |
| `SCHEDULER_POLL_INTERVAL`                  | Intervalo de polling do scheduler (segundos) | `a definir`                                  | env                 | `30` (dev)                                     |
| `GRACEFUL_SHUTDOWN_TIMEOUT`                | Timeout de shutdown gracioso (segundos)      | `a definir`                                  | env                 | `300` (dev)                                    |
| `BACKUP_TASK_TIMEOUT`                      | Timeout máximo de uma execução de backup (segundos) — após o handshake SSH, que já tinha seu próprio timeout de 30s (RN-BACKUP-015) | Não | env | `21600` (6h) |
| `WATCHDOG_POLL_INTERVAL`                   | Intervalo do watchdog que marca como `failed` runs presos em `running` além de `BACKUP_TASK_TIMEOUT` (segundos) | Não | env | `300` (5min) |

- Arquivos de exemplo: `backend/.env.example` e `frontend/.env.example` (rastreados no Git). Há também um `.env-example` na raiz (sem ponto antes de "example", nome diferente do padrão) — checar se é intencional ou duplicata a consolidar.
- `.env` **deve** estar no `.gitignore` — já está (`!.env.example` cobre a exceção correta); `.env-example` (raiz, sem ponto) não corresponde ao padrão `.env.*` nem `.env.example`, então **não está coberto pela negação do `.gitignore`** hoje, mas por não começar com `.env` (ponto) ele também não é excluído pela primeira regra — verificar se isso é proposital.

## Build

```bash
# Backend
cd backend && go build ./...

# Frontend
cd frontend && npm run build

# Imagens (dev)
docker-compose build

# Imagens (prod) — pipeline de CI/CD ainda não coberto para o worker
docker stack deploy -c docker-compose.prod.yml backapeando-backup
```

## Deploy

### Procedimento

1. `a definir` — procedimento de deploy em produção não documentado nesta tarefa; consultar quem opera o Swarm/Traefik/Portainer (geridos fora deste repositório).

### Rollback

1. `a definir`

- Tempo estimado de rollback: `a definir`
- Responsável: `a definir`

## Observabilidade

| Item     | Solução                                                               | Onde consultar                                             |
| -------- | --------------------------------------------------------------------- | ---------------------------------------------------------- |
| Logs     | `log/slog` estruturado (JSON), `middleware.Logging` por requisição    | stdout do container (`docker logs` / Swarm logging driver) |
| Métricas | `a definir` — nenhuma solução de métricas (Prometheus etc.) detectada | —                                                          |
| Alertas  | `a definir`                                                           | —                                                          |

Regra: logs não podem conter credenciais, tokens nem dados pessoais desnecessários — `middleware.Recover` já evita vazar stack trace ao cliente; erros de SSH são sanitizados antes de log de resposta (`sanitizeSSHTestError`), mas o erro completo ainda vai para o log do servidor (`h.Logger`).

## Backup e recuperação

| Item                                                           | Frequência                       | Retenção            | Local                                            | Restauração testada em                                                |
| -------------------------------------------------------------- | -------------------------------- | ------------------- | ------------------------------------------------ | --------------------------------------------------------------------- |
| Backups gerenciados pelo sistema (bancos remotos dos clientes) | Configurável por servidor (cron) | GFS — RN-BACKUP-003 | Storage target configurado (Azure/S3/filesystem) | `a definir`                                                           |
| Backup do próprio Postgres operacional (`postgres_data`)       | `a definir`                      | `a definir`         | `a definir`                                      | `a definir` — nenhuma estratégia de backup do banco interno detectada |

## Pipeline de CI/CD

### Publicação do diagrama de arquitetura no GitHub Pages

Novo workflow (`.github/workflows/deploy-arquitetura-pages.yml`):

- Gatilho: `push` na branch `main` quando `docs/arquitetura.html` muda, + `workflow_dispatch` para disparo manual.
- Permissões: `contents: read`, `pages: write`, `id-token: write` (escopadas ao mínimo).
- Ação: Copia `docs/arquitetura.html` para `_site/index.html`, configura GitHub Pages via `actions/configure-pages@v5`, envia artefato via `actions/upload-pages-artifact@v3`, deploya via `actions/deploy-pages@v4`.
- Resultado: `docs/arquitetura.html` (gerado pela skill `archify`) fica disponível de forma interativa em `https://c3t4r4.github.io/Backapeando-Backup-Manager/` (derivado de `git remote`).
- **Pré-requisito**: GitHub Pages deve ser habilitado **uma única vez** via API antes do workflow funcionar — a auto-inicialização do `configure-pages` não completou automaticamente aqui. Habilitar via:
  ```bash
  gh api --method POST repos/{owner}/{repo}/pages -f "build_type=workflow"
  ```
  ou manualmente em Settings → Pages → Build and deployment → Source → "GitHub Actions".

### Build/publish de imagem do `worker`

Não coberto neste repositório para produção — o serviço já está declarado em `docker-compose.prod.yml`, mas o pipeline de build/publish de imagem ainda não existe (restrição já conhecida do `CLAUDE.md`). Esta linha diz respeito exclusivamente a imagens Docker do `worker`, não ao workflow de Pages acima.

## Regras de alteração

1. Consultar este arquivo.
2. Alterações de infraestrutura exigem confirmação do usuário antes de executar.
3. Nunca alterar redes, volumes ou secrets de produção sem autorização explícita.
4. Implementar.
5. Atualizar este arquivo.
6. Executar Security Specialist — infraestrutura **sempre** exige revisão de segurança.

## Histórico

| Data       | Alteração                                                                                                    | Ambiente  | Motivo                   | Plano                                                                    |
| ---------- | ------------------------------------------------------------------------------------------------------------ | --------- | ------------------------ | ------------------------------------------------------------------------ |
| 2026-09-15 | Documento criado do zero a partir de `docker-compose*.yml` e `Dockerfile`s reais, ausente antes desta tarefa | dev, prod | `/init-project --update` | `.claude/plans/Backapeando-2026-09-15-17-41-inicializacao-governanca.md` |
| 2026-09-16 | Novas env vars `BACKUP_TASK_TIMEOUT` (default 6h) e `WATCHDOG_POLL_INTERVAL` (default 5min) no `worker` — não adicionadas aos `docker-compose*.yml` (usam o default do código; adicionar explicitamente se o operador quiser um valor diferente) | dev, prod | Corrigir backup preso em `running` para sempre (nenhum timeout após o handshake SSH) | `.claude/plans/Backapeando-2026-09-16-09-19-historico-todos-servidores-erro-detalhado-fix-cron.md` |
| 2026-09-16 | Novo workflow `.github/workflows/deploy-arquitetura-pages.yml` para publicar `docs/arquitetura.html` no GitHub Pages; README atualizado com link renderizado | ci/cd | Publicar diagrama de arquitetura de forma interativa e acessível no GitHub Pages | `.claude/plans/Backapeando-2026-09-16-preciso-que-crie-um-actions-github-pages-arquitetura.md` |
