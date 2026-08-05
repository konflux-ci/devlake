# DevLake MySQL SSL Enforcement — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Enable DevLake to connect to MySQL/RDS over SSL — both locally (docker-compose-dev.yml) and in the Helm-deployed staging/production environments.

**Architecture:** DevLake's `MakeDbConnection` in `backend/core/runner/db.go` already supports two SSL modes for MySQL:
1. `tls=true` — go-sql-driver uses Go's system CA pool (`crypto/x509.SystemCertPool()`). This requires the RDS CA cert to be in the system trust store, or pointed to via `SSL_CERT_FILE` env var.
2. `tls=custom&ca-cert=/path/to/ca.pem` — DevLake loads the CA cert explicitly, registers a custom TLS config, and connects. The `ca-cert` param is stripped from the DSN before passing to the MySQL driver.

**Current problem (staging):** The staging Helm deployment sets `tls=true` in `DB_URL` and mounts the OpenShift trusted CA bundle at `/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem`. However, the container image is Debian-based (`python:3.9-slim-bookworm`), and Go's `SystemCertPool()` on Debian reads from `/etc/ssl/certs/ca-certificates.crt` — it does NOT read `/etc/pki/`. So `tls=true` fails because Go can't find the RDS CA certificate.

**Tech Stack:** Go (crypto/tls, go-sql-driver/mysql), Docker Compose, Kustomize/Helm

## Global Constraints

- The DevLake backend image is based on `python:3.9-slim-bookworm` (Debian). Go's `x509.SystemCertPool()` reads CA certs from `/etc/ssl/certs/ca-certificates.crt` or from the path in `SSL_CERT_FILE` env var.
- The `docker-compose-custom-ca.yml` overlay exists but handles a different concern (HTTP client CA for GitLab API calls, not MySQL TLS). Do not conflate them.
- Changes to `backend/core/runner/db.go` affect all environments — test both SSL and non-SSL paths.
- The RDS CA bundle is already downloaded locally at `certs/rds-combined-ca-bundle.pem` (via ai-stack setup).
- Production (`infra-common-deployments/components/konflux-devlake/internal-production/`) does NOT currently use SSL for MySQL. This plan adds it.

---

### Task 1: Fix the `gorm.Config` bug in the SSL code path

There is a bug in `backend/core/runner/db.go:148` — when the SSL path is taken, `gorm.Open` is called with `&gorm.Config{}` (empty) instead of the `conf` parameter. This silently drops GORM logging config and session settings when SSL is enabled.

**Files:**
- Modify: `backend/core/runner/db.go:148`
- Test: `backend/core/runner/db_test.go` (create if doesn't exist)

**Interfaces:**
- Consumes: `MakeDbConnection(dbUrl string, conf *gorm.Config)` — existing function
- Produces: same function, bug-fixed — `conf` is now used on the SSL path

- [ ] **Step 1: Write the failing test**

Create `backend/core/runner/db_test.go` with a test that verifies the `conf` parameter is respected on the TLS path. Since we can't connect to a real MySQL in a unit test, test the `sanitizeQuery` function and verify the TLS registration logic structurally.

```go
package runner

import (
	"net/url"
	"testing"
)

func TestSanitizeQuery_RemovesCaCert(t *testing.T) {
	q := url.Values{}
	q.Set("charset", "utf8mb4")
	q.Set("ca-cert", "/etc/ssl/rds-ca.pem")
	q.Set("loc", "UTC")

	result := sanitizeQuery(q)

	if q.Get("ca-cert") != "" {
		t.Error("sanitizeQuery should remove ca-cert from query")
	}
	if q.Get("loc") != "UTC" {
		t.Error("sanitizeQuery should preserve existing loc value")
	}
	// ca-cert should not appear in the encoded result
	if result != "charset=utf8mb4&loc=UTC" {
		t.Errorf("unexpected query string: %s", result)
	}
}

func TestSanitizeQuery_SetsDefaultLoc(t *testing.T) {
	q := url.Values{}
	q.Set("charset", "utf8mb4")

	sanitizeQuery(q)

	if q.Get("loc") != "Local" {
		t.Errorf("expected default loc=Local, got %s", q.Get("loc"))
	}
}
```

- [ ] **Step 2: Run test to verify it passes** (these test existing behavior)

Run: `cd backend && go test ./core/runner/ -run TestSanitizeQuery -v`
Expected: PASS

- [ ] **Step 3: Fix the gorm.Config bug**

In `backend/core/runner/db.go`, change line 148 from:

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: db,
}), &gorm.Config{})
```

to:

```go
gormDB, err := gorm.Open(mysql.New(mysql.Config{
    Conn: db,
}), conf)
```

- [ ] **Step 4: Run all tests**

Run: `cd backend && go test ./core/runner/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/core/runner/db.go backend/core/runner/db_test.go
git commit -m "fix(db): use provided gorm.Config on MySQL TLS path

