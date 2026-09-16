# Harness Multiagente — Backapeando

## Objetivo

Garantir que toda tarefa relevante seja planejada, implementada, validada, testada e revisada quanto à segurança.

## Fluxo obrigatório

1. Leitura das instruções globais.
2. Leitura das instruções locais.
3. Leitura da documentação, incluindo `docs/RegrasNegocio.md`.
4. Recuperação de contexto conforme `docs/RAG.md`.
5. Planner.
6. Coder.
7. Validator.
8. Tester.
9. Security Specialist.
10. Correções, se necessárias.
11. Nova validação.
12. Sincronização obrigatória de documentação — os onze documentos avaliados, os impactados atualizados ou criados, os demais declarados `sem alteração` ou `n/a`.

## Quem executa cada papel

Papel é responsabilidade; agente é quem a executa. Preenchido com o que foi detectado neste ambiente em 2026-09-15 (`_scan.sh capacidades` — 29 agentes carregáveis).

| Papel | Agente preferido | Alternativa | Neste projeto |
| --- | --- | --- | --- |
| Planner | `planner` | `task-decomposition-expert` | `planner` |
| Coder | `coder` | especialista do domínio (`backend-dev`, `frontend-developer`) | `coder` |
| Validator | `code-reviewer` | `reviewer` | `code-reviewer` |
| Tester | `test-engineer` | — | `test-engineer` |
| Security Specialist | `security-auditor` | — | `security-auditor` |

Especialistas que o Coder aciona quando o assunto pedir (todos detectados neste ambiente):

| Assunto | Agente |
| --- | --- |
| Arquitetura de backend | `backend-architect`, `system-architect` |
| Modelagem e migrações | `database-architect` |
| Frontend | `frontend-developer`, `ui-ux-designer` |
| Docker, deploy, CI/CD | `devops-engineer`, `deployment-engineer`, `cicd-engineer` |
| Documentação de API | `api-documenter`, `api-docs` |
| Documentação geral | `documentation-expert` |

Regras:

- **`security-auditor` só tem `Read`, `Grep` e `Glob`** — auditoria não escreve. As correções que ele apontar voltam para o Coder.
- Delegar isola contexto, mas custa uma rodada. Tarefa trivial não delega.
- O Coder continua sem poder aprovar o próprio trabalho, delegando ou não.
- Se um agente não existir no ambiente, o papel roda inline. Ausência de agente nunca dispensa o papel.

## Planner

Responsabilidades:

- Ler `~/.claude/CLAUDE.md`.
- Ler `./CLAUDE.md`.
- Ler `docs/RegrasNegocio.md` e identificar as regras afetadas.
- Ler a documentação aplicável.
- Verificar o Git.
- Criar um plano curto em `.claude/plans/`.
- Identificar riscos técnicos e de segurança.
- Listar os IDs das regras que serão criadas ou alteradas.
- Preencher no plano a seção `## Sincronização de documentação` com o estado **previsto** dos onze documentos.
- Definir critérios de aceite.

Nome do plano: `<nome-da-pasta-do-projeto>-<AAAA-MM-DD-HH-MM>-<sufixo-descritivo>.md`.

## Coder

Responsabilidades:

- Implementar conforme o plano.
- Consultar `docs/RegrasNegocio.md` antes de alterar comportamento.
- Parar e perguntar se a mudança contradiz uma regra `confirmada`.
- Consultar a documentação antes de alterar APIs, frontend, autenticação, banco ou infraestrutura.
- Criar ou atualizar testes.
- Executar a sincronização obrigatória de documentação e preencher a tabela com o estado **real**.
- Regenerar `docs/arquitetura.html` via skill `archify` quando `docs/Arquitetura.md` mudar (disponível neste ambiente).
- Criar o documento condicional ausente cujo gatilho disparou, a partir do template correspondente.
- Não incluir segredos.
- Não aprovar o próprio trabalho.

### Tabela de sincronização

Conforme a seção `Sincronização obrigatória de documentação` de `~/.claude/CLAUDE.md`, que é a fonte única.

