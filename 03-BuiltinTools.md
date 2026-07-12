# 03 - 内置工具 (Built-in Tools) 详细分析

本文档逐工具拆解 opencode 的 16 个内置工具：每个工具的作用、实现方式（含 file:line）、编码进请求体 `tools` 字段的内容（description + inputSchema 参数）、以及模型如何调用 / 返回什么结构。

> 调研基线：`Z:\Playground\MyReferenceRepository\opencode`（packages/opencode/src），工具源码位于 `packages/opencode/src/tool/`。

---

## 0. 总览与统一机制

### 0.1 工具注册与序列化

所有内置工具在 `tool/registry.ts:204-244` 经 `Tool.init(...)` 初始化并放入 `builtin` 数组，最终由 `ToolRegistry.tools`（`registry.ts:286-335`）产出 `AITool` 列表。每个工具被序列化为请求体 `tools[]` 时输出统一结构（`registry.ts:305-334`）：

```ts
{
  id,            // 模型可见的工具名（见下表）
  description,   // 来自 .txt 或硬编码字符串
  parameters,    // Effect Schema 定义
  jsonSchema,    // 由 schema 导出的 JSON Schema，送入 model
  execute,       // 执行函数
  formatValidationError,
}
```

`plugin.trigger("tool.definition", { toolID }, output)` 允许插件改写 `description` / `jsonSchema`。

### 0.2 统一 execute 契约与截断

所有工具的 `execute` 返回统一结构（`tool.ts:48-53`）：

```ts
{ title: string, metadata: M, output: string, attachments?: FilePart[] }
```

返回前经 `tool.ts` 的 `wrap`（`tool.ts:99-149`）处理：

1. `Schema.decodeUnknownEffect(parameters)` 校验参数，失败抛 `InvalidArgumentsError`；
2. 若 `result.metadata.truncated === undefined`，调用 `truncate.output()`（默认 `MAX_LINES=2000` / `MAX_BYTES=50*1024`，可由配置 `tool_output` 覆盖，`truncate.ts:15,75-83`）做全局截断。

> **特殊**：`shell` 与 `read` 在 `metadata` 中**显式设置 `truncated`**，故跳过二次截断，自行用 `Truncate.write()` 落盘并返回 `outputPath`（`truncate.ts:68`）。

### 0.3 权限 key 与项目外访问

| 工具 | 权限 key | 备注 |
|------|----------|------|
| shell | `bash`（命令扫描另含 `external_directory`） | id 是 `bash` |
| read | `read` | 项目外触发 `external_directory` |
| edit / write / apply_patch | `edit` | 三者共用一个 key |
| glob | `glob` | |
| grep | `grep` | |
| webfetch | `webfetch` | |
| websearch | `websearch` | gated |
| skill | `skill` | |
| task | `task` | |
| todo | `todowrite` | 注意 id 是 `todowrite` |
| question | `question` | |
| lsp | `lsp` | |
| plan_exit | `plan_enter`/`plan_exit` | 权限动作 |
| invalid | — | 占位 |
| execute | — | code mode |

所有文件工具另经 `assertExternalDirectoryEffect(ctx, filepath, { kind })` 在项目外目录触发 `external_directory` ask（`external-directory.ts`）。

### 0.4 gating 一览

| 工具 | 门控条件 | 代码位置 |
|------|----------|----------|
| `invalid` | 始终存在 | `registry.ts:227` |
| `question` | `client ∈ {app,cli,desktop}` 或 `enableQuestionTool` | `registry.ts:202,228` |
| `lsp` | `experimentalLspTool` | `registry.ts:242` |
| `plan_exit` | `experimentalPlanMode && client === "cli"` | `registry.ts:243` |
| `execute` | `experimentalCodeMode`（且 `codeModeDescription` 非空才纳入 `tools()`） | `registry.ts:113-114,221,241,300-303` |
| `websearch` | `opencode` provider 或 `exa`/`parallel` flag | `webSearchEnabled` `registry.ts:58-60` |
| `apply_patch` | 仅 `gpt-`（非 oss、非 gpt-4） | `registry.ts:292-295` |

### 0.5 关于 `gitSafe` 与 "先 Read 再编辑"

- 源码 `src/tool/` 中**不存在 `gitSafe` 符号**（已 grep 确认）。权限模型是 `ctx.ask({ permission, patterns, always })` + `external_directory` 检查，无独立 `gitSafe` 机制。
- `read`/`edit`/`write` 的 `.txt` 提示文本要求"先 Read 再编辑/写入"，但**源码未做代码级 gate**——`edit.ts`/`write.ts` 实际只校验 `oldString`/`exists`，没有"本会话已读该文件"的强制校验。该约束仅存在于 prompt 文本。

---

## 1. shell（模型 id：`bash`）

**文件**：`tool/shell.ts`（`shell/shell.txt`、`shell/id.ts`、`shell/prompt.ts`）

### 1.1 作用
执行 shell 命令的持久命令会话工具（bash / PowerShell / cmd / pwsh），支持超时、工作目录、输出流式捕获、超长输出落盘截断，并基于 tree-sitter 解析命令做路径与权限扫描。

### 1.2 实现方式
- `Tool.define(ShellID.ToolID /* "bash" */, …)`（`shell.ts:338`）。`ShellID.ToolID = "bash"`（`shell/id.ts:16`，保留别名以兼容 plugin/permission）。
- 实际执行用 Effect 的 `ChildProcess`（`effect/unstable/process`）：`cmd()`（`shell.ts:293`）构造进程——Windows 上 PowerShell 用 `ChildProcess.make(shell, ["-NoLogo","-NoProfile","-NonInteractive","-Command",command], …)`，否则 `ChildProcess.make(command, [], { shell, … })`。
- 解析用 **web-tree-sitter** 懒加载 WASM（`shell.ts:311`）：`tree-sitter-bash.wasm` + `tree-sitter-powershell.wasm`。
- 权限/路径扫描：`parse()` → `collect()`（`shell.ts:378`）遍历 `command` 节点，对 `FILES`/`CMD_FILES` 集合内的命令（`rm/cp/mv/mkdir/touch/chmod/cat…`、`dir/del/erase/move/ren…`）提取路径参数 `argPath()` → `resolvePath()`（`shell.ts:358`，含 Windows `cygpath` 转换）。对每个项目外目录触发 `external_directory` ask，对每个命令模式触发 `ctx.ask({ permission: "bash", … })`（`shell.ts:263`）。
- 输出处理（`run`，`shell.ts:428`）：以 `limits.maxBytes*2` 为滚动缓冲，超阈值调用 `Truncate.write()` 落盘（`shell.ts:505`）；监听 `exitCode` / `abort` / `timeout`（默认 `defaultTimeoutMs = flags.bashDefaultTimeoutMs ?? 2*60*1000`，`shell.ts:347`）竞速（`shell.ts:542`），超时/中止时 `handle.kill`。
- 描述与参数由 `ShellPrompt.render(name, platform, limits, timeout)`（`prompt.ts:273`）用 `shell.txt` 模板生成。

