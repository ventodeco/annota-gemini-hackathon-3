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

            # Database clients (servers run via Docker Compose)
            postgresql_16
            redis

            # Development tools
            gnumake
            curl
            jq
          ];

          # Environment variables
          env = {
            GEMINI_API_KEY = "";
            APP_BASE_URL = "http://localhost:8080";
            FRONTEND_BASE_URL = "http://localhost:5173";
            PORT = "8080";
            DB_PATH = "data/app.db";
            UPLOAD_DIR = "data/uploads";
          };

          # Shell hook - runs when entering the shell
          shellHook = ''
            # Check for required environment variables
            if [ -z "$GEMINI_API_KEY" ]; then
              echo "⚠️  GEMINI_API_KEY is not set"
              echo "   Set it with: export GEMINI_API_KEY=your_api_key_here"
            fi

            echo "═══════════════════════════════════════════════════════════"
            echo "  Nix Development Shell - Gemini Hackathon OCR App"
            echo "═══════════════════════════════════════════════════════════"
            echo ""
            echo "  Available commands:"
            echo "    Backend:   cd backend && go run cmd/server/main.go"
            echo "    Frontend:  cd web && bun run dev"
            echo "    Tests:     cd backend && go test ./..."
            echo "               cd web && bun test"
            echo ""
            echo "  Database services (run separately via Docker):"
            echo "    docker-compose up -d"
            echo ""
            echo "═══════════════════════════════════════════════════════════"
          '';
        };
      }
    );
}
