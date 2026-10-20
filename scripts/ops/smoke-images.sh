#!/usr/bin/env bash
# Exercise final images with disposable, local-only dependencies. No provider calls.
set -euo pipefail

mode="${1:-all}"
case "$mode" in
  all|backend|frontend) ;;
  *) echo "Usage: $0 [all|backend|frontend]" >&2; exit 2 ;;
esac

for tool in docker python3; do
  command -v "$tool" >/dev/null || { echo "Required tool missing: $tool" >&2; exit 1; }
done
docker info >/dev/null

repo_root="$(cd "$(dirname "$0")/../.." && pwd)"
backend_image="${BACKEND_IMAGE:-annota-be:issue-43}"
frontend_image="${FRONTEND_IMAGE:-annota-fe:issue-43}"
postgres_image="${SMOKE_POSTGRES_IMAGE:-postgres:14}"
redis_image="${SMOKE_REDIS_IMAGE:-redis:7-alpine}"
probe_image="${SMOKE_PROBE_IMAGE:-python:3.12-alpine}"
prefix="annota-smoke-$$-${RANDOM}"
network_created=false
containers=()
probe_count=0

cleanup() {
  local status=$?
  trap - EXIT
  for container in "${containers[@]-}"; do
    [[ -n "$container" ]] || continue
    if [[ "$status" -ne 0 ]]; then
      docker logs --tail 60 "$container" >&2 || true
    fi
    docker rm -fv "$container" >/dev/null || true
  done
  if [[ "$network_created" == true ]]; then
    docker network rm "$prefix" >/dev/null || true
  fi
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

start_container() {
  local name="$1"
  shift
  docker create --name "$name" --network "$prefix" "$@" >/dev/null
  containers+=("$name")
  docker start "$name" >/dev/null
}

run_python() {
  probe_count=$((probe_count + 1))
  local name="$prefix-probe-$probe_count"
  docker create --interactive --name "$name" --network "$prefix" \
    "$probe_image" python3 - "$@" >/dev/null
  containers+=("$name")
  docker start --attach --interactive "$name"
}

wait_for_command() {
  local attempt
  for ((attempt = 0; attempt < 45; attempt++)); do
    if "$@" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  echo "Timed out waiting for disposable service" >&2
  return 1
}

wait_for_http() {
  run_python "$1" <<'PY'
import sys
import time
import urllib.error
import urllib.request

deadline = time.monotonic() + 60
while time.monotonic() < deadline:
    try:
        with urllib.request.urlopen(sys.argv[1], timeout=2) as response:
            if response.status == 200:
                break
    except (OSError, urllib.error.URLError):
        pass
    time.sleep(1)
else:
    sys.exit("Timed out waiting for final-image HTTP response")
PY
}

expect_http() {
  run_python "$1" "$2" <<'PY'
import sys
import time
import urllib.error
import urllib.request

url, want = sys.argv[1], int(sys.argv[2])
status = None
deadline = time.monotonic() + 60
while time.monotonic() < deadline:
    try:
        with urllib.request.urlopen(url, timeout=5) as response:
            status = response.status
    except urllib.error.HTTPError as error:
        status = error.code
    except OSError:
        status = None
    if status == want:
        print(f"{url} -> {status}")
        break
    time.sleep(1)
else:
    sys.exit(f"Expected HTTP {want} from {url}; last status {status}")
PY
}

docker network create --internal "$prefix" >/dev/null
network_created=true

if [[ "$mode" == all || "$mode" == backend ]]; then
  docker image inspect --format 'Backend image: {{.Id}}' "$backend_image"
  # This deliberately fails when the final image omits runtime migrations.
  if ! docker run --rm --network none --entrypoint sh "$backend_image" -c \
    'test -d /app/migrations && test -n "$(find /app/migrations -name "*.sql" -print)"'; then
    echo "Final backend image is missing runtime SQL migrations" >&2
    exit 1
  fi

  start_container "$prefix-pg" \
    -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=smoke-only -e POSTGRES_DB=annota_smoke \
    "$postgres_image"
  start_container "$prefix-redis" "$redis_image"
  wait_for_command docker exec "$prefix-pg" pg_isready -h 127.0.0.1 -U postgres -d annota_smoke
  wait_for_command docker exec "$prefix-redis" redis-cli ping

  start_container "$prefix-backend" \
    --tmpfs /app/data/uploads \
    -e "DB_CONNECTION_STRING=postgres://postgres:smoke-only@$prefix-pg:5432/annota_smoke?sslmode=disable" \
    -e "REDIS_ADDR=$prefix-redis:6379" \
    -e APP_ENV=development -e PORT=8080 -e UPLOAD_DIR=/app/data/uploads \
    -e AI_PROVIDER=minimax -e OPENROUTER_API_KEY=smoke-fixture -e MINIMAX_API_KEY=smoke-fixture \
    -e OPENROUTER_BASE_URL=http://127.0.0.1:9 \
    -e MINIMAX_ANTHROPIC_BASE_URL=http://127.0.0.1:9 \
    -e MINIMAX_TTS_BASE_URL=http://127.0.0.1:9 \
    -e CLERK_SECRET_KEY=sk_test_smoke-fixture \
    -e CLERK_PUBLISHABLE_KEY=pk_test_c21va2UuY2xlcmsuYWNjb3VudHMuZGV2JA \
    -e FRONTEND_BASE_URL=http://localhost:5173 \
    "$backend_image"
  wait_for_http "http://$prefix-backend:8080/readyz"

  expected_migrations="$(python3 - "$repo_root/backend/migrations" <<'PY'
from pathlib import Path
import sys
print("\n".join(sorted(p.name for p in Path(sys.argv[1]).rglob("*.sql"))))
PY
  )"
  actual_migrations="$(docker exec "$prefix-pg" psql -U postgres -d annota_smoke -At \
    -c 'SELECT name FROM schema_migrations ORDER BY name')"
  if [[ -z "$expected_migrations" || "$actual_migrations" != "$expected_migrations" ]]; then
    echo "Final backend did not apply the complete migration inventory" >&2
    exit 1
  fi

  # Readiness must follow dependency availability while liveness stays up.
  backend_url="http://$prefix-backend:8080"
  for dependency in redis pg; do
    docker stop "$prefix-$dependency" >/dev/null
    expect_http "$backend_url/readyz" 503
    expect_http "$backend_url/healthz" 200
    docker start "$prefix-$dependency" >/dev/null
    expect_http "$backend_url/readyz" 200
  done
  echo "Backend final-image startup, migration inventory, and readiness passed"
