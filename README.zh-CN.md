
# Sing-Hub

面向 Sing-box 基础设施的现代化、声明式 DevOps 控制平面。

[English](README.md) | [简体中文](README.zh-CN.md) 

Sing-Hub 是一款专为 [Sing-box](https://github.com/SagerNet/sing-box) 全能网络代理核心打造的轻量级配置编排与运维平台。系统深度实践**不可变基础设施（Immutable Infrastructure）**理念，将抽象语法母版、业务运行时参数与多端下发流水线彻底解耦。

---

## 🏛️ 核心架构哲学

Sing-Hub 严格遵循 **三层解耦控制平面模型**：

```
[ 原子母版 (Foundry) ]  ---> 1:1 映射官方 JSON 字段语法（无业务侵染）
         │
         ▼
[ 成品积木 (Modules) ]  ---> 母版实参化实例（注入节点、密钥、规则与 @node 变量）
         │
         ▼
[ 方案配方 (Studio) ]   ---> 流水线编排装配（秒级编译全局合规的 config.json）
         │
         ▼
[ 节点集群 (Fleet) ]    ---> Agentless 极客运维（直连 Podman Quadlet / Systemd / Docker）
```

1. **原子母版 (Foundry)**：官方语法 1:1 映射，无任何业务逻辑，专注文法模板（如出站协议、DNS 策略）。
2. **成品积木 (Modules)**：母版的具体实参化实体，填入 UUID、端口、分流规则，并可动态引用网络节点属性。
3. **方案配方 (Studio)**：积木流水线装配大盘。自由上下拖拽组合积木，实时多维对比镜像配置，支持基于内嵌 Git 的版本快照与 Diff 回滚。
4. **节点集群 (Fleet)**：无代理（Agentless）纳管机器实体。探活探测延迟，无感知下发配置并触发内核热重载。

---

## ✨ 核心特性

- **声明式动态节点引用 (`@node`)**：支持在积木中直接引用节点属性（如 `@node:hk-01.ipv4`）。节点更换公网 IP 时，只需在节点处修改一次，全网关联的方案与积木**毫秒级级联热更新**并重新编译。
- **双轨内核官方语法 Linter**：内置集成 Sing-box **Stable（稳定版）** 与 **Alpha（尝鲜版）** 官方内核深度校验引擎。双轨并发语法诊断，采用“只告警、不阻断”策略，确保极端环境下的业务可用性。
- **Agentless 极客运维推送**：目标机器无需安装任何受控 Agent，基于纯 SSH 通道部署。原生完美支持 **Podman 无根容器 (Rootless Quadlet)**、用户态 Systemd 服务及标准 Docker。
- **无挂载预检黑科技 (`/dev/stdin`)**：针对容器内部路径与宿主机不一致痛点，通过标准输入流将新配置直接喂入容器内核预检，杜绝路径映射错误导致的预检崩溃。
- **GitOps 全量审计回溯**：每次编译与保存均自动生成 Git Commit。提供完整的时空时光机版本轴，支持一键对比代码差异（Diff）与安全回退。
- **绝对沙箱（零系统污染）**：严格遵循 `config.toml` 单一权威源。SQLite 数据库、Git 审计仓库、备份文件以及内核二进制均受控存放在 `data_dir`（默认 `./data`），绝不在系统目录乱写垃圾。

---

## 🚀 极速部署

### 使用 Podman Quadlet 部署 (生产推荐)


启动无根容器服务：
```bash
systemctl --user daemon-reload
systemctl --user start sing-hub
```

### 本地源码编译与开发

**运行环境准备：**
- Go `1.27+`
- Deno `2.9+` 

```bash
# 1. 构建前端
cd frontend
deno install
deno task build
cd ..

# 2. 运行后端 (开发模式)
go run ./cmd/sing-hub
```

浏览器访问 `http://127.0.0.1:5173` 即可进入控制中心。


---

## 📄 开源许可证

本项目基于 [MIT License](LICENSE) 开源。
