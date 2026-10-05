# AGENTS.md — Backapeando

## Ordem obrigatória de leitura

Antes de qualquer plano, leitura de código, comando ou alteração:

1. `~/.cursor/rules/personal-md-global.mdc`
2. `./AGENTS.md` (este arquivo)
3. `./docs/RegrasNegocio.md`
4. `./docs/Arquitetura.md`
5. `./docs/Organograma.md`
6. `./docs/Infraestrutura.md`
7. `./docs/API.md`
8. `./docs/Frontend.md`
9. `./docs/Auth.md`
10. `./docs/RAG.md`
11. `./docs/Progresso.md`
12. `./docs/Memoria.md`
13. `./docs/Harness.md`

## Regras obrigatórias

- Ler a regra global do Cursor antes deste arquivo.
- Ler este arquivo antes de criar planos ou executar alterações.
- Consultar `docs/RegrasNegocio.md` antes de alterar qualquer comportamento do sistema.
- Atualizar `docs/RegrasNegocio.md` sempre que o comportamento mudar.
- Seguir o protocolo retrieval-first de `docs/RAG.md` antes de leitura ampla de código.
- Utilizar o MCP de memória no início da sessão e após tarefas significativas.
- Utilizar RTK conforme as instruções globais.
- Iniciar tarefas não triviais em modo plan.
- Criar planos em `.cursor/plans/`, sempre na raiz deste projeto (onde está este arquivo).
- Não sobrescrever arquivos sem confirmação.
- Consultar documentação antes de alterar API, frontend, autenticação, banco ou infraestrutura.
- Executar a sincronização obrigatória de documentação ao fim de **toda** implementação.
- Utilizar Planner, Coder, Validator, Tester e Security Specialist.
- Uma tarefa só está concluída após aprovação do Validator, Tester e Security Specialist e com a tabela de sincronização preenchida.

## Stack confirmada

- Objetivo: gerenciar backups de PostgreSQL/MySQL/SQL Server em servidores remotos via SSH, com retenção GFS e upload para Azure/S3/filesystem.
- Frontend: Vue 3 + Vite + TypeScript + Tailwind CSS + shadcn-vue (parcial)
- Backend: Go 1.26, stdlib `net/http` (ServeMux)
- Banco de dados: PostgreSQL 18+ via `pgx/v5` (SQL manual, sem ORM)
- Autenticação: sessão HTTP em cookie (revogável) + CSRF double-submit; senhas com Argon2id
- API própria: sim (REST, mesma origem via Nginx/Traefik)
- APIs externas: Azure Blob, S3-compatível (conforme storage targets)
- Docker: Compose (dev) e Swarm (prod); imagens `c3t4r4/backapeando:{backend,worker,frontend}-vX.Y.Z`
- Testes: `go test` (unitário + integração) e Vitest (frontend)

## Comandos

- Instalação: `docker compose up -d` (raiz); frontend: `cd frontend && npm ci`
- Desenvolvimento: `docker compose up -d`; frontend local `npm run dev` (proxy `/api` → `:8081`)
- Build imagens prod: `./build-images.sh` (bump de `prod-version` + rewrite de `docker-stack.yml`)
- Deploy Swarm: `docker stack deploy -c docker-stack.yml backapeando`
- Lint frontend: `cd frontend && ./node_modules/.bin/eslint src --ext .vue,.ts`
- Build frontend: `cd frontend && npm run build`
- Testes backend: `cd backend && go test ./...`
- Testes frontend: `cd frontend && npm test`
- Health: `curl -sS https://<APP_DOMAIN>/api/health` (deve incluir `"version"`)

## Regras de negócio

- Consultar `docs/RegrasNegocio.md` antes de alterar telas, fluxos, validações ou lógica de decisão.
- Verificar se a mudança pedida contradiz alguma regra `confirmada`; se contradisser, parar e perguntar.
- Registrar toda regra nova com ID estável (`RN-<DOMINIO>-<NNN>`).
- Regras extraídas do código entram como `⚠ inferida` até confirmação do usuário.
- Nunca apagar regra: marcar como `revogada`, com data e motivo.

