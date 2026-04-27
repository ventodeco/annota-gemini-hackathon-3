# Nix Flakes - Learning Guide

This document explains how Nix Flakes work and how this project's flake is configured.

## What is Nix Flake?

A **Nix Flake** is a declarative, reproducible way to define Nix inputs, packages, and development environments. Flakes bring several improvements over traditional Nix:

- **Locked dependencies** - `flake.lock` pins exact versions
- **Reproducible** - Same input = Same output
- **Composable** - Easy to reference other flakes
- **Self-contained** - All configuration in `flake.nix`

## Key Concepts

### 1. Inputs
Inputs are dependencies - other flakes or external sources (like GitHub repos).

```nix
inputs = {
  nixpkgs.url = "github:NixOS/nixpkgs?ref=nixos-unstable";
  flake-utils.url = "github:numtide/flake-utils";
};
```

### 2. Outputs
Outputs are what the flake produces - packages, dev shells, etc.

```nix
outputs = { self, nixpkgs, flake-utils }: {
  # ...
};
```

### 3. Dev Shells
A `devShell` defines a reproducible development environment with specific packages.

```nix
devShells.default = pkgs.mkShell {
  packages = with pkgs; [ go_1_25 nodejs_22 bun ];
};
```

## This Project's Flake Structure

```
flake.nix
├── inputs          # nixpkgs, flake-utils
├── outputs
│   └── devShells.default  # The development environment
│       ├── packages       # go_1_25, nodejs_22, bun, etc.
│       ├── env            # Environment variables
│       └── shellHook      # Startup messages/hooks
└── flake.lock     # Auto-generated lock file
```

## Quick Start

### Prerequisites

Install Nix with flakes support:

```bash
# On macOS or Linux
curl -L https://nixos.org/nix/install | sh

# After installation, enable flakes (one-time)
mkdir -p ~/.config/nix
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
```

Install direnv for automatic shell activation:

```bash
# macOS
brew install direnv

# Linux (NixOS)
nix-env -iA nixpkgs.direnv

# Add to your shell config (~/.zshrc or ~/.bashrc)
eval "$(direnv hook zsh)"  # for zsh
# or
eval "$(direnv hook bash)" # for bash
```

### Using This Project's Flake

```bash
# Navigate to project
cd nix-flake-worktree

# Allow direnv (first time only)
direnv allow

# Or manually enter the shell
nix develop

# Verify packages are available
which go bun node
```

## Common Commands

| Command | Description |
|---------|-------------|
| `nix flake check` | Validate flake syntax |
| `nix flake show` | Display what the flake provides |
| `nix flake update` | Update locked dependencies |
| `nix develop` | Enter the dev shell |
| `nix build` | Build a package from the flake |
| `nix run nixpkgs#package` | Run a package directly |

## How Direnv Integration Works

1. `.envrc` contains `use flake`
2. When you `cd` into the directory, direnv reads `.envrc`
3. Direnv calls `nix flake` to get the environment
4. Environment variables and PATH are automatically activated
5. When you leave the directory, the environment is deactivated

```
project/
├── flake.nix      # Nix flake configuration
├── flake.lock     # Locked versions (commit this!)
└── .envrc         # "use flake" - tells direnv to use the flake
```

## Understanding the Packages

| Package | Purpose | Why in Nix |
|---------|---------|------------|
| `go_1_25` | Go compiler | Build and run backend |
| `nodejs_22` | Node.js | Required by Vite build tool |
| `bun` | JS package manager | Install frontend deps |
| `postgresql_16` | PostgreSQL client | Connect to Docker DB |
| `redis` | Redis client | Connect to Docker Redis |

## Environment Variables

Set in the flake's `env` attribute:

```nix
env = {
  GEMINI_API_KEY = "";
  APP_BASE_URL = "http://localhost:8080";
  # ...
};
```

Override at runtime:

```bash
export GEMINI_API_KEY=your_key_here
```

## Flake.lock Explained

The `flake.lock` file locks all inputs to specific revisions/commit hashes.

```
nix-flake-worktree/
├── flake.nix      # Your flake definition
├── flake.lock     # Auto-generated - COMMIT THIS!
└── .envrc         # Direnv integration
```

**Important**: Commit `flake.lock` to ensure reproducible builds across machines and time.

## Adding New Dependencies

Edit `flake.nix`:

```nix
packages = with pkgs; [
  go_1_25
  nodejs_22
  bun
  postgresql_16
  redis
  # Add new package here, e.g:
  # golangci-lint
  # nodePackages.typescript
];
```

Then update the lock:

```bash
nix flake update
```

## Common Patterns

### Pattern 1: Simple Dev Shell

```nix
pkgs.mkShell {
  packages = with pkgs; [ hello git ];
}
```

### Pattern 2: With Environment Variables

```nix
pkgs.mkShell {
  packages = with pkgs; [ go_1_25 ];
  env = { MY_VAR = "value"; };
}
```

### Pattern 3: With Shell Hook

```nix
pkgs.mkShell {
  packages = with pkgs; [ go_1_25 ];
  shellHook = ''
    echo "Welcome to the dev shell!"
    # Any bash code here runs on shell entry
  '';
}
```

### Pattern 4: Multiple Outputs

```nix
outputs = { self, nixpkgs }: {
  packages.x86_64-linux.default = /* ... */;
  devShells.x86_64-linux.default = /* ... */;
};
```

## Troubleshooting

### "command not found: nix"

Nix is not installed or not in PATH. Install or source your nix profile:

```bash
source ~/.nix-profile/etc/profile.d/nix.sh
```

### "flake is not yet supported"

Enable flakes in nix.conf:

```bash
mkdir -p ~/.config/nix
echo "experimental-features = nix-command flakes" >> ~/.config/nix/nix.conf
```

Then restart your shell.

### Direnv not activating

```bash
# Check if direnv is working
direnv allow
direnv status
```

### Package not found

The package name might be different in nixpkgs. Search:

```bash
nix search nixpkgs go
```

## Further Learning

- [Nix Flakes](https://nixos.org/manual/nix/stable/command-ref/new-cli/nix3-flake.html) - Official docs
- [Nix.dev](https://nix.dev) - Tutorials and guides
- [Nixpkgs Manual](https://nixos.org/manual/nixpkgs/stable/) - Package references
- [awesome-nix](https://github.com/nix-community/awesome-nix) - Community resources

## Next Steps

1. Run `direnv allow` in the project directory
2. Try `nix develop` to enter the shell
3. Experiment with `nix search nixpkgs` to find packages
4. Read other open-source project flake.nix files for patterns
