# System Prompt 合成管线完整梳理

> 本文档归纳 opencode 中 system prompt 的所有来源、分叉逻辑、边缘状况，以及所有相关配置的详细用法。

---

## 1. Agent Prompt 或 Provider 模板（二选一）

**代码位置：** `packages/opencode/src/session/llm/request.ts:60`

```typescript
input.agent.prompt ? [input.agent.prompt] : SystemPrompt.provider(input.model)
```

| 条件 | 使用内容 |
|---|---|
| Agent 的 .md 文件正文中有 prompt 内容 | 使用 agent prompt（来自 .md 文件 `---` 分隔后的正文） |
| 否则 | 使用 provider 模板（根据模型名分派） |

### Provider 分派表（system.ts:26-40）

| 模型名匹配 | 模板文件 |
|---|---|
| `gpt-4` / `o1` / `o3` | `beast.txt` |
| `codex` | `codex.txt` |
| 其他 `gpt` | `gpt.txt` |
| `gemini-` | `gemini.txt` |
| `claude` | `anthropic.txt` |
| `trinity` | `trinity.txt` |
| `kimi` | `kimi.txt` |
| 兜底 | `default.txt` |

### Agent prompt 来源（config/agent.ts:24-27）

```typescript
const config = {
  name,
  ...md.data,        // frontmatter 数据（含 permission 等）
  prompt: md.content.trim(),  // --- 分隔后的正文
}
```

**.md 文件路径规则：** `{agent,agents}/**/*.md`，即 `.opencode/agent/` 或 `.opencode/agents/` 下的所有子目录中的 `.md` 文件。

---

## 2. Environment Info

**代码位置：** `packages/opencode/src/session/system.ts:58-94`

每次请求自动注入，不可关闭。内容结构：

```
You are powered by the model named X. The exact model ID is A/B
<env>
  Working directory: X
  Workspace root folder: X
  Is directory a git repo: yes/no
  Platform: X
  Today's date: X
</env>
```

如果有 `references` 配置且包含 description，追加：

```
Project references provide additional directories that can be accessed when relevant.
<available_references>
  <reference>
    <name>sdk</name>
    <path>/.../repos/owner/repo</path>
    <description>SDK implementation details</description>
  </reference>
</available_references>
```

---

## 3. Instructions（全局 / 本地 / 竞态 / 读取触发）

### 3a. 全局段（globalFiles）

**代码位置：** `packages/opencode/src/session/instruction.ts:60-63, 110-120`

```typescript
globalFiles = [
  path.join(global.config, "AGENTS.md"),        // ~/.opencode/AGENTS.md
  "~/.claude/CLAUDE.md",                        // 受 flag 控制
]
```

行为：**找到第一个存在的就 break**。

| `~/.opencode/AGENTS.md` | `~/.claude/CLAUDE.md` | 结果 |
|---|---|---|
| 存在 | 任意 | 只加载 AGENTS.md，CLAUDE.md 不检查 |
| 不存在 | 存在 | 加载 CLAUDE.md |
| 不存在 | 不存在 | 不加载 |

**控制 flag：** `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT`（或 `OPENCODE_DISABLE_CLAUDE_CODE`）

### 3b. 项目段（instructionFiles）

**代码位置：** `packages/opencode/src/session/instruction.ts:64-68, 122-133`

```typescript
instructionFiles = ["AGENTS.md", "CLAUDE.md", "CONTEXT.md"]
```

使用 `fs.findUp(file, ctx.directory, ctx.worktree)` 从当前工作目录向上搜索到 worktree 根。

**关键行为：收集所有匹配，然后 break。**

```typescript
const matches = yield* fs.findUp(file, ctx.directory, ctx.worktree)
if (matches.length > 0) {
  matches.forEach((item) => paths.add(path.resolve(item)))
  break
}
```

所以如果 `c:\a\b\c\d\AGENTS.md`、`c:\a\b\c\AGENTS.md`、`c:\a\b\AGENTS.md` 都存在，**全部加载**。

