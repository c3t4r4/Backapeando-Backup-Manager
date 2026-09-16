# Frontend — Backapeando

> Consultar antes de criar componentes. Reutilizar o que já existe. Atualizar após alterações aprovadas.

## Stack

| Item | Versão / Detalhe |
| --- | --- |
| Linguagem | TypeScript |
| Framework | Vue 3.5 (Composition API) |
| Bundler | Vite 5 |
| Roteamento | Vue Router 4.5, `authGuard` global (`beforeEach`) |
| CSS | Tailwind CSS 3.4 |
| Componentes | shadcn-vue parcial (`reka-ui` + `class-variance-authority` + `tailwind-merge`), fixado em `1.0.3` por compatibilidade Tailwind v3 |
| Gerenciador de estado | composables (`useAuth`) — sem Pinia/Vuex detectado |
| Requisições | Axios |
| Gerenciador de pacotes | npm (`package-lock.json`) |
| Runtime | Node — versão exata `a definir` (não fixada em `.nvmrc`/`engines`) |
| Gráficos | Chart.js + vue-chartjs |
| Ícones | lucide-vue-next |

## Design system

`a definir` para a maior parte dos tokens (cores, espaçamento, tipografia) — não confirmados na entrevista desta inicialização. Tailwind é usado com configuração própria em `frontend/tailwind.config.*` (consultar o arquivo real antes de assumir uma paleta).

**Exceção — paleta categórica de gráficos** (primeira convenção de cor de gráfico documentada do projeto, ver "Gráficos por destino em T-02" abaixo): 8 cores hex fixas, validadas via a skill `dataviz` para contraste AA e distinguibilidade sob daltonismo (protan/deutan/tritan), em ordem fixa que nunca deve ser reordenada ou ter cores removidas do meio da sequência sem revalidar (a adjacência entre cores consecutivas é o que garante a separação sob daltonismo — um subconjunto arbitrário quebra essa garantia, confirmado empiricamente nesta tarefa).

| Slot | Hue | Hex |
| --- | --- | --- |
| 1 | azul | `#2a78d6` |
| 2 | laranja | `#eb6834` |
| 3 | água | `#1baf7a` |
| 4 | amarelo | `#eda100` |
| 5 | magenta | `#e87ba4` |
| 6 | verde | `#008300` |
| 7 | violeta | `#4a3aa7` |
| 8 | vermelho | `#e34948` |

Categoria fixa fora da paleta acima: "Sem destino" sempre usa `#6b7280` (cinza neutro, reaproveitado do status "Desabilitado" do doughnut de T-02) — nunca uma cor da paleta categórica, para não ser confundido com um destino real.

## Estrutura de diretórios do frontend

```text
frontend/
  src/
    api/            # cliente Axios + funções HTTP + types
    composables/     # useAuth (estado de sessão)
    components/
      *.vue          # componentes de domínio (BackupTable, ServerList, StorageTargetList, ...)
      ui/            # primitivos shadcn-vue (button, card, dialog, input, label, select, table, badge)
    views/           # telas (ver docs/RegrasNegocio.md, Índice de telas)
    router/          # Vue Router 4 + authGuard
  index.html, vite.config.ts
```

## Mapa de componentes

