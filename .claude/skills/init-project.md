# Skill: /init-project

Initialize a new project with proper structure, tooling, and AI rules configuration.

---

## Description

This skill helps you set up a new project from scratch with:
- Appropriate directory structure based on project type
- AI rules configuration via `dotai`
- Git initialization
- Basic tooling configuration

---

## Usage

```
/init-project [project-type]
```

Where `project-type` is one of: `go`, `python`, `typescript`, or leave empty for interactive selection.

---

## Process

### Step 1: Determine project type

Ask the user (if not provided):
- What type of project? (Go, Python, TypeScript)
- Project name?
- Git repository URL (optional)?

### Step 2: Create directory structure

Based on project type, create the appropriate structure:

**Go project:**
```
project/
├── cmd/
│   └── <project-name>/
│       └── main.go
├── internal/
├── pkg/
├── docs/
│   └── adr/
├── testdata/
├── go.mod
├── Makefile
└── .gitignore
```

**Python project:**
```
project/
├── src/
│   └── <package_name>/
│       └── __init__.py
├── tests/
│   ├── unit/
│   └── integration/
├── docs/
│   └── adr/
├── pyproject.toml
├── requirements.txt
└── .gitignore
```

**TypeScript project:**
```
project/
├── src/
│   └── index.ts
├── tests/
├── docs/
│   └── adr/
├── package.json
├── tsconfig.json
└── .gitignore
```

### Step 3: Initialize AI rules

Run `dotai init` with appropriate profiles:

```bash
# Go project
dotai init --profile go --tool claude

# Python project
dotai init --profile python --tool claude

# TypeScript project
dotai init --profile typescript --tool claude
```

### Step 4: Sync rules

```bash
dotai sync
```

This creates:
- `CLAUDE.md` - AI assistant rules
- `.claude/skills/` - Available skills

### Step 5: Initialize Git

```bash
git init
git add .
git commit -m "Initial project setup"
```

### Step 6: Create initial files

Create minimal starter files:

**TODO.md:**
```markdown
# TODO

## Current Sprint

- [ ] Set up development environment
- [ ] Implement core functionality
```

**README.md:**
```markdown
# <Project Name>

Brief description of the project.

## Getting Started

### Prerequisites

- [List prerequisites]

### Installation

[Installation steps]

## Development

[Development commands]
```

---

## Example Session

```
User: /init-project

Claude: What type of project would you like to create?
1. Go
2. Python
3. TypeScript

User: Go

Claude: What's the project name?

User: my-service

Claude: Creating Go project structure...
✓ Created directory structure
✓ Initialized go.mod
✓ Created Makefile
✓ Ran dotai init --profile go
✓ Ran dotai sync
✓ Initialized git repository
✓ Created initial commit

Your project is ready! Next steps:
1. cd my-service
2. Review CLAUDE.md for AI collaboration rules
3. Start coding in cmd/my-service/main.go
```

---

## Notes

- This skill requires `dotai` to be installed
- Git must be available in PATH
- For Go projects, Go 1.21+ is required
- For Python projects, uv is recommended
- For TypeScript projects, pnpm is recommended
