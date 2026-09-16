# Backapeando Backup Manager

Sistema de gestão de backups de bancos de dados PostgreSQL via SSH com retenção configurável (GFS), upload para múltiplos destinos de armazenamento (Azure Blob Storage, S3-compatível, filesystem/NFS), e interface web para automação.

## System Architecture

[`Arquitetura`](./docs/arquitetura.html) — [Ver renderizado ↗](https://c3t4r4.github.io/Backapeando-Backup-Manager/)

## Quick Start (Development)

```bash
# Clone repository
git clone <repo>

# Start services with Docker Compose
docker-compose up -d

# Access
# Frontend: http://localhost:3000
# Backend: http://localhost:8081
# Postgres: localhost:5432

# Create admin user
docker exec -it backapeando-api /app/api create-admin
# CPF: (11 digits, validated by check digit)
# Email: admin@example.com
# Password: (12+ characters, Argon2id hashed)

# Stop services
docker-compose down
```

## Architecture

- **Backend**: Go 1.26+ (stdlib net/http, pgx, crypto/\*)
- **Frontend**: Vue 3 + Vite + TypeScript + Tailwind CSS
- **Database**: PostgreSQL 18+
- **Cache/Rate Limiting**: Redis 7
- **Deployment**: Docker Swarm; Traefik (reverse proxy) e Portainer (UI) geridos externamente

## Project Structure

```
backend/
  cmd/
    api/           # HTTP API + admin CLI (create-admin, reset-password)
    worker/        # Async scheduler worker (polling + SKIP LOCKED)
  internal/
    auth/          # Session + CSRF + rate limiting (Redis-backed or noop)
    azureblob/     # Azure Blob Storage client
    backupcore/    # Core backup execution logic
    config/        # Environment configuration (fail-fast)
    cpf/           # Brazilian CPF validation
    crypto/        # AES-256-GCM encryption
    db/            # Database + migrations (golang-migrate)
    domain/        # Core domain types
    fsstorage/     # Filesystem/NFS storage backend
    httpapi/       # HTTP routes + handlers
    repository/    # Data access layer (pgx)
    retention/     # GFS retention algorithm
    s3storage/     # S3-compatible storage backend
    scheduler/     # Async backup scheduler
    storage/       # Storage abstraction layer
    sshclient/     # SSH client + TOFU host key pinning
    sshkeys/       # SSH key generation (ed25519)

frontend/
  src/
    api/           # HTTP client + types
    composables/   # Reusable logic (useAuth)
    components/    # Vue components
    views/         # Page components
  public/          # Static assets
  index.html       # Entry point
  vite.config.ts   # Vite config

docker-compose.yml       # Development stack (all services)
docker-compose.prod.yml  # Production Docker Swarm stack
docs/                    # Governance & architecture documentation
```

## Development

### Prerequisites

- Go 1.26+
- Node.js 20+
- Docker & Docker Compose
- PostgreSQL 18 (via Docker)

### Backend Setup

```bash
cd backend

# Build
go build -o api ./cmd/api

# Run tests
go test ./...

# Run linter
go vet ./...
gofmt -l .

# Run server
export DATABASE_URL="postgres://backapeando_user:backapeando_password_dev@localhost:5432/backapeando_backup?sslmode=disable"
export MASTER_ENCRYPTION_KEY="0000000000000000000000000000000000000000000000000000000000000000"
./api

# Create admin (interactive)
./api create-admin

# Reset password (interactive)
./api reset-password
```

### Frontend Setup

```bash
cd frontend

# Install dependencies
npm install

# Run dev server (with API proxy to http://localhost:8080)
npm run dev

# Build for production
npm run build

# Run tests
npm run test

# Lint + format
npm run lint
npm run lint:fix
```

## Docker & Deployment

### Development (Docker Compose)

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

Services:

- **api** (:8081) — Backend HTTP server
- **web** (:3000) — Frontend Nginx server
- **postgres** (:5432) — PostgreSQL 18 database
- **redis** — Rate limiting & session cache (requires `REDIS_URL` in dev; fallback to in-memory noop if unset)

### Production (Docker Swarm)

#### Prerequisites

- Docker Swarm initialized (`docker swarm init` on manager node)
- 1 manager node + 2+ worker nodes
- Secrets pre-created:

```bash
docker secret create database_url - << 'EOF'
postgres://user:password@postgres:5432/backapeando_backup?sslmode=disable
EOF

docker secret create master_encryption_key - << 'EOF'
<32-byte hex string generated with openssl rand -base64 32>
EOF

docker secret create postgres_password - << 'EOF'
<secure postgres password>
EOF
```

#### Build & Deploy

```bash
# Build images
docker build -t backapeando-api:latest backend/cmd/api
docker build -t backapeando-web:latest frontend/

# Traefik (reverse proxy) e Portainer (UI de gestão) são implantados a
# partir de fora deste repositório — não há mais docker-compose.yml para
# eles aqui.

# Deploy application
docker stack deploy -c docker-compose.prod.yml backapeando-backup

# Verify deployment
docker stack ls
docker service ls
docker stack ps backapeando-backup
```

#### Access

- **Frontend**: http://backup.local (or IP; managed by Traefik/Portainer externally)
- **Backend API**: http://api.backup.local/api (or IP/api; managed by Traefik/Portainer externally)

Note: Traefik (reverse proxy) and Portainer (Swarm UI) are deployed outside this repository.
See your infrastructure documentation for their setup and access.

#### Scaling

```bash
# Scale backend API to 5 replicas
docker service scale backapeando-backup_api=5

# Check service status
docker service ps backapeando-backup_api
```

#### Troubleshooting

```bash
# View service logs
docker service logs backapeando-backup_api
docker stack logs backapeando-backup

# Check service health
docker service inspect backapeando-backup_api

# Force restart service
docker service update --force backapeando-backup_api

# Remove stack
docker stack rm backapeando-backup
```

## API Documentation

See [`docs/API.md`](./docs/API.md) for complete endpoint documentation.

Key endpoint groups:

- **Auth**: `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`
- **Servers**: CRUD + `test-connection`, `enable`/`disable`, `backup-now`, `run-now`, `regenerate-key`, `reset-host-key`, list runs
- **Backup History**: `GET /api/backup-runs`, `GET /api/servers/{id}/backup-runs`
- **Storage Targets**: CRUD for Azure Blob, S3-compatible, and filesystem backends
- **Retention Policies**: GET/PUT global default, GET/PUT per-server overrides
- **Admin Users**: CRUD (create/read/list/edit/delete admin users)
- **Dashboard**: `GET /api/dashboard/backup-stats`, `GET /api/dashboard/summary`
- **Health**: `GET /api/health`, `GET /healthz` (Docker, Traefik checks)

## Configuration

### Environment Variables

| Variable                         | Default                      | Required | Purpose                                                       |
| -------------------------------- | ---------------------------- | -------- | ------------------------------------------------------------- |
| `DATABASE_URL`                   | —                            | Yes      | PostgreSQL connection string (or `_FILE` variant for Swarm)   |
| `MASTER_ENCRYPTION_KEY`          | —                            | Yes      | 32-byte AES-256 key in base64 (or `_FILE` variant for Swarm)  |
| `MASTER_ENCRYPTION_KEY_PREVIOUS` | —                            | No       | Previous key for rotation (see `docs/Infraestrutura.md`)      |
| `HTTP_ADDR`                      | `:8081`                      | No       | HTTP server address                                           |
| `SESSION_COOKIE_NAME`            | `backapeando_backup_session` | No       | HTTP cookie name for sessions                                 |
| `SESSION_IDLE_TTL`               | `43200` (12h, in seconds)    | No       | Session inactivity timeout                                    |
| `SESSION_ABSOLUTE_TTL`           | `604800` (7d, in seconds)    | No       | Max session lifetime                                          |
| `SECURE_COOKIES`                 | —                            | Yes      | Must be exactly `"true"` or `"false"` (no implicit default)   |
| `CORS_ALLOWED_ORIGIN`            | (empty)                      | No       | Enable CORS for exactly one origin; never use `*`             |
| `REDIS_URL`                      | (empty)                      | No       | Redis URL for distributed rate limiting (e.g., `redis://...`) |

See [`backend/.env.example`](./backend/.env.example) for template.

## Security

- **Passwords**: Hashed with Argon2id (m=64MB, t=3, p=4, PHC format)
- **Secrets at rest**: AES-256-GCM encryption (via `MASTER_ENCRYPTION_KEY`)
- **SSH keys & storage secrets**: Encrypted before database storage
- **Sessions**: HttpOnly + Secure + SameSite=Strict cookies, server-side revogable
- **CSRF**: Double-submit cookie + header validation
- **Rate Limiting**: 5 login attempts per minute per IP (Redis-backed or in-memory)
- **CPF**: Validated by check digit (Brazilian national ID, `RN-AUTH-001`)
- **SSH**: TOFU (Trust On First Use) with SHA256 host key fingerprinting; no permanent MITM
- **Handshake**: SSH dial + handshake both have timeouts

For complete details, see [`docs/Auth.md`](./docs/Auth.md) and [`docs/Infraestrutura.md`](./docs/Infraestrutura.md).

## Documentation

- [`CLAUDE.md`](./CLAUDE.md) — Project guidelines & conventions
- [`docs/RegrasNegocio.md`](./docs/RegrasNegocio.md) — Business rules & validation (screens, state machines, permissions)
- [`docs/Arquitetura.md`](./docs/Arquitetura.md) — System architecture & architectural decisions (ADRs)
- [`docs/Organograma.md`](./docs/Organograma.md) — Module & flow diagrams
- [`docs/API.md`](./docs/API.md) — REST API endpoints & contracts
- [`docs/Frontend.md`](./docs/Frontend.md) — Frontend structure, components & UI patterns
- [`docs/Auth.md`](./docs/Auth.md) — Authentication, authorization & security details
- [`docs/Infraestrutura.md`](./docs/Infraestrutura.md) — Docker, Docker Swarm, deployment & observability
- [`docs/RAG.md`](./docs/RAG.md) — Context recovery strategy for agents
- [`docs/Progresso.md`](./docs/Progresso.md) — Development phases, status & changelog
- [`docs/Memoria.md`](./docs/Memoria.md) — Known issues, solutions & learnings
- [`docs/Harness.md`](./docs/Harness.md) — Agent roles, approval flow & tooling

## Testing

```bash
# Backend — unit tests (no external services required)
cd backend
go test ./...                    # All tests
go test -v ./internal/auth      # Specific package
go test -run TestAuth           # Specific test

# Backend — integration tests (requires real PostgreSQL)
export DATABASE_URL="postgres://backapeando_user:backapeando_password_dev@localhost:5432/backapeando_backup?sslmode=disable"
go test ./internal/httpapi/... -v   # HTTP integration tests
go test ./internal/repository/... -v  # Database integration tests

# Frontend
cd frontend
npm run test                     # All tests (Vitest)
npm run test:coverage           # With coverage report
npm run lint                     # TypeScript & ESLint

# Integration (full stack with Docker)
docker-compose up -d
docker-compose logs -f
# Then run backend integration tests above with DATABASE_URL pointing to docker-compose Postgres
```

## Build & Release

```bash
# Backend
cd backend
go build -o api ./cmd/api
go build -o worker ./cmd/worker

# Frontend
cd frontend
npm run build
# Output: dist/

# Docker images
docker build -t backapeando-api:v1.0.0 backend/cmd/api
docker build -t backapeando-web:v1.0.0 frontend/
docker push registry.example.com/backapeando-api:v1.0.0
docker push registry.example.com/backapeando-web:v1.0.0
```

## Contributing

1. Create a feature branch
2. Follow test-driven development (write tests first)
3. Run `go test`, `go vet`, `gofmt` (backend) + `npm test`, `npm run lint` (frontend)
4. Update relevant docs (see [docs](./docs/) directory)
5. Submit pull request

## License

[License TBD]

## Support

For issues or questions, see:

- [`docs/Memoria.md`](./docs/Memoria.md) — Known issues & solutions
- [`docs/Progresso.md`](./docs/Progresso.md) — Project status
- Issue tracker (GitHub Issues)