**控制 flag：** `Flag.OPENCODE_DISABLE_PROJECT_CONFIG`（控制整个项目段，包括 AGENTS.md/CLAUDE.md/CONTEXT.md + config.instructions 本地路径）

### 3c. AGENTS.md vs CLAUDE.md 竞态关系

两段循环各自独立：

| 阶段 | 文件 | 关系 |
|---|---|---|
| 全局段 | `~/.opencode/AGENTS.md` vs `~/.claude/CLAUDE.md` | 二选一（短路） |
| 项目段 | `AGENTS.md` vs `CLAUDE.md` vs `CONTEXT.md` | AGENTS.md 先匹配则 CLAUDE.md 不检查；但 AGENTS.md 有多个层级时全部加载 |

全局和项目的文件是**叠加**关系，不是二选一。

### 3d. `resolve` — 读取工作目录中子目录的文件

**代码位置：** `packages/opencode/src/session/instruction.ts:179-221`

触发条件：用户调用 `read` 工具读取 worktree 内部的文件（read.ts:300）。

```typescript
const root = path.resolve(yield* InstanceState.directory)
const target = path.resolve(filepath)
let current = path.dirname(target)

while (current.startsWith(root) && current !== root) {
  const found = yield* find(current)
  if (!found || found === target || sys.has(found) || already.has(found)) {
    current = path.dirname(current)
    continue
  }
  // 注入 found 的内容
  current = path.dirname(current)
}
```

行为：从被读文件的目录开始，逐级 `path.dirname` 向上，直到 worktree 根。每层用 `find()` 检查是否存在 AGENTS.md/CLAUDE.md/CONTEXT.md，找到第一个存在的就注入。

**跳过条件（4 个）：**
- 当前目录没有 instruction 文件
- 找到的就是被读文件本身
- 已在 `sys` 中（systemPaths 已加载过）
- 已在 `already` 中（本次消息已加载过）

**防重机制：** `claims: Map<MessageID, Set<string>>` 确保每个 assistant message 只注入一次。

### 3e. `resolve` — 读取非工作目录的文件

**行为：完全不触发。**

条件 `current.startsWith(root)` 在第一轮就不满足，while 循环体一次都不执行，返回空数组。

worktree 外部的文件被视为外部引用，不触发附近指令文件的自动注入。

---

## 4. Config.Instructions

### 4a. 配置位置

`~/.opencode/opencode.jsonc` 或项目级 `.opencode/opencode.jsonc`：

```jsonc
{
  "instructions": [
    "rules.md",
    "~/custom/instructions.md",
    "/absolute/path/to/file.md",
    "rules/**/*.md",
    "https://example.com/rules.md"
  ]
}
```

### 4b. 处理逻辑（instruction.ts:135-150）

每条指令按类型分三路处理：

```typescript
for (const raw of config.instructions) {
  if (raw.startsWith("https://") || raw.startsWith("http://")) continue  // ← 走 fetch（见 4d）
  
  const instruction = raw.startsWith("~/") 
    ? path.join(global.home, raw.slice(2))   // ← ~/ 展开为 home
    : raw                                    // ← 保持原样
  
  const matches = yield* (
    path.isAbsolute(instruction)
      ? fs.glob(path.basename(instruction), {     // ← 绝对路径：用 basename 在当前目录 glob
          cwd: path.dirname(instruction),
          absolute: true,
          include: "file",
        })
      : relative(instruction)                      // ← 相对路径：用 globUp 从当前目录向上搜索
  )
  matches.forEach((item) => paths.add(path.resolve(item)))
}
```

### 4c. 相对路径的搜索方式（globUp）

```typescript
const globUp = (pattern, start, stop?) => {
  let current = start
  while (true) {
    const matches = glob(pattern, { cwd: current, absolute: true, include: "file", dot: true })
    result.push(...matches)
    if (stop === current) break
    current = dirname(current)
  }
  return result
}
```