| Componente | Arquivo | Finalidade | Reutilizável |
| --- | --- | --- | --- |
| Layout | `components/Layout.vue` | Casca da página (provavelmente Navbar+Sidebar+slot) | Sim |
| Navbar | `components/Navbar.vue` | Barra de navegação superior | Sim |
| Sidebar | `components/Sidebar.vue` | Navegação lateral | Sim |
| BackupTable | `components/BackupTable.vue` | Tabela de execuções de backup | Sim |
| ServerList | `components/ServerList.vue` | Listagem de servidores | Sim |
| StorageTargetList | `components/StorageTargetList.vue` | Listagem de destinos de armazenamento | Sim |
| Pagination | `components/Pagination.vue` | Paginação genérica | Sim |
| ConfirmDialog | `components/ConfirmDialog.vue` | Confirmação de ação destrutiva | Sim |
| LoadingSpinner | `components/LoadingSpinner.vue` | Indicador de carregamento | Sim |
| BackupRunDetailsDialog | `components/BackupRunDetailsDialog.vue` | Modal de detalhes de uma execução de backup (`ui/dialog`) — blob, tamanho, durações, erro, log | Sim |
| ui/button, ui/card, ui/dialog, ui/input, ui/label, ui/select, ui/table, ui/badge | `components/ui/**` | Primitivos shadcn-vue | Sim |
| dumpCommand | `lib/dumpCommand.ts` | Função pura que monta a prévia do comando remoto de dump (espelha `backend/internal/scheduler/dumpcommand.go`) | Sim (usado por `ServerNewView.vue`/`ServerEditView.vue`) |

`lib/format.ts` (novo, `frontend/src/lib/`) centraliza `formatDate`, `formatSize`, `formatDurationMs`, `formatBackupStatus`, `formatServerId`, `formatCPF` — reaproveitado por `BackupTable.vue` e `BackupRunDetailsDialog.vue` (evita duplicar a mesma formatação em dois componentes). `lib/apiError.ts` (novo) centraliza `apiErrorMessage(err, fallback)` para exibir a mensagem literal de erro vinda do backend (usado pelas telas de administradores ao mostrar bloqueios de exclusão em `409`), e `logApiError(context, err)` — log de erro em dev que nunca imprime o corpo bruto da requisição (evita vazar a senha no console nos formulários de admin_users, achado do Security Specialist).

Todos os componentes de domínio têm `*.spec.ts` correspondente (Vitest + Vue Test Utils).

## Telas e rotas

| Rota | Tela | Componente raiz | Acesso | Regras de negócio |
| --- | --- | --- | --- | --- |
| `/login` | T-01 | `LoginView.vue` | público | RN-AUTH-001 — ver `docs/RegrasNegocio.md` |
| `/dashboard` | T-02 | `DashboardView.vue` | admin | RN-BACKUP-002, RN-BACKUP-031 |
| `/servers` | T-03 | `ServersView.vue` | admin | RN-BACKUP-001, RN-BACKUP-002 |
| `/servers/new` | T-04 | `ServerNewView.vue` | admin | RN-BACKUP-001, RN-BACKUP-017, RN-BACKUP-018, RN-BACKUP-021 |
| `/servers/:id/edit` | T-05 | `ServerEditView.vue` | admin | RN-BACKUP-001, RN-BACKUP-005, RN-BACKUP-013, RN-BACKUP-017 a 021, RN-BACKUP-023, RN-BACKUP-024 |
| `/history` | T-06 | `HistoryView.vue` | admin | RN-BACKUP-014, RN-BACKUP-016, RN-BACKUP-022, RN-BACKUP-025, RN-BACKUP-026, RN-BACKUP-027 |
| `/storage-targets` | T-07 | `StorageTargetsView.vue` | admin | RN-STORAGE-001 |
| `/storage-targets/new` | T-08 | `StorageTargetNewView.vue` | admin | RN-STORAGE-001 |
| `/storage-targets/:id/edit` | T-09 | `StorageTargetEditView.vue` | admin | RN-STORAGE-001 |
| `/settings` | T-10 | `SettingsView.vue` | admin | RN-BACKUP-003, RN-BACKUP-004 |
| `/admin-users` | T-11 | `AdminUsersView.vue` | admin | RN-AUTH-001, RN-AUTH-002, RN-AUTH-003 |
| `/admin-users/new` | T-12 | `AdminUserNewView.vue` | admin | RN-AUTH-001 |
| `/admin-users/:id/edit` | T-13 | `AdminUserEditView.vue` | admin | RN-AUTH-001, RN-AUTH-002, RN-AUTH-003 |

A lógica de decisão de cada tela é documentada em `docs/RegrasNegocio.md`, não aqui.

