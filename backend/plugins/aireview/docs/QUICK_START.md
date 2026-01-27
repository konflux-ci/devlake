# AI Review Plugin Quick Start

Get started with the AI Review plugin in 5 minutes.

## Prerequisites

- DevLake instance running
- GitHub or GitLab connection configured
- Pull request data already collected

## Step 1: Verify Plugin is Loaded

```bash
curl -s http://localhost:8080/plugins | jq '.[] | select(.plugin == "aireview")'
```

Expected output:
```json
{
  "plugin": "aireview",
  "metric": {
    "requiredDataEntities": [...],
    "runAfter": ["github", "gitlab"],
    "isProjectMetric": true
  }
}
```

## Step 2: Enable Plugin for Your Project

Add the `aireview` metric to your project:

```bash
curl -X PATCH http://localhost:8080/projects/YOUR_PROJECT_NAME \
  -H "Content-Type: application/json" \
  -d '{
    "metrics": [
      {"pluginName": "aireview", "pluginOption": {}, "enable": true}
    ]
  }'
```

## Step 3: Trigger Data Collection

Run the project blueprint to collect and analyze data:

```bash
# Find your blueprint ID
curl -s http://localhost:8080/projects/YOUR_PROJECT_NAME | jq '.blueprint.id'

# Trigger the blueprint
curl -X POST http://localhost:8080/blueprints/BLUEPRINT_ID/trigger
```

## Step 4: Verify Data Collection

Check that AI reviews were extracted:

```bash
# Via API
curl -s http://localhost:8080/plugins | jq '.'

# Via MySQL
podman exec devlake-mysql-1 mysql -u merico -pmerico lake \
  -e "SELECT ai_tool, risk_level, COUNT(*) FROM _tool_aireview_reviews GROUP BY ai_tool, risk_level;"
```

## Step 5: View in Grafana

1. Open Grafana: http://localhost:4000/grafana/
2. Go to **Dashboards** → **AI Code Review Analytics**
3. Or import the dashboard from `grafana/dashboards/AIReview.json`

## Configuration Options

### Scope Config

Customize AI tool detection patterns:

```json
{
  "codeRabbitEnabled": true,
  "codeRabbitUsername": "coderabbitai",
  "codeRabbitPattern": "(?i)(coderabbit|walkthrough|summary by coderabbit)",
  "cursorBugbotEnabled": false,
  "cursorBugbotUsername": "cursor-bugbot",
  "riskHighPattern": "(?i)(critical|security|vulnerability|breaking)",
  "riskMediumPattern": "(?i)(warning|caution|potential|moderate)",
  "riskLowPattern": "(?i)(minor|suggestion|consider|nitpick)"
}
```

### Running for a Specific Repository

```bash
curl -X POST http://localhost:8080/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "AI Review - Single Repo",
    "plan": [[{
      "plugin": "aireview",
      "options": {
        "repoId": "github:GithubRepo:1:123456789"
      }
    }]]
  }'
```

### Running for Entire Project

```bash
curl -X POST http://localhost:8080/pipelines \
  -H "Content-Type: application/json" \
  -d '{
    "name": "AI Review - Project",
    "plan": [[{
      "plugin": "aireview",
      "options": {
        "projectName": "my-project"
      }
    }]]
  }'
```

## Supported AI Tools

| Tool | Default Username | Detection Pattern |
|------|------------------|-------------------|
| CodeRabbit | `coderabbitai` | Summary by CodeRabbit, Walkthrough |
| Cursor Bugbot | `cursor-bugbot` | (disabled by default) |

## Troubleshooting

### No reviews extracted

1. Check that PR comments exist:
   ```sql
   SELECT COUNT(*) FROM pull_request_comments;
   ```

2. Verify CodeRabbit comments are present:
   ```sql
   SELECT * FROM pull_request_comments
   WHERE account_id LIKE '%coderabbit%'
      OR body LIKE '%CodeRabbit%'
   LIMIT 5;
   ```

3. Check plugin logs:
   ```bash
   podman logs devlake-devlake-1 | grep aireview
   ```

### Database migration required

```bash
curl -s http://localhost:8080/proceed-db-migration
```

### Plugin not loaded

Rebuild the DevLake container:
```bash
podman compose -f docker-compose-dev.yml up -d --build devlake
```

## Next Steps

- See [METRICS_REFERENCE.md](METRICS_REFERENCE.md) for detailed metric documentation
- Customize the Grafana dashboard for your needs
- Add more AI tool patterns to `scope_config.go`
