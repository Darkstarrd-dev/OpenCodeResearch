# 00 - System Prompt 合成全流程

本文档梳理 opencode 发送给 LLM 的 `system` 部分（`system: { role: "system", content: "..." }`）是如何从多个来源逐步拼接、并序列化为最终 payload 的。覆盖：合成流程、依据、源代码位置、关键片段、以及不同分支情况。

> 调研基线：`C:\opencode\MyReferenceRepository\opencode`（packages/opencode/src）

---

## 1. 总览：三期管线

system 提示词并非一次性生成，而是经过三层加工：

```
┌─────────────────────────────────────────────────────────────────┐
│ 1) 内存组装  session/llm.ts:60-85                                  │
│    system: string[] = [ header ] + [ 主体（join 成一段） ]          │
├─────────────────────────────────────────────────────────────────┤
│ 2) 序列化      session/llm.ts:191-206                              │
│    system[].map(x => ({ role: "system", content: x }))            │
│    → 成为 messages 数组里的 system 消息（payload 里看到的形式）    │
├─────────────────────────────────────────────────────────────────┤
│ 3) Provider SDK 转换  provider/sdk/*                              │
│    各 SDK 把 system role 消息映射为对应 API 字段                   │
│    （Anthropic 独立 system 参数 / OpenAI chat 的 system role 等） │
└─────────────────────────────────────────────────────────────────┘
```

本文档聚焦第 1、2 期（即 `system` 的"内容"是如何构成的）。第 3 期由各 provider SDK 负责格式转换，不影响内容本身。

---

## 2. 内存组装：`system: string[]`（`session/llm.ts:60-85`）

```ts
// session/llm.ts:60
const system = SystemPrompt.header(input.model.providerID)
system.push(
  [
    // 用 agent prompt，否则回退到 provider prompt
    ...(input.agent.prompt ? [input.agent.prompt] : SystemPrompt.provider(input.model)),
    // 框架自动注入：environment + AGENTS.md/instructions
    ...input.system,
    // 调用方在本条消息里临时携带的 system 覆盖
    ...(input.user.system ? [input.user.system] : []),
  ]
    .filter((x) => x)
    .join("\n"),
)
```

要点：
- `header` 与"主体"是两个独立的数组元素（用于后续缓存结构，见 §6）。
- 主体由 **四块** 用 `\n` 拼接：`agent/provider prompt` + `input.system` + `user.system`。
- `StreamInput.system` 字段类型见 `session/llm.ts:31-42`（`system: string[]`）。

---

## 3. 四块来源详解

### 3.1 header —— `SystemPrompt.header(providerID)`（`session/system.ts:22-25`）

```ts
export function header(providerID: string) {
  if (providerID.includes("anthropic")) return [PROMPT_ANTHROPIC_SPOOF.trim()]
  return []
}
```

- **仅当 providerID 含 `anthropic`** 时前置一段 spoof 文本。
- 内容来自 `session/prompt/anthropic_spoof.txt`（全文仅 1 行）：

  ```
  You are Claude Code, Anthropic's official CLI for Claude.
  ```

- 非 anthropic provider：`header` 为空数组，整个 `system` 只有"主体"一段。
- 另有 `SystemPrompt.instructions()`（`system.ts:27-29`）返回 `PROMPT_CODEX_INSTRUCTIONS`，仅用于 Codex（OAuth）路径，见 §6。

### 3.2 agent prompt 或 provider prompt（二选一）—— `session/llm.ts:64`

```ts
...(input.agent.prompt ? [input.agent.prompt] : SystemPrompt.provider(input.model)),
```

**互斥分支**：
- 若 agent 显式定义了 `prompt` → 整段用它，**完全跳过** provider 级 prompt（`SystemPrompt.provider` 不被调用，`api.id` 匹配在此不发生）。
- 否则 → 走 `SystemPrompt.provider(model)` 按模型选择 provider 级 prompt。

