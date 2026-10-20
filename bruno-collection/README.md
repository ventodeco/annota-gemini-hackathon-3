# API client collection (Bruno or Postman)

The hand-maintained Bruno and Postman files used to live here. They covered 10 of the 27 API operations and had drifted from the API, so they were removed. Generate a collection from the OpenAPI spec instead; it is kept current with every API change:

- Spec: [`backend/docs/openapi.yaml`](../backend/docs/openapi.yaml)
- Guide with curl examples: [`backend/docs/api.md`](../backend/docs/api.md)

## Import

1. Bruno: **Import Collection → OpenAPI V3** and choose `backend/docs/openapi.yaml`. Postman: **Import** and choose the same file.
2. Create an environment (for example "Local") with these variables:

   | Variable | Value |
   |----------|-------|
   | `baseUrl` | `http://localhost:8080` |
   | `token` | Clerk session token (see below) |

3. On the collection, set **Auth → Bearer Token** to `{{token}}`. The imported requests inherit it. `/healthz` and `/readyz` work without a token.

Re-import after the spec changes rather than editing requests by hand.

## Getting a token

The API accepts Clerk session tokens only (see [`docs/clerk-auth.md`](../docs/clerk-auth.md)).

1. Sign in to the local web app with the Clerk development instance.
2. In the browser devtools console, run `await window.Clerk.session.getToken()` and copy the result.
3. Paste it into the environment's `token` variable. Don't commit it, and don't save it in a shared environment.

Session tokens expire after about 60 seconds. Fetch a new one when requests return 401.