**行为：** 从 `ctx.directory`（当前工作目录）向上搜索到 `ctx.worktree`（worktree 根），每层都用 glob 模式匹配。

### 4d. 各配置值的实际行为

| 配置值 | 实际行为 | 搜索范围 |
|---|---|---|
| `"rules.md"` | `globUp("rules.md", workdir, worktree)` | 从当前目录向上到 worktree 根，每层找 `rules.md` |
| `"~/custom/rules.md"` | 展开为 `~/custom/rules.md`（绝对路径）→ `glob("rules.md", cwd=~/custom)` | `~/custom` 目录下的 `rules.md` |
| `"/abs/path/file.md"` | `glob("file.md", cwd=/abs/path)` | `/abs/path` 目录下的 `file.md` |
| `"rules/**/*.md"` | `globUp("rules/**/*.md", workdir, worktree)` | 从当前目录向上，每层用 glob 模式匹配 |
| `"https://..."` | HTTP fetch | 不读文件，直接 fetch URL |

### 4e. 核心区别：glob 不是 findUp

`config.instructions` 的相对路径**不是**精确匹配某个文件是否存在，而是用 **glob 模式**搜索。

- `findUp("AGENTS.md", ...)` → 检查每层是否有 `AGENTS.md` 这个**精确文件**
- `globUp("rules.md", ...)` → 每层用 **glob 模式** `rules.md` 搜索，匹配 `rules.md`、`Rules.md`、`rules.md.bak` 等

**但默认 glob 模式是大小写敏感的**（取决于底层 Glob.scan 实现），通常 `rules.md` 只匹配字面意义的 `rules.md`。

### 4f. 支持 glob 通配符

因为底层用的是 `Glob.scan`，支持标准 glob 语法：

| 配置值 | 匹配示例 |
|---|---|
| `"*.md"` | 每层目录下的所有 `.md` 文件 |
| `"rules/*.md"` | 每层目录下的 `rules/` 子目录中的 `.md` 文件 |
| `"rules/**/*.md"` | 每层目录下 `rules/` 子目录及其所有子目录中的 `.md` 文件 |
| `"**/instructions.md"` | 从当前目录到 worktree 根，所有子目录中的 `instructions.md` |

### 4g. 与 AGENTS.md 搜索的区别

| 特性 | AGENTS.md（项目段） | config.instructions |
|---|---|---|
| 搜索方式 | `findUp`（精确文件名） | `globUp`（glob 模式） |
| 搜索到哪 | 找到第一个匹配的文件类型就 break | 所有匹配全部加入 paths |
| 匹配 | 每层只检查 `AGENTS.md` / `CLAUDE.md` / `CONTEXT.md` 中的一个 | 每层用自定义 glob 模式匹配 |
| 去重 | 无（Set 自然去重） | Set 自然去重 |

### 4h. 是否必须在 opencode.jsonc 里定义才会触发？

**是的。** `config.instructions` 未定义时，整个块不执行。不配置就不会触发。

### 4i. 仅支持 .md 文件？

**不是。** 底层用的是 `Glob.scan`，匹配的是 glob 模式，不限制扩展名。

| 配置值 | 匹配结果 |
|---|---|
| `"*.md"` | 只匹配 `.md` |
| `"*.txt"` | 只匹配 `.txt` |
| `"*.json"` | 只匹配 `.json` |
| `"*"` | 匹配所有文件 |
| `"Makefile"` | 精确匹配 `Makefile` |

### 4j. 文件内容有什么格式要求？

**没有。** 内容被原样读取，只加一个前缀：

```typescript
results.push({ filepath: found, content: `Instructions from: ${found}\n${content}` })
```

最终注入 system prompt 的形式就是：

```
Instructions from: c:\a\b\rules.md
<rules.md 的原始内容>
```

内容可以是任意文本：自然语言指令、Markdown、伪代码、甚至代码片段。LLM 看到的就是一段文本指令，格式完全由你决定。

---

## 5. User.System

### 5a. 是否存在于 opencode 本身？