#### 3.2.1 各内置 agent 是否有专属 prompt（`session/agent/agent.ts`）

| Agent | 有 `prompt`? | 实际生效的 system 主体首块 |
|-------|--------------|---------------------------|
| `build` | 无（`agent.ts:67-79`） | `SystemPrompt.provider(model)` |
| `plan` | 无（`agent.ts:80-96`） | `SystemPrompt.provider(model)` |
| `general` | 无（`agent.ts:97-111`） | `SystemPrompt.provider(model)` |
| `explore` | 有 `PROMPT_EXPLORE`（`agent.ts:134`） | `PROMPT_EXPLORE`（跳过 provider 匹配） |

> `plan` 与 `general` 的区别在于权限/模式（`plan` 是 primary 且编辑受限；`general` 是 subagent 且禁用 todoread/todowrite），**系统提示词内容来源一致**。

#### 3.2.2 provider 级 prompt 选择 —— `SystemPrompt.provider(model)`（`session/system.ts:31-38`）

```ts
export function provider(model: Provider.Model) {
  if (model.api.id.includes("gpt-5")) return [PROMPT_CODEX]
  if (model.api.id.includes("gpt-") || model.api.id.includes("o1") || model.api.id.includes("o3"))
    return [PROMPT_BEAST]
  if (model.api.id.includes("gemini-")) return [PROMPT_GEMINI]
  if (model.api.id.includes("claude")) return [PROMPT_ANTHROPIC]
  return [PROMPT_ANTHROPIC_WITHOUT_TODO]
}
```

匹配对象是 **`model.api.id`**，不是 `name`，默认也不是 config key（见 §4）。

| 匹配模式（`api.id` 包含） | 文件 | 行数 | 内容定位 |
|---------------------------|------|------|----------|
| `gpt-5` | `session/prompt/codex.txt` | 318 | Codex 风格：AGENTS.md 规范、preamble、planning、sandbox/approvals、验收流程 |
| `gpt-*` / `o1` / `o3` | `session/prompt/beast.txt` | 147 | 自主 agent：反复迭代直至解决、强制 webfetch/google、详尽 workflow |
| `gemini-*` | `session/prompt/gemini.txt` | 155 | 工程规范、绝对路径、安全规则、示例驱动 |
| `claude*` | `session/prompt/anthropic.txt` | 105 | OpenCode 身份、tone/style、Task 工具、TodoWrite 示例 |
| 其他（兜底） | `session/prompt/qwen.txt` | 109 | 极简/安全导向、禁止注释、口头禅式 verbosity 约束 |

`PROMPT_*` 常量导入见 `session/system.ts:10-17`。

### 3.3 input.system = environment() + custom()（`session/prompt.ts:596`）

```ts
system: [...(await SystemPrompt.environment()), ...(await SystemPrompt.custom())],
```

这是框架**每轮自动注入**的部分，与 agent 是否有 prompt 无关（即便 `explore` 用自己的 prompt，这部分依旧追加在其后）。

#### 3.3.1 environment()（`session/system.ts:40-63`）

返回当前环境信息：
```
Here is some useful information about the environment you are running in:
<env>
  Working directory: <Instance.directory>
  Is directory a git repo: yes/no
  Platform: <process.platform>
  Today's date: <Date().toDateString()>
</env>
<files>
  （恒为空：代码里 `project.vcs === "git" && false`，文件树特征被禁用）
</files>
```

#### 3.3.2 custom()（`session/system.ts:79-137`）

按固定顺序收集"指令文档"，统一以 `"Instructions from: <path/url>\n<内容>"` 形式返回：

1. **本地规则文件**（`LOCAL_RULE_FILES`，`system.ts:65-69`）：向上查找 `AGENTS.md` / `CLAUDE.md` / `CONTEXT.md`（已废弃）。
2. **全局规则文件**（`GLOBAL_RULE_FILES`，`system.ts:70-77`）：
   - `<Global.Path.config>/AGENTS.md`
   - `~/.claude/CLAUDE.md`（除非 `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT`）
   - `OPENCODE_CONFIG_DIR/AGENTS.md`（若设置）
