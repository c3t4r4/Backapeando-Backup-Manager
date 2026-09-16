# Autenticação e autorização — Backapeando

> Consultar antes de qualquer alteração em login, sessão, papéis ou permissões.

## Estratégia

| Item                 | Valor                                                                                                                            |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------------- |
| Tipo de autenticação | Usuário (e-mail) e senha                                                                                                         |
| Provedores externos  | nenhum                                                                                                                           |
| Mecanismo de sessão  | Cookie de sessão revogável no servidor (não JWT)                                                                                 |
| Expiração            | Idle TTL configurável (`SESSION_IDLE_TTL`, padrão 12h) com teto absoluto (`SESSION_ABSOLUTE_TTL`, padrão 7 dias) desde a criação |
| Refresh token        | Não — sessão desliza (idle) até o teto absoluto, sem token separado                                                              |
| MFA                  | Não implementado — sistema de operador único                                                                                     |
| Recuperação de senha | `./api reset-password` no host (sem SMTP), ou `PUT /api/admin-users/{id}` via web com sessão autenticada (desde 2026-09-15)      |

## Armazenamento de senhas

- Algoritmo obrigatório: **Argon2id** (`golang.org/x/crypto/argon2`)
- Parâmetros: `⚠ inferida` — CLAUDE.md anterior citava m=64MB, t=3, p=4, formato PHC; não confirmado byte a byte no código nesta tarefa
- Nunca utilizar MD5, SHA-1 ou SHA-256 puro para senhas.
- Nunca armazenar senha em texto puro.
- Nunca registrar senha em log, mensagem de erro ou telemetria.

## Tokens/Cookies

| Cookie                                                              | Formato                                      | Expiração                              | Onde é armazenado                                              | Revogação                                                     |
| ------------------------------------------------------------------- | -------------------------------------------- | -------------------------------------- | -------------------------------------------------------------- | ------------------------------------------------------------- |
| Sessão (`SESSION_COOKIE_NAME`, padrão `backapeando_backup_session`) | ID opaco de sessão                           | Idle TTL, deslizante até teto absoluto | `HttpOnly+SameSite=Strict`, `Secure` conforme `SECURE_COOKIES` | Servidor (`admin_sessions`), `SessionManager.Clear` na logout |
| CSRF (`backapeando_backup_csrf`)                                    | Token opaco, legível por JS (não `HttpOnly`) | Igual à sessão (emitido junto)         | `SameSite=Strict`, `Secure` conforme `SECURE_COOKIES`          | Limpo junto no logout                                         |

- Nome do cookie de sessão é **configurável** via `SESSION_COOKIE_NAME` (`internal/config/config.go`) — corrige a suposição anterior do `CLAUDE.md` de que era um valor fixo.
- Segredo/chave da sessão: não há assinatura de token — o ID de sessão é validado por lookup em `admin_sessions` (banco), não por HMAC/JWT.

## Papéis e permissões

| Papel | Descrição                                       | Concedido por                                                              |
| ----- | ----------------------------------------------- | -------------------------------------------------------------------------- |
| admin | Único papel identificado — opera todo o sistema | `./api create-admin` (CLI, no host) ou `POST /api/admin-users` (web, T-12) |

`⚠ inferida`: `domain.AdminUser` tem campo `Role`, mas não foi confirmado se há mais de um valor de papel em uso real — tratar como single-role até confirmação. O CRUD web de admin_users (T-11/T-12/T-13) hardcoda `role="admin"` na criação (sem seletor), consistente com essa suposição — se o sistema deixar de ser single-role no futuro, este ponto precisa ser revisitado.

**Gestão de contas via web (desde 2026-09-15):** `GET/POST/PUT/DELETE /api/admin-users*` permitem listar, criar, editar (e-mail/CPF/senha) e excluir administradores sem precisar de acesso ao host — decisão explícita do usuário, com duas guardas novas (RN-AUTH-002, RN-AUTH-003 em `docs/RegrasNegocio.md`): um admin não pode excluir a própria conta, e o último admin restante nunca pode ser excluído. O CLI (`create-admin`/`reset-password`) continua existindo como fallback. Não há checagem de `role` nesses endpoints — herdam apenas `RequireAuth`+`RequireCSRF`, como qualquer outra rota `/api/*`.

Matriz completa: `docs/RegrasNegocio.md`, seção `Regras transversais`.

## Middlewares e pontos de verificação

| Middleware / guarda    | Onde se aplica                                                     | O que verifica                                          | Resposta em falha                               |
| ---------------------- | ------------------------------------------------------------------ | ------------------------------------------------------- | ----------------------------------------------- |
| `RequireAuth`          | Todas as rotas `/api/*` exceto `publicPaths`                       | Cookie de sessão válido (`SessionManager.Authenticate`) | `401 {"error":"authentication required"}`       |
| `RequireCSRF`          | Todas as rotas mutantes (`!GET/HEAD/OPTIONS`) exceto `publicPaths` | Header `X-CSRF-Token` == cookie CSRF                    | `403 {"error":"csrf token missing or invalid"}` |
| `authGuard` (frontend) | Todas as rotas Vue Router com `meta.requiresAuth`                  | Estado local `isLoggedIn` do composable `useAuth`       | Redireciona para `/login`                       |

