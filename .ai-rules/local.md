# DevLake Local Development (AI Reference)

Quick reference for AI agents working on this project.

## Environment

- **Container runtime**: podman (not docker)
- **Compose file**: `docker-compose-dev.yml`
- **Database**: MySQL 8, credentials `merico:merico`, database `lake`
- **Test database**: `lake_test` (auto-created via `scripts/mysql-init.sql`)
- **MySQL root**: `root:admin` (for admin operations)
- **Go version**: 1.21+

## Service URLs

| Service | URL |
|---------|-----|
| Config UI | http://localhost:4000 |
| DevLake API | http://localhost:8080 |
| Grafana | http://localhost:4000/grafana/ |
| MySQL | localhost:3306 |

## Custom CA (CEE GitLab)

For internal services using Red Hat CA:

```bash
# Create combined CA bundle: Mozilla CAs + Red Hat Root CAs (macOS)
curl -sL https://curl.se/ca/cacert.pem -o ./custom-ca.crt
security find-certificate -a -p -c "Root CA" /Library/Keychains/System.keychain >> ./custom-ca.crt

# Set in .env
CA_CERT_FILE=./custom-ca.crt

# Start with custom CA
podman compose -f docker-compose-dev.yml -f docker-compose-custom-ca.yml up -d
```

## Common Commands

```bash
# Start all services
podman compose -f docker-compose-dev.yml up -d

# Start just MySQL (for local Go dev)
podman compose -f docker-compose-dev.yml up -d mysql

# Stop services
podman compose -f docker-compose-dev.yml down

# Rebuild and restart devlake container
podman compose -f docker-compose-dev.yml up -d --build devlake

# View logs
podman compose -f docker-compose-dev.yml logs -f devlake
```

## Building (from backend/)

```bash
cd backend
make go-dep      # Install dependencies
make build       # Build everything (plugins + server)
make build-plugin # Build plugins only
make run         # Run server
make dev         # Build and run
```

## Testing

```bash
cd backend

# All tests (unit + e2e)
make test

# Unit tests only
make unit-test

# E2E tests (requires MySQL running with lake_test)
export E2E_DB_URL="mysql://merico:merico@localhost:3306/lake_test?charset=utf8mb4&parseTime=True"
make e2e-test

# Specific plugin tests
go test ./plugins/aireview/... -v

# Lint and format
make lint
make fmt
```

## Database

```bash
# Connect to MySQL
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake

# Check migrations
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "SELECT * FROM _devlake_migration_history ORDER BY created_at DESC LIMIT 10;"

# Trigger migration after schema changes
curl -s http://localhost:4000/api/proceed-db-migration
```

## Plugin Verification

```bash
# Check plugin is loaded
curl -s http://localhost:4000/api/plugins | jq '.[] | select(.plugin == "aireview")'

# Check aireview tables
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "SHOW TABLES LIKE '_tool_aireview%';"
```

## Before Running Commands

1. Ensure podman machine is running: `podman machine start`
2. Check if services are already running: `podman ps`
3. For E2E tests, MySQL must be running with `lake_test` database

## If E2E tests fail with "Access denied for lake_test"

Existing MySQL volumes may not have `lake_test`. Fix with:

```bash
podman compose -f docker-compose-dev.yml exec mysql mysql -uroot -padmin -e "
  CREATE DATABASE IF NOT EXISTS lake_test;
  GRANT ALL PRIVILEGES ON lake_test.* TO 'merico'@'%';
  FLUSH PRIVILEGES;
"
```

## Project Conventions

- Plugin code: `backend/plugins/<plugin-name>/`
- Tests next to source: `foo.go` → `foo_test.go`
- E2E test fixtures: `e2e/raw_tables/*.csv`
- Migrations: `models/migrationscripts/`
