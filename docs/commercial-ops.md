# ANNOTA Commercial Operations Runbook

## Release And Rollback

1. Build frontend with `cd web && bun run build`.
2. Run backend verification with `cd backend && go test ./...`.
3. Apply database migrations before traffic is shifted.
4. Keep the previous backend image and frontend build artifact until smoke tests pass.
5. Roll back by redeploying the previous image/artifact and restoring the prior environment values.

## Backup And Restore

- Postgres: run a daily `pg_dump` for the application database and test restore into a staging database weekly.
- Uploads: snapshot `UPLOAD_DIR` or object-storage bucket daily. Keep file backups aligned with database backup timestamps.
- Restore order: database first, uploaded files second, then run `/healthz` and smoke-test login, upload, annotation, and PDF open flows.

## Secrets Rotation

- Rotate `JWT_SECRET`, Google OAuth secret, Gemini API key, Redis credentials, and database credentials on a scheduled cadence or immediately after exposure.
- When rotating `JWT_SECRET`, expect existing sessions to be invalidated.
- Production must use `APP_ENV=production`, explicit `ALLOWED_ORIGINS`, and secure HTTPS origins.

## Support And Data Requests

- User lookup key: email address from Google OAuth.
- For deletion requests, ask the user to use the in-app Delete account action first. If manual deletion is needed, delete the user row so cascades remove scans, annotations, documents, and related rows, then remove files in `UPLOAD_DIR`.
- For abuse investigations, collect request IDs, user ID, target route, timestamp, and AI rate-limit status from structured logs.

## Billing And Entitlements

- Current entitlement foundation exposes `/v1/entitlements/me` with the active plan and configured usage limits.
- Until Stripe or another billing provider is added, all users are treated as `free` and constrained by `AI_RATE_LIMIT`, `AI_RATE_LIMIT_WINDOW_SECONDS`, and `MAX_UPLOAD_SIZE`.
- Paid plans should map provider subscription state to entitlements before raising limits.
