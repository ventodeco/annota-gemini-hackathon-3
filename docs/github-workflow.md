# GitHub Issue Workflow with gh CLI

This project uses GitHub Issues with GitHub CLI (`gh`) for tracking all work.

## Quick Start

Choose your workflow:

### Option 1: Main Branch (Default - Simple)

```bash
# 1. List issues
gh issue list

# 2. Create issue for your work
gh issue create --label backend --title "[TASK] Add feature"

# 3. Ensure on main
git checkout main
git pull origin main

# 4. Work and commit with issue references
git commit -m "feat: add feature (#42)"

# 5. Push to main
git push origin main

# 6. Close issue when done
gh issue close #42 --comment "Fixed in main"
```

### Option 2: Feature Branch (Optional - For Isolation)

```bash
# 1. List issues
gh issue list

# 2. Create issue
gh issue create --label backend --title "[TASK] Add feature"

# 3. Create branch
git checkout -b feature/issue-42-description

# 4. Work and commit
git commit -m "feat: add feature (#42)"

# 5. Push branch
git push -u origin feature/issue-42-description

# 6. Create PR
gh pr create --title "feat: add feature (#42)" --body "Fixes #42"

# 7. After merge, close issue
gh issue close #42
```

## Prerequisites

Install GitHub CLI:

```bash
# macOS
brew install gh

# Login to GitHub
gh auth login
```

## GitHub CLI Commands

### Issue Management

```bash
# List all open issues
gh issue list

# List by label
gh issue list --label frontend
gh issue list --label "bug,backend"

# Search issues
gh issue list --search "camera"

# View issue details
gh issue view #42
gh issue view #42 --web  # Open in browser

# Create issue
gh issue create
gh issue create --label bug,backend --title "[BUG] Title" --body "..."

# Close issue
gh issue close #42 --comment "Fixed"

# Reopen issue
gh issue reopen #42

# Add comment
gh issue comment #42 --body "Update"

# Edit issue
gh issue edit #42 --title "New title" --add-label priority-high
```

### PR Management (for Feature Branch workflow)

```bash
# Create PR
gh pr create --title "feat: add feature (#42)" --body "Fixes #42"

# View PR
gh pr view
gh pr view #43

# List PRs
gh pr list
gh pr list --state merged

# Checkout PR locally
gh pr checkout #43

# Check PR status
gh pr checks #43

# Merge PR
gh pr merge #43
gh pr merge #43 --squash
gh pr merge #43 --auto
```

## Workflows in Detail

### Main Branch Workflow (Default)

Best for:
- Solo development
- Small teams (2-3 people)
- Quick fixes
- Fast iteration

Steps:
1. Create or find issue: `gh issue create`
2. Ensure on main: `git checkout main && git pull`
3. Work and commit with issue ref: `git commit -m "feat: ... (#42)"`
4. Push: `git push origin main`
5. Verify CI: `gh run list`
6. Close issue: `gh issue close #42`

### Feature Branch Workflow (Optional)

Best for:
- Larger teams
- Code review requirements
- Long-running features
- Risky changes

Steps:
1. Create issue: `gh issue create`
2. Create branch: `git checkout -b feature/issue-42-desc`
3. Work and commit: `git commit -m "feat: ... (#42)"`
4. Push branch: `git push -u origin feature/issue-42-desc`
5. Create PR: `gh pr create --body "Fixes #42"`
6. Get review, merge
7. Issue auto-closes on merge

## Commit Message Format

Always reference the issue:

```bash
# Format: <type>: <description> (#<issue>)
git commit -m "feat: add user login (#42)"
git commit -m "fix: handle null in scanner (#42)"
git commit -m "test: add auth tests (#42)"
git commit -m "docs: update README (#42)"
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation
- `style`: Styling/formatting
- `refactor`: Code restructuring
- `test`: Tests
- `perf`: Performance
- `chore`: Maintenance

## Labels

| Label | Use For |
|-------|---------|
| `bug` | Something is broken |
| `enhancement` | New feature |
| `task` | Technical work |
| `backend` | Backend changes |
| `frontend` | Frontend changes |
| `api` | API changes |
| `database` | Database changes |
| `gemini` | Gemini integration |
| `security` | Security-related |
| `performance` | Performance work |
| `priority-high` | Urgent |
| `priority-low` | Nice to have |

## Tips

```bash
# Set up aliases
gh alias set il 'issue list --assignee @me'
gh alias set ic 'issue create'
gh alias set close 'issue close'

# View repo in browser
gh repo view --web

# Open issue in browser
gh issue view --web

# Check CI status
gh run list
gh run view
```

## Choosing Your Workflow

| Scenario | Recommended |
|----------|-------------|
| Solo dev | Main branch |
| Small team | Main branch |
| Large team | Feature branches |
| Critical code | Feature branches |
| Quick fix | Main branch |
| Big refactor | Feature branch |