**存在，但是内部的。** 在 `PromptInput` schema 中定义了：

```typescript
// packages/opencode/src/session/prompt.ts:1510
system: Schema.optional(Schema.String),
```

但在**公开 API 中没有暴露**。

### 5b. 公开 API 对比

| 层级 | 字段 |
|---|---|
| 内部 `PromptInput` | `system` ✅ 存在 |
| HTTP API `PromptPayload` | 从 `PromptInput` 去掉 `sessionID`，**但 `system` 不在暴露字段中** |
| OpenAPI/Swagger | `PromptInput` 只有 `text`, `files`, `agents` |
| JS SDK `PromptInput` | `text`, `files?`, `agents?` — **没有 `system`** |

### 5c. 谁在用这个字段？

只被内部代码使用：

| 调用方 | 位置 |
|---|---|
| `acp/service.ts` | ACP (Agent Communication Protocol) 内部 |
| `cli/cmd/run.ts` | CLI `run` 命令内部 |
| `cli/cmd/github.handler.ts` | GitHub PR 处理内部 |
| `tool/task.ts` | task 工具内部 |

这些调用方在传入 prompt 时，**不会传递 `system` 字段**（该字段在 schema 中是 optional，默认 undefined）。

### 5d. 结论

**是的，这是为二次开发预留的扩展点。**

- 字段在 `PromptInput` schema 中存在，但**没有在任何公开 API 中暴露**
- 内部调用方不使用它
- 如果你想让自定义调用方支持 `system` 字段，需要：
  1. 在 `PromptPayload` schema 中添加 `system` 字段
  2. 在 HTTP API handler 中传递该字段
  3. 在 OpenAPI/Swagger 中更新 schema 定义

---

## 6. References

### 6a. 配置位置

**文件：** `~/.opencode/opencode.jsonc` 或项目级 `.opencode/opencode.jsonc`

**字段：** `references`（推荐）或 `reference`（已弃用，兼容）

### 6b. 配置格式

```jsonc
{
  "references": {
    "sdk": "github.com/owner/repo",
    "sdk2": {
      "type": "git",
      "repository": "github.com/owner/repo",
      "branch": "develop",
      "description": "SDK implementation details"
    },
    "internal": {
      "type": "local",
      "path": "/home/user/projects/internal",
      "description": "Internal API reference"
    }
  }
}
```

### 6c. Schema 结构

```
references: Record<string, Entry>

Entry = string (git 简写) | Git | Local

Git = {
  type: "git",
  repository: string,
  branch?: string,              // 默认 "main"
  description?: string,
  hidden?: boolean
}

Local = {
  type: "local",
  path: string,
  description?: string,
  hidden?: boolean
}
```

### 6d. 各类型详解

#### Git 简写（最常用）

```jsonc
"references": {
  "sdk": "github.com/owner/repo"
}
```

等价于：
```jsonc
"sdk": {
  "type": "git",
  "repository": "github.com/owner/repo"
}
```

#### Git 完整配置

```jsonc
"sdk": {
  "type": "git",
  "repository": "github.com/owner/repo",
  "branch": "develop",
  "description": "SDK implementation details"
}
```

#### Local 本地目录

```jsonc
"internal": {
  "type": "local",
  "path": "/home/user/projects/internal",
  "description": "Internal API reference"
}
```

### 6e. 运行时行为

| 类型 | 初始化行为 |
|---|---|
| `local` | 直接使用配置的路径 |
| `git` | 解析 repository → 计算 cache 路径 → `RepositoryCache.ensure` 克隆/更新 → 缓存到 `~/.opencode/repos/` |

### 6f. 注入 system prompt 的格式

仅包含有 `description` 的引用：

```
Project references provide additional directories that can be accessed when relevant.
<available_references>
  <reference>
    <name>sdk</name>
    <path>/home/user/.opencode/repos/github.com/owner/repo</path>
    <description>SDK implementation details</description>
  </reference>
  <reference>
    <name>internal</name>
    <path>/home/user/projects/internal</path>
    <description>Internal API reference</description>
  </reference>
</available_references>
```