Regra: autorização verificada **no servidor** (`RequireAuth`/`RequireCSRF`). O `authGuard` do frontend é UX, não controle de acesso — esconder uma rota no cliente não substitui a checagem no servidor.

## Proteções obrigatórias

| Proteção                                | Situação                                                                                                            | Detalhe                                                                                                           |
| --------------------------------------- | ------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Rate limiting no login                  | Ativo                                                                                                               | `RedisRateLimiter`, fixed-window por IP, limite configurável (`maxTriesPerMin`) — CLAUDE.md anterior citava 5/min |
| Bloqueio após tentativas                | Ativo (via rate limit acima)                                                                                        | Sem bloqueio de conta separado detectado — apenas rate limit por IP                                               |
| Mensagem de erro genérica no login      | `a definir` — não confirmado se `AuthHandlers.Login` evita enumeração de e-mail                                     | —                                                                                                                 |
| Expiração de sessão inativa             | Ativo                                                                                                               | Idle TTL configurável, ver acima                                                                                  |
| Invalidação de sessão na troca de senha | `a definir` — não confirmado                                                                                        | —                                                                                                                 |
| Proteção CSRF                           | Ativo                                                                                                               | Double-submit cookie (`internal/auth/csrf.go`)                                                                    |
| Auditoria de acesso                     | `a definir` — logging estruturado existe (`middleware.Logging`), mas não confirmado como trilha de auditoria formal | —                                                                                                                 |

**⚠ Pendência técnica encontrada nesta auditoria:** `internal/auth/ratelimit.go` contém `LoginRateLimiter`/`NewLoginRateLimiter`, marcados `Deprecated`, que retornam um `noOpRateLimiter` (sempre permite). Se instanciados por engano em vez de `RedisRateLimiter`, o rate limit de login é silenciosamente desativado. Registrado em `docs/Progresso.md` como dívida técnica — não corrigido nesta tarefa (fora de escopo de inicialização de governança).

## Fluxos

### Login

1. `POST /api/auth/login` com e-mail e senha.
2. Rate limit por IP verificado antes de qualquer comparação de senha.
3. Senha comparada via Argon2id.
4. Sessão criada (`admin_sessions`), cookie de sessão e cookie CSRF emitidos.

### Recuperação de senha

1. Operador executa `./api reset-password` diretamente no host (acesso à máquina/container, não via API/HTTP).

- Sem token de recuperação por e-mail — adequado para ferramenta interna de operador único.

### Logout

1. `POST /api/auth/logout` — `SessionManager.Clear`: apaga a sessão em `admin_sessions` e limpa cookies de sessão e CSRF no browser.

## Dados sensíveis

- Nunca registrar em log: senhas, chaves SSH privadas, tokens SAS/S3, respostas de MFA (N/A — sem MFA).
- Nunca indexar credenciais em RAG — ver `docs/RAG.md`.
- Retenção de logs de autenticação: `a definir`.

## Testes obrigatórios

- [ ] Login com credenciais válidas.
- [ ] Login com credenciais inválidas.
- [ ] Acesso a recurso protegido sem sessão.
- [ ] Acesso a recurso protegido com sessão expirada (idle e absoluta).
- [ ] Rate limiting no login (Redis).
- [ ] Requisição mutante sem token CSRF.
- [ ] Autoexclusão de admin bloqueada (RN-AUTH-002).
- [ ] Exclusão do último admin restante bloqueada (RN-AUTH-003).
- [ ] E-mail/CPF duplicado em `POST`/`PUT /api/admin-users*` retorna `409`.

## Regras de alteração

1. Consultar este arquivo.
2. Consultar `docs/RegrasNegocio.md` para as regras de permissão afetadas.
3. Implementar.
4. Atualizar este arquivo e a matriz de permissões.
5. Executar os testes de autenticação e autorização.
6. Executar Security Specialist — alteração de auth **sempre** exige revisão de segurança.

## Histórico

| Data       | Alteração                                                                                                                                  | Motivo                                                | Plano                                                                            |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------ | ----------------------------------------------------- | -------------------------------------------------------------------------------- |
| 2026-09-15 | Documento criado do zero; corrigida a suposição de cookie de sessão com nome fixo (na verdade configurável via `SESSION_COOKIE_NAME`)      | `docs/` ausente antes desta tarefa                    | `.claude/plans/Backapeando-2026-09-15-17-41-inicializacao-governanca.md`         |
| 2026-09-15 | Gestão de admin_users passa a ter uma via web (`/api/admin-users*`), além do CLI; `GET /auth/me`/`login` passam a retornar `id` do usuário | CRUD de usuário administrador solicitado pelo usuário | `.claude/plans/Backapeando-2026-09-15-18-16-crud-admin-users-e-fix-historico.md` |