| Documento | Gatilho | Sem gatilho |
| --- | --- | --- |
| `docs/RegrasNegocio.md` | comportamento, validação, permissão, máquina de estado ou regra de cálculo mudou | `sem alteração` |
| `docs/Arquitetura.md` | camada, módulo, padrão, dependência, fluxo de dados, schema ou decisão arquitetural mudou — regenerar também `docs/arquitetura.html` via skill `archify` | `sem alteração` |
| `docs/Organograma.md` | módulo, camada, fluxo de negócio, tela, integração externa ou relação entre componentes mudou de forma que o diagrama fique desatualizado | `sem alteração` |
| `docs/Infraestrutura.md` | Docker, Compose, deploy, rede, volume, variável de ambiente, build ou observabilidade mudou | `sem alteração` / `n/a` |
| `docs/API.md` | rota, método, payload, header, código HTTP, autenticação de endpoint, paginação ou filtro mudou | `sem alteração` / `n/a` |
| `docs/Frontend.md` | componente, tela, rota de UI, token de design, padrão de estado ou acessibilidade mudou | `sem alteração` / `n/a` |
| `docs/Auth.md` | authn, authz, sessão, token, papel, permissão ou política de senha mudou | `sem alteração` / `n/a` |
| `docs/RAG.md` | fonte indexada, corpus, exclusão de segurança ou capacidade do ambiente mudou | `sem alteração` |
| `docs/Progresso.md` | **sempre** — toda tarefa concluída gera uma linha | nunca `sem alteração` |
| `docs/Memoria.md` | aprendizado não óbvio, armadilha de ambiente, comando descoberto ou decisão com motivo | `sem alteração` |
| `docs/Harness.md` | papel, agente, fluxo de aprovação ou ferramenta do harness mudou | `sem alteração` |

`docs/arquitetura.html` é o companion renderizado de `docs/Arquitetura.md`, gerado pela skill `archify`. Não é um documento novo — não altera a contagem dos onze.

## Validator

Verificar:

- Aderência ao plano.
- Aderência à arquitetura.
- Aderência às regras de negócio documentadas.
- Se houve mudança de comportamento sem regra correspondente registrada.
- Se regras novas têm ID, status e rastreabilidade.
- Se `docs/Organograma.md` foi tocado, se o diagrama Mermaid ainda reflete a lógica real do projeto.
- A tabela de sincronização **contra o diff real**.
- Qualidade, tratamento de erros, compatibilidade, casos extremos, documentação, testes, convenções do projeto (`docs/Arquitetura.md`).

Resultado: `APPROVED` ou `REJECTED` com problema/motivo/correção.

## Tester

Executar, quando aplicável:

- `go test ./... -v` (unitário); `go test ./internal/httpapi/... -v` requer `DATABASE_URL`.
- `go vet ./...`, `gofmt -l .`.
- `npm run test` (Vitest, frontend); `npm run lint`.
- Testes de API, autenticação, das regras de negócio alteradas.
- Build (`go build ./...`, `npm run build`).
- Testes Docker/regressão, quando o risco justificar.

Registrar comandos e resultados reais. Resultado: `APPROVED` ou `REJECTED`.

**Nota conhecida (`docs/Arquitetura.md`, `CLAUDE.md`):** há uma data race pré-existente em `backend/internal/scheduler/pool_test.go` (`TestPoolStart`, `TestPoolTaskError`, `TestPoolTaskFieldsPreserved`), só visível com `go test -race`; conhecida, não corrigida.

## Security Specialist

Analisar toda implementação quanto a: SQL/command injection, XSS, CSRF, SSRF, path traversal, validação de entrada, Argon2id, senhas em texto puro, IDOR/BOLA, rate limiting, segredos em commits, `.env` versionado, CORS, headers, containers root, secrets em Dockerfile/Compose — lista completa em `~/.claude/CLAUDE.md`.

Pendências já conhecidas neste projeto (ver `docs/Progresso.md`, `docs/Infraestrutura.md`, `docs/Auth.md`):

- `MASTER_ENCRYPTION_KEY` de dev versionada em `docker-compose.yml`.
- `LoginRateLimiter`/`noOpRateLimiter` deprecated em `internal/auth/ratelimit.go` (bypass silencioso de rate limit se mal-instanciado).
- Imagens runtime `api`/`worker` em `alpine:latest`, não fixada.

Resultado: `APPROVED` ou `REJECTED` com severidade/arquivo/problema/impacto/ação corretiva/teste recomendado. Falhas críticas ou altas bloqueiam a conclusão.

## Aprovação final

A tarefa somente está concluída quando Validator, Tester e Security Specialist retornarem `APPROVED`, os testes aplicáveis foram executados, e a sincronização obrigatória de documentação foi concluída com os onze documentos avaliados.

## Registro

Registrar em `docs/Progresso.md`: data, plano, descrição, arquivos alterados, regras de negócio criadas/alteradas, número de rodadas, resultado de Validator/Tester/Security Specialist, testes executados, riscos encontrados, documentos `atualizado`/`criado` da sincronização.

## Histórico

| Data | Alteração | Motivo |
| --- | --- | --- |
| 2026-09-15 | Documento criado do zero a partir do template, com a coluna "Neste projeto" preenchida pelos agentes reais detectados no ambiente | `/init-project --update`, `docs/` ausente antes desta tarefa |
