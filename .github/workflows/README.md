# GitHub Actions Workflows

This directory contains CI/CD workflows for the CLS Backend project.

## Workflows

### 1. CI Validation (`ci.yml`)

**Triggers:**
- On all pull requests
- On pushes to any branch

**Jobs:**
- **Lint**: Code formatting and static analysis
  - `go fmt` validation
  - `go vet` checks
  - `golangci-lint` analysis
- **Test**: Unit tests and coverage
  - Unit tests from `make test-unit`
  - Coverage reports uploaded to Codecov (if configured)
- **Build**: Binary compilation
  - Builds the `backend-api` binary
  - Verifies successful compilation
- **Docker Build**: Container build test
  - Tests Docker image builds without pushing
  - Uses build cache for performance

**Status:** All jobs must pass for PRs to be mergeable.

---

### 2. Build and Push to Quay.io (`release.yml`)

**Triggers:**
- On pushes to `main` branch (i.e., when PRs are merged)

**Jobs:**
- **Build and Push**: Multi-platform container build
  - Builds for `linux/amd64` and `linux/arm64`
  - Pushes to Quay.io with multiple tags

**Image Tags:**
- `<SHORT_SHA>` - 7-character commit SHA (e.g., `693fb2c`)
- `latest` - Always points to the most recent main branch build
- `<YYYYMMDD>` - Date-based tag (e.g., `20251202`)

**Example:**
```
quay.io/your-org/cls-backend:693fb2c
quay.io/your-org/cls-backend:latest
quay.io/your-org/cls-backend:20251202
```

---

## Required GitHub Secrets

The release workflow requires the following secrets to be configured in your GitHub repository:

### Quay.io Credentials

1. **`QUAY_USERNAME`**
   - Your Quay.io username or robot account name
   - Example: `your-org+github_actions`

2. **`QUAY_PASSWORD`**
   - Your Quay.io password or robot account token
   - For robot accounts, use the generated token

3. **`QUAY_REPOSITORY`** (optional)
   - Full repository path on Quay.io
   - Default: `cls-backend/cls-backend`
   - Example: `your-org/cls-backend`

### Setting Up Secrets

#### Via GitHub UI:

1. Go to your repository on GitHub
2. Navigate to **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add each secret with its corresponding value

#### Via GitHub CLI:

```bash
# Set Quay.io username
gh secret set QUAY_USERNAME --body "your-org+github_actions"

# Set Quay.io password (will prompt for input)
gh secret set QUAY_PASSWORD

# Set Quay.io repository (optional)
gh secret set QUAY_REPOSITORY --body "your-org/cls-backend"
```

---

## Creating a Quay.io Robot Account

For automated CI/CD, it's recommended to use a Quay.io robot account instead of personal credentials:

1. Go to your Quay.io organization: `https://quay.io/organization/your-org`
2. Click **Robot Accounts** in the left sidebar
3. Click **Create Robot Account**
4. Name it (e.g., `github_actions`)
5. Grant **Write** permissions to the `cls-backend` repository
6. Copy the generated credentials:
   - **Username**: `your-org+github_actions`
   - **Token**: (long token string)
7. Add these as GitHub secrets

---

## Testing Workflows Locally

### Testing the CI Workflow

You can test individual CI steps locally:

```bash
# Run linting
make fmt
go vet ./...

# Run tests
make test-unit
make test-coverage

# Build binary
make build

# Build Docker image
docker build -t cls-backend:test .
```

### Testing with Act

Install [act](https://github.com/nektos/act) to run GitHub Actions locally:

```bash
# Install act
brew install act  # macOS
# or
curl https://raw.githubusercontent.com/nektos/act/master/install.sh | sudo bash

# Run CI workflow
act pull_request

# Run release workflow (requires secrets)
act push --secret-file .secrets
```

---

## Workflow Status Badges

Add these badges to your main README.md:

```markdown
[![CI](https://github.com/apahim/cls-backend/actions/workflows/ci.yml/badge.svg)](https://github.com/apahim/cls-backend/actions/workflows/ci.yml)
[![Release](https://github.com/apahim/cls-backend/actions/workflows/release.yml/badge.svg)](https://github.com/apahim/cls-backend/actions/workflows/release.yml)
```

---

## Troubleshooting

### CI Workflow Issues

**Problem:** Linting fails with formatting errors
```bash
# Fix locally:
make fmt
git add .
git commit -m "Fix formatting"
```

**Problem:** Tests fail
```bash
# Run tests locally to debug:
make test-unit
```

### Release Workflow Issues

**Problem:** "Error: Username and password required"
- **Solution:** Verify `QUAY_USERNAME` and `QUAY_PASSWORD` secrets are set correctly

**Problem:** "Error: denied: access forbidden"
- **Solution:** Verify robot account has Write permissions to the repository

**Problem:** Build fails for ARM64
- **Solution:** This is usually due to platform-specific dependencies. Review Dockerfile for multi-arch compatibility

---

## Workflow Optimization

### Build Cache

Both workflows use GitHub Actions cache to speed up builds:
- Go module cache (dependencies)
- Docker layer cache (container builds)

The cache is automatically managed and can significantly reduce build times for subsequent runs.

### Pull Request Strategy

For best results:
1. Create feature branch from `main`
2. Open PR - CI workflow runs automatically
3. Address any CI failures
4. Merge PR - Release workflow builds and pushes container

---

## Security Best Practices

1. **Never commit secrets** to the repository
2. **Use robot accounts** for CI/CD instead of personal accounts
3. **Limit robot account permissions** to only what's needed (Write to specific repository)
4. **Rotate credentials regularly** (every 90 days recommended)
5. **Review workflow runs** for any suspicious activity
6. **Use branch protection** to require CI checks before merging

---

## Additional Resources

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Quay.io Documentation](https://docs.quay.io/)
- [Docker Buildx Documentation](https://docs.docker.com/buildx/working-with-buildx/)
- [Go CI/CD Best Practices](https://golang.org/doc/go1.16#go-mod-download)
