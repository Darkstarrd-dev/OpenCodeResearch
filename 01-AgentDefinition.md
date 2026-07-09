# 01 - Agent 定义、合并与优先级

本文档梳理 opencode 中 agent 的定义方式、built-in 与 custom 的作用范围差异、覆盖语义，以及多层配置 / 多来源同名时的优先级，并说明 `@` 子 agent 调用的运行机制。

> 调研基线：`C:\opencode\MyReferenceRepository\opencode`（packages/opencode/src）

---

## 1. 定义 agent 的方式

共三类，最终**都汇入 `cfg.agent`**，再经 `Agent.state()` 的合并循环与 built-in 合并（`session/agent/agent.ts:187`）。

| 方式 | 来源 | 代码位置 |
|------|------|----------|
| **Built-in（硬编码）** | `agent.ts` 内 `build` / `plan` / `general` / `explore` / `compaction` / `title` / `summary` | `agent.ts:66-185` |
| **配置文件** | `opencode.jsonc` / `opencode.json` 的 `agent` 字段 | schema `config.ts:527-603`，并入 `config.ts:118,131` |
| **Markdown 文件** | `{agent,agents}/**/*.md`，实际发现于 `.opencode/agent/`、`agent/`、`agents/`、`.opencode/agents/` 等目录 | `AGENT_GLOB` + `loadAgent` `config.ts:256-286` |

Markdown 方式：frontmatter 写元数据（model/description/mode/...），**正文即 `prompt`**（`config.ts:276` `prompt: md.content.trim()`）。

### 1.1 合并总链路

```
built-in (agent.ts)
   └─ 被 cfg.agent 覆盖（agent.ts:187 循环，字段级 ??）
cfg.agent = 全局 opencode.jsonc
   └─ mergeDeep 目录级 opencode.jsonc（目录级胜）
       └─ mergeDeep loadAgent(md)（md 胜）
           └─ mergeDeep loadMode(md)
```

---

## 2. built-in vs custom 作用范围差异

两者走同一条合并路径，运行时无任何"特权分支"。可区分点：

- **`native` 标志**：built-in 设 `native: true`（`agent.ts:78,95,110,137...`），custom 新建时 `native: false`（`agent.ts:200`）。该标志在核心流程中不被用作行为开关（grep 未见其作为条件分支），属元信息。
- **`mode` 决定可见性/可用性**（对两者一致，`tool/task.ts:24` 据此过滤）：
  - `primary` —— 可切换的主 agent；`default_agent` 必须是 primary（`config.ts:885-889`）。
  - `subagent` —— 经 Task 工具 / `@` 调用。
  - `all` —— 两者兼具（custom 新建默认 `mode:"all"`，`agent.ts:197`）。
- built-in 中 `compaction` / `title` / `summary` 带 `hidden: true`，不参与普通选择。

> 结论：**作用范围无本质差异**；差异仅在"是否内置"标记与个别 hidden 内置 agent。`mode` 对 built-in / custom 一视同仁。

---

## 3. custom 能否覆盖 built-in？部分还是完全？

**能覆盖，且是"字段级部分覆盖"，不是整对象替换。** 合并循环 `agent.ts:187-213`：

```ts
for (const [key, value] of Object.entries(cfg.agent ?? {})) {
  if (value.disable) { delete result[key]; continue }   // 完全移除 built-in
  let item = result[key]
  if (!item) item = result[key] = { name: key, mode: "all",
      permission: PermissionNext.merge(defaults, user), options: {}, native: false }  // 新建 custom
  if (value.model) item.model = Provider.parseModel(value.model)
  item.prompt       = value.prompt       ?? item.prompt
  item.description  = value.description  ?? item.description
  item.temperature  = value.temperature  ?? item.temperature
  item.topP         = value.top_p        ?? item.topP
  item.mode         = value.mode         ?? item.mode
  item.color        = value.color        ?? item.color
  item.hidden       = value.hidden       ?? item.hidden
  item.name         = value.name         ?? item.name
  item.steps        = value.steps        ?? item.steps
  item.options      = mergeDeep(item.options, value.options ?? {})        // 深合并
  item.permission   = PermissionNext.merge(item.permission, ...)          // 合并，非替换
}
```

| 字段 | 覆盖语义 |
|------|----------|
| `model` | 设了就**替换** |
| `prompt` | `value.prompt ?? item.prompt` → 提供则替换，否则保留 built-in |
| `description` / `temperature` / `topP` / `mode` / `color` / `hidden` / `name` / `steps` | 同上，**提供才覆盖** |
| `options` | `mergeDeep` → **深合并**（custom 只补增量） |
| `permission` | `PermissionNext.merge` → **合并**（与 default/user 规则叠加，非替换） |
| `disable: true` | **整条删除**该 built-in（`agent.ts:188-190`） |

- **新建**（key 在 built-in 中不存在）→ 创建全新 custom agent，默认 `native:false`、`mode:"all"`、permission = `defaults+user`、options `{}`。
- **覆盖同名 built-in**（如 `agent: { build: {...} }`）→ **就地部分覆盖**：只改你写的字段，其余 built-in 属性（含 `native:true`、原 permission、原 mode）保留。
- `build` / `plan` / `general` / `explore` 的"身份"无法被改名抹除，只能靠 `disable` 整条移除。

