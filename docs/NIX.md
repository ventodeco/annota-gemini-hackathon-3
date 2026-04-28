# Nix Flake Setup Guide

Quick reference for setting up and using Nix Flakes in this project.

---

## Table of Contents

1. [Setup Nix](#1-setup-nix)
2. [Using Nix in This Project](#2-using-nix-in-this-project)
3. [What Nix Changes](#3-what-nix-changes)
4. [How It Works](#4-how-it-works)
5. [Common Commands](#5-common-commands)
6. [Troubleshooting](#6-troubleshooting)

---

## 1. Setup Nix

### 1.1 Enable Nix in Your Shell

Nix is already installed on your machine. Add to your `~/.zshrc`:

```bash
# Add this line to ~/.zshrc
source /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
```

Then restart terminal or run:
```bash
source ~/.zshrc
```

### 1.2 Verify Nix Works

```bash
nix --version
# Should show: nix version 2.x.x or higher
```

### 1.3 Enable Flakes (One-Time)

```bash
mkdir -p ~/.config/nix
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
```

### 1.4 Install Direnv (Optional but Recommended)

Direnv auto-activates the Nix shell when you `cd` into the project.

```bash
# macOS
brew install direnv

# Linux
nix-env -iA nixpkgs.direnv
```

Add to `~/.zshrc`:
```bash
eval "$(direnv hook zsh)"
```

---

## 2. Using Nix in This Project

### 2.1 Navigate to Project

```bash
cd /path/to/annota-gemini-hackathon-3
```

### 2.2 Enter Development Shell

**Option A: With Direnv (Recommended)**
```bash
direnv allow
# Shell auto-activates when you cd into project
```

**Option B: Without Direnv**
```bash
nix develop
```

### 2.3 Verify Packages

```bash
go version    # Should show go1.25.x
bun --version # Should show bun 1.x.x
node --version # Should show node 22.x.x
```

### 2.4 Run the Application

```bash
# Start database services
docker-compose up -d

# Backend (in one terminal)
cd backend && go run cmd/server/main.go

# Frontend (in another terminal)
cd web && bun run dev
```

---

## 3. What Nix Changes

### Impact on Project

| Aspect | Before | After |
|--------|--------|-------|
| New collaborator setup | 30-60 min manual install | `nix develop` (5 min) |
| Go version management | Manual | Declarative in `flake.nix` |
| Bun/Node versions | Manual | Declarative in `flake.nix` |
| Cleanup | Uninstaller needed | Just `exit` shell |

### What Stays the Same

```
✅ docker-compose up -d      # PostgreSQL & Redis
✅ go run cmd/server/main.go # Backend
✅ bun run dev              # Frontend
✅ All existing workflows    # No changes
```

### What Nix Adds

```
nix develop      # Enter isolated dev environment
nix flake check  # Validate flake
nix flake update # Update dependencies
```

---

## 4. How It Works

### File Structure

```
repository-root/
├── flake.nix      # Nix configuration (what packages/versions)
├── flake.lock     # Locked versions (reproducible)
└── .envrc         # Direnv instruction ("use flake")
```

### How Nix Dev Shell Works

```nix
# flake.nix structure
{
  description = "Gemini Hackathon OCR App";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs?ref=nixos-unstable";
  };

  outputs = { self, nixpkgs }: {
    devShells.aarch64-darwin.default = pkgs.mkShell {
      packages = with pkgs; [
        go_1_25        # Go 1.25.x
        nodejs_22       # Node.js 22.x
        bun             # Bun package manager
        postgresql_16  # PostgreSQL client
        redis           # Redis client
      ];
    };
  };
}
```

### Packages Included

| Package | Version | Purpose |
|---------|---------|---------|
| `go_1_25` | 1.25.9 | Backend runtime |
| `nodejs_22` | 22.x | Frontend tooling (Vite) |
| `bun` | 1.3.x | Frontend package manager |
| `postgresql_16` | 16.x | DB client (connects to Docker) |
| `redis` | 8.x | Cache client (connects to Docker) |

### Direnv Flow

```
cd /path/to/annota-gemini-hackathon-3
    │
    ▼
.direnv reads .envrc
    │
    ▼
.envrc says "use flake"
    │
    ▼
Nix provides environment
    │
    ▼
go, bun, node available
```

---

## 5. Common Commands

### Development

| Command | Description |
|---------|-------------|
| `nix develop` | Enter dev shell |
| `direnv allow` | Enable direnv auto-activation |
| `exit` | Exit dev shell |

### Flake Management

| Command | Description |
|---------|-------------|
| `nix flake check` | Validate flake syntax |
| `nix flake show` | Display available outputs |
| `nix flake update` | Update locked dependencies |
| `nix flake metadata` | Show flake metadata |

### Package Discovery

| Command | Description |
|---------|-------------|
| `nix search nixpkgs go` | Search for Go packages |
| `nix search nixpkgs nodejs` | Search for Node packages |

### Example: Adding a New Package

1. Edit `flake.nix`:
```nix
packages = with pkgs; [
  go_1_25
  nodejs_22
  bun
  postgresql_16
  redis
  golangci-lint  # NEW
];
```

2. Update lock file:
```bash
nix flake update
```

3. Commit changes:
```bash
git add flake.nix flake.lock
git commit -m "feat: add golangci-lint to dev shell"
```

---

## 6. Troubleshooting

### "command not found: nix"

Nix not in PATH. Source your nix profile:

```bash
source ~/.nix-profile/etc/profile.d/nix.sh
# or
source /nix/var/nix/profiles/default/etc/profile.d/nix-daemon.sh
```

### "flake is not yet supported"

Enable flakes in nix.conf:

```bash
mkdir -p ~/.config/nix
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
source ~/.zshrc
```

### Direnv not activating

```bash
# Allow direnv in project
direnv allow

# Check status
direnv status

# Reload if needed
direnv reload
```

### Package not found in nixpkgs

Package names differ. Search first:

```bash
nix search nixpkgs golang
# or
nix search nixpkgs nodejs
```

---

## Further Reading

- [Nix Flakes Official Docs](https://nixos.org/manual/nix/stable/command-ref/new-cli/nix3-flake.html)
- [Nix.dev](https://nix.dev) - Tutorials and guides
- [Nixpkgs Manual](https://nixos.org/manual/nixpkgs/stable/) - Package references

---

## Quick Reference

```bash
# First time setup
source ~/.zshrc
direnv allow

# Enter dev shell
nix develop

# Update dependencies
nix flake update
```
