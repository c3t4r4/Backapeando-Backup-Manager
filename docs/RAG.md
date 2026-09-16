# RAG e recuperação de contexto — Backapeando

> Contrato de recuperação de informação deste projeto: o que está indexado, o que **nunca** pode ser indexado, em que ordem consultar e quando reindexar.

Última atualização: 2026-09-15

---

## Capacidades do ambiente

Estado detectado em 2026-09-15 via `_scan.sh capacidades`. Ambiente muda; reconferir quando algo aqui não bater.

| Capacidade            | Situação                     | Uso neste projeto                                                                 | Fallback em uso               |
| --------------------- | ---------------------------- | --------------------------------------------------------------------------------- | ----------------------------- |
| MCP `claude-mem`      | DISPONÍVEL                   | Base — observações, corpora e busca AST                                           | Glob/Grep com escopo restrito |
| skill `archify`       | DISPONÍVEL                   | `docs/arquitetura.html` a partir de `docs/Arquitetura.md`                         | —                             |
| skill `graphify`      | DISPONÍVEL, NÃO USADO ainda  | Opcional — grafo de conhecimento sob demanda (`/graphify`), operação cara, opt-in | segue sem grafo               |
| agentes do harness    | DISPONÍVEIS (29 carregáveis) | Papéis de `docs/Harness.md` com contexto isolado                                  | —                             |
| skill `ui-ux-pro-max` | DISPONÍVEL                   | Apoio a decisões de design system do `docs/Frontend.md`, quando necessário        | —                             |

Capacidade ausente degrada o trabalho, **nunca o impede**. Instruções de instalação em `~/.claude/templates/init-project/_capacidades.md`.

---

## Índices deste projeto

| Índice               | Tipo                  | Conteúdo                                                                                                           | Como atualizar                             |
| -------------------- | --------------------- | ------------------------------------------------------------------------------------------------------------------ | ------------------------------------------ |
| `Backapeando-geral`  | corpus claude-mem     | Decisões, correções e descobertas do projeto                                                                       | `build_corpus`                             |
| `Backapeando-regras` | corpus claude-mem     | Filtrado por `types=decision,feature` — apoio ao `RegrasNegocio.md`                                                | `build_corpus`                             |
| Código-fonte         | busca AST sob demanda | Símbolos, funções, classes                                                                                         | `smart_search` lê o disco, não exige build |
| `graphify-out/`      | grafo                 | Não gerado ainda — projeto pequeno (172 arquivos de código), sem necessidade detectada no momento da inicialização | `/graphify . --update`, quando solicitado  |

---

## Protocolo retrieval-first

**Ordem obrigatória.** Só avançar para o passo seguinte quando o anterior não responder.

1. **Pergunta sobre comportamento ou regra de negócio** → `docs/RegrasNegocio.md`, pelo ID da regra ou pelo índice de telas.
2. **Pergunta sobre histórico, decisão anterior ou bug já resolvido** → `observation_search`; se houver corpus primado, `query_corpus`.
3. **Localizar código** → `smart_search` (encontra o símbolo) → `smart_outline` (estrutura do arquivo) → `smart_unfold` (só o trecho necessário).
4. **Leitura completa de arquivos** — somente depois dos passos acima, e apenas nos arquivos identificados.

### Quando o RAG não serve

- Antes de editar um arquivo — ler o arquivo.
- Ao verificar se uma regra `⚠ inferida` é real — ler a implementação.
- Ao auditar segurança — ler, não recuperar.

---

## Exclusões de segurança

**Nunca indexar, nunca gravar em observação, nunca incluir em corpus:**

- `.env` e variantes (`.env.local`, `.env.production`) — nenhum encontrado versionado nesta auditoria.
- Secrets, chaves privadas, certificados, tokens.
- Credenciais de banco, strings de conexão com senha — **atenção**: `docker-compose.yml` versiona `MASTER_ENCRYPTION_KEY` de dev em texto plano (ver `docs/Infraestrutura.md`); não incluir esse valor em nenhuma observação ou corpus, mesmo sendo um valor "só de dev".
- Dumps de banco ou fixtures com dados pessoais reais (CPF, dados de servidores reais de clientes).
- Logs de produção com dados de usuário.

Caminhos excluídos da indexação:

```text
**/.env
**/.env.*
!.env.example
docker-compose.yml        # contém valor de MASTER_ENCRYPTION_KEY de dev — indexar apenas com esse valor mascarado, se necessário
```

Se um segredo entrar em um índice por engano: reconstruir o índice do zero e registrar em `docs/Memoria.md`.

---

## Política de refresh

| Evento                                          | Ação                              |
| ----------------------------------------------- | --------------------------------- |
| Tarefa aprovada que muda decisão ou arquitetura | Reconstruir `Backapeando-geral`   |
| Regras de negócio alteradas                     | Reconstruir `Backapeando-regras`  |
| Mudança estrutural grande no código             | `/graphify . --update`, se em uso |
| Índice desatualizado ou suspeito                | Reconstruir do zero               |

`/learn-codebase` lê **todo arquivo-fonte na íntegra** — é caro e demorado. É **opt-in**: só executar com confirmação explícita do usuário. Não solicitado nesta inicialização.

---

## Estado atual

- Corpora existentes: `Backapeando-geral`, `Backapeando-regras` (construídos nesta tarefa, autorizado pelo usuário; `observation_count=0` no momento da criação — crescem conforme observações do projeto se acumulam em sessões futuras)
- Última indexação: 2026-09-15
- Grafo: não gerado
- Observações registradas: 0 no momento da criação dos corpora

---

## Histórico

| Data       | Índice                                    | Ação    | Motivo                                            |
| ---------- | ----------------------------------------- | ------- | ------------------------------------------------- |
| 2026-09-15 | `Backapeando-geral`, `Backapeando-regras` | criados | `/init-project --update`, autorizado pelo usuário |