**Nota de estilo (T-11/T-12/T-13):** essas três telas usam os componentes shadcn-vue (`ui/table`, `ui/dialog`, `ui/input`, `ui/label`, `ui/button`) em vez do Tailwind puro usado por `ServerList.vue`/`StorageTargetList.vue` — decisão consciente do usuário nesta tarefa, mesmo divergindo do padrão visual das demais telas de listagem/formulário. Telas futuras devem escolher explicitamente um dos dois padrões, não misturar sem decisão.

**Nota de estilo (T-04/T-05, multi-engine):** `ServerNewView.vue`/`ServerEditView.vue` continuam majoritariamente Tailwind puro, mas os dois seletores novos (`dbEngine`/`deploymentMode`) e o card de prévia do comando usam os componentes shadcn-vue `ui/select` e `ui/card` — primeiro uso real de `ui/select` no projeto (antes só existia instalado, sem consumidor). Decisão desta tarefa: misturar Tailwind puro com shadcn pontualmente nos campos novos, sem migrar o form inteiro.

### Novos campos em T-04/T-05 (multi-engine, Docker-ou-host, prévia de comando)

- **Seletor "Motor do Banco de Dados"** (`db-engine-select`, `ui/select`): `postgres`/`mysql`/`sqlserver`.
- **Seletor "Onde o Banco Roda"** (`deployment-mode-select`, `ui/select`): `docker`/`host`. Controla a visibilidade do campo `containerName` (RN-BACKUP-018).
- **Campo "Senha do Banco"** (`db-password-input`, `type="password"`): opcional para Postgres, obrigatório para MySQL/SQL Server (RN-BACKUP-017). Na edição, vazio mantém a senha atual (RN-BACKUP-019) — placeholder muda conforme `hasDbPassword`.
- **Campos de argumentos extras condicionais** (`pg-dump-extra-args-input`/`mysql-dump-extra-args-input`/`sqlcmd-extra-args-input`): só o do engine ativo é renderizado (`v-if`/`v-else-if`), mesmo padrão já usado por `StorageTargetNewView.vue` para campos condicionais por tipo.
- **Card de prévia do comando** (`dump-command-preview-card`, `dump-command-preview-line`): `computed()` reativo que chama `frontend/src/lib/dumpCommand.ts` (`buildDumpCommandPreview`) a cada mudança nos campos relevantes. Nunca exibe a senha real — sempre mascarada como `***` (RN-BACKUP-021).
- **`frontend/src/lib/dumpCommand.ts`** (novo): réplica pura em TypeScript da lógica de `backend/internal/scheduler/dumpcommand.go` (mesmo `shellQuote`, mesma ordem de tokens) — os dois arquivos se referenciam um ao outro em comentário de cabeçalho para reduzir risco de divergência futura.
- **Campo "Container Docker" — texto de ajuda** (`ServerNewView.vue`/`ServerEditView.vue`): agora avisa que aceita nome parcial, resolvido por busca no momento da execução (RN-BACKUP-023).
- **Botão "Ativar Servidor"/"Desativar Servidor"** (`enable-server-button`/`disable-server-button`, T-05): reaproveita o mesmo `ConfirmDialog` já usado por Regenerar Chave/Reset Host Key; alterna conforme o `enabled` atual do servidor carregado (RN-BACKUP-024).
- **Testando `ui/select` em Vitest:** reka-ui (base do `ui/select`) depende de captura de ponteiro e foco reais, que o ambiente `happy-dom` dos testes não implementa. `frontend/src/test/setup.ts` (novo, registrado em `vitest.config.ts` via `setupFiles`) faz um shim mínimo de `hasPointerCapture`/`setPointerCapture`/`releasePointerCapture`/`scrollIntoView`. Mesmo com o shim, simular a abertura do popover teleportado via pointer/teclado não reproduz de forma confiável a seleção real (limitação conhecida de testar Radix/reka-ui fora de um browser real) — os testes interagem diretamente com o contrato `v-model` do componente (`wrapper.findAllComponents(Select)[i].vm.$emit('update:modelValue', valor)`), documentado inline em `ServerNewView.spec.ts`/`ServerEditView.spec.ts`.