fi

if [[ "$mode" == all || "$mode" == frontend ]]; then
  docker image inspect --format 'Frontend image: {{.Id}}' "$frontend_image"
  start_container "$prefix-frontend" "$frontend_image"
  frontend_url="http://$prefix-frontend:80"
  wait_for_http "$frontend_url/"
  worker_paths="$(docker exec "$prefix-frontend" find /usr/share/nginx/html/assets \
    -type f -name 'pdf.worker*.mjs')"

  run_python "$frontend_url" "$worker_paths" <<'PY'
from html.parser import HTMLParser
import json
import sys
from urllib.parse import urlsplit
import urllib.request

base_url, worker_paths = sys.argv[1:]

def fetch(path, content_types):
    if not path.startswith("/") or path.startswith("//"):
        raise SystemExit(f"Expected a local generated asset path: {path}")
    with urllib.request.urlopen(base_url + path, timeout=10) as response:
        body = response.read()
        mime = response.headers.get_content_type()
        if response.status != 200 or mime not in content_types or not body:
            raise SystemExit(f"Unexpected response for {path}: {response.status}, {mime}")
        if mime != "text/html" and body.lstrip().lower().startswith((b"<!doctype html", b"<html")):
            raise SystemExit(f"Asset returned HTML: {path}")
        return body

class EntryAssets(HTMLParser):
    def __init__(self):
        super().__init__()
        self.scripts = []
        self.styles = []
        self.external_styles = []
        self.manifests = []

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "script" and attrs.get("src"):
            self.scripts.append(attrs["src"])
        if tag == "link" and attrs.get("href"):
            rel = attrs.get("rel", "").split()
            if "stylesheet" in rel:
                target = urlsplit(attrs["href"])
                if target.scheme or target.netloc:
                    self.external_styles.append(attrs["href"])
                else:
                    self.styles.append(attrs["href"])
            if "manifest" in rel:
                self.manifests.append(attrs["href"])

index = fetch("/", {"text/html"})
assets = EntryAssets()
assets.feed(index.decode())
if not assets.scripts or not assets.styles or not assets.manifests or not worker_paths:
    raise SystemExit("Final frontend is missing entry scripts, CSS, manifest, or PDF worker")
for path in assets.scripts:
    fetch(path, {"application/javascript", "text/javascript"})
for path in assets.styles:
    fetch(path, {"text/css"})
for path in assets.manifests:
    manifest = json.loads(fetch(path, {"application/manifest+json", "application/json"}))
    if not isinstance(manifest, dict) or not manifest.get("name"):
        raise SystemExit("Invalid PWA manifest")
for path in worker_paths.splitlines():
    fetch(path.removeprefix("/usr/share/nginx/html"), {"application/javascript", "text/javascript"})
if fetch("/login", {"text/html"}) != index:
    raise SystemExit("Direct client route did not serve the SPA entry document")
if assets.external_styles:
    print(f"External stylesheets outside the final-image check: {len(assets.external_styles)}")
print("Frontend final-image HTML, JS, CSS, manifest, PDF worker, and client route passed")
PY
fi
