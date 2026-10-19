# Sing-Hub

A modern, declarative control plane for Sing-box infrastructure.
[中文文档](#中文说明)

Sing-Hub is a robust DevOps configuration management system tailored for the [Sing-box](https://github.com/SagerNet/sing-box) universal proxy platform. It implements an Immutable Infrastructure philosophy, decoupling template syntax, runtime parameters, and deployment pipelines.

## Architecture & Philosophy

Sing-Hub follows a strict **Three-Tier Architecture (Control Plane)**:
1. **Foundry (Templates)**: 1:1 mapping to Sing-box official JSON fields. Pure syntax, no business logic.
2. **Modules (Inventory)**: Derived instances from Foundry. Inject runtime parameters (IPs, Ports, UUIDs) and Node references.
3. **Studio (Profiles)**: The assembly line. Drag-and-drop Modules to compile a globally valid `config.json`.

**Key Features:**
- **Dynamic Node References (`@node`)**: Change a node's IP once, and all related profiles auto-recompile.
- **Dual-Track Linter**: Built-in non-blocking syntax validation against both Stable and Alpha Sing-box cores.
- **Agentless Deployment**: Zero-dependency SSH push to target instances (Podman/Docker/Native).

---

## 中文说明

Sing-Hub 是一个为 Sing-box 基础设施量身定制的现代化、声明式 DevOps 控制平面。

### 核心设计哲学
1. **三层解耦模型**: 
   - `原子母版 (Foundry)`: 纯粹的 Sing-box 官方字段模板。
   - `成品积木 (Modules)`: 注入实参（如多出站、分流规则）的实体。
   - `方案配方 (Profiles)`: 装配积木生成最终 `config.json` 的流水线。
2. **声明式动态引用 (`@node`)**: 积木中通过 `@node:hk-01.ipv4` 绑定节点。节点 IP 发生变更时，全网关联配置毫秒级自动热更新。
3. **内置非阻塞双轨 Linter**: 集成 Stable / Alpha 双版本核心校验，只报警不阻断，保证极端情况下的配置可用性。

---

## 🤖 AI Context Guide (For LLM Assistants)

When contributing to this repository, please adhere strictly to the following architectural and stylistic rules:

### Tech Stack
- **Backend:** Go 1.22+ (Standard Library `net/http` routing, `log/slog`, `text/template`, SQLite `modernc.org/sqlite`).
- **Frontend:** Svelte 5 (Runes `$state`, `$derived`, `$effect`), Tailwind CSS v4, TypeScript.

### Design Principles
1. **Svelte 5 Strictness:** DO NOT use Svelte 4 syntax. Never use `<script>` context for `{@const}` (use `$derived` instead). `$props()` must not have type arguments (e.g., use `let {}: Props = $props()`).
2. **CSS/Styling:** Use standard Tailwind classes. Avoid microscopic fonts (`text-[10px]`), standardize on `text-xs` (12px) and `text-sm` (14px). Emphasize visual hierarchy using rounded corners (`rounded-xl`, `rounded-2xl`), subtle borders, and contrast backgrounds (`bg-zinc-50/70`).
3. **Error Handling:** Backend returns JSON for all APIs. `400 Bad Request` should return `{ "error": "...", "json": "..." }` when applicable. Frontend must gracefully catch and toast errors.

### Directory Structure
```text
sing-hub/
├── cmd/sing-hub/main.go       # Go entrypoint & Dependency Injection
├── internal/
│   ├── assembler/engine.go    # Template compilation & Linter logic
│   ├── model/types.go         # Core structs (Component, Module, Profile, Device)
│   ├── remote/deployer.go     # SSH Agentless deployment logic
│   ├── server/router.go       # HTTP API Handlers
│   ├── store/sqlite.go        # DB operations & Migrations
│   └── updater/singbox.go     # Background binary fetcher
└── frontend/
    ├── src/lib/components/    # Svelte UI Components & Modals
    ├── src/lib/workspaces/    # 4 Main Workspace Views
    ├── src/lib/workspace.svelte.ts # Global State Manager
    └── src/routes/+page.svelte# Main App Layout