### Paginação e filtro por status em T-06 (Histórico)

- **Seletor "Status"** (`status-select`, Tailwind puro, mesmo padrão do seletor de servidor existente): Todos/Enfileirado/Em execução/Sucesso/Falhou. Trocar o status reseta para a página 1 e refaz a busca (RN-BACKUP-025).
- **Paginação passa a ser server-side**: `pageSize` default mudou de 10 para 50; `Pagination.vue` (já existente, genérico) não precisou mudar — só passou a receber `total`/`page` vindos da resposta do backend em vez de calculados localmente sobre um array já carregado por inteiro.
- **Seleção de servidor deixou de ser obrigatória (RN-BACKUP-026)**: sem servidor selecionado, a tela chama `GET /api/backup-runs` (novo endpoint, `api.getAllBackupRuns`) já no `onMounted`, mostrando os últimos 50 backups de todos os servidores. Selecionar um servidor no dropdown passa `serverId` para o mesmo endpoint, filtrando para aquele servidor — o antigo endpoint aninhado (`api.getBackupRuns`) não é mais chamado pelo frontend, mas continua existindo no backend sem mudança de contrato. A coluna "Servidor" (`BackupTable.vue`) agora mostra o nome do servidor (resolvido de `servers` já carregado para o dropdown, via novo prop `serverNames`) em vez do UUID truncado, já que a visão padrão mistura vários servidores.
- **Mensagem de erro e log de saída (`BackupRunDetailsDialog.vue`, RN-BACKUP-027)**: nenhuma mudança de frontend — o componente já renderizava `errorMessage`/`logOutput` condicionalmente; a mudança foi só o backend passar a preencher esses campos com detalhe real em vez de uma frase genérica/`null` (ver `docs/API.md`).

### Gráficos por destino em T-02 (Dashboard)

- **Gráfico de linha removido** ("Tamanho e duração dos backups ao longo do tempo", por execução individual) — substituído por **4 gráficos de barras empilhadas** (`vue-chartjs` `Bar`, não mais `Line`), todos agrupados por destino de armazenamento (`storage_target`): contagem de backups/30 dias (diário), soma de dados em MB/30 dias (diário), contagem/ano corrente (mensal), soma de dados em MB/ano corrente (mensal). `data-testid`: `chart-daily-count`, `chart-daily-bytes`, `chart-monthly-count`, `chart-monthly-bytes`.
- **Fonte de dados**: novo endpoint `GET /api/dashboard/backup-stats` (`api.getDashboardBackupStats`), buscado em paralelo com `GET /api/dashboard/summary` via `Promise.all` no `onMounted` — falha em qualquer um dos dois mostra o mesmo alerta de erro já existente na tela.
- **Estado vazio por gráfico**: como a API sempre retorna `labels`/séries zero-preenchidos (nunca arrays vazios) para manter os 4 gráficos alinhados, o estado vazio ("Nenhum backup nos últimos 30 dias."/"Nenhum backup no ano corrente.") é decidido no frontend checando se **todo** valor de todo destino é zero (`seriesHasData`), não pelo tamanho do array de labels.
- **Cor por destino**: consistente entre os 4 gráficos e entre re-renders — atribuída por posição em `destinations` (a mesma lista, na mesma ordem, retornada pela API para as 4 séries), nunca recalculada a partir dos dados exibidos. Ver a paleta em "Design system" acima.
- **RN-BACKUP-031**: destino refletido é o atual do servidor, não o histórico real no momento de cada execução — ver `docs/RegrasNegocio.md`.

## Guarda de rota (authGuard)

`frontend/src/router/index.ts`: rota com `meta.requiresAuth=true` e usuário não logado → redireciona para `/login`; usuário logado acessando `/login` → redireciona para `/dashboard`. Exportado separadamente de `router.beforeEach` para ser testável isoladamente (`router.spec.ts`).