### 1.3 编码进请求体的内容
- `description`：`shell.txt` 文本经占位符替换后的完整字符串（首行 `Executes a given bash command in a persistent shell session with optional timeout…`，含 `Be aware: OS: …`、临时目录用法、`DO NOT use it for file operations`、Git/GitHub 段等）。
- `inputSchema`（`prompt.ts:15 parameterSchema()`）：
  - `command`: `String` — "The command to execute"
  - `timeout`: `optional(PositiveInt)` — "Optional timeout in milliseconds"
  - `workdir`: `optional(String)` — "The working directory to run the command in. Defaults to the current directory. Use this instead of 'cd' commands."

### 1.4 模型调用 / 返回
- 调用参数：`{ command: string, timeout?: number, workdir?: string }`。
- 返回（`shell.ts:585`）：
  ```ts
  {
    title: input.command,
    metadata: {
      output: last || preview(output),
      exit: code,            // number | null
      truncated: cut,        // boolean（显式设置 → 跳过 wrap 二次截断）
      ...(cut && file ? { outputPath: file } : {}),
    },
    output,                  // 尾部内容，可能含 "...output truncated... Full output saved to: <file>" 与 "<shell_metadata>"
  }
  ```

---

## 2. read

**文件**：`tool/read.ts`（`read.txt`）

### 2.1 作用
从本地文件系统读取**文件或目录**（含图片/PDF 作为 attachment）。支持 `offset`/`limit` 逐段读取、超长行截断、镜像目录列表，缺失路径给出"did you mean"提示。

### 2.2 实现方式
- `Tool.define("read", …)`（`read.ts:64`）。依赖 `FSUtil / Instruction / LSP / Scope`。
- 相对路径用 `path.resolve(instance.directory, …)`；Windows 下 `FSUtil.normalizePath`（`read.ts:235-240`）。
- 权限：① `assertExternalDirectoryEffect(ctx, filepath, { kind })`（项目外 ask，项目内跳过）；② `ctx.ask({ permission:"read", patterns:[relative], always:["*"], … })`（`read.ts:255`）。
- 目录分支：`list()`（`read.ts:101`）调 `fs.readDirectoryEntries`，目录名加 `/`，按 `offset/limit`（默认 `DEFAULT_READ_LIMIT=2000`，`read.ts:13`）切片。
- 文件分支：`readSample(SAMPLE_BYTES=4096)` 嗅探 MIME；图片（`image/jpeg|png|gif|webp`）或 PDF → 读全量并 base64 作为 `attachments`（`read.ts:306-325`）。
- 二进制判定 `isBinaryFile()`（`read.ts:182`）：扩展名黑名单 + NUL/不可打印字符 >30%。
- 文本读取 `lines()`（`read.ts:137`）：手搓 `TextDecoder`+流式分行，行超 `MAX_LINE_LENGTH=2000` 截断加后缀，累计字节超 `MAX_BYTES=50*1024` 截断并用 `ReadStop` 中止流（`read.ts:170-173`）。
- 输出形如 `<path>…</path>\n<type>file</type>\n<content>\n<行号>: <内容>…`（`read.ts:338-351`）。

### 2.3 编码进请求体的内容
- `description`：`read.txt` 原文（含 "Read a file or directory…"、"By default, this tool returns up to 2000 lines…"、"Any line longer than 2000 characters is truncated."、"This tool can read image files and PDFs and return them as file attachments."）。
- `inputSchema`（`read.ts:28`）：
  - `filePath`: `String` — "The absolute path to the file or directory to read"
  - `offset`: `optional(NonNegativeInt)` — "The line number to start reading from (1-indexed)"
  - `limit`: `optional(NonNegativeInt)` — "The maximum number of lines to read (defaults to 2000)"

### 2.4 模型调用 / 返回
- 调用参数：`{ filePath: string, offset?: number, limit?: number }`。
- 返回三种形态：
  - **目录**（`read.ts:272`）：`{ title, output: "<path>…</path>\n<type>directory</type>\n<entries>…", metadata:{ preview, truncated, loaded:[], display:{type:"directory",…} } }`
  - **图片/PDF**（`read.ts:309`）：`{ output:"Image read successfully"|"PDF read successfully", metadata:{ truncated:false, loaded }, attachments:[{ type:"file", mime, url:"data:<mime>;base64,..." }] }`
  - **文本文件**（`read.ts:359`）：`{ output, metadata:{ preview, truncated, loaded, display:{type:"file",path,text,lineStart,lineEnd,totalLines,truncated} } }`
- 缺失文件时 `miss()` 返回含相似文件名（"Did you mean one of these?"）的错误（`read.ts:76-99`）。

---

## 3. edit

**文件**：`tool/edit.ts`（`edit.txt`）

### 3.1 作用
对已有文件做**精确字符串替换**（Cline/Gemini-CLI 风格的容错 diff-apply）。支持多种"近似匹配"回退策略，单一/全部替换，返回 diff 与 LSP 诊断。

### 3.2 实现方式
- `Tool.define("edit", …)`（`edit.ts:58`）。依赖 `LSP / FSUtil / Format / EventV2Bridge`。
- 校验：`oldString === newString` → 抛错（`edit.ts:75`）；`oldString===""` 对已存在文件抛错（`edit.ts:90-96`），仅允许对不存在文件用空 oldString 创建。
- 路径：`assertExternalDirectoryEffect(ctx, filePath)`（`edit.ts:83`）；`Bom.readFile` 保留 BOM；行尾 `detectLineEnding`/`convertToLineEnding` 统一（`edit.ts:22-33`）。
- 并发锁：`lock(filePath)` 基于 `FSUtil.resolve` 的 `Semaphore`（`edit.ts:35-45`）。
- 核心 `replace(content, oldString, newString, replaceAll)`（`edit.ts:682`）：依次尝试 9 个 `Replacer`（`SimpleReplacer`、`LineTrimmedReplacer`、`BlockAnchorReplacer`、`WhitespaceNormalizedReplacer`、`IndentationFlexibleReplacer`、`EscapeNormalizedReplacer`、`TrimmedBoundaryReplacer`、`ContextAwareReplacer`、`MultiOccurrenceReplacer`），含 `isDisproportionateMatch` 防匹配跨度过大；多匹配且无 `replaceAll` 抛 "Found multiple matches…"。
- 写盘：`afs.writeWithDirs` → `format.file` → `Bom.syncFile`；发布 `FileSystem.Event.Edited` 与 `Watcher.Event.Updated`。
- 权限：`ctx.ask({ permission:"edit", patterns:[relative], always:["*"], metadata:{filepath, diff} })`（`edit.ts:145`）。
- 诊断：`diffLines` 统计 additions/deletions；`lsp.touchFile` + `lsp.diagnostics()` 报告 LSP 错误（`edit.ts:175-201`）。
- `metadata` 不设 `truncated` → 经 `wrap` 二次截断。

