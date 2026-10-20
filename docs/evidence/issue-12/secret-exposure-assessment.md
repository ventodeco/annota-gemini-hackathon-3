# Issue #12 Secret Exposure Assessment

Date: 2026-10-20. Base: `origin/dev` at `58deb43`. Every check below records
metadata or redacted pattern matches only. No `.env` file, key value, or
historical blob content was printed or read into this record.

## Claim under review

Issue #12 says real API keys, a JWT secret, and OAuth credentials were
**committed** in `backend/.env`. The repository evidence does not support that.
The values were most likely in a developer's untracked local `backend/.env`. Git
cannot show whether they left that machine.

## Results

| Check | Result |
| --- | --- |
| Tracked env files (`git ls-files`) | No `.env`. Only `.envrc`, `backend/.env.example`, and `web/.env.example`. |
| Refs scanned | All 22 remote branches plus all 26 `refs/pull/*/head` refs, fetched locally. |
| Env-like paths ever recorded (`git rev-list --all --objects`) | `.env.example` (root, since removed), `backend/.env.example`, `web/.env.example`, and `.envrc`. No path named `.env`. |
| Provider key patterns in 1,145 reachable blobs | None: Google API key, Google OAuth client secret, `sk-`/OpenRouter, Clerk secret, AWS access key, GitHub token, or private-key block. |
| Non-placeholder `*SECRET*`/`*PASSWORD*`/`*TOKEN*` assignments | Only code identifiers and test fixtures in `backend/internal/config/config_test.go`. |
| Ignore rules (`git check-ignore -v`) | `.gitignore:24` ignores `.env` and `web/.env`. `backend/.gitignore:1` ignores `backend/.env`. Docker contexts exclude `**/.env` and `**/.env.*`. |
| Templates | Placeholder values only. |
| Default PostgreSQL password | `docker-compose.yml` and `backend/.env.example` use `password`. `config.Load` falls back to `gemini_password`. Production had no guard against either. |

## Reproduce

Run from the repository root:

```bash
git fetch origin '+refs/pull/*/head:refs/remotes/origin-pr/*'
git ls-files | grep -iE '(^|/)\.env'
git rev-list --all --objects | awk '{print $2}' | grep -iE '(^|/)\.env' | sort -u
git check-ignore -v backend/.env web/.env .env
python3 -I docs/evidence/issue-12/scan_history_secrets.py .
```

The scanner prints the rule, the path, and value lengths only.

## Consequences

- **History rewrite:** Not needed, because no secret is in git history.
  Rewriting history would not un-expose a credential that was pushed anyway.
  Rotating it is the remedy.
- **Repository fixes in this change:** Production now refuses known default
  database passwords. Local compose services bind to loopback. The secrets
  runbook in `docs/commercial-ops.md` documents rotation and verification.
- **Still open (owner access required):** Decide whether to rotate the values
  that were in the local `backend/.env`. Verify production configuration on
  the host without printing values. GitHub environment and credential-scope
  administration is already tracked in #43/#50.