The SSL code path in MakeDbConnection was using an empty gorm.Config{}
instead of the conf parameter, silently dropping logging and session
settings when TLS was enabled."
```

---

### Task 2: Fix staging Helm deployment — point Go to the mounted CA bundle

The OpenShift trusted CA injection mounts the bundle at `/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem`, but Go on Debian doesn't read from `/etc/pki/`. The fix is to set the `SSL_CERT_FILE` environment variable, which Go's `crypto/x509` respects natively.

**Files:**
- Modify: `../infra-common-deployments/components/konflux-devlake/internal-staging/helm-values.yaml`

**Interfaces:**
- Consumes: OpenShift trusted CA ConfigMap mounted at `/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem`
- Produces: Go's `SystemCertPool()` can find the RDS CA cert

- [ ] **Step 1: Add SSL_CERT_FILE to staging helm-values.yaml**

In `infra-common-deployments/components/konflux-devlake/internal-staging/helm-values.yaml`, add the environment variable under `lake.envs`:

```yaml
    # Point Go's crypto/x509 to the OpenShift-injected CA bundle.
    # The container is Debian-based; Go reads /etc/ssl/certs/ by default,
    # not /etc/pki/ where OpenShift mounts the trusted CA ConfigMap.
    SSL_CERT_FILE: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
```

- [ ] **Step 2: Verify the ArgoCD sync / pod restart picks it up**

After merging, watch the staging pod logs:
```bash
oc logs -f -l app.kubernetes.io/name=devlake -n konflux-devlake --tail=50
```
Expected: no `tls: failed to verify certificate` or `x509: certificate signed by unknown authority` errors.

- [ ] **Step 3: Verify the staging UI loads**

Open: https://konflux-devlake-ui-konflux-devlake.apps.rosa.kflux-c-stg-i01.qfla.p3.openshiftapps.com/projects
Expected: projects page loads with data.

- [ ] **Step 4: Commit**

```bash
cd /path/to/infra-common-deployments
git add components/konflux-devlake/internal-staging/helm-values.yaml
git commit -m "fix(konflux-devlake): set SSL_CERT_FILE so Go finds the mounted CA bundle

tls=true relies on Go's SystemCertPool(), which on the Debian-based
container reads /etc/ssl/certs/. The OpenShift trusted-ca ConfigMap
is mounted at /etc/pki/. SSL_CERT_FILE bridges the gap."
```

---

### Task 3: Add SSL support to the local docker-compose-dev.yml

Create a new compose overlay file `docker-compose-external-db.yml` that:
1. Mounts the RDS CA bundle at the same path OpenShift uses (`/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem`)
2. Sets `SSL_CERT_FILE` to point Go's `SystemCertPool()` at the mounted bundle
3. Overrides `DB_URL` to include `tls=true`

This mirrors the staging/production approach exactly — same mount path, same env var, same `tls=true` mode — so local dev catches CA issues before they hit deployed environments.

**Files:**
- Create: `docker-compose-external-db.yml`
- Modify: `env.example` (document the new SSL vars)

**Interfaces:**
- Consumes: `certs/rds-combined-ca-bundle.pem` (already downloaded by ai-stack)
- Produces: DevLake connects to a remote MySQL over SSL when started with `-f docker-compose-dev.yml -f docker-compose-external-db.yml`

- [ ] **Step 1: Create docker-compose-external-db.yml**

```yaml
# MySQL SSL overlay for connecting DevLake to a remote RDS instance
#
# Usage:
#   podman compose -f docker-compose-dev.yml -f docker-compose-external-db.yml up -d devlake
#
# Prerequisites:
#   - RDS CA bundle at certs/rds-combined-ca-bundle.pem
#   - Set DB_SSL_HOST, DB_SSL_USER, DB_SSL_PASS in .env
#
services:
  devlake:
    volumes:
      - ./certs/rds-combined-ca-bundle.pem:/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem:ro
    environment:
      DB_URL: "mysql://${DB_SSL_USER}:${DB_SSL_PASS}@${DB_SSL_HOST}:${DB_SSL_PORT:-3306}/${DB_SSL_DATABASE:-lake}?charset=utf8mb4&parseTime=True&loc=UTC&tls=true"
      SSL_CERT_FILE: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