3. **`config.instructions`**（`system.ts:98-136`）：
   - `http(s)://` 开头 → fetch 内容（5s 超时）。
   - `~/` 开头 → 展开为用户目录。
   - 绝对路径 → glob 扫描匹配文件。
   - 相对路径 → `Filesystem.globUp` 向上查找。

收集顺序：本地规则 → 全局规则 → instructions 文件 → instruction URL。

### 3.4 user.system —— 调用方在消息里临时携带（`session/llm.ts:68`）

```ts
...(input.user.system ? [input.user.system] : []),
```

- 来源：`PromptInput.system`（`session/prompt.ts:101`，可选字符串）→ 创建 user 消息时存为 `User.system`（`message-v2.ts:315`、`prompt.ts:831`）。
- **opencode 自身从不自动生成它**：
  - config 无此键；
  - CLI 无 `--system` flag（cli 目录无相关参数）；
  - tool / subagent 调用不传它；
  - 唯一赋值点 `prompt.ts:831` 直接来自发消息请求的 `system` 字段。
- 因此**默认恒为空**（该轮 system 末尾不会追加任何内容）。
- 仅当编程式调用方（SDK / server prompt 接口 / ACP·IDE 集成 / 自动化脚本）在发送某条消息的 JSON payload 里显式带 `"system": "..."` 时才有值，且**仅作用于该轮**。

---

## 4. 关键澄清：`api.id` 来源与 `name` 无关

匹配用的是 `model.api.id`（非 `Model.id`，也非 `name`）。

`api.id` 取值优先级（`session/provider/provider.ts:695-696`）：

```ts
api: {
  id: model.id ?? existingModel?.api.id ?? modelID,
}
```

| 优先级 | 值 | 说明 |
|--------|----|------|
| 1 | config 模型里显式 `id:` 子字段 | 若写了 `id`，用它 |
| 2 | `existingModel?.api.id` | 若该 key 命中 models.dev 已知模型，继承其 api id |
| 3 | `modelID` | config `models:` 映射的 **key**（模型定义前的键名） |

- `Model.id`（`provider.ts:694`）永远等于 config key；但 prompt 匹配走的是 `api.id`。
- `name` 字段（`provider.ts:688-692`）仅用于展示/选择标签，**完全不参与** prompt 匹配。
- models.dev 来源模型：`api.id = model.id`（`provider.ts:550-551`）。

例：`models: { "my-gpt": { name: "My GPT", id: "gpt-5-mini" } }` → 匹配按 `gpt-5-mini`（命中 codex.txt）；`name: "My GPT"` 不参与；若删掉 `id:`，则回退按 key `my-gpt`（不含已知子串 → 兜底 qwen.txt）。

---

## 5. 序列化为 payload（`session/llm.ts:191-206`）

```ts
messages: [
  ...(isCodex
    ? [{ role: "user", content: system.join("\n\n") } as ModelMessage]   // Codex 特例
    : system.map((x): ModelMessage => ({ role: "system", content: x }))),
  ...input.messages,
],
```

- 默认：每个 `system` 数组元素 → 一条 `{ role: "system", content: <字符串> }` 消息。
- 你抓到的 `system: {"role":"system","content":"You are opencode..."}` 正是此形态；其 `content` 就是 §3 各块 `\n` 拼接后的整段文本。
- `user.system` 若有值，已包含在该 `content` 文本**末尾**，不是独立字段。

---

## 6. 分支情况汇总

### 6.1 agent 是否有专属 prompt
- 有（如 `explore`）：用 `PROMPT_EXPLORE`，`api.id` 匹配不发生。
- 无（如 `build`/`plan`/`general`）：按 `api.id` 选 provider prompt。

