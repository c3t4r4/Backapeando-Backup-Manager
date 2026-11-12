# Plano: debug do backup local e smoke de dump

## Objetivo

Mostrar em qual etapa um backup local (sem SSH) está parado, e dar um teste isolado de `docker exec` + `pg_dump` de cerca de 2 MB que não depende da API, do worker nem de credenciais do projeto.

## Contexto

`ExecuteBackup` só registra o início e o sucesso. Resolução de container, `docker exec` e upload compartilham `BACKUP_TASK_TIMEOUT` (6 h). Um banco pequeno parado não produz linha nenhuma no meio. O Compose de desenvolvimento usa a imagem publicada, então mudança no código do worker só aparece depois de um build local.

## Arquivos envolvidos

- `backend/internal/scheduler/executor.go`
- `backend/internal/scheduler/progress.go`
- `backend/internal/scheduler/progress_test.go`
- `backend/cmd/worker/main.go`
- `backend/cmd/worker/loglevel_test.go`
- `scripts/smoke-local-dump.sh`
- `docker-compose.debug.yml`
- `docs/Infraestrutura.md`, `docs/Memoria.md`, `docs/Progresso.md`, `README.md`

## Etapas

1. Logar cada etapa bloqueante (resolver container, pré-comando, stream, espera do processo) sem incluir o comando nem a senha.
2. Enquanto o upload ou o `wait` estiver bloqueado, emitir um batimento com bytes lidos e tempo decorrido (`BACKUP_PROGRESS_INTERVAL`, padrão 15 s).
3. Honrar `LOG_LEVEL` no worker (`debug`/`info`/`warn`/`error`).
4. Script que sobe um Postgres descartável, gera ~2 MB e mede o `pg_dump` via `docker exec`, com limite de 60 s.
5. Overlay de Compose que builda o worker deste checkout com `LOG_LEVEL=debug`.

## Sincronização de documentação

| Documento | Previsto | Motivo |
| --- | --- | --- |
| `docs/RegrasNegocio.md` | sem alteração | observabilidade; sucesso/falha do backup não muda |
| `docs/Arquitetura.md` | sem alteração | mesmo fluxo de dump/upload |
| `docs/Organograma.md` | sem alteração | nenhum módulo novo |
| `docs/Infraestrutura.md` | atualizado | env vars, overlay e script |
| `docs/API.md` | sem alteração | contrato HTTP igual |
| `docs/Frontend.md` | sem alteração | sem tela |
| `docs/Auth.md` | sem alteração | sem auth |
| `docs/RAG.md` | sem alteração | corpus igual |
| `docs/Progresso.md` | atualizado | sempre |
| `docs/Memoria.md` | atualizado | silêncio entre início e upload; imagem publicada |
| `docs/Harness.md` | sem alteração | papéis iguais |
| `README.md` | atualizado | comando de diagnóstico |

## Validação

- [ ] Validator aprovou.
- [ ] Tester aprovou.
- [ ] Security Specialist aprovou.
- [ ] Tabela de sincronização preenchida com o estado real e conferida contra o diff.
- [ ] Documentos condicionais cujo gatilho disparou foram criados.
- [ ] `docs/Progresso.md` atualizado.

## Riscos e decisões pendentes

- O batimento não encurta o timeout de 6 h. Para falhar mais cedo no teste, o operador define `BACKUP_TASK_TIMEOUT`.
- O overlay é opt-in. `docker compose up` sem ele continua na imagem publicada.