### 3.3 编码进请求体的内容
- `description`：`edit.txt` 原文（含 "Performs exact string replacements in files."、"You must use your `Read` tool at least once…"、"The edit will FAIL if `oldString` is not found…"、"Use `replaceAll` for replacing and renaming strings…"）。
- `inputSchema`（`edit.ts:47`）：
  - `filePath`: `String` — "The absolute path to the file to modify"
  - `oldString`: `String` — "The text to replace"
  - `newString`: `String` — "The text to replace it with (must be different from oldString)"
  - `replaceAll`: `optional(Boolean)` — "Replace all occurrences of oldString (default false)"

### 3.4 模型调用 / 返回
- 调用参数：`{ filePath: string, oldString: string, newString: string, replaceAll?: boolean }`。
- 返回（`edit.ts:203`）：
  ```ts
  {
    metadata: { diagnostics, diff, filediff },
    title: path.relative(instance.worktree, filePath),
    output: "Edit applied successfully." + (LSP 错误时追加 "\n\nLSP errors detected in this file, please fix:\n<block>")
  }
  ```

---

## 4. write

**文件**：`tool/write.ts`（`write.txt`）

### 4.1 作用
将**整文件内容**写入（覆盖或新建）本地文件，返回 diff 与（本文件 + 项目内其他文件的）LSP 诊断。

### 4.2 实现方式
- `Tool.define("write", …)`（`write.ts:27`）。`trimDiff` 复用自 `edit.ts`。
- 路径：`assertExternalDirectoryEffect(ctx, filepath)`（`write.ts:44`）；已存在则 `Bom.readFile` 取旧内容做 `createTwoFilesPatch`。
- 权限：`ctx.ask({ permission:"edit", patterns:[relative], always:["*"], metadata:{filepath, diff} })`（`write.ts:54`）——**复用 `edit` 权限 key**。
- 写盘：`fs.writeWithDirs` → `format.file` → `Bom.syncFile`；发布 `FileSystem.Event.Edited` 与 `Watcher.Event.Updated`（存在=`change`，新建=`add`，`write.ts:68-72`）。
- 诊断：`lsp.diagnostics()` 遍历所有文件，`MAX_PROJECT_DIAGNOSTICS_FILES=5` 上限报告其他文件的 LSP 错误（`write.ts:78-90`）。`metadata` 不设 `truncated` → 经 `wrap` 全局截断。

### 4.3 编码进请求体的内容
- `description`：`write.txt` 原文（"Writes a file to the local filesystem."、"This tool will overwrite the existing file if there is one…"、"If this is an existing file, you MUST use the Read tool first…"、"ALWAYS prefer editing existing files…"、"NEVER proactively create documentation files (*.md) or README files…"）。
- `inputSchema`（`write.ts:20`）：
  - `content`: `String` — "The content to write to the file"
  - `filePath`: `String` — "The absolute path to the file to write (must be absolute, not relative)"

### 4.4 模型调用 / 返回
- 调用参数：`{ content: string, filePath: string }`。
- 返回（`write.ts:92`）：
  ```ts
  {
    title: path.relative(instance.worktree, filepath),
    metadata: { diagnostics, filepath, exists },
    output: "Wrote file successfully." + (本文件 LSP 错误) + (其他文件 LSP 错误)
  }
  ```

---

## 5. apply_patch（模型 id：`apply_patch`）

**文件**：`tool/apply_patch.ts`（`apply_patch.txt`）

### 5.1 作用
以 opencode 自定义 patch 语言（`*** Begin Patch` / `*** Add/Update/Delete File` / `*** Move to` / `@@` + `-`/`+` 行）一次性对**多文件**做增/删/改/重命名。是 `edit`/`write` 的"批处理"替代工具。

### 5.2 实现方式
- `Tool.define("apply_patch", …)`（`apply_patch.ts:22`）。依赖 `LSP / FSUtil / Format / EventV2Bridge`。
- 解析：`Patch.parsePatch(params.patchText)` → `hunks`（`apply_patch.ts:41`）；空 patch 或 0 hunk 直接抛错。
- 逐 hunk 处理（`apply_patch.ts:72-191`）：`add` / `update` / `delete` / `move`（`hunk.move_path`）。
- 权限：**单一 `ctx.ask({ permission:"edit", patterns: relativePaths, always:["*"], metadata:{filepath, diff: totalDiff, files} })`**（对所有文件合并一次，`apply_patch.ts:206`）——同样复用 `edit` 权限 key。
- 写盘（`apply_patch.ts:220-258`）：按类型 `writeWithDirs` + 发布 `FileSystem.Event.Edited`；`move` 写新路径 + 删旧路径。
- 输出汇总：`A/D/M <相对路径>` 列表 + 各文件 LSP 错误。`metadata` 不设 `truncated` → 经 `wrap` 全局截断。

### 5.3 编码进请求体的内容
- `description`：`apply_patch.txt` 原文（33 行），解释 patch 信封格式：`*** Begin Patch` / 文件段 / `*** End Patch`，三类 header（`*** Add File:` / `*** Delete File:` / `*** Update File:`，及 `*** Move to:`），强调 "You must prefix new lines with `+` even when creating a new file"。含示例 patch。
- `inputSchema`（`apply_patch.ts:18`）：
  - `patchText`: `String` — "The full patch text that describes all changes to be made"

### 5.4 模型调用 / 返回
- 调用参数：`{ patchText: string }`（完整 `*** Begin Patch … *** End Patch` 文本）。
- 返回（`apply_patch.ts:295`）：
  ```ts
  {
    title: output,   // "Success. Updated the following files:\nA …\nM …"
    metadata: { diff: totalDiff, files, diagnostics },
    output,          // 同 title；含各文件 "LSP errors detected in <rel>, please fix:\n<block>"
  }
  ```
  `files` 元素：`{ filePath, relativePath, type:"add"|"update"|"delete"|"move", patch, additions, deletions, movePath? }`。

### 5.5 与 edit/write 的互斥门控（关键）
`registry.ts:292-295`：
```ts
const usePatch =
  input.modelID.includes("gpt-") && !input.modelID.includes("oss") && !input.modelID.includes("gpt-4")
if (tool.id === ApplyPatchTool.id) return usePatch
if (tool.id === EditTool.id || tool.id === WriteTool.id) return !usePatch
```
即：**仅当模型 id 含 `gpt-`（且非 `oss`、非 `gpt-4`）时**，暴露 `apply_patch` 并**隐藏 `edit`/`write`**；其它所有模型则暴露 `edit`/`write` 并**隐藏 `apply_patch`**。三者权限 key 都是 `edit`，历史已授权限兼容。