### 6.2 anthropic vs 非 anthropic
- **anthropic**：`header` = `[PROMPT_ANTHROPIC_SPOOF]` → `system` 含 2 个元素（header + 主体）。
- 之后 `llm.ts:80-85` 维持 2 段结构以便 Anthropic/Bedrock 做 `cacheControl`（`provider/transform.ts:141-181` 的 `applyCaching`）：
  ```ts
  if (system.length > 2 && system[0] === header) {
    const rest = system.slice(1)
    system.length = 0
    system.push(header, rest.join("\n"))
  }
  ```
  即线上发送为 2 条 system 消息：spoof header + 长正文。
- **非 anthropic**：`header` 为空 → `system` 仅 1 个元素 → payload 里 1 条 system 消息（即你抓到的形态）。

### 6.3 Codex（OpenAI OAuth）路径
- provider 为 `openai` 且 auth.type === `oauth`（`llm.ts:89`）。
- system 整体被 join 成**一条 `role: "user"`** 消息（`llm.ts:192-198`），而非 system role。
- 额外设置 `options.instructions = SystemPrompt.instructions()`、`store: false`（`llm.ts:102-105`），并在 headers 注入 `originator/session_id` 等（`llm.ts:172-179`）。

### 6.4 plugin 改写
- `experimental.chat.system.transform`（`llm.ts:76`）：plugin 可整体改写 `system` 数组；若改为空则回退原始（`llm.ts:77-79`）。

### 6.5 user.system 是否存在
- 默认空（见 §3.4）；仅编程式调用方在单条消息 payload 带 `system` 字段时才有值，追加在 system 正文末尾。

---

## 7. 源代码位置速查表

| 主题 | 文件:行 |
|------|---------|
| `StreamInput` 类型 | `session/llm.ts:31-42` |
| system 内存组装 | `session/llm.ts:60-85` |
| 序列化为 system 消息 / Codex 特例 | `session/llm.ts:191-206` |
| 缓存 2 段结构维护 | `session/llm.ts:80-85` |
| `SystemPrompt.header` | `session/system.ts:22-25` |
| `SystemPrompt.instructions`（Codex） | `session/system.ts:27-29` |
| `SystemPrompt.provider`（api.id 匹配） | `session/system.ts:31-38` |
| `SystemPrompt.environment` | `session/system.ts:40-63` |
| `SystemPrompt.custom`（AGENTS.md/instructions） | `session/system.ts:79-137` |
| `PROMPT_*` 导入 | `session/system.ts:10-17` |
| 内置 agent 定义（含 explore 的 PROMPT_EXPLORE） | `session/agent/agent.ts:67-138` |
| `PromptInput.system`（发消息请求字段） | `session/prompt.ts:101` |
| `input.system = environment + custom` | `session/prompt.ts:596` |
| user 消息存储 `system: input.system` | `session/prompt.ts:831` |
| `User.system` 类型 | `session/message-v2.ts:315` |
| `Model.api.id` / `name` 来源（config） | `session/provider/provider.ts:693-706` |
| `Model.api.id`（models.dev） | `session/provider/provider.ts:550-551` |
| `Model` schema | `session/provider/provider.ts:459-527` |
| caching 应用 | `provider/transform.ts:141-181` |

## 8. prompt 文本文件清单

| 文件 | 用途 |
|------|------|
| `session/prompt/anthropic_spoof.txt` | anthropic 的 header spoof（1 行） |
| `session/prompt/anthropic.txt` | claude* provider prompt |
| `session/prompt/beast.txt` | gpt*/o1/o3 provider prompt |
| `session/prompt/codex.txt` | gpt-5 provider prompt |
| `session/prompt/gemini.txt` | gemini-* provider prompt |
| `session/prompt/qwen.txt` | 兜底（其他模型）provider prompt |
| `session/prompt/codex_header.txt` | Codex 路径的 instructions |
| `session/agent/prompt/explore.txt` | `explore` agent 专属 prompt（文件搜索专家） |
