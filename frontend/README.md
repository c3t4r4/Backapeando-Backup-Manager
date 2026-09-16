# Backapeando Backup Manager — Frontend

Frontend Vue 3 + TypeScript + Tailwind CSS para gerenciamento de backups.

## Desenvolvimento

```bash
npm install
npm run dev     # Dev server em :3000
npm run test    # Rodar testes
npm run build   # Build produção
```

## Estrutura

- `src/api/` — Cliente HTTP (Axios)
- `src/composables/` — Lógica reutilizável (useAuth, useBackupHistory, etc.)
- `src/components/` — Componentes Vue
- `src/views/` — Páginas
- `src/router/` — Roteamento Vue Router

## Backend

Dev server proxy:
- Frontend: http://localhost:3000
- Backend: http://localhost:8080
- Proxy configurado em `vite.config.ts` (URL `/api/*` → http://localhost:8080/api/*)