## Sincronização de documentação

Regra herdada de `~/.cursor/rules/personal-md-global.mdc`. Toda tarefa que altere código, configuração, schema, API, autenticação, infraestrutura ou documentação técnica só está concluída depois que os onze documentos abaixo forem avaliados com estado explícito.

| Documento | Gatilho | Sem gatilho |
| --- | --- | --- |
| `docs/RegrasNegocio.md` | comportamento, validação, permissão, máquina de estado ou regra de cálculo mudou | `sem alteração` |
| `docs/Arquitetura.md` | camada, módulo, padrão, dependência, fluxo de dados, schema ou decisão arquitetural mudou — regenerar também `docs/arquitetura.html` via skill `archify`, quando disponível | `sem alteração` |
| `docs/Organograma.md` | módulo, camada, fluxo de negócio, tela, integração externa ou relação entre componentes mudou de forma que o diagrama fique desatualizado | `sem alteração` |
| `docs/Infraestrutura.md` | Docker, Compose, deploy, rede, volume, variável de ambiente, build ou observabilidade mudou | `sem alteração` / `n/a` |
| `docs/API.md` | rota, método, payload, header, código HTTP, autenticação de endpoint, paginação ou filtro mudou | `sem alteração` / `n/a` |
| `docs/Frontend.md` | componente, tela, rota de UI, token de design, padrão de estado ou acessibilidade mudou | `sem alteração` / `n/a` |
| `docs/Auth.md` | authn, authz, sessão, token, papel, permissão ou política de senha mudou | `sem alteração` / `n/a` |
| `docs/RAG.md` | fonte indexada, corpus, exclusão de segurança ou capacidade do ambiente mudou | `sem alteração` |
| `docs/Progresso.md` | **sempre** — toda tarefa concluída gera uma linha | nunca `sem alteração` |
| `docs/Memoria.md` | aprendizado não óbvio, armadilha de ambiente, comando descoberto ou decisão com motivo | `sem alteração` |
| `docs/Harness.md` | papel, agente, fluxo de aprovação ou ferramenta do harness mudou | `sem alteração` |

Documentos condicionais neste projeto: todos existem (`Infraestrutura`, `API`, `Frontend`, `Auth`).

## Regras de recuperação de contexto

- Seguir a ordem definida em `docs/RAG.md`.
- Nunca indexar `.env`, secrets, chaves, tokens ou dados pessoais.
- `/learn-codebase` é opt-in e caro — só com confirmação explícita.

## Regras de autenticação

- Senhas: Argon2id apenas. Ver `docs/Auth.md`.
- Papel único: `admin`.

## Regras de API

- Consultar e atualizar `docs/API.md` em toda mudança de contrato.
- Expurgo global: `POST /api/retention-sweep` (202) e `GET /api/retention-sweep/latest`.
- Se `POST /api/retention-sweep` retornar 404 em produção: verificar `GET /api/health` — ausência de `version` ou versão antiga indica binário Swarm desatualizado (não bug de path). Ver `docs/Infraestrutura.md`.

## Regras de frontend

- Consultar `docs/Frontend.md`. Vue 3 + Tailwind; preferir componentes existentes.

## Regras de infraestrutura

- Tags de versão fixa em `docker-stack.yml` (`backend-vX.Y.Z`), nunca confiar só em `-latest` + `--force`.
- Após `./build-images.sh`, sempre `docker stack deploy -c docker-stack.yml backapeando`.
- Ver `docs/Infraestrutura.md`.

## Fluxo multiagente

1. Planner → plano em `.cursor/plans/`
2. Coder → implementação + sync docs
3. Validator, Tester, Security Specialist → `APPROVED` obrigatório

## Restrições conhecidas

- Deploy Swarm/Traefik/Portainer geridos fora deste repositório.
- Retenção pós-backup é não-fatal (backup pode suceder com sweep falho — ver logs).
- Prefixo de blobs é `servers.blob_prefix` (fixado na criação, RN-BACKUP-034); rename de `name` não move blobs. Renames **antes** dessa coluna podem ter orphans sob o slug antigo.