例：
```jsonc
// opencode.jsonc
agent: {
  build:    { prompt: "自定义 build 提示词" },  // 仅覆盖 prompt，其余 built-in 保留
  explore:  { disable: true },                  // 完全移除 explore
  myagent:  { prompt: "新 agent", mode: "subagent" }  // 新建 custom
}
```

---

## 4. 多来源同名时的优先级

### 4.1 jsonc `agent` 字段 vs markdown 同名（同目录）

**两者都生效，字段级深合并，markdown 胜出。**

加载顺序（`config.ts`）：
```ts
result = mergeConfigConcatArrays(result, await loadFile(dir/opencode.jsonc))  // ① jsonc 的 agent
...
result.agent = mergeDeep(result.agent, await loadAgent(dir))                 // ② markdown 叠在其上
```
② 用 `mergeDeep` 叠在 ① 之上（source 胜），同名 agent 逐字段合并，冲突标量（如 `prompt`）取 markdown。

叠加 built-in 层后完整优先级：**markdown > jsonc > built-in**（同名 agent 字段级）。

### 4.2 全局 vs 目录级 opencode.json（同名）

**目录级覆盖全局。**

`config.ts:91-134` 的 `directories` 数组：`[Global.Path.config(全局), ...up(cwd→worktree), ...up(home)]`，循环里后加载者胜（`mergeConfigConcatArrays` = `mergeDeep`，source 胜）。全局最先加载（最低优先级），目录级后加载（胜出）。

⚠️ 多层目录级反直觉点：若目录级存在**多层** `.opencode`，`up()` 从 cwd 向上遍历到 worktree（`util/filesystem.ts:43`），cwd 先、worktree 后 → **更靠近根/stop 的层覆盖更靠近 cwd 的层**（上层覆盖下层），与常见"最近者优先"相反，但代码即如此。

### 4.3 `.opencode/agent` 与 `agent/` 同名（同根目录）

**两者在同一次递归扫描中发现，同名时"最后扫描到的整个替换"，不做字段合并。**

```ts
const AGENT_GLOB = new Bun.Glob("{agent,agents}/**/*.md")   // config.ts:256，递归
...
result[config.name] = parsed.data                           // loadAgent:273-280，硬赋值
```

- `.opencode/agent/foo.md` 与 `agent/foo.md` 会被同一次 `loadAgent(dir)` 的递归扫描同时命中（`config.ts:131`）。
- 同名 `foo` → `result["foo"]` 被赋值两次，**最后一次 wins（整体替换）**；两个 md 之间不像"jsonc↔md"那样 `mergeDeep`。
- 扫描顺序由 Bun.Glob 遍历顺序决定，无明确文档化优先级。

→ 实践建议：**不要在两个目录定义同名 agent**。

---

## 5. `@` 子 agent 调用语义

`tool/task.ts` 的 Task 工具机制：

- 创建**独立子会话**，`parentID: ctx.sessionID`（`task.ts:69`）——不是切换到该 agent。
- 子会话用 `agent: agent.name` 运行（`task.ts:152`）；model 取 subagent 自己的，未设则回退父会话（`task.ts:133-136`）。
- 子会话**完整使用 subagent 定义**（prompt / permission / tools），对话发生在子 session。
- 结束后只把文本 + `<task_metadata> session_id` 作为 tool output 返回父会话（`task.ts:174-185`）。**父会话上下文不变**。
- 即在 build 模式下 `@explore ...`：以 explore 定义发一次独立请求，结果回到 build，下次不带 `@` 仍是 build agent。`@` 是对子 agent 的**工具调用**，不是 agent 切换。
- 子 agent 默认禁用 `task` / `todowrite` / `todoread`（除非显式授予权限，`task.ts:71-90,154-157`）。

可用 subagent 列表 = `Agent.list()` 过滤 `mode !== "primary"`（`task.ts:24`），并受调用方 `task` 权限约束（`task.ts:27-30`）。

---

## 6. 源代码位置速查表

| 主题 | 文件:行 |
|------|---------|
| 内置 agent 定义 | `session/agent/agent.ts:66-185` |
| built-in/custom 合并循环 | `session/agent/agent.ts:187-213` |
| `Agent.Info` 类型 | `session/agent/agent.ts:18-42` |
| config 加载目录顺序 | `config.ts:91-134` |
| jsonc 与 md 合并 | `config.ts:118,131-132` |
| `Filesystem.up`（cwd→worktree） | `util/filesystem.ts:43-56` |
| `mergeConfigConcatArrays`（source 胜） | `config.ts:27-36` |
| `loadAgent`（md 解析，硬赋值） | `config.ts:256-286` |
| `AGENT_GLOB` | `config.ts:256` |
| agent 配置 schema | `config.ts:527-603` |
| Task 工具（子会话/返回） | `tool/task.ts:23-186` |
| 子 agent 列表过滤 | `tool/task.ts:24` |
