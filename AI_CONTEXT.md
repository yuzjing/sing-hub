# Sing-Hub AI & LLM Context Guide

> **Notice for AI Assistants & LLMs:**
> You are collaborating on **Sing-Hub**, a professional DevOps control plane for Sing-box infrastructure. 
> When generating code, writing migrations, or refactoring components, you **MUST STRICTLY** adhere to the architectural invariants and coding patterns described in this document.

---

## 1. Tech Stack & Version Invariants

| Layer | Technology | Rules & Constraints |
| :--- | :--- | :--- |
| **Backend** | **Go 1.27+** | Standard library `net/http` (Go 1.27+ routing `METHOD /path/{param}`), `log/slog`, `modernc.org/sqlite` (pure Go, CGO-free). |
| **Frontend** | **Svelte 5** | Strict **Runes** mode (`$state`, `$derived`, `$props`, `$effect`). **Never** use legacy Svelte 3/4 stores or syntax. |
| **Styling** | **Tailwind CSS v4** | Clean, modern tokens. No microscopic text (`text-[10px]` is prohibited; use `text-xs` or `text-sm`). Rounded corners: `rounded-xl`, `rounded-2xl`. |
| **Tooling** | **Deno 2.9** | Frontend workspace management, task runner, and bundling. |
| **Host OS** | **ArchLinux / Linux** | Zero host pollution. All paths derive from `data_dir` in `config.toml`. |

---

## 2. Directory Structure & System Boundaries

```text
sing-hub/
├── cmd/sing-hub/main.go            # Entrypoint, DI wiring (config -> store -> updater -> server)
├── config.toml                     # Single source of truth (Listen, DataDir, LogLevel)
├── internal/
│   ├── config/config.go            # TOML parser & safe env defaults
│   ├── model/types.go              # Pure domain data structures (Component, Module, Profile, Device)
│   ├── store/sqlite.go             # SQLite schema, migrations & CRUD with mutex locks
│   ├── assembler/engine.go         # Template interpolation, node property dynamic binding (@node)
│   ├── remote/deployer.go          # SSH Agentless deployment & container probe engine
│   ├── updater/singbox.go          # GitHub release API client, mirror multiplexer & bin manager
│   ├── versioning/git.go           # Embedded GitOps commit & diff engine
│   └── server/
│       ├── router.go               # HTTP REST routes & embedded static file handler
│       └── dist/                   # Built frontend artifacts (embed.FS)
└── frontend/
    ├── src/lib/
    │   ├── api.ts                  # Typed Fetch wrappers
    │   ├── types.ts                # TypeScript interfaces (1:1 parity with model/types.go)
    │   ├── workspace.svelte.ts     # Central reactive store using Svelte 5 Runes
    │   └── components/
    │       ├── workspaces/         # 4 Core Tab Views: Studio, Foundry, Modules, Fleet
    │       └── Modals & Drawers    # DeployModal, SettingsDrawer, ComponentModal, etc.
```

---

## 3. Core Architectural Rules

### A. Three-Tier Domain Decoupling
1. **Component (Foundry)**: Represents 1:1 official Sing-box JSON blocks (e.g., `inbounds`, `outbounds`, `route.rules`). **No hardcoded secrets or environment values.**
2. **Module (Inventory)**: Derives from a Component. Contains concrete `params_json` and tags. Can dynamically reference devices via `@node:<device_id>.<property>`.
3. **Profile (Studio)**: An ordered assembly pipeline of Modules. Compiles into a complete, valid Sing-box `config.json`.
4. **Device (Fleet)**: Network node. Stores SSH credentials, deployment strategy, and the target profile binding.

### B. Single Source of Truth (SSOT) Storage
- Never hardcode `./data` or read `os.Getenv` directly inside business packages.
- All derived paths (`sing-hub.db`, `backups/`, `bin/`, `.git/`) **MUST** be explicitly injected from `cfg.Server.DataDir`.

### C. Network Fallback Strategy (Updater Engine)
- **Control Plane (GitHub API):** Route through HTTP/SOCKS5 proxy (`github_proxy`) to fetch release tags.
- **Data Plane (Binary Download):** 
  - Priority 1: Prefix with `github_mirror` (e.g., `https://gh-proxy.net/https://github.com/...`) for direct high-speed CDN streaming.
  - Priority 2: Fallback to `github_proxy` if mirror fails.
  - Verification: Never trust database records alone. Check physical existence on disk via `os.Stat` and test runtime output with `sing-box version`.

### D. SSH Agentless Deployment
- Always inject rootless environment headers to prevent `$XDG_RUNTIME_DIR not set` errors:
  ```bash
  export PATH=$PATH:/usr/local/bin:/opt/bin:~/.local/bin:/usr/bin:/bin;
  CURRENT_UID=$(id -u 2>/dev/null || echo 0);
  if [ "$CURRENT_UID" -ne 0 ] && [ -d "/run/user/$CURRENT_UID" ]; then
    export XDG_RUNTIME_DIR="/run/user/$CURRENT_UID";
    export DBUS_SESSION_BUS_ADDRESS="unix:path=$XDG_RUNTIME_DIR/bus";
  fi;
  ```
- **Container Pre-flight Checks:** Always pass configuration via stdin pipe:
  `podman exec -i <svc> sing-box check -c /dev/stdin < tempPath`
  This eliminates host vs. container volume mount path discrepancies.

---

## 4. Frontend (Svelte 5) Implementation Standards

### ✅ Required Syntax
- Use `$state(...)` for local state and reactive properties.
- Use `$derived(...)` or `$derived.by(() => ...)` for computed values.
- Declare props using `let { propA, propB = $bindable() }: Props = $props();`.
- Use `SvelteSet` from `svelte/reactivity` for reactive Set collections.

### ❌ Prohibited Anti-Patterns
- **Do not** import `writable` or `readable` from `'svelte/store'`.
- **Do not** place `{@const ...}` tags inside ordinary HTML tags (e.g., inside `<div>`). `{@const}` **must** be an immediate child of `{#each}`, `{#if}`, or `{#snippet}`.
- **Do not** write duplicate handler functions within the same component.
- **Do not** use `any` type for caught errors. Use `catch (e: unknown)` and cast safely.

---

## 5. Backend (Go) Engineering Standards

- **SQL Mappings:** When adding columns to `internal/store/sqlite.go`, you must update:
  1. `CREATE TABLE` DDL.
  2. `ALTER TABLE` incremental migrations.
  3. `SELECT ...` column counts.
  4. `rows.Scan(...)` argument list (**Column count must match argument count exactly**).
  5. `INSERT INTO ... VALUES (...)` placeholder count.
  6. `DumpSQL` backup queries.
- **Error Formatting:** `fmt.Errorf` requires a constant format string (e.g., `fmt.Errorf("%s", errStr)` instead of `fmt.Errorf(errStr)`).
- **Concurrency:** Any mutating operation on `Store` or `Updater` must be guarded by `s.mu.Lock()`.
