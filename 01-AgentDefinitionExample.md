# 01b - Agent 定义示例（两种等价写法）

配套 `01-AgentDefinition.md`。本文给出**两套完整且等价**的 agent 定义：一套用 `opencode.jsonc` 的 `agent` 字段，一套用 Markdown 文件。两者覆盖**所有可使用的 agent 定义字段**，并产生**完全相同的运行时 `Agent.Info`**。

---

## 1. 可用字段全集

| 字段 | 类型 | jsonc `agent` | Markdown frontmatter | 说明 |
|------|------|:---:|:---:|------|
| `name` | string | key 即 name；frontmatter 可覆盖 | 默认取文件名，frontmatter 可覆盖 | agent 标识 |
| `description` | string | ✅ | ✅ | `@` 自动补全里展示的用途说明 |
| `mode` | `subagent`/`primary`/`all` | ✅ | ✅ | 可见性/可用性 |
| `hidden` | boolean | ✅ | ✅ | 在 `@` 菜单中隐藏（仅 subagent 有意义） |
| `model` | string `"provider/model"` | ✅ | ✅ | 运行时解析为 `{providerID, modelID}` |
| `temperature` | number | ✅ | ✅ | 采样温度 |
| `top_p` / `topP` | number | `top_p` | `top_p` | 核采样（运行时归一为 `topP`） |
| `color` | `#RRGGBB` | ✅ | ✅ | TUI 显示色 |
| `prompt` | string | ✅ | **正文（body）** | 系统提示词主体 |
| `options` | record | ✅ | ✅ | 透传给 provider 的额外参数（深合并） |
| `permission` | ruleset | ✅ | ✅ | 权限规则（与 default/user 合并） |
| `steps` | int>0 | ✅ | ✅ | 最大 agentic 迭代次数 |
| `disable` | boolean | ✅（仅用于移除 built-in） | ❌ | 整条删除同名 built-in |
| `tools` | record | ✅（**已废弃**） | ✅（**已废弃**） | 映射为 `permission` |
| `maxSteps` | int | ✅（**已废弃**） | ✅（**已废弃**） | 映射为 `steps` |
| `native` | boolean | ❌（代码自动设） | ❌ | built-in=true / custom=false，用户不可设 |

> 注：`model` 在两种写法里都是**字符串** `"provider/model"`；进入 `Agent.Info` 后被 `Provider.parseModel` 拆成 `{providerID, modelID}`。

---

## 2. 定义示例：agent `reviewer`

### 2.1 写法 A —— `opencode.jsonc` 的 `agent` 字段

```jsonc
// opencode.jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "agent": {
    "reviewer": {
      "description": "代码审查子代理，负责检查改动的质量、风格与潜在风险",
      "mode": "subagent",
      "hidden": false,
      "model": "anthropic/claude-opus-4",
      "temperature": 0.3,
      "top_p": 0.9,
      "color": "#FF5733",
      "steps": 20,
      "prompt": "你是一名资深代码审查员。\n\n规则：\n- 聚焦 diff 而非整文件\n- 指出安全/_correctness 问题优先于风格\n- 用 file_path:line 引用问题位置\n- 不要自行修改代码",
      "options": {
        "reasoningEffort": "high",
        "anthropic": { "cacheControl": true }
      },
      "permission": {
        "read": "allow",
        "edit": "deny",
        "bash": "ask",
        "task": "deny"
      }
    }
  }
}
```

### 2.2 写法 B —— Markdown 文件

文件：`agent/reviewer.md`（或 `.opencode/agent/reviewer.md`）。frontmatter 写元数据，**正文即 `prompt`**。

```md
---
description: 代码审查子代理，负责检查改动的质量、风格与潜在风险
mode: subagent
hidden: false
model: anthropic/claude-opus-4
temperature: 0.3
top_p: 0.9
color: "#FF5733"
steps: 20
options:
  reasoningEffort: high
  anthropic:
    cacheControl: true
permission:
  read: allow
  edit: deny
  bash: ask
  task: deny
---

你是一名资深代码审查员。

规则：
- 聚焦 diff 而非整文件
- 指出安全/correctness 问题优先于风格
- 用 file_path:line 引用问题位置
- 不要自行修改代码
```

> 说明：Markdown 的 `name` 默认取文件名 `reviewer`；若 frontmatter 写了 `name:` 则以它为准。本例未写，故 `name = "reviewer"`，与写法 A 的 key 一致。

---

## 3. 两种写法产生的等价 `Agent.Info`

无论用写法 A 还是 B，最终进入 `Agent.state()` 且被合并循环处理后的对象完全一致：

```ts
// session/agent/agent.ts:18 Agent.Info（custom 新建分支 agent.ts:197-200）
{
  name: "reviewer",
  description: "代码审查子代理，负责检查改动的质量、风格与潜在风险",
  mode: "subagent",
  native: false,                 // 代码自动设置：custom 恒为 false
  hidden: false,
  temperature: 0.3,
  topP: 0.9,                     // 由 top_p 归一
  color: "#FF5733",
  model: { providerID: "anthropic", modelID: "claude-opus-4" },  // 由 "anthropic/claude-opus-4" 解析
  prompt: "你是一名资深代码审查员。\n\n规则：...不要自行修改代码",
  steps: 20,
  options: {
    reasoningEffort: "high",
    anthropic: { cacheControl: true }
  },
  permission: [ /* read:allow, edit:deny, bash:ask, task:deny 与 defaults+user 合并后的规则集 */ ]
}
```

---

## 4. 关键等价点对照

| 关注点 | 写法 A（jsonc） | 写法 B（md） | 运行时归一 |
|--------|----------------|--------------|-----------|
| agent 标识 | key `reviewer` | 文件名 `reviewer`（或 frontmatter `name`） | `name: "reviewer"` |
| 模型 | `model: "anthropic/claude-opus-4"` | 同左 | `model: {providerID, modelID}` |
| 核采样 | `top_p: 0.9` | `top_p: 0.9` | `topP: 0.9` |
| 提示词 | `prompt:` 字段 | 文件正文 | `prompt:` 字符串 |
| 额外参数 | `options:` 对象 | `options:` YAML | 原样透传（provider 层深合并） |
| 权限 | `permission:` 对象 | `permission:` YAML | 与 default/user 合并 |

---

## 5. 废弃字段映射（兼容写法，不推荐新用）

若仍使用旧字段，加载期会被转换（`config.ts:557-603`）：

- `tools: { "write": true }` → 等价于 `permission: { "edit": "allow" }`（write/edit/patch/multiedit 统一映射到 `edit`）。
- `maxSteps: 30` → 等价于 `steps: 30`。

---

## 6. 同时存在的优先级提醒

若同一目录**同时**提供写法 A 与写法 B 的同名 agent（`reviewer`），依 `01-AgentDefinition.md` §4.1：Markdown（写法 B）通过 `mergeDeep` 叠在 jsonc（写法 A）之上，**markdown 在冲突字段上胜出**（整体仍是字段级合并，非替换）。因此不要把同一 agent 同时用两种方式定义；本文两套示例应视为**二选一的等价写法**。
