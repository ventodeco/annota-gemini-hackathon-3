{
  description = "Gemini Hackathon OCR App - Nix Flake Development Environment";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs?ref=nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { system = system; };
        devServices = pkgs.writeShellApplication {
          name = "dev-services";
          runtimeInputs = with pkgs; [
            coreutils
            gnugrep
            postgresql_16
            redis
          ];
          text = ''
            set -euo pipefail

            command="''${1:-status}"
            state_dir="''${ANNOTA_DEV_STATE_DIR:-''${PWD}/.direnv/dev-services}"
            pgdata="''${state_dir}/postgres"
            redis_dir="''${state_dir}/redis"
            run_dir="''${state_dir}/run"
            log_dir="''${state_dir}/logs"

            postgres_host="''${POSTGRES_HOST:-localhost}"
            postgres_port="''${POSTGRES_PORT:-5432}"
            postgres_user="''${POSTGRES_USER:-postgres}"
            postgres_password="''${POSTGRES_PASSWORD:-password}"
            postgres_db="''${POSTGRES_DB:-gemini_db}"
            redis_addr="''${REDIS_ADDR:-localhost:6379}"
            redis_host="''${redis_addr%:*}"
            redis_port="''${redis_addr##*:}"

            validate_name() {
              local label="$1"
              local value="$2"
              if ! printf '%s' "$value" | grep -Eq '^[A-Za-z_][A-Za-z0-9_]*$'; then
                echo "Invalid $label: $value" >&2
                exit 1
              fi
            }

            prepare_dirs() {
              mkdir -p "$pgdata" "$redis_dir" "$run_dir" "$log_dir"
            }

            postgres_running() {
              [[ -d "$pgdata" ]] && pg_ctl -D "$pgdata" status >/dev/null 2>&1
            }

            postgres_available() {
              pg_isready -h "$postgres_host" -p "$postgres_port" -d postgres >/dev/null 2>&1
            }

            redis_running() {
              [[ -f "$run_dir/redis.pid" ]] && kill -0 "$(cat "$run_dir/redis.pid")" >/dev/null 2>&1
            }

            redis_available() {
              redis-cli -h "$redis_host" -p "$redis_port" ping >/dev/null 2>&1
            }

            init_postgres() {
              if [[ -f "$pgdata/PG_VERSION" ]]; then
                return
              fi

              initdb -D "$pgdata" --auth-local=trust --auth-host=trust >/dev/null
            }

            start_postgres() {
              if postgres_running; then
                echo "PostgreSQL is already running from $pgdata"
                return
              fi

              if postgres_available; then
                echo "PostgreSQL is already accepting connections at $postgres_host:$postgres_port"
                return
              fi

              init_postgres
              pg_ctl \
                -D "$pgdata" \
                -l "$log_dir/postgres.log" \
                -o "-h $postgres_host -p $postgres_port -k $run_dir" \
                start

              pg_isready -h "$postgres_host" -p "$postgres_port" -d postgres >/dev/null
            }

            ensure_postgres_database() {
              validate_name "POSTGRES_USER" "$postgres_user"
              validate_name "POSTGRES_DB" "$postgres_db"

              if ! psql -h "$postgres_host" -p "$postgres_port" -d postgres -tAc "SELECT 1 FROM pg_roles WHERE rolname = '$postgres_user'" | grep -q 1; then
                psql -h "$postgres_host" -p "$postgres_port" -d postgres -v ON_ERROR_STOP=1 -v password="$postgres_password" >/dev/null <<SQL
            CREATE ROLE "$postgres_user" LOGIN SUPERUSER PASSWORD :'password';
            SQL
              fi

              psql -h "$postgres_host" -p "$postgres_port" -d postgres -v ON_ERROR_STOP=1 -v password="$postgres_password" >/dev/null <<SQL
            ALTER ROLE "$postgres_user" WITH PASSWORD :'password';
            SQL

              if ! psql -h "$postgres_host" -p "$postgres_port" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname = '$postgres_db'" | grep -q 1; then
                psql -h "$postgres_host" -p "$postgres_port" -d postgres -v ON_ERROR_STOP=1 -c "CREATE DATABASE \"$postgres_db\" OWNER \"$postgres_user\";" >/dev/null
              fi
            }

            start_redis() {
              if redis_running; then
                echo "Redis is already running from $redis_dir"
                return
              fi

              if redis_available; then
                echo "Redis is already accepting connections at $redis_host:$redis_port"
                return
              fi

              redis-server \
                --daemonize yes \
                --bind "$redis_host" \
                --port "$redis_port" \
                --dir "$redis_dir" \
                --dbfilename dump.rdb \
                --appendonly yes \
                --pidfile "$run_dir/redis.pid" \
                --logfile "$log_dir/redis.log"

              redis-cli -h "$redis_host" -p "$redis_port" ping >/dev/null
            }

            start_services() {
              prepare_dirs
              start_postgres
              ensure_postgres_database
              start_redis
              echo "Local PostgreSQL and Redis are ready."
            }

            stop_services() {
              if postgres_running; then
                pg_ctl -D "$pgdata" stop
              else
                echo "PostgreSQL is not running from $pgdata"
              fi

              if redis_running; then
                kill "$(cat "$run_dir/redis.pid")"
                rm -f "$run_dir/redis.pid"
              else
                echo "Redis is not running from $redis_dir"
              fi
            }

            status_services() {
              if postgres_running; then
                echo "PostgreSQL: running ($postgres_host:$postgres_port, data: $pgdata)"
              elif postgres_available; then
                echo "PostgreSQL: running ($postgres_host:$postgres_port, external/local service)"
              else
                echo "PostgreSQL: stopped ($pgdata)"
              fi

              if redis_running; then
                echo "Redis: running ($redis_host:$redis_port, data: $redis_dir)"
              elif redis_available; then
                echo "Redis: running ($redis_host:$redis_port, external/local service)"
              else
                echo "Redis: stopped ($redis_dir)"
              fi
            }

            show_logs() {
              echo "PostgreSQL log: $log_dir/postgres.log"
              if [[ -f "$log_dir/postgres.log" ]]; then
                sed -n '1,160p' "$log_dir/postgres.log"
              fi
              echo ""
              echo "Redis log: $log_dir/redis.log"
              if [[ -f "$log_dir/redis.log" ]]; then
                sed -n '1,160p' "$log_dir/redis.log"
              fi
            }

            case "$command" in
              start)
                start_services
                ;;
              stop)
                stop_services
                ;;
              status)
                status_services
                ;;
              logs)
                show_logs
                ;;
              *)
                echo "Usage: dev-services {start|stop|status|logs}" >&2
                exit 2
                ;;
            esac
          '';
        };
      in
      {
        devShells.default = pkgs.mkShell {
          inherit system;

          packages = with pkgs; [
            # Go - Backend
            go_1_25

            # Node.js - Frontend tooling (Vite, TypeScript)
            nodejs_22

            # Bun - Frontend package manager
            bun

            # Local database services
            postgresql_16
            redis
            devServices

            # Development tools
            gnumake
            curl
            jq
          ];

          # Environment variables
          env = {
            APP_BASE_URL = "http://localhost:8080";
            FRONTEND_BASE_URL = "http://localhost:5173";
            PORT = "8080";
            POSTGRES_HOST = "localhost";
            POSTGRES_PORT = "5432";
            POSTGRES_USER = "postgres";
            POSTGRES_PASSWORD = "password";
            POSTGRES_DB = "gemini_db";
            POSTGRES_SSLMODE = "disable";
            REDIS_ADDR = "localhost:6379";
            UPLOAD_DIR = "data/uploads";
          };

          # Shell hook - runs when entering the shell
          shellHook = ''
            export ANNOTA_DEV_STATE_DIR="$PWD/.direnv/dev-services"

            echo "═══════════════════════════════════════════════════════════"
            echo "  Nix Development Shell - Gemini Hackathon OCR App"
            echo "═══════════════════════════════════════════════════════════"
            echo ""
            echo "  Available commands:"
            echo "    Backend:   cd backend && go run cmd/server/main.go"
            echo "    Frontend:  cd web && bun run dev"
            echo "    Tests:     cd backend && go test ./..."
            echo "               cd web && bun run test"
            echo ""
            echo "  Local database services:"
            echo "    dev-services start   # PostgreSQL + Redis, no Docker"
            echo "    dev-services status"
            echo "    dev-services stop"
            echo ""
            echo "═══════════════════════════════════════════════════════════"
          '';
        };
      }
    );
}