---

## 6. glob

**文件**：`tool/glob.ts`（`glob.txt`）

### 6.1 作用
基于 **ripgrep** 的快速文件名 glob 匹配（支持 `**/*.js`、`src/**/*.ts`），返回匹配的文件绝对路径，上限 100 条。

### 6.2 实现方式
- `Tool.define("glob", …)`（`glob.ts:17`）。依赖 `FSUtil / Ripgrep`。
- 权限：`ctx.ask({ permission:"glob", patterns:[params.pattern], always:["*"], … })`（`glob.ts:28`）。
- 路径：`params.path ?? ins.directory`；相对路径 `path.resolve(ins.directory, …)`；stat 校验必须是目录，否则抛 "glob path must be a directory"（`glob.ts:38-43`）；`assertExternalDirectoryEffect(ctx, search, {kind:"directory"})`（`glob.ts:44`）。
- 执行：`ripgrep.glob({ cwd: search, pattern: params.pattern, limit: 100 })`（`glob.ts:50`），`truncated = files.length === limit`。

### 6.3 编码进请求体的内容
- `description`：`glob.txt` 原文（"- Fast file pattern matching tool…"、"- Supports glob patterns like "**/*.js"…"、"- Returns matching file paths"、"- When you are doing an open-ended search… use the Task tool instead"）。
- `inputSchema`（`glob.ts:10`）：
  - `pattern`: `String` — "The glob pattern to match files against"
  - `path`: `optional(String)` — "The directory to search in. If not specified, the current working directory will be used…"

### 6.4 模型调用 / 返回
- 调用参数：`{ pattern: string, path?: string }`。
- 返回（`glob.ts:65`）：
  ```ts
  {
    title: path.relative(ins.worktree, search),
    metadata: { count: files.length, truncated },
    output: files.map(f => path.resolve(search, f.path)).join("\n")
             + (truncated ? "\n\n(Results are truncated: showing first 100 results…)" : "")
             // 无结果时 "No files found"
  }
  ```

---

## 7. grep

**文件**：`tool/grep.ts`（`grep.txt`）

### 7.1 作用
基于 **ripgrep** 的全仓库正则内容搜索，返回匹配的文件路径、行号与匹配行；上限 100 条。

### 7.2 实现方式
- `Tool.define("grep", …)`（`grep.ts:20`）。依赖 `FSUtil / Ripgrep`。
- 校验：`!params.pattern` → 抛 "pattern is required"（`grep.ts:35`）；`ctx.ask({ permission:"grep", patterns:[params.pattern], always:["*"], … })`（`grep.ts:39`）。
- 路径：解析 `requested`；`assertExternalDirectoryEffect(ctx, requested, { kind })`（`grep.ts:55`）；`cwd = 目录则用该目录，否则取其父目录`（`grep.ts:62`）。
- 执行：`ripgrep.grep({ cwd, pattern: params.pattern, include: params.include, limit: 100 })`（`grep.ts:63`）；`hasMore = truncated || result.length === limit`。
- 输出按文件路径分组（`<path>:` 后跟 `  Line <n>: <text>`），`truncated` 时追加提示。

### 7.3 编码进请求体的内容
- `description`：`grep.txt` 原文（"- Fast content search tool…"、"- Searches file contents using regular expressions"、"- Supports full regex syntax…"、"- If you need to identify/count the number of matches… use the Bash tool with `rg`"、"- When you are doing an open-ended search… use the Task tool instead"）。
- `inputSchema`（`grep.ts:10`）：
  - `pattern`: `String` — "The regex pattern to search for in file contents"
  - `path`: `optional(String)` — "The directory to search in. Defaults to the current directory."
  - `include`: `optional(String)` — 'File pattern to include in the search (e.g. "*.js", "*.{ts,tsx}")'

### 7.4 模型调用 / 返回
- 调用参数：`{ pattern: string, path?: string, include?: string }`。
- 返回（`grep.ts:101`）：
  ```ts
  {
    title: params.pattern,
    metadata: { matches: total, truncated },
    output: `Found ${total} matches${hasMore ? " (more matches available)" : ""}`
             + 分组行 ("<path>:\n  Line <n>: <text>…")
             + (truncated ? "\n\n(Results truncated. Consider using a more specific path or pattern.)" : "")
    // 无匹配：{ title: params.pattern, metadata:{matches:0,truncated:false}, output:"No files found" }
  }
  ```

---

## 8. webfetch（模型 id：`webfetch`）

**文件**：`tool/webfetch.ts`（`webfetch.txt`）

### 8.1 作用
从给定 URL 抓取 Web/HTTP 内容，按请求格式（`markdown`/`text`/`html`，默认 markdown）返回。直接 HTTP GET（Effect HTTP client），剥离 HTML 样板，可选将图片作为 base64 attachment 返回。不调用 LLM 或 MCP——纯原始 HTTP fetch。

### 8.2 实现方式
- `Tool.define("webfetch", …)`（`webfetch.ts:24`）。`HttpClient.HttpClient` + `HttpClient.filterStatusOk`（非 2xx 抛错）。
- 参数校验：拒绝非 `http://`/`https://` 的 URL（`webfetch.ts:35`）；HTTP 自动升级 HTTPS。
- `Accept` 头按格式加权（`webfetch.ts:53-68`），Chrome UA（`webfetch.ts:70-74`）。
- **Cloudflare 重试**：`403` + `cf-mitigated: challenge` 时用 UA `"opencode"` 重试（`webfetch.ts:79-92`）。
- **限制**：`MAX_RESPONSE_SIZE = 5MB`、`DEFAULT_TIMEOUT = 30s`、`MAX_TIMEOUT = 120s`（`webfetch.ts:9-11`）。
- HTML→Markdown 用 `turndown`；HTML→纯文本用 `htmlparser2` 跳过 `script/style/noscript/iframe`（提取函数 `webfetch.ts:158-192`）。
- 图片经 `isImageAttachment(mime)` → base64 `data:` attachment（`webfetch.ts:110-124`）。
- 权限：`ctx.ask({ permission: "webfetch", patterns: [params.url], always: ["*"], …})`（`webfetch.ts:39-48`）。

### 8.3 编码进请求体的内容
- `description`（`webfetch.txt` 原文）：
  ```
  - Fetches content from a specified URL
  - Takes a URL and optional format as input
  - Fetches the URL content, converts to requested format (markdown by default)
  - Returns the content in the specified format
  - Use this tool when you need to retrieve and analyze web content
  Usage notes:
    - IMPORTANT: if another tool is present that offers better web fetching capabilities, prefer using that tool instead of this one.
    - The URL must be a fully-formed valid URL
    - HTTP URLs will be automatically upgraded to HTTPS
    - Format options: "markdown" (default), "text", or "html"
    - This tool is read-only and does not modify any files
    - Results may be summarized if the content is very large
  ```
- `inputSchema`（`webfetch.ts:13-22`）：
  - `url`: `String` — "The URL to fetch content from"
  - `format`: `"text" | "markdown" | "html"`（默认 `"markdown"`） — "The format to return the content in…"
  - `timeout`: `optional(Number)` — "Optional timeout in seconds (max 120)"

### 8.4 模型调用 / 返回
- 调用参数：`{ url, format?, timeout? }`。
- 返回（`webfetch.ts:112-152`）：`title = "${params.url} (${contentType})"`，按内容类型返回：
  - 图片：`{ output: "Image fetched successfully", metadata: {}, attachments:[{ type:"file", mime, url:"data:<mime>;base64,..." }] }`
  - markdown：`{ output: <markdown>, metadata: {} }`
  - text/html：对应提取文本或原始内容。

---

## 9. websearch（模型 id：`websearch`）

**文件**：`tool/websearch.ts`（`websearch.txt` + `mcp-websearch.ts`）

### 9.1 作用
通过外部 MCP 搜索后端（Exa 或 Parallel）执行实时网络搜索，可选抓取 URL 内容。Provider 按会话选择，由运行时 flag/环境变量配置。**gated**——仅当 provider 为 `opencode` 或设置 exa/parallel flag 时暴露。

### 9.2 实现方式
- `Tool.define("websearch", …)`（`websearch.ts:99`）。依赖 `HttpClient / RuntimeFlags`。
- **Provider 选择** `selectWebSearchProvider`（`websearch.ts:30-37`）：`OPENCODE_WEBSEARCH_PROVIDER` 环境变量 override → `flags.parallel` → `flags.exa` → 否则按 `checksum(sessionID) % 2` 确定性选择。
- **`callProvider`**（`websearch.ts:60-97`）：`parallel` → `McpWebSearch.PARALLEL_URL`（`https://search.parallel.ai/mcp`），tool `web_search`；`exa` → `McpWebSearch.EXA_URL`（`https://mcp.exa.ai/mcp`），tool `web_search_exa`。
- `mcp-websearch.ts`：构造 JSON-RPC 2.0 `tools/call` POST，解析 SSE `data:` 行或直接 JSON 取 `result.content[].text`。Parallel 加 `Authorization: Bearer` 头。
- 权限：`ctx.ask({ permission: "websearch", patterns: [params.query], always: ["*"], …})`（`websearch.ts:119-131`）。

### 9.3 编码进请求体的内容
- `description`（`websearch.txt`，运行时把 `{{year}}` 替换为当前年，`websearch.ts:106-108`）：
  ```
  - Search the web using the session's web search provider…
  - Provides up-to-date information for current events and recent data
  - Supports configurable result counts…
  - Use this tool for accessing information beyond knowledge cutoff
  The current year is {{year}}. You MUST use this year when searching for recent information or current events
  ```
- `inputSchema`（`websearch.ts:10-25`）：
  - `query`: `String` — "Websearch query"
  - `numResults`: `optional(Number)` — "Number of search results to return (default: 8)"
  - `livecrawl`: `optional("fallback" | "preferred")` — "'fallback': use live crawling as backup… 'preferred': prioritize live crawling (default: 'fallback')"
  - `type`: `optional("auto" | "fast" | "deep")` — "'auto': balanced search (default)…"
  - `contextMaxCharacters`: `optional(Number)` — "Maximum characters for context string optimized for LLMs (default: 10000)"

### 9.4 模型调用 / 返回
- 调用参数：`{ query, numResults?, livecrawl?, type?, contextMaxCharacters? }`。
- 返回（`websearch.ts:135-139`）：
  ```ts
  {
    output: <search text from provider> ?? "No search results found. Please try a different query.",
    title: `${label}: ${params.query}`,   // label: "Parallel Web Search"/"Exa Web Search"/"Web Search"
    metadata: { provider },
  }
  ```

### 9.5 gating 确认
`webSearchEnabled(providerID, flags)`（`registry.ts:58-60`）：
```ts
return providerID === ProviderV2.ID.opencode || flags.exa || flags.parallel
```
仅在 `opencode` provider 或 exa/parallel flag 开启时纳入 `tools()`。

---

## 10. skill（模型 id：`skill`）

**文件**：`tool/skill.ts`（`skill.txt`）

### 10.1 作用
**loader 工具**。当活跃任务匹配 system prompt 中列出的 skill 时，模型以 skill 名调用 `skill`，工具加载该 skill 的 `SKILL.md` 内容 + 其目录的采样文件列表，注入对话上下文。它自身不做研究——只"浮出" skill 指令。

### 10.2 实现方式
- `Tool.define("skill", …)`（`skill.ts:12`）。依赖 `Skill.Service / Ripgrep.Service`。
- `skill.require(params.name)` 按名解析 skill；`Skill.NotFoundError` → `Effect.die`（`skill.ts:23-25`）。
- 权限：`ctx.ask({ permission: "skill", patterns: [params.name], always: [params.name], …})`（`skill.ts:27-32`）。
- 用 **Ripgrep**（`ripgrep.find`）在 skill 目录内 pattern `!**/SKILL.md`（`hidden: true, limit: 10`）采样同级文件（`skill.ts:34-43`）。
- 返回 raw `info.content`（SKILL.md 正文）+ 基目录 + 文件列表。

### 10.3 编码进请求体的内容
- `description`（`skill.txt` 原文）：
  ```
  Load a specialized skill when the task at hand matches one of the skills listed in the system prompt.
  Use this tool to inject the skill's instructions and resources into current conversation…
  The skill name must match one of the skills listed in your system prompt.
  ```
- `inputSchema`（`skill.ts:8-10`）：
  - `name`: `String` — "The name of the skill from available_skills"

### 10.4 模型调用 / 返回
- 调用参数：`{ name }`（须匹配 system prompt 中 `available_skills` 的某个 skill）。
- 返回（`skill.ts:45-66`）：单个 `output` 字符串包裹在 `<skill_content name="...">` 中，含：
  ```
  # Skill: <name>
  <SKILL.md content>
  Base directory for this skill: <dir>
  Relative paths in this skill (e.g., scripts/, reference/) are relative to this base directory.
  <skill_files><file><absolute path></file>…</skill_files>
  ```
  `metadata: { name, dir }`，`title: "Loaded skill: <name>"`。无 `attachments`。

---

## 11. task（模型 id：`task`，子代理）

**文件**：`tool/task.ts`（`task.txt`）

### 11.1 作用
启动一个**子代理 (subagent)**（独立会话/agent）自主处理复杂多步任务。支持前台（阻塞至完成）与后台（`background=true`，立即返回，完成时通知）。子会话从父会话派生，带作用域权限/工具拒绝。

### 11.2 实现方式
- `Tool.define("task", …)`（`task.ts:81`）。依赖 `Agent / BackgroundJob / Config / Session / Scope / RuntimeFlags / Database`。
- **后台门控**：`background=true` 需 `flags.experimentalBackgroundSubagents`，否则失败（`task.ts:97-102`）。
- **权限**：除非 `bypassAgentCheck`，`ctx.ask({ permission: "task", patterns: [params.subagent_type], always: ["*"], …})`（`task.ts:104-114`）。
- **子代理查找**：`agent.get(params.subagent_type)`；未知类型失败（`task.ts:116-119`）。
- **会话创建（子会话 spawn）**：有 `task_id` 则恢复，否则 `sessions.create({ parentID, title, agent, permission })`（`task.ts:142-158`）。子权限经 `deriveSubagentSessionPermission`（`task.ts:125-128`）。
- **工具拒绝注入**（`task.ts:129-141`）：始终拒绝 `todowrite` 与 `task`（除非 agent 自身权限允许），加 `cfg.experimental.primary_tools`。
- **执行**：用 `ctx.extra.promptOps`，`runTask` 解析 prompt 并 `ops.prompt({ sessionID, model, agent, parts })` 驱动子会话（`task.ts:186-200`）。
- **后台流**：`background.start(...)` 立即返回 `state="running"`；完成经 `inject`/`notify` 注入父会话（`task.ts:242-272`）。前台用 `Effect.acquireUseRelease` + `Effect.raceFirst`（`task.ts:303-333`）。

### 11.3 编码进请求体的内容
- `description`（base = `task.txt`，含 "Launch a new agent to handle complex, multistep tasks autonomously."、"When NOT to use…"、"Usage notes: 1. Launch multiple agents concurrently…" 等）。
- 若 `flags.experimentalBackgroundSubagents`，registry 经 `describeTask(input.agent)`（`registry.ts:320-322`）把可用 agent 列表追加到 description；并追加 `BACKGROUND_DESCRIPTION`（`task.ts:25-30`）。
- `inputSchema`（`task.ts:43-62`）：
  - `description`: `String` — "A short (3-5 words) description of the task"
  - `prompt`: `String` — "The task for the agent to perform"
  - `subagent_type`: `String` — "The type of specialized agent to use for this task"
  - `task_id`: `optional(String)` — "resume a previous task…"
  - `command`: `optional(String)` — "The command that triggered this task"
  - `background`: `optional(Boolean)` — "Run the agent in the background…"（仅在完整 schema 暴露；experimental 时从 JSON schema 移除）

### 11.4 模型调用 / 返回
- 调用参数：`{ description, prompt, subagent_type, task_id?, command?, background? }`。
- 经 `renderOutput`（`task.ts:64-79`）包裹为 `<task>` XML：
  ```
  <task id="<sessionID>" state="running|completed|error">
  [<summary>...</summary>]
  <task_result>(或 <task_error>)<text></task_result>
  </task>
  ```
- **前台完成**（`task.ts:316-320`）：`{ title: params.description, metadata, output: renderOutput({ state:"completed", text: result.output }) }`。
- **后台**：立即返回 `state="running"` + `summary` + `metadata:{ background:true, jobId }`；实际结果稍后作为合成消息注入父会话，非同步返回。
- `metadata` 始终带 `parentSessionId`, `sessionId`, `model`；后台加 `background:true`, `jobId`。

---

## 12. todo（模型 id：`todowrite`）

**文件**：`tool/todo.ts`（`todowrite.txt`）

### 12.1 作用
创建/维护当前编码会话的结构化任务列表。模型传入**完整更新后的** todo 列表（替换先前状态），工具持久化到会话存储，向用户展示进度。

### 12.2 实现方式
- `Tool.define<typeof Parameters, Metadata, Todo.Service>("todowrite", …)`（`todo.ts:14`）。依赖 `Todo.Service`。
- 权限：`ctx.ask({ permission: "todowrite", patterns: ["*"], always: ["*"], …})`（`todo.ts:24-29`）——始终自动批准。
- 持久化：`todo.update({ sessionID: ctx.sessionID, todos: params.todos })`（`todo.ts:31-34`）到会话 DB。
- 无外部 HTTP/MCP/子代理——纯本地状态写入。

### 12.3 编码进请求体的内容
- `description`（`todowrite.txt` 原文）：含 "Create and maintain a structured task list…"、"## When to use"、"## When NOT to use"、"## States: pending/in_progress/completed/cancelled"、"## Rules…"。
- `inputSchema`（`todo.ts:6-8`）：
  - `todos`: `Array<Todo.Info>`（mutable） — "The updated todo list"
    - `Todo.Info`（`packages/schema/src/session-todo.ts`）：
      - `content`: `String` — "Brief description of the task"
      - `status`: `String` — "pending, in_progress, completed, cancelled"
      - `priority`: `String` — "high, medium, low"

### 12.4 模型调用 / 返回
- 调用参数：`{ todos: [{ content, status, priority }, …] }`——每次调用传**整个**修订列表（replace 语义）。
- 返回（`todo.ts:36-42`）：
  ```ts
  {
    title: "<N> todos",          // N = status !== "completed" 的数量
    output: JSON.stringify(params.todos, null, 2),
    metadata: { todos: params.todos },
  }
  ```
  无 `attachments`。

---

## 13. question

**文件**：`tool/question.ts`（`question.txt`）

### 13.1 作用
在执行过程中向用户提问，阻塞等待用户回答，再把答案回填给模型。用于澄清需求、确认实现选项、收集偏好。

### 13.2 实现方式
- `Tool.define("question", …)`（`question.ts:14`），`description` 来自 `./question.txt`（`question.ts:4,20`）。
- `execute` 调 `Question.Service.ask({ sessionID, questions: params.questions, tool })`（`question.ts:24-28`）。
- 阻塞机制：`question/index.ts:87-112` 创建 `Deferred`，存入 `pending` Map，发布 `Event.Asked`，`Deferred.await` 挂起整个 Effect 直到用户 `reply`（`Deferred.succeed`）或 `reject`（`Deferred.fail`）。由 UI/客户端事件触发。
- 返回前把答案格式化为 `"<问题>"="<答案>"` 串（`question.ts:30-32`）；空答案显 `"Unanswered"`。
- 用户拒绝（`Question.RejectedError`）→ 工具以错误形式返回。

### 13.3 编码进请求体的内容
- `description`（`question.txt` 原文）："Use this tool when you need to ask the user questions during execution. This allows you to: 1. Gather user preferences… 2. Clarify ambiguous instructions… 3. Get decisions on implementation choices… 4. Offer choices…"、"Usage notes: - When `custom` is enabled (default)…"、"- Answers are returned as arrays of labels; set `multiple: true`…"、"- If you recommend a specific option, make that the first option… add `"(Recommended)"`"。
- `inputSchema`（`question.ts:6-8`）：
  ```ts
  Schema.Struct({
    questions: Schema.mutable(Schema.Array(Question.Prompt))
      .annotate({ description: "Questions to ask" }),
  })
  ```
  其中 `Question.Prompt`（`@/question` schema）含 `question` / `header` / `options:[{label,description}]` / `custom` / `multiple` 等字段。

### 13.4 模型调用 / 返回
- 调用参数：`{ questions: [ { question, header, options:[{label,description}], custom, multiple } ] }`。
- 返回（`question.ts:34-40`）：
  ```ts
  {
    title: "Asked N question(s)",
    output: "User has answered your questions: <formatted>. You can now continue with the user's answers in mind.",
    metadata: { answers },   // ReadonlyArray<Question.Answer>
  }
  ```

---

## 14. lsp

**文件**：`tool/lsp.ts`（`lsp.txt`）

### 14.1 作用
与 Language Server Protocol (LSP) 服务器交互，提供代码智能（跳定义、找引用、hover、符号树、调用层级等 9 种操作）。

### 14.2 实现方式
- `Tool.define("lsp", …)`（`lsp.ts:37`），`description` = `DESCRIPTION` from `./lsp.txt`。
- `operations`（`lsp.ts:11-21`）：`goToDefinition, findReferences, hover, documentSymbol, workspaceSymbol, goToImplementation, prepareCallHierarchy, incomingCalls, outgoingCalls`。
- 路径：绝对或相对拼接 `instance.directory`（`lsp.ts:48`）；`assertExternalDirectoryEffect(ctx, file)`（`lsp.ts:49`）。
- 权限：`ctx.ask({ permission: "lsp", patterns: ["*"], always: ["*"], metadata })`（`lsp.ts:56-61`）。
- 可用性：`lsp.hasClients(file)`（`lsp/lsp.ts:328-342`）按扩展名+root 判断；无则抛 "No LSP server available for this file type."（`lsp.ts:78`）。
- 打开文档：`lsp.touchFile(file, "document")`（`lsp/lsp.ts:344-362`）发 `notify.open` 并等诊断。
- 真实调用：按 `args.operation` switch 分发到 `lsp.definition / references / hover / …`（`lsp.ts:82-103`）。位置 1-based→0-based（`lsp.ts:64`）。

### 14.3 编码进请求体的内容
- `description`（`lsp.txt`）："Interact with Language Server Protocol (LSP) servers to get code intelligence features. Supported operations: - goToDefinition … - workspaceSymbol … Note: LSP servers must be configured for the file type. If no server is available, an error will be returned."
- `inputSchema`（`lsp.ts:23-35`）：
  ```ts
  Schema.Struct({
    operation: Schema.Literals(operations).annotate({ description: "The LSP operation to perform" }),
    filePath: Schema.String.annotate({ description: "The absolute or relative path to the file" }),
    line: Schema.Int.check(Schema.isGreaterThanOrEqualTo(1)).annotate({ description: "The line number (1-based)" }),
    character: Schema.Int.check(Schema.isGreaterThanOrEqualTo(1)).annotate({ description: "The character offset (1-based)" }),
    query: Schema.optional(Schema.String).annotate({ description: "Search query for workspaceSymbol." }),
  })
  ```

### 14.4 模型调用 / 返回
- 调用参数：`{ operation, filePath, line, character, query? }`。
- 返回（`lsp.ts:105-109`）：
  ```ts
  {
    title: `goToDefinition <relPath>:<line>:<character>` (或仅 `<operation>`),
    output: JSON.stringify(result, null, 2),   // 或 "No results found for <operation>"
    metadata: { result },   // unknown[]
  }
  ```

---

## 15. plan（实际只有 `plan_exit` 工具）

**文件**：`tool/plan.ts`（`plan-exit.txt`；`plan-enter.txt` 存在但未被任何代码引用）

### 15.1 作用
退出计划模式：询问用户是否切换到 build agent 开始实现；同意后向会话注入一条 build-agent 用户消息并触发实现。**注意：这是 `plan_exit`，没有对应的 `plan_enter` 工具。** 进入计划模式由 `plan_enter` *权限动作* + agent 模式切换实现（见下）。

### 15.2 实现方式
- `Tool.define("plan_exit", …)`（`plan.ts:15`）——**id 是 `"plan_exit"`，不是 `"plan"`**。
- `description` = `EXIT_DESCRIPTION` from `./plan-exit.txt`。
- 依赖：`Session / Question / Provider`（`plan.ts:18-20`）。
- `execute`：取 session 与计划文件路径 `Session.plan(info, instance)`（`plan.ts:28-29`）；`question.ask` 弹问（Yes/No，`plan.ts:30-44`）。
  - 选 `No` → `new Question.RejectedError()`（`plan.ts:46`）→ 工具以错误回传，留 plan agent。
  - 选 `Yes` → 找最后 user 消息的 model（否则 `provider.defaultModel()`），构造 `{ role:"user", agent:"build", model }` 消息 + 合成 text `The plan at <plan> has been approved…` 更新会话（`plan.ts:48-69`）。
- 模式切换权限基础：`build` agent 有 `plan_enter: "allow"`，`plan` agent 有 `plan_exit: "allow"`，二者默认 `plan_enter/plan_exit: "deny"`。TUI 监听 `part.tool === "plan_enter"`/`"plan_exit"` 切换本地 `local.agent`。

### 15.3 编码进请求体的内容
- `description`（`plan-exit.txt`）："Use this tool when you have completed the planning phase and are ready to exit plan agent… This tool will ask the user if they want to switch to build agent… Call this tool: - After you have written a complete plan… Do NOT call this tool: - Before you have created or finalized the plan…"
- `inputSchema`（`plan.ts:13`）：`Schema.Struct({})` —— 无参数。

### 15.4 模型调用 / 返回
- 模型调用时不提供参数（空对象）。
- 返回（`plan.ts:71-75`）：
  ```ts
  {
    title: "Switching to build agent",
    output: "User approved switching to build agent. Wait for further instructions.",
    metadata: {},
  }
  ```
  若拒绝（`No`）：抛 `Question.RejectedError`（"The user dismissed this question"），模型可重试或继续计划。

> `plan-enter.txt` 文案当前**未被工具消费**；进入计划模式的实际控制点在权限规则与 TUI/agent 配置，而非 `plan_enter` 工具。

---

## 16. invalid

**文件**：`tool/invalid.ts`

### 16.1 作用
始终存在于工具列表中的占位/错误工具，用于向模型表明某次工具调用无效（把"无效工具调用"作为结构化结果回传，而非抛出未处理异常）。

### 16.2 实现方式
- `Tool.define("invalid", Effect.succeed({ … }))`（`invalid.ts:9-11`）——直接 `Effect.succeed` 提供定义。
- `description` 硬编码 `"Do not use"`（`invalid.ts:12`）。
- `execute`（`invalid.ts:14-19`）直接 `Effect.succeed` 返回，无外部调用、无权限检查、无副作用。

### 16.3 编码进请求体的内容
- `description`：`"Do not use"`（`invalid.ts:12`）。
- `inputSchema`（`invalid.ts:4-7`）：
  ```ts
  Schema.Struct({
    tool: Schema.String,
    error: Schema.String,
  })
  ```

### 16.4 模型调用 / 返回
- 正常情况下模型**不应**调用它（`description: "Do not use"`）。框架在检测到无效工具调用时带入 `{ tool, error }`。
- 返回（`invalid.ts:15-19`）：
  ```ts
  {
    title: "Invalid Tool",
    output: "The arguments provided to the tool are invalid: <error>",   // <error> = params.error
    metadata: {},
  }
  ```

---

## 17. execute（code-mode）

**文件**：`tool/code-mode.ts`

### 17.1 作用
实验性「代码模式」工具：运行一段受限的编排脚本（confined orchestration script），脚本可访问已连接的 MCP 工具。仅在 code mode 下可用，由 `experimentalCodeMode` 标志门控。

### 17.2 实现方式
- `Tool.define("execute", …)`（`code-mode.ts:188`），`CODE_MODE_TOOL = "execute"`（`code-mode.ts:12`）。
- `DESCRIPTION`（硬编码，`code-mode.ts:14`）："Run a confined orchestration script with access to connected MCP tools."
- `Parameters`（`code-mode.ts:16-20`）：`{ code: Schema.String.annotate({ description: "Script body executed by the confined interpreter." }) }`。
- 实际行为：取 `MCP / Agent / Session / Plugin`（`code-mode.ts:191-194`）；按 agent+session permission 计算 `ruleset`，`Permission.visibleTools` 过滤可见 MCP 工具（`code-mode.ts:209-210`）；`servers` 来自 `mcp.clients()`。
- 用 `groupByServer` + `toolTree` 把 MCP 工具映射成 `@opencode-ai/codemode` 的 `SandboxTool.Definition` 树（`code-mode.ts:39-56,120-132`）。
- 构造 `CodeMode.make({ tools, onToolCallStart, onToolCallEnd })` 运行时（`code-mode.ts:239-260`）；`onToolCallStart/End` 把每次子工具调用状态写入 `calls[]` 并 `ctx.metadata(...)` 实时回传进度。
- 执行：`runtime.execute(params.code)` 与 abort 信号 `Effect.raceFirst` 竞速（`code-mode.ts:274`），可被 `ctx.abort` 取消。
- 子工具调用 `invokeChildTool`（`code-mode.ts:134-186`）：`plugin.trigger("tool.execute.before/after")` + `ctx.ask({permission, patterns:["*"], always:["*"]})` + `entry.tool.client.callTool(...)` 真正调用 MCP，结果经 `projectMcpResult`（`code-mode.ts:75-116`）规范化（text 合并、image/audio/resource 转 `attachments` 的 data-url `file` 附件）。
- 失败：`result.ok` 为 false 时拼 `error.message + suggestions` 并 `Effect.fail`（`code-mode.ts:281-291`）；取消返回 `"Execution cancelled."`。
- 输出：`result.logs` 追加到 output 末尾（`code-mode.ts:295-303`）。
- `describeCatalog`（`code-mode.ts:58-65`）生成补充说明，在 `registry.ts:300-303` 注入到 description。

### 17.3 编码进请求体的内容
- `description`（硬编码，`code-mode.ts:14`）："Run a confined orchestration script with access to connected MCP tools." + registry 追加 `codeModeDescription`（catalog 描述）。
- `inputSchema`（`code-mode.ts:16-20`）：
  ```ts
  Schema.Struct({
    code: Schema.String.annotate({ description: "Script body executed by the confined interpreter." }),
  })
  ```

### 17.4 模型调用 / 返回
- 调用参数：`{ code: "<脚本文本>" }`，脚本在该会话可见的 MCP 工具命名空间下运行。
- 返回（`code-mode.ts:200-206,300-305`）：
  ```ts
  {
    title: "execute",
    output: <脚本返回值或 JSON.stringify> + (logs.length>0 ? "\n\nLogs:\n<logs>" : ""),
    // 失败/取消: "Execution cancelled." 或错误信息
    metadata: { toolCalls: CallEntry[], error?: boolean },  // CallEntry = { tool, status, input? }
    ...(attachments.length > 0 ? { attachments } : {}),     // 子工具返回 media/resource 时才带
  }
  ```

---

## 附录 A：工具 id 与 registry 局部名对照

| registry 局部名 | 模型可见 id | 备注 |
|----------------|-------------|------|
| `shell` | `bash` | 兼容别名（`shell/id.ts:16`） |
| `read` | `read` | |
| `edit` | `edit` | 与 write/patch 共用 `edit` 权限 |
| `write` | `write` | |
| `patch` | `apply_patch` | `Tool.define("apply_patch")`（`apply_patch.ts:22`） |
| `glob` | `glob` | |
| `grep` | `grep` | |
| `fetch` | `webfetch` | 局部名 `fetch` 不影响 id |
| `search` | `websearch` | 局部名 `search` 不影响 id |
| `skill` | `skill` | |
| `task` | `task` | |
| `todo` | `todowrite` | **id 是 `todowrite`** |
| `question` | `question` | |
| `lsp` | `lsp` | |
| `plan` | `plan_exit` | **id 是 `plan_exit`** |
| `invalid` | `invalid` | |
| `execute` | `execute` | code mode |

## 附录 B：权限 key 汇总

| 权限 key | 对应工具 |
|----------|----------|
| `bash` | shell（命令扫描另含 `external_directory`） |
| `read` | read |
| `edit` | edit / write / apply_patch（三者共用） |
| `glob` | glob |
| `grep` | grep |
| `webfetch` | webfetch |
| `websearch` | websearch |
| `skill` | skill |
| `task` | task |
| `todowrite` | todo |
| `question` | question |
| `lsp` | lsp |
| `plan_enter` / `plan_exit` | plan（权限动作） |
| — | invalid / execute（占位 / code mode） |

## 附录 C：请求体 `tools` 字段最终构成（回顾）

`tools` 字段含：内置工具 + 自定义 `{tool,tools}/*.{js,ts}` + 插件 `p.tool` + MCP 工具 + MCP 资源工具 + `skill` loader。**不含** skill 定义（进 system prompt 的 `<available_skills>`）与 plugin 配置（进 hooks / `<mcp_instructions>`）。组装管线：`SessionTools.resolve` → `handle.process({tools})` → `LLMRequestPrep.prepare`（`resolveTools`）→ `streamText({tools, activeTools})`。详见 `02-Tools.md`（待补）。
