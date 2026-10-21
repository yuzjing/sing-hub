# Sing-Hub

A modern, declarative control plane for Sing-box infrastructure.

[English](README.md) | [简体中文](README.zh-CN.md) 

Sing-Hub is a robust, lightweight DevOps configuration management system tailored for the [Sing-box](https://github.com/SagerNet/sing-box) universal proxy platform. It implements an **Immutable Infrastructure** philosophy, decoupling template syntax, runtime parameters, and deployment pipelines.

---

## 🏛️ Architecture & Philosophy

Sing-Hub adopts a **Three-Tier Decoupled Architecture**:

```
[ Foundry (Templates) ]  ---> Pure Sing-box JSON syntax templates (No business logic)
         │
         ▼
[ Modules (Inventory) ]  ---> Reusable blocks with injected params (IPs, Ports, @node bindings)
         │
         ▼
 [ Studio (Profiles) ]   ---> Assembly pipeline compiling globally valid config.json
         │
         ▼
  [ Fleet (Deploy) ]    ---> Agentless SSH dispatch (Podman Quadlet / Systemd / Docker)
```

1. **Foundry (Templates)**: 1:1 mapping to official Sing-box schema. Zero business logic, purely structural templates.
2. **Modules (Inventory)**: Concrete instances derived from Foundry. Injects real parameters (UUIDs, endpoints, routing rules) and dynamic node references.
3. **Studio (Profiles)**: Visual assembly line. Drag, drop, and sort modules to generate full-featured configurations with live Git-backed diffing.
4. **Fleet (Devices)**: Fleet manager. Inspects remote node health, verifies checksums, and pushes deployments.

---

## ✨ Key Features

- **Declarative Dynamic References (`@node`)**: Reference node attributes directly (e.g., `@node:us-east.ipv4`). Updating a node's IP automatically cascades and recompiles all associated profiles in milliseconds.
- **Dual-Track Kernel Linter**: Built-in non-blocking deep syntax verification against both **Stable** and **Alpha** Sing-box binaries. Flags syntax issues before deployment without halting delivery.
- **Agentless Remote Deployment**: Zero-dependency deployment pipeline using pure SSH. Natively supports **Podman (Rootless Quadlet)**, Systemd User Services, and Docker with automated stdin pre-flight checks and rollback.
- **GitOps-Ready Engine**: Embedded Git tracking for every profile modification. Instant rollback, commit timeline, and visual configuration diffing.
- **Single-Source-of-Truth Storage**: Zero host pollution. All configurations, SQLite databases, snapshots, and runtime sandbox binaries strictly reside in `data_dir` (`./data`).

---

## 🚀 Quick Start

### Run with Podman Quadlet (Recommended)

Reload and start:
```bash
systemctl --user daemon-reload
systemctl --user start sing-hub
```

### Local Development

**Prerequisites:**
- Go `1.27+`
- Deno `2.9+` (or latest)

```bash
# 1. Build Frontend
cd frontend
deno install
deno task build
cd ..

# 2. Run Backend
go run ./cmd/sing-hub
```

Access the dashboard at `http://127.0.0.1:5173`.


---

## 📄 License

This project is licensed under the MIT License.
