# Issue #17 — Test coverage for surviving handler/hook gaps (camera lifecycle)

## Reconciliation of the original list against `dev` (35a0058)

| Original gap | Current state | Action |
| --- | --- | --- |
| `internal/auth/google.go` | Removed by #39 (Clerk). `clerk_test.go` and `identity_test.go` cover the replacement. | None, since it's obsolete |
| `internal/handlers/auth.go` | Removed by #39. | None, since it's obsolete |
| `internal/handlers/user.go` | `TestUserHandlers`, account-deletion tests in `handlers_test.go`, privacy tests from #41/#42. | None, since it's covered |
| `internal/middleware/logging.go` | `logging_test.go` exists. | None, since it's covered |
| `hooks/useAnnotationById.ts` | Thin `useQuery` wrapper. Backend ownership (403/404) is covered by `TestCrossAccountRequestsCannotReadOrChangeOwnerData`. The page has an explicit error branch. | None, because the risk is low |
| `hooks/useDrawerHeight.ts` | Pure state-to-height map. `AnnotationDrawer` tests exercise the drawer. | None, because the risk is low |
| `components/camera/*` | Presentational wrappers around `useCamera`. | None, since the behaviour lives in the hook |
| `hooks/useCamera.ts` | **No tests, with real lifecycle bugs** (below). | **Fix and add a focused test** |

## User story

As a user scanning a page, I want the camera to turn off when I leave the screen and to switch to the
camera I picked. I also want a photo taken after switching cameras to keep the right proportions.

## Bugs in `useCamera`

1. **Camera left on after leaving mid-prompt.** If the component unmounts, or `stopCamera` runs, while
   `getUserMedia` is still pending, the stream that resolves later is stored and never stopped. The
   camera indicator stays on.
2. **Switch camera races.** `switchCamera` calls `startCamera` from the old closure, so it requests the
   *old* facing mode. Meanwhile `CameraPage`'s effect restarts with the new mode. Two requests run at
   the same time, and the one that resolves last wins. The losing stream is never stopped.
3. **Stale capture size.** The canvas width and height are fixed at the first capture. After switching
   to a camera with a different resolution, captures are stretched or cropped.

## Change (`web/src/hooks/useCamera.ts` only)

- A request counter ref. `stopCamera`, every new open, and unmount invalidate pending requests. A
  stale resolved stream has its tracks stopped and is ignored, and a stale error is ignored.
- `openCamera(mode)` is shared by `startCamera()` (current mode) and `switchCamera()` (next mode).
- `streamRef` is updated where the stream is set, which removes the effect that mirrored state into
  the ref.
- Canvas dimensions are synced to the video's current size on every capture.
- The public return shape is unchanged, so `CameraPage` and the components are untouched.

## Edge cases

- Permission denied: the error is surfaced and the stream stays null. A retry clears the error on
  success.
- An unsupported browser gets the existing message.
- Repeated start, stop and switch calls leave at most one live stream.

## Acceptance criteria

- Unmounting while permission is pending stops the late stream's tracks.
- `switchCamera` requests the other facing mode, and any superseded stream is stopped.
- Denied permission exposes the error message, and a retry clears it.
- Capture uses the current video dimensions.

## Verification

- `cd web && bun run test -- src/hooks/__tests__/useCamera.test.ts` (focused).
- Required lint, typecheck and test gates run once in CI on the PR. No E2E or browser matrix: the hook's
  behaviour is fully observable via mocked `getUserMedia` and the UI is unchanged.
