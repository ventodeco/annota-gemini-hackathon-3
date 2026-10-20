# ANNOTA API guide

The OpenAPI spec, [`openapi.yaml`](openapi.yaml), is the reference for every route, schema and error. This guide covers how to call the API from a terminal. To browse the spec, import it into Bruno, Postman or Swagger Editor, or run `bunx @redocly/cli preview-docs backend/docs/openapi.yaml`.

## Base URL and authentication

- Local backend: `http://localhost:8080`. The web app calls it through `VITE_API_BASE_URL`.
- `/healthz` and `/readyz` are public. Every `/v1/*` route needs a Clerk session token in `Authorization: Bearer <token>`. There are no API keys, and the old Google OAuth and `x-token` routes no longer exist.
- The server maps the Clerk user to a numeric local account. Clients never send a user ID; every scan, document and annotation is scoped to the caller.

To get a token for local testing:

1. Sign in to the local web app with the Clerk development instance.
2. In the browser devtools console, run `await window.Clerk.session.getToken()`.
3. Export the result in your shell. Don't paste it into files, issues or chat.

```sh
export ANNOTA_URL=http://localhost:8080
export ANNOTA_TOKEN='<paste the session token here>'
```

Session tokens expire after about 60 seconds. When a request returns `401 Unauthorized: invalid token`, get a new one. Authentication errors (401, 403, 409, 503) are listed in the spec description.

## Errors

Errors are JSON objects:

```json
{ "error": "Too Many Requests", "message": "You have used today's explanation allowance. It resets daily at 00:00 UTC.", "code": "usage_quota_exceeded", "feature": "explanation", "limit": 50, "used": 50, "remaining": 0, "resetAt": "2026-10-21T00:00:00Z" }
```

Branch on `code` when it is present, not on `message`:

| Status | `code` | Meaning |
|--------|--------|---------|
| 429 | `rate_limited` | Burst limit on AI-backed POSTs; wait `Retry-After` seconds |
| 429 | `usage_quota_exceeded` | Your daily allowance for `feature` is used; resets at `resetAt` (00:00 UTC) |
| 429 | `usage_budget_exhausted` | The shared daily beta budget is used |
| 429 | `ai_provider_limited` | The upstream AI provider is limiting requests |
| 413 | `usage_request_too_large` | Selection is too long to speak (`max` characters) |
| 503 | `usage_unavailable` | Allowance could not be checked; retry shortly |

Check the remaining allowance with `GET /v1/entitlements/me`. The default daily limits are 10 OCR pages, 50 explanations and 1500 speech characters, with at most 250 characters per speech request. See [`docs/usage-quotas.md`](../../docs/usage-quotas.md).

## Examples

These examples use placeholder files and IDs. Use your own test images and PDFs, not private documents. Requests to `/v1/scans`, `/v1/ai/analyze` and `/v1/ai/speech` call the configured AI provider and count against your allowance.

Health:

```sh
curl -i "$ANNOTA_URL/healthz"
curl -i "$ANNOTA_URL/readyz"
```

Preferences and allowance:

```sh
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/users/me"
curl -s -X PATCH -H "Authorization: Bearer $ANNOTA_TOKEN" -H 'Content-Type: application/json' \
  -d '{"preferredLanguage":"EN"}' "$ANNOTA_URL/v1/users/me"
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/entitlements/me"
```

Upload an image, then poll until `status` is `ready` or `failed`:

```sh
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" \
  -F 'image=@./page.jpg;type=image/jpeg' "$ANNOTA_URL/v1/scans"
# {"scanId":42,"imageUrl":"/v1/scans/42/image","sourceType":"image","status":"processing"}

curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/scans/42"
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" -o scan-42.jpg "$ANNOTA_URL/v1/scans/42/image"
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/scans?page=1&size=20"
```

Upload a PDF, read a page, and get the page's scan for explanations:

```sh
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" \
  -F 'file=@./sample.pdf;type=application/pdf' "$ANNOTA_URL/v1/documents"
# {"documentId":3,"pageCount":12,"filename":"sample.pdf"}

curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/documents/3/pages/4"
curl -s -X POST -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/documents/3/pages/4/scan"
# {"scanId":41}   (201 when created, 200 when it already existed)

curl -s -X PATCH -H "Authorization: Bearer $ANNOTA_TOKEN" -H 'Content-Type: application/json' \
  -d '{"lastPageNumber":4}' "$ANNOTA_URL/v1/documents/3/progress"
```

Explain a selection. With `scanId`, the server uses the scan's text as context. Keep the response for saving:

```sh
curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" -H 'Content-Type: application/json' \
  -d '{"textToAnalyze":"お疲れ様です","context":"会議が終わった後、同僚にお疲れ様ですと言いました。","scanId":42}' \
  "$ANNOTA_URL/v1/ai/analyze" > analysis.json
```

Speak a selection. The audio format depends on the provider, so check `Content-Type`:

```sh
curl -s -D - -H "Authorization: Bearer $ANNOTA_TOKEN" -H 'Content-Type: application/json' \
  -d '{"highlightedText":"お疲れ様です"}' -o speech.audio "$ANNOTA_URL/v1/ai/speech"
```

Save the explanation as an annotation. Send the analyze response back unchanged as `nuanceData`; its `provenance` is kept only if it is unmodified. This example uses `jq`. Then list or delete annotations:

```sh
jq -n --slurpfile analysis analysis.json \
  '{scanId: 42, highlightedText: "お疲れ様です", contextText: "会議が終わった後、同僚にお疲れ様ですと言いました。", nuanceData: $analysis[0]}' |
  curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" -H 'Content-Type: application/json' \
    -d @- "$ANNOTA_URL/v1/annotations"
# {"annotationId":7,"status":"saved"}

curl -s -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/annotations?documentId=3&pageNumber=4"
curl -s -X DELETE -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/annotations/7"
```

Delete the account. This permanently removes all uploads and annotations and the Clerk user. Retry if it returns 502:

```sh
curl -i -X DELETE -H "Authorization: Bearer $ANNOTA_TOKEN" "$ANNOTA_URL/v1/users/me"
```

## Keeping the docs current

Any change to a route, request or response field, status code or error `code` updates `openapi.yaml` in the same pull request. Validate with:

```sh
bunx @redocly/cli lint backend/docs/openapi.yaml
```
