# DevLake Local Development

## Container Runtime

This project uses **podman** instead of docker. Use `podman` and `podman compose` commands.

## Development Services

### Stop Running Services

```bash
cd /Users/kpiwko/devel/practices-team/devlake

# Stop all devlake services
podman compose -f docker-compose-dev.yml down
```

### Rebuild the Backend

```bash
# Option 1: Build Go binary locally
cd backend
go build -o bin/devlake ./cmd/lake-server/

# Option 2: Build the container image
cd /Users/kpiwko/devel/practices-team/devlake
podman compose -f docker-compose-dev.yml build devlake
```

### Start Services

```bash
cd /Users/kpiwko/devel/practices-team/devlake

# Start all services (MySQL, Grafana, DevLake, Config-UI)
podman compose -f docker-compose-dev.yml up -d

# Or start just MySQL for local development
podman compose -f docker-compose-dev.yml up -d mysql

# Then run devlake locally (without container)
cd backend
go run ./cmd/lake-server/
```

### View Logs

```bash
# View devlake logs
podman compose -f docker-compose-dev.yml logs -f devlake

# View all logs
podman compose -f docker-compose-dev.yml logs -f
```

### Access Services

| Service | URL |
|---------|-----|
| Config UI | http://localhost:4000 |
| DevLake API | http://localhost:8080 |
| Grafana | http://localhost:3002 |
| MySQL | localhost:3306 (user: merico, pass: merico) |

## Running Tests

### Unit Tests

```bash
cd backend
go test ./plugins/aireview/tasks/... -v
```

### E2E Tests

E2E tests require MySQL running. The `.env` file uses `localhost` for `E2E_DB_URL`:

```bash
# Ensure MySQL is running
podman compose -f docker-compose-dev.yml up -d mysql

# Create test database if needed
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico -e "CREATE DATABASE IF NOT EXISTS lake_test;"

# Run e2e tests
cd backend
go test ./plugins/aireview/e2e/... -v
```

### All Plugin Tests

```bash
cd backend
go test ./plugins/aireview/... -v
```

## Database Operations

### Connect to MySQL

```bash
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake
```

### Check Migrations

```bash
# List migration history
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "SELECT * FROM _devlake_migration_history ORDER BY created_at DESC LIMIT 10;"
```

### Verify aireview Tables

```bash
# Check aireview_reviews table structure
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "DESCRIBE _tool_aireview_reviews;"

# Check for new columns (effort_rating, pre_merge_checks_*)
podman compose -f docker-compose-dev.yml exec mysql mysql -umerico -pmerico lake -e \
  "DESCRIBE _tool_aireview_reviews;" | grep -E "effort_rating|pre_merge"
```