## Padrões de estado da interface

`a definir` — não confirmado se há um padrão consistente de loading/empty/error entre as telas (existe `LoadingSpinner.vue`, mas o padrão de uso não foi auditado tela a tela nesta tarefa).

## Formulários e validação

- Biblioteca de formulário: `a definir` — não detectada biblioteca dedicada (ex.: vee-validate) nas dependências de `package.json`
- Regra: validação no cliente **nunca** substitui validação no servidor.

## Acessibilidade

`a definir` — sem requisitos formais registrados nesta inicialização; seguir os padrões default do `agent-skills:frontend-ui-engineering` quando aplicável (contraste mínimo 4.5:1, foco visível, `aria-label` em botões só-ícone, navegação por teclado).

## Testes de frontend

- Framework: Vitest 1.6
- Biblioteca de testes de componente: `@vue/test-utils` 2.4
- Ambiente DOM: `happy-dom` / `jsdom`
- Cobertura: `@vitest/coverage-v8`
- Comando: `npm run test` (ou `npm run test:coverage`)
- Lint: `eslint src --ext .vue,.ts && prettier --check src`

## Regras de alteração

1. Consultar este arquivo e o mapa de componentes.
2. Consultar `docs/RegrasNegocio.md` se a alteração muda comportamento ou regra de exibição.
3. Reutilizar componente existente sempre que possível.
4. Respeitar o design system (quando definido).
5. Implementar.
6. Atualizar o mapa de componentes.
7. Atualizar as regras de negócio afetadas.
8. Atualizar ou criar testes.
9. Executar Validator, Tester e Security Specialist.

## Histórico

| Data | Componente / tela | Alteração | Motivo |
| --- | --- | --- | --- |
| 2026-09-15 | todos | Documento criado do zero a partir do código real, ausente antes desta tarefa | `/init-project --update` |
| 2026-09-15 | T-11/T-12/T-13 (Administradores), `BackupRunDetailsDialog`, `HistoryView.vue` | Novas telas de CRUD de admin_users (shadcn-vue); botão "Detalhes" do Histórico passa a abrir modal real (antes era `console.log` sem efeito) | CRUD de usuário administrador + correção de bug reportados pelo usuário |
| 2026-09-15 | T-04/T-05 (Servidores), `lib/dumpCommand.ts` (novo), `test/setup.ts` (novo) | Seletores de motor de banco e Docker-ou-host, campo de senha do banco, campos de argumentos extras condicionais por engine, card de prévia do comando remoto (primeiro uso real de `ui/select`); shim de pointer-capture para testar `ui/select` em `happy-dom` | Suporte a MySQL/SQL Server e a bancos rodando direto no host, com prévia do comando de dump |
| 2026-09-15 | T-05 (Servidores), T-06 (Histórico) | Botão Ativar/Desativar servidor (reaproveita `ConfirmDialog`); texto de ajuda do campo container avisando nome parcial; T-06 ganha seletor de Status e paginação server-side (50/página, era 10 client-side) | Correção de bug de duração + resolução de container por grep + ativação/desativação manual + paginação/filtro no histórico, solicitados pelo usuário |
| 2026-09-16 | T-06 (Histórico) | Seleção de servidor deixa de ser obrigatória — tela busca os últimos 50 backups de todos os servidores por padrão (`GET /api/backup-runs`); coluna "Servidor" passa a mostrar o nome em vez do UUID truncado | Usuário reportou que a tela não trazia nada até selecionar um servidor |
| 2026-09-16 | T-02 (Dashboard) | Gráfico de linha "Tamanho e duração dos backups" (por execução) substituído por 4 gráficos de barras empilhadas por destino (contagem/bytes × 30 dias diário/ano mensal); primeira paleta de cor de gráfico validada e documentada do projeto (skill `dataviz`) | Usuário pediu que o gráfico mostrasse backups e soma de dados por destino, últimos 30 dias e mês a mês do ano atual |