```

- [ ] **Step 2: Add the new variables to env.example**

Append to `env.example`:

```
###############################################################################
# MySQL SSL — remote database via docker-compose-external-db.yml overlay
# Used when connecting to RDS or other remote MySQL with SSL
###############################################################################
# DB_SSL_HOST=<rds-endpoint.region.rds.amazonaws.com>
# DB_SSL_PORT=3306
# DB_SSL_USER=<db-user>
# DB_SSL_PASS=<db-password>
# DB_SSL_DATABASE=lake
```

- [ ] **Step 3: Test locally — start DevLake with SSL overlay**

Set the vars in `.env`, then:

```bash
podman compose -f docker-compose-dev.yml -f docker-compose-external-db.yml up -d devlake
podman compose -f docker-compose-dev.yml -f docker-compose-external-db.yml logs -f devlake
```

Expected: DevLake starts and connects to the remote MySQL with SSL. No `x509` or `tls` errors in logs.

- [ ] **Step 4: Test without overlay — non-SSL path still works**

```bash
podman compose -f docker-compose-dev.yml up -d
```

Expected: DevLake connects to local MySQL without SSL (unchanged behavior).

- [ ] **Step 5: Commit**

```bash
git add docker-compose-external-db.yml env.example
git commit -m "feat: add docker-compose-external-db.yml overlay for remote MySQL with TLS

Mounts the RDS CA bundle at the OpenShift CA path and sets SSL_CERT_FILE
so Go's SystemCertPool() finds it. Uses tls=true, mirroring staging."
```

---

### Task 4: Enable SSL for production

Once staging is verified, apply the same pattern to production.

**Files:**
- Modify: `../infra-common-deployments/components/konflux-devlake/internal-production/helm-values.yaml`
- Modify: `../infra-common-deployments/components/konflux-devlake/internal-production/kustomization.yaml` (add trusted-ca ConfigMap if not present)

**Interfaces:**
- Consumes: Same OpenShift trusted CA injection pattern as staging
- Produces: Production DevLake connects to RDS over SSL

- [ ] **Step 1: Check if production already has trusted-ca ConfigMap**

```bash
ls ../infra-common-deployments/components/konflux-devlake/internal-production/trusted-ca-configmap.yaml
```

If missing, create it (same as staging):

```yaml
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: trusted-ca
  labels:
    config.openshift.io/inject-trusted-cabundle: "true"
```

And add it to `internal-production/kustomization.yaml` resources.

- [ ] **Step 2: Add DB_URL with tls=true and SSL_CERT_FILE to production helm-values.yaml**

Add under `lake.envs`:

```yaml
    DB_URL: "mysql://$(MYSQL_USER):$(MYSQL_PASSWORD)@$(MYSQL_SERVER):$(MYSQL_PORT)/$(MYSQL_DATABASE)?charset=$(DB_CHARSET)&parseTime=$(DB_PARSE_TIME)&loc=$(DB_LOCATION)&tls=true"

    SSL_CERT_FILE: "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"
```

Add the volume and volumeMount (same as staging):

```yaml
  volumes:
    - name: trusted-ca
      configMap:
        name: trusted-ca
        items:
          - key: ca-bundle.crt
            path: tls-ca-bundle.pem
  volumeMounts:
    - name: trusted-ca
      mountPath: /etc/pki/ca-trust/extracted/pem
      readOnly: true
```

- [ ] **Step 3: Verify production after ArgoCD sync**

```bash
oc logs -f -l app.kubernetes.io/name=devlake -n konflux-devlake --tail=50
```

Expected: no TLS errors, projects page loads.

- [ ] **Step 4: Commit**

```bash
cd /path/to/infra-common-deployments
git add components/konflux-devlake/internal-production/
git commit -m "feat(konflux-devlake): enforce SSL for database connection in production"
```