### 6g. 边缘状况

| 条件 | 行为 |
|---|---|
| Git clone 失败 | 静默跳过（`catchCause` + `logWarning`） |
| 重复 repository+branch | 跳过 |
| 无 description | 不出现在 system prompt 中 |
| `hidden: true` | 仍出现在 system prompt 中，前端可能不展示 |

### 6h. 本质

**只是指针，不自动读取内容。** AI 需要在需要时主动调用 `read`/`glob`/`grep` 等工具访问这些目录。AI 判断不需要时，这些目录完全不会被触碰。

### 6i. 字符串形式的规则（重要）

**字符串被识别为 local 的条件：** 必须以 `.`、`/` 或 `~` 开头。

| 字符串值 | 识别为 | 示例 |
|---|---|---|
| `github.com/owner/repo` | git | `"sdk": "github.com/owner/repo"` |
| `./relative/path` | local | `"docs": "./docs"` |
| `/abs/path` | local | `"docs": "/home/user/docs"` |
| `~/home/path` | local | `"docs": "~/docs"` |
| `Z:\Windows\path` | **git（会失败）** | ❌ 不能这样用 |

Windows 绝对路径（如 `Z:\...`）不符合 local 条件，必须用完整对象形式。

### 6j. 最简形式的限制（重要）

**字符串简写形式不会被出现在 system prompt 中。**

原因：
1. 字符串简写（如 `"tinyrouter": "https://github.com/..."`）会被正确解析为 Git 引用
2. 但 `description` 会被设为 `undefined`
3. `reference/guidance.ts` 过滤掉 `description === undefined` 的引用

**结论：references 必须用完整形式，必须包含 `description`。**

只有完整形式（带 `description`）才能让引用出现在 system prompt 中，告诉 AI "这个目录存在，你可以在需要时访问"。

最简形式实际上没什么用：
- 你定义了引用 → 目的是让 AI 知道
- 但没有 description → system prompt 不展示
- AI 不知道 → 无法使用

**永远用完整形式，必须包含 `description`。**

---

## 7. Instructions 与 References 的关系

### 7a. 本质相同

两种方式最终都是向 system prompt 注入一段字符串：

| 方式 | 注入格式 |
|---|---|
| `references` | `<available_references>...` XML 块 |
| `instructions` | `Instructions from: {path}\n{内容}` |

**理论上确实没有本质区别。** 如果只是为了让 AI 看到某些信息，两种方式都可以实现。

### 7b. 实际区别

| 特性 | references | instructions |
|---|---|---|
| 格式 | 固定 XML 格式 | 原始文件内容 |
| 自动克隆 | Git 类型自动克隆/更新 | 无 |
| 路径解析 | 自动解析相对路径为绝对路径 | 无 |
| 语义 | AI 识别为"额外目录列表" | AI 不确定用途 |
| 过滤 | `description !== undefined` | 无 |

### 7c. 使用建议

| 场景 | 推荐方式 |
|---|---|
| 给 AI 提供额外指令 | `instructions` |
| 让 AI 知道有哪些额外目录可访问 | `references`（格式更标准，AI 更容易理解） |
| 只需要 AI 看到某些信息 | 两种方式都可以 |

### 7d. 设计意图

- `references` 的设计目的是让 AI 知道有哪些额外目录可访问，并提供标准化的格式
- `instructions` 的设计目的是提供额外的指令文本

虽然功能有重叠，但 `references` 有一些额外的便利功能（自动克隆、路径解析、标准化格式），所以如果目的是让 AI 访问额外目录，还是应该用 `references`。

---

## 附加：Flag 汇总

