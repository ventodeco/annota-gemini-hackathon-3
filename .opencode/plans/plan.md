# Authentication Loop & 503 Error Fix Plan

## Issue 1: Authentication Loop (Critical)

**Root Cause:** Hard redirect in `web/src/lib/api.ts` lines 57-60

When `getUserProfile()` returns 401, the code does:
```typescript
if (isAuthError(response.status)) {
  clearAuthToken()
  window.location.href = '/login'  // <-- PROBLEM
}
```

This causes a full page reload that bypasses React Router's `ProtectedRoute`, creating a race condition where the user gets kicked back to login after successful OAuth.

**Fix:** Remove the `window.location.href = '/login'` line. The `ProtectedRoute` component already handles redirects naturally when `isAuthenticated` becomes false.

**File:** `web/src/lib/api.ts`
**Lines:** 57-60
**Change:** Remove line 59

---

## Issue 2: 503 Error - Frontend Not Built (Critical)

**Root Cause:** Backend tries to serve frontend from `web/dist/` but build doesn't exist

From backend logs:
```
GET /auth/callback 503
Body: "Frontend not built. Run: cd web && bun run build"
```

The OAuth callback redirects to `/auth/callback` (a frontend route handled by React Router), but the backend can't serve it because `web/dist/index.html` is missing.

**Location:** `backend/cmd/server/main.go` lines 103-118

**Solution:** Build the frontend before running the backend

**Commands:**
```bash
cd web
bun install  # if not already installed
bun run build
```

This creates `web/dist/` with the production build that the backend can serve.

---

## Additional Issue: Missing Knowledge CSV (Warning - Non-blocking)

**Log:**
```
Warning: Failed to load knowledge CSV from data/knowledge/knowledge-service.md
```

This is just a warning and doesn't affect authentication. The app continues without knowledge context.

---

## Execution Order

1. **Fix Issue 1** - Remove hard redirect from `web/src/lib/api.ts`
2. **Fix Issue 2** - Build frontend: `cd web && bun run build`
3. **Restart backend** - `cd backend && go run ./cmd/server`
4. **Test login flow** - Should now work without redirect loop

## Files to Modify

1. `web/src/lib/api.ts` - Remove hard redirect on auth error
2. (No code change needed for Issue 2, just build step)

## Verification Steps

1. Build frontend
2. Start backend
3. Navigate to login page
4. Click "Login with Google"
5. Complete OAuth flow
6. Should redirect to `/welcome` and stay there (not kicked back to login)
7. Refresh page - should remain on `/welcome`
8. Logout - should redirect to `/login`
9. Try accessing protected route without login - should redirect to `/login`