| Flag | 影响范围 | 生效位置 |
|---|---|---|
| `OPENCODE_DISABLE_CLAUDE_CODE_PROMPT` | 全局 CLAUDE.md + 项目 CLAUDE.md | instruction.ts:62, 66 |
| `OPENCODE_DISABLE_CLAUDE_CODE` | 同上 + CLAUDE.md skills | runtime-flags.ts:24-30 |
| `Flag.OPENCODE_DISABLE_PROJECT_CONFIG` | 项目段全部（AGENTS.md/CLAUDE.md/CONTEXT.md + config.instructions 本地路径） | instruction.ts:81, 123 |

---

## 最终组装顺序（单次请求）

```
最终 system string =
  [1] agent.prompt 或 provider模板
  + [2] env（始终）
  + [3] 全局 instruction 文件
  + [4] 项目 instruction 文件
  + [5] config.instructions（HTTP/本地）
  + [6] MCP instructions（过滤后）
  + [7] skills 列表
  + [8] user.system（如果有）
  + [9] structured output prompt（条件）
  + [10] max steps prompt（条件）
  + [plugin transform 可能修改]
```

每段之间用 `\n` 连接。如果某段为空（如 CLAUDE.md 不存在、无 MCP server），则该段跳过。

---

## 附加：Agent Permission 配置（.md 文件中）

### 在 .md 文件中定义 Agent 的 permission

**格式：**

```yaml
---
permission:
  "*": deny
  grep: allow
  glob: allow
  bash: allow
  read: allow
---
你的 agent prompt 内容
```

### 为什么 `"*": deny` 能生效

1. YAML 解析后得到 `{ permission: { "*": "deny" } }`
2. `Permission.fromConfig` 将 `"deny"` 字符串值转换为规则 `{ permission: "*", action: "deny", pattern: "*" }`
3. `Permission.evaluate` 使用 `findLast` 查找匹配规则——最后一条匹配的规则生效，所以 `"deny"` 会覆盖默认配置中的 `"allow"`

### 测试用例验证（packages/opencode/test/config/config.test.ts:769-785）

```yaml
permission:
  bash: allow
  "*": deny
  edit: ask
```

测试期望：
```typescript
expect(Object.keys(config.agent?.ordered?.permission ?? {})).toEqual(["bash", "*", "edit"])
```

---

## 附加：Resolve 遍历示例

### 示例：读取 `c:\a\b\c\d\e`（worktree 根 `c:\a\b`）

| 循环 | `current` | `find(current)` | 注入？ |
|---|---|---|---|
| 1 | `c:\a\b\c\d` | 找到 `c:\a\b\c\d\AGENTS.md` | ✅ |
| 2 | `c:\a\b\c` | 找到 `c:\a\b\c\AGENTS.md` | ✅ |
| 3 | `c:\a\b` | 找到 `c:\a\b\AGENTS.md` | ✅（但会被 `sys` 跳过） |
| 4 | `c:\a` | 不在 root 内 | 循环结束 |

**注入两个：** `c:\a\b\c\d\AGENTS.md` 和 `c:\a\b\c\AGENTS.md`。

`c:\a\b\AGENTS.md` 虽然 `find` 找到了，但它在 `sys` 里（项目级已加载），被跳过。

---

## 附加：References 配置示例

### 正确配置本地路径

```jsonc
{
  "references": {
    "opencode": {
      "type": "local",
      "path": "Z:\\Playground\\MyReferenceRepository\\opencode",
      "description": "opencode codebase"
    }
  }
}
```

**注意事项：**

1. **Windows 路径分隔符：** JSON/JSONC 中 `\` 需要转义，写成 `\\`，或者用正斜杠 `/`：
   ```jsonc
   "path": "Z:/Playground/MyReferenceRepository/opencode"
   ```

2. **description 必须有：** 没有 description 的引用不会出现在 system prompt 中。

3. **路径有效性：** 启动时会用 `fs.existsSafe` 检查，不存在的路径不会报错但也不会生效。

4. **注入形式：** 启动后 system prompt 中会出现：
   ```
   <available_references>
     <reference>
       <name>opencode</name>
       <path>Z:\Playground\MyReferenceRepository\opencode</path>
       <description>opencode codebase</description>
     </reference>
   </available_references>
   ```
