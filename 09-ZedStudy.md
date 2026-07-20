# Zed 项目学习研究笔记

> 本文件用于记录对 [zed-industries/zed](https://github.com/zed-industries/zed) 项目的学习研究成果。

## 目录
- [项目概览](#项目概览)
- [架构分析](#架构分析)
- [关键模块](#关键模块)
- [学习笔记](#学习笔记)
  - Agent 相关
    - [Zed Agent 三种默认模式](#zed-agent-三种默认模式)
    - [Profile 切换与上下文共享机制](#profile-切换与上下文共享机制)
    - [三模式共享的 system prompt 可否自定义](#三模式共享的-system-prompt-可否自定义)
    - [System prompt 的拼接行为](#system-prompt-的拼接行为)
    - [Agent 编辑审核 UI](#agent-编辑审核-ui)
    - [工具权限系统与 YOLO mode](#工具权限系统与-yolo-mode)
  - Skill 相关
    - [Skill 定义与触发机制](#skill-定义与触发机制)
    - [Skill 是否支持脚本等复杂行为](#skill-是否支持脚本等复杂行为)
  - 线程与持久化
    - [Threads 侧边栏持久化机制](#threads-侧边栏持久化机制)
  - 编辑预测
    - [编辑预测（Edit Prediction）触发机制](#编辑预测edit-prediction触发机制)
  - Language Server
    - [Language Server 安装与加载机制](#language-server-安装与加载机制)
  - 终端
    - [Terminal 多 Profile 支持](#terminal-多-profile-支持)
    - [Terminal 打开位置设置](#terminal-打开位置设置)
  - 设置与格式
    - [settings.json 格式：JSONC 支持](#settingsjson-格式jsonc-支持)
    - [Settings 迁移非幂等循环问题](#settings-迁移非幂等循环问题)
  - 界面布局
    - [界面各部分位置交换](#界面各部分位置交换)
    - [聚焦到界面各部分的直观操作](#聚焦到界面各部分的直观操作)
    - [全面板 Toggle/Focus 动作对照表](#全面板-togglefocus-动作对照表)
    - [各面板聚焦后可用动作详解](#各面板聚焦后可用动作详解)
    - [全局焦点循环（`FocusNextPart` / `FocusPreviousPart`）](#全局焦点循环focusnextpart--focuspreviouspart)
    - [关键代码位置](#关键代码位置)
- [参考资料](#参考资料)

---

## 项目概览

<!-- 待补充 -->

## 架构分析

<!-- 待补充 -->

## 关键模块

<!-- 待补充 -->

## 学习笔记

### Zed Agent 三种默认模式

Zed Agent 内置三个不可更改的模式（built-in profiles），定义在 `crates/agent_settings/src/agent_profile.rs`：

| 模式 | Profile ID | 说明 |
|------|-----------|------|
| Write | `"write"` | 完整工具权限，可读写文件、执行终端命令等 |
| Ask | `"ask"` | 仅限读取/搜索类工具，无写入权限 |
| Minimal | `"minimal"` | 无任何工具权限，仅根据已有上下文回答 |

这三个 ID 是硬编码常量，不可删除或重命名。但可通过 Fork 创建自定义 profile，Fork 会复制原 profile 的 `tools`、`enable_all_context_servers`、`context_servers`、`default_model` 作为起点。

#### 每个模式可配置的内容

每个模式（profile）仅可定义以下字段：

- **`default_model`** — 该模式激活时自动切换的模型（含 provider、model、enable_thinking、effort、speed）
- **`tools`** — 每个 Agent 工具的启用/禁用开关（`IndexMap<工具名, bool>`）
- **`enable_all_context_servers`** — 是否默认启用所有 MCP 服务器工具
- **`context_servers`** — 每个 MCP 服务器内各工具的启用/禁用覆盖
- **`name`** — 显示名称（仅用于 UI，不影响行为）

#### 三模式共享同一个 system prompt

三个模式使用 **同一个** Handlebars 模板：`crates/agent/src/templates/system_prompt.hbs`。

区别仅由模板变量 `available_tools` 驱动：

- **Write** 模式下，所有工具可用（包括 `edit_file`、`write_file`、`terminal`、`search_web`、`spawn_agent` 等，且 `enable_all_context_servers: true`）
- **Ask** 模式下，仅保留读取/搜索类工具 + `fetch` + `spawn_agent` + `create_thread`，屏蔽所有写入工具
- **Minimal** 模式下，工具列表为空，system prompt 渲染 fallback 段落，告知模型无工具可用

**即：三模式之间没有独立的 system prompt 内容，差异完全由工具权限列表决定。**

#### 参考文件

- `crates/agent_settings/src/agent_profile.rs` — Profile 定义与创建逻辑
- `crates/agent_settings/src/agent_settings.rs` — 全局 Agent 设置与 profile 容器
- `crates/settings_content/src/agent.rs` — JSON 序列化结构体
- `crates/agent/src/templates/system_prompt.hbs` — 共享的 system prompt 模板
- `crates/agent/src/thread.rs` — system prompt 构建与工具过滤逻辑
- `crates/agent_ui/src/agent_configuration/manage_profiles_modal.rs` — 管理 profile 的 UI 与 Fork 功能
- `assets/settings/default.json` — 三模式的默认工具配置

### Threads 侧边栏持久化机制

Threads 侧边栏使用三个独立的 SQLite 数据库存储不同线程类型的数据，全部位于 `<data_dir>/threads/`。

#### 三种线程类型对比

| 线程类型 | 元数据持久化 | 内容数据持久化 | 数据库文件 |
|---------|-------------|--------------|-----------|
| **Agent 线程**（原生 Zed） | 是 — `sidebar_threads` 表 | 是 — `threads` 表（zstd 压缩 JSON） | `threads.db` + `thread_metadata` db |
| **ACP 线程**（外部代理） | 是 — `sidebar_threads` 表 | **由外部 ACP 代理服务器管理**，Zed 仅缓存元数据 | `thread_metadata` db |
| **终端线程** | 是 — `sidebar_terminal_threads` 表 | **不存储**（重启后丢失） | `terminal_thread_metadata` db |

#### 引用文件

- `crates/agent/src/db.rs` — `ThreadsDatabase` 结构体，`threads` 表（`id`、`summary`、`updated_at`、`data_type`、`data` BLOB）
- `crates/agent/src/thread_store.rs` — `ThreadStore`，包装 `ThreadsDatabase` 提供面向侧边栏的线程列表，过滤子代理线程
- `crates/agent_ui/src/thread_metadata_store.rs` — `ThreadMetadataStore` / `ThreadMetadata` / `ThreadMetadataDb`，`sidebar_threads` 表
- `crates/agent_ui/src/terminal_thread_metadata_store.rs` — `TerminalThreadMetadataStore` / `TerminalThreadMetadata` / `TerminalThreadMetadataDb`，`sidebar_terminal_threads` 表
- `crates/agent_ui/src/draft_prompt_store.rs` — 草稿未发送提示词的 KVP 存储
- `crates/agent_ui/src/threads_archive_view.rs` — 归档线程历史视图
- `crates/agent_ui/src/agent_panel.rs` — Agent 面板，负责创建/保存线程和终端
- `crates/agent_ui/src/conversation_view.rs` — 对话视图，ACP 会话生命周期与元数据更新
- `crates/acp_thread/src/acp_thread.rs` — `AcpThread` 运行时结构体
- `crates/acp_thread/src/connection.rs` — `AgentSessionInfo`、`SessionListUpdate`、`AgentConnection` trait
- `crates/sidebar/src/sidebar.rs` — 侧边栏渲染，组合 `ThreadMetadataStore` 和 `TerminalThreadMetadataStore`
- `crates/ui/src/components/ai/thread_item.rs` — 侧边栏中线程项的 UI 组件

#### Agent 线程数据模型

```rust
// crates/agent_ui/src/thread_metadata_store.rs:308
pub struct ThreadMetadata {
    pub thread_id: ThreadId,                      // 本地 UUID
    pub session_id: Option<acp::SessionId>,       // ACP 会话 ID（草稿为 None）
    pub agent_id: AgentId,                        // 代理标识符
    pub title: Option<SharedString>,              // 代理生成标题
    pub title_override: Option<SharedString>,     // 用户覆盖标题（优先）
    pub updated_at: DateTime<Utc>,
    pub created_at: Option<DateTime<Utc>>,
    pub interacted_at: Option<DateTime<Utc>>,     // 最后用户交互时间
    pub worktree_paths: WorktreePaths,             // 项目路径关联
    pub remote_connection: Option<RemoteConnectionOptions>,
    pub archived: bool,                           // 是否归档
}
```

#### 终端线程数据模型

```rust
// crates/agent_ui/src/terminal_thread_metadata_store.rs:48
pub struct TerminalThreadMetadata {
    pub terminal_id: TerminalId,
    pub title: SharedString,
    pub custom_title: Option<SharedString>,
    pub created_at: DateTime<Utc>,
    pub worktree_paths: WorktreePaths,
    pub remote_connection: Option<RemoteConnectionOptions>,
    pub working_directory: Option<PathBuf>,
}
```

#### 完整 Agent 对话数据模型

```rust
// crates/agent/src/db.rs:54
pub struct DbThread {
    pub title: SharedString,
    pub messages: Vec<Arc<DbMessage>>,
    pub updated_at: DateTime<Utc>,
    pub detailed_summary: Option<SharedString>,
    pub initial_project_snapshot: Option<Arc<ProjectSnapshot>>,
    pub cumulative_token_usage: language_model::TokenUsage,
    pub request_token_usage: HashMap<ClientUserMessageId, TokenUsage>,
    pub model: Option<DbLanguageModel>,
    pub profile: Option<AgentProfileId>,
    pub subagent_context: Option<SubagentContext>,
    pub speed: Option<Speed>,
    pub thinking_enabled: bool,
    pub thinking_effort: Option<String>,
    pub draft_prompt: Option<Vec<acp::ContentBlock>>,
    pub ui_scroll_position: Option<SerializedScrollPosition>,
    pub sandboxed_terminal_temp_dir: Option<PathBuf>,
    pub sandbox_grants: DbSandboxGrants,
}
```

#### 数据库表结构

**`sidebar_threads` 表**（`thread_metadata_store.rs:1373`）：

```sql
CREATE TABLE IF NOT EXISTS sidebar_threads(
    thread_id BLOB PRIMARY KEY,
    session_id TEXT,
    agent_id TEXT,
    title TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    created_at TEXT,
    folder_paths TEXT,
    folder_paths_order TEXT,
    archived INTEGER DEFAULT 0,
    main_worktree_paths TEXT,
    main_worktree_paths_order TEXT,
    remote_connection TEXT,
    interacted_at TEXT,
    title_override TEXT
) STRICT;
```

**`sidebar_terminal_threads` 表**（`terminal_thread_metadata_store.rs:448`）：

```sql
CREATE TABLE IF NOT EXISTS sidebar_terminal_threads(
    terminal_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    custom_title TEXT,
    created_at TEXT NOT NULL,
    working_directory TEXT,
    folder_paths TEXT,
    folder_paths_order TEXT,
    main_worktree_paths TEXT,
    main_worktree_paths_order TEXT,
    remote_connection TEXT
) STRICT;
```

**`threads` 表**（`db.rs:440`）：

```sql
CREATE TABLE IF NOT EXISTS threads (
    id TEXT PRIMARY KEY,
    summary TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    data_type TEXT NOT NULL,
    data BLOB NOT NULL
)
```

#### 持久化关键逻辑

- **Agent 线程**：每次消息发送后，`save_thread_sync()` 将整个 `DbThread` 序列化为 JSON，用 **zstd level 3 压缩** 存入 `data BLOB` 列（`db.rs:522-524`）。重启后通过 `AgentPanel::load()` 读取 `KeyValueStore` 中的 `last_active_thread`，在 `ThreadMetadataStore` 中查找并重建完整对话。
- **ACP 线程**：Zed 仅持久化元数据（通过 `ThreadMetadataStore`），完整对话数据由外部 ACP 代理服务器通过 `AgentConnection` trait 管理。
- **终端线程**：**仅元数据持久化**（标题、ID、路径、工作目录）。终端内容（scrollback buffer、命令历史、输出）**不存储**。终端本质上是**有状态的 shell 进程**，其状态存在于进程内存中，Zed 关闭时进程被 SIGTERM 杀死，scrollback buffer 随进程消亡。重启后侧边栏重建一个同名同路径的终端条目，但底层是**全新的 shell 会话**。
- **子代理线程**：`ThreadStore::spawn_reload()`（`thread_store.rs:115`）显式跳过 `parent_session_id.is_some()` 的线程，不在侧边栏展示。
- **草稿线程**：`session_id = None` 表示草稿，元数据持久化；未发送的提示词通过 `KeyValueStore`（namespace `"agent_draft_prompts"`）额外存储，首次消息发送后删除。
- **归档线程**：`archived = true` 的线程从侧边栏隐藏，仅通过归档历史视图可见。空窗口（无 project folder）创建的线程默认归档。
- **`ZED_STATELESS` 环境变量**：启用后 `threads.db` 回退到内存数据库，所有线程数据不持久化。

### Profile 切换与上下文共享机制

#### 同一对话中是否可以切换模式

**可以。** 切换 profile（Write/Ask/Minimal 或自定义 profile）时，当前对话线程 **完整保留**，不会重置。

切换逻辑在 `crates/agent/src/thread.rs:2216-2237` 的 `Thread::set_profile()` 中：

```rust
pub fn set_profile(&mut self, profile_id: AgentProfileId, cx: &mut Context<Self>) {
    self.profile_downgraded_for_restricted_workspace = false;
    if self.profile_id == profile_id {
        return;
    }
    self.profile_id = profile_id.clone();
    // 切换到 profile 的首选模型（如果有）
    if let Some(model) = Self::resolve_profile_model(&self.profile_id, cx) {
        self.set_model(model, cx);
    }
    // 传播到子代理
    for subagent in &self.running_subagents {
        subagent.update(cx, |thread, cx| thread.set_profile(profile_id.clone(), cx)).ok();
    }
}
```

切换仅做三件事：
1. 更新 `self.profile_id`
2. 如有配置，切换到该 profile 的首选模型
3. 传播到所有运行中的子代理

**`self.messages` 不被触碰**，所有历史消息完整保留。

UI 层通过 `crates/agent_ui/src/profile_selector.rs` 的 `ProfileSelector::cycle_profile()` 切换，调用路径为 `ProfileSelector → ConversationView → Thread::set_profile()`。

#### 共享上下文是 built-in 还是可设置

**硬编码（built-in），不可设置。**

`Thread` 结构体（`crates/agent/src/thread.rs:1216-1276`）只有一个 `messages: Vec<Arc<Message>>` 字段。没有 `HashMap<AgentProfileId, Vec<Message>>` 或其他按 profile 隔离上下文的机制。

所有 profile 共享同一个 `messages` 列表。切换 profile 时，system prompt 会被重建（`thread.rs:4217-4232`），但对话历史始终来自同一个 `self.messages`。

`AgentSettings` 中也没有任何设置项可以按 profile 拆分上下文。

#### 参考文件

- `crates/agent/src/thread.rs` — `set_profile()` 与 `messages` 字段
- `crates/agent_ui/src/profile_selector.rs` — 下拉选择器切换逻辑
- `crates/agent_ui/src/conversation_view.rs` — `ProfileProvider` 实现

### Skill 定义与触发机制

#### Skill 存放位置

Skill 以 `SKILL.md` 文件形式存放，按优先级分为三个来源：

| 来源 | 路径 | 优先级 |
|------|------|--------|
| **Built-in** | 编译到 Zed 二进制中（目前仅 `create-skill`） | 最低 |
| **Global** | `~/.agents/skills/<name>/SKILL.md` | 中 |
| **Project-local** | `{project}/.agents/skills/<name>/SKILL.md` | 最高 |

同级目录结构：
```
~/.agents/skills/
  ├── my-skill/
  │   └── SKILL.md          # 含 YAML frontmatter，定义 name/description
  └── another-skill/
      └── SKILL.md
```

#### Skill 文件格式

`SKILL.md` 开头包含 YAML frontmatter（`crates/agent_skills/agent_skills.rs:206-211`）：

```yaml
---
name: my-skill
description: 描述该 skill 的用途，模型据此判断何时调用
disable-model-invocation: true  # 可选，设为 true 后模型不会自动调用
---
```

#### 触发机制

Skill 有两种触发方式：

**1. 模型自动调用（默认）**

- 加载后，所有 skill 的 `name` + `description` 被汇总到 system prompt 的 `<available_skills>` 块中
- 模型收到用户请求后，自行判断是否匹配某个 skill 的描述，如果匹配则调用 `skill` 工具
- 该工具在 **Write 和 Ask** 两个 profile 中默认启用（`assets/settings/default.json:1194,1216`）
- `skill` 工具接收一个参数 `name: String`，返回 skill 的完整内容包裹在 `<skill_content>` 中
- 对于非 built-in skill，首次使用需要用户授权（Allow-Once / Deny）

**2. 用户手动调用（slash command）**

- 在输入框中输入 `/:<skill_name>` 触发当前 project 的全局/built-in skill
- 输入 `/<worktree>:<skill_name>` 触发特定 worktree 的 project-local skill
- 该方式不依赖 `disable-model-invocation` 设置

#### `disable-model-invocation` 的作用

设为 `true` 后，该 skill 不会出现在 system prompt 的 `<available_skills>` 列表中，模型无法自动感知和调用，但用户仍可通过 slash command 手动触发。

#### Skill 加载流程

```
SKILL.md 文件位于磁盘
  → load_skills_from_directory() 扫描目录（agent_skills.rs:543-579）
  → combine_skills() 合并三个来源，apply_skill_overrides() 按优先级去重（agent.rs:3623-3703）
  → 存入 project state 的 skills 列表
  → select_catalog_skills() 过滤掉 disable_model_invocation 的 skill，截断超过 50KB 的描述（agent.rs:3481-3543）
  → 加入 ProjectContext 渲染到 system prompt
  → 模型调用 skill tool → SkillTool::run() 读取文件内容并返回
```

#### 参考文件

- `crates/agent_skills/agent_skills.rs` — Skill 结构体定义、元数据解析、加载、枚举
- `crates/agent/src/tools/skill_tool.rs` — `skill` 工具的 AgentTool 实现
- `crates/agent/src/agent.rs` — `combine_skills()`、`apply_skill_overrides()`、`select_catalog_skills()`
- `crates/agent/src/templates/system_prompt.hbs` — `<available_skills>` 渲染模板
- `crates/agent/src/tools.rs` — 工具注册宏
- `assets/settings/default.json` — Write/Ask profile 中 `skill` 工具默认启用

### Skill 是否支持脚本等复杂行为

**不支持。Skill 是纯文本的 Markdown 指令，不包含任何可执行代码。**

`Skill` 结构体（`crates/agent_skills/agent_skills.rs:75-93`）没有任何脚本/命令相关字段：

```rust
pub struct Skill {
    pub name: String,
    pub description: String,
    pub source: SkillSource,
    pub directory_path: PathBuf,
    pub skill_file_path: PathBuf,
    pub load_warnings: Vec<SkillLoadWarning>,
    pub disable_model_invocation: bool,
    pub embedded_body: Option<&'static str>,
}
```

`skill` 工具的执行流程（`crates/agent/src/tools/skill_tool.rs:164-236`）仅做三件事：
1. 按名称查找 skill
2. 读取文件内容（或取 built-in 的嵌入式内存）
3. 将内容包裹在 `<skill_content>` XML 标签中返回

**Skill 本身不执行任何操作。** 模型读取 skill 内容后，自行使用已有的工具（`read_file`、`write_file`、`terminal`、`grep` 等）来执行指令。

官方 README（`crates/agent_skills/README.md:266`）明确注明"动态上下文注入（SKILL.md 中嵌入 shell 命令并展开）"是 **待定功能**，因需要独立的安全模型。

#### 参考文件

- `crates/agent_skills/agent_skills.rs` — Skill 结构体与加载逻辑
- `crates/agent/src/tools/skill_tool.rs` — `skill` 工具实现（`render_skill_envelope()`、`run()`）
- `crates/agent_skills/README.md` — 官方说明文档，标明了限制

### 三模式共享的 system prompt 可否自定义

**可以部分自定义，但不能直接覆盖模板文件。**

#### 可自定义的途径

**1. 个人 AGENTS.md（全局）**
- 路径：`~/.config/zed/AGENTS.md`（平台相关，通过 `paths::agents_file()` 确定）
- 内容会被渲染到 system prompt 末尾的 `## User's Custom Instructions > Personal AGENTS.md` 中
- 自动热加载，文件变化即生效
- 空文件或仅含空白字符视为无文件

**2. Project Rules（项目级）**
- 项目根目录下的规则文件（如 `AGENTS.md`、`.rules/` 目录下的文件）
- 渲染在 `## User's Custom Instructions > Project Rules` 中
- 优先级高于个人 AGENTS.md

#### 不可自定义的

- **没有 `system_prompt` 设置项** — `AgentSettings` 中不存在相关字段
- **模板文件嵌入在二进制中** — 通过 `rust_embed` 编译进 Zed，无法从外部覆盖
- **没有类似 Cursor/Copilot 的"自定义指令"UI 设置**

#### 原始 system prompt 完整内容

以下为 `crates/agent/src/templates/system_prompt.hbs`（277 行）的完整内容：

`````handlebars
You are the Zed coding agent running inside the Zed editor. You help users
complete software engineering tasks by understanding their codebase, making
careful changes, and explaining your work clearly. Use your broad knowledge
of programming languages, frameworks, design patterns, and engineering best
practices to solve problems pragmatically.

## Communication

- Default to a tone that is concise, direct, and friendly. Communicate
  efficiently and prioritize actionable guidance over verbose narration.
- Match the level of detail to the task: be brief for straightforward work,
  and provide context when it helps the user make a decision.
- Be accurate and truthful. Do not fabricate details.
- Prioritize technical correctness over affirming the user's assumptions.
- Be transparent about uncertainty.
- Do not over-apologize when results are unexpected.

## Formatting Responses

Format responses in markdown. Use backticks for file paths, directories,
commands, functions, classes, and other code identifiers.
Supports markdown images and Mermaid diagrams (flowchart, sequence, class,
state, ER, gantt, pie, gitgraph, mindmap, timeline, quadrant chart, xy chart,
journey). Do NOT include {{%%{init}%%}} directives or inline HTML in mermaid.
Mermaid diagrams are automatically themed to match the user's editor theme.

{{#if (gt (len available_tools) 0)}}
## Tool Use

- Follow the available tool schemas exactly.
- Use only the tools that are currently available.
- Prefer the most direct tool for the job.
- Gather enough context before acting; do not use placeholders.
- Call independent tools in parallel; sequentialize dependent steps.
- Set timeout_ms for long-running commands.
- Do not re-read files after write_file/edit_file.
- Send a brief preamble before groups of related tool calls.

## Task Execution

- Keep going until the task is completely resolved.
- Autonomously resolve the task; ask only when information is genuinely
  unavailable.
- Do not guess or make up an answer.

## Searching and Reading

- Use project-relative paths starting with a root directory name.
- Do not guess file paths.
- Prefer the `grep` tool for symbol search.
- Scope searches to targeted subtrees.

## Making Code Changes

- Fix the root cause, not surface-level patches.
- Keep changes consistent with existing code style.
- Do not overwrite changes you did not make.
- Update related tests/docs/config when part of the change.
- Do not fix unrelated bugs.
- Do not commit changes unless explicitly requested.
- Do not add comments that merely restate the code.

## Ambition vs. Precision

- For new tasks: be ambitious and creative.
- For existing codebases: do exactly what the user asks with surgical
  precision.

## Validation

- Run tests if available to verify work.
- Start specific, then broaden.
- Report actual results, not claims.

## Fixing Diagnostics

- Make 1-2 focused attempts, then defer to the user.

## Debugging

- Only change code if confident in root cause.
- Prefer reproducing the issue first.
- Add descriptive logging.

## Calling External APIs

- Use appropriate APIs consistently with project expectations.
- Never hardcode secrets.
- Be explicit about cost, rate-limit, privacy implications.

{{#if (contains available_tools 'spawn_agent')}}
## Multi-agent delegation

Sub-agents can help move faster on large tasks with multiple well-defined
scopes, independent parallel steps, or information-gathering tasks.
Create concrete, self-contained subtasks with all needed context.
{{/if}}

## Final Message

- Briefly summarize what changed, reference files, state validation results.
- Offer obvious follow-ups as questions.

{{else}}
You are being tasked with providing a response, but you have no ability to
use tools or to read or write any aspect of the user's system other than the
context the user provides. Give the best answer you can from available
context.
{{/if}}

## System Information

Operating System: {{os}}
Default Shell: {{shell}}
Today's Date: {{date}}
Project root directories: {{#each worktrees}}`{{abs_path}}`{{/each}}

{{#if sandboxing}}
## Terminal sandbox
...（沙箱权限说明，约 50 行，按平台条件渲染）
{{/if}}

{{#if model_name}}
## Model Information
You are powered by the model named {{model_name}}.
{{/if}}

{{#if has_skills}}
## Agent Skills
<available_skills>{{#each skills}}
  <skill><name>{{name}}</name><description>{{description}}</description>
  <location>{{{location}}}</location></skill>{{/each}}
</available_skills>
To use a Skill: identify match → use `skill` tool → follow instructions.
{{/if}}

{{#if (or user_agents_md has_rules)}}
## User's Custom Instructions

{{#if user_agents_md}}
### Personal AGENTS.md
````
{{{user_agents_md}}}
````
{{/if}}

{{#if has_rules}}
### Project Rules
{{#each worktrees}}{{#if rules_file}}
`{{root_name}}/{{rules_file.path_in_worktree}}`:
````
{{{rules_file.text}}}
````
{{/if}}{{/each}}
{{/if}}
{{/if}}
`````

**关键模板变量：**

| 变量 | 来源 | 说明 |
|------|------|------|
| `available_tools` | 当前 profile 启用的工具列表 | 控制工具使用指引和部分条件块 |
| `os` / `shell` / `date` | 运行时环境 | 系统信息 |
| `worktrees` | 当前项目 | 工作区根目录列表 |
| `sandboxing` | 项目配置 | 是否启用沙箱模式 |
| `model_name` | 当前模型 | 模型名称显示 |
| `has_skills` | 项目 skill 列表 | 是否有可用 skill |
| `user_agents_md` | `~/.config/zed/AGENTS.md` | 用户自定义指令 |
| `has_rules` | 项目规则文件 | 项目级规则 |
| `spawn_agent` | 是否在 available_tools 中 | 多代理章节条件 |

#### 参考文件

- `crates/agent/src/templates/system_prompt.hbs` — 完整的 system prompt 模板
- `crates/agent/src/templates/experimental_system_prompt.hbs` — 实验性简化版（未使用，dead code）
- `crates/agent/src/templates.rs` — `SystemPromptTemplate` 结构体与模板选择
- `crates/agent/src/thread.rs` — system prompt 构建（`user_agents_md` 注入位置）
- `crates/agent_settings/src/user_agents_md.rs` — 个人 AGENTS.md 加载与热重载

### System prompt 的拼接行为

#### 拼接层次结构

system prompt 的组装在 `crates/agent/src/thread.rs:4207-4246` 的 `build_request_messages_until()` 中完成。最终渲染结果按以下顺序拼接：

```
1. 核心提示文本（system_prompt.hbs 硬编码，~140 行指令）
2. 工具使用指引（条件性，取决于 available_tools）
3. 系统信息（OS、Shell、日期、工作树路径）
4. 终端沙箱说明（条件性，取决于 sandboxing 设置）
5. 模型信息（条件性，取决于 model_name）
6. Agent Skill 目录（条件性，取决于 has_skills）
7. 用户自定义指令（条件性，取决于 user_agents_md 或 has_rules）
   ├── 7a. 个人 AGENTS.md（全局，~/.config/zed/AGENTS.md）
   └── 7b. 项目规则（项目级，优先级高于全局）
```

**注意：不存在目录级别的规则合并。** 规则文件只在每个 worktree 根目录下检测，不会递归扫描子目录。

#### 规则文件搜索优先级

每个 worktree 按以下列表顺序扫描，**命中第一个即停止**（`crates/agent/src/agent.rs:1254-1298`）：

```rust
pub const RULES_FILE_NAMES: &[&str] = &[
    ".rules",
    ".cursorrules",
    ".windsurfrules",
    ".clinerules",
    ".github/copilot-instructions.md",
    "AGENT.md",
    "AGENTS.md",
    "CLAUDE.md",
    "GEMINI.md",
];
```

注意：`.rules` 指的是**单个文件**，不是目录。Cline 的 `.clinerules` 目录格式暂不支持。

#### 全局 vs 项目级合并逻辑

- **个人 AGENTS.md**（`~/.config/zed/AGENTS.md`）— 全局生效，渲染在 `User's Custom Instructions > Personal AGENTS.md` 中
- **项目规则** — 每个 worktree 最多一个规则文件，渲染在 `User's Custom Instructions > Project Rules` 中
- 模板中明确说明：项目规则优先级高于个人 AGENTS.md（"They take precedence over the personal AGENTS.md above when they conflict"）

#### 注入的系统环境信息

system prompt 中注入以下系统信息：

| 变量 | 来源 | 示例 |
|------|------|------|
| `{{os}}` | `std::env::consts::OS` | `"windows"`, `"linux"`, `"macos"` |
| `{{arch}}` | `std::env::consts::ARCH` | `"x86_64"`, `"aarch64"` |
| `{{shell}}` | `ShellKind::new()` 检测 | `"powershell"`, `"bash"`, `"zsh"` |
| `{{date}}` | `Local::now().format("%Y-%m-%d")` | `"2026-07-19"` |
| `{{model_name}}` | 当前线程模型 | `"gpt-4"`, `"claude-3"` 等 |
| `worktrees` | 项目所有可见工作树 | 绝对路径列表 |
| `sandboxing` | `sandboxing_enabled_for_project()` | `true` / `false` |

**不注入环境变量**，也没有专门的 Git 仓库元数据注入。

#### 加载的模板和指令文件（完整清单）

除了个人 AGENTS.md 外，以下文件也会被加载并注入 system prompt：

| 文件类型 | 路径 | 说明 |
|---------|------|------|
| **个人 AGENTS.md** | `~/.config/zed/AGENTS.md` | 全局自定义指令，热加载 |
| **项目规则文件** | `{worktree}/.rules` | 按 RULES_FILE_NAMES 列表扫描，命中第一个即止 |
| **项目规则文件** | `{worktree}/.cursorrules` | 同上 |
| **项目规则文件** | `{worktree}/.windsurfrules` | 同上 |
| **项目规则文件** | `{worktree}/.clinerules` | 同上 |
| **项目规则文件** | `{worktree}/.github/copilot-instructions.md` | 同上 |
| **项目规则文件** | `{worktree}/AGENT.md` | 同上 |
| **项目规则文件** | `{worktree}/AGENTS.md` | 同上 |
| **项目规则文件** | `{worktree}/CLAUDE.md` | 同上 |
| **项目规则文件** | `{worktree}/GEMINI.md` | 同上 |
| **全局 Skill** | `~/.agents/skills/<name>/SKILL.md` | 自动发现，目录摘要注入 system prompt |
| **项目本地 Skill** | `{worktree}/.agents/skills/<name>/SKILL.md` | 同上，优先级高于全局 |
| **内置 Handlebars 模板** | `crates/agent/src/templates/*.hbs` | 通过 `rust_embed` 编译进二进制 |

#### 提示模板覆盖机制（prompt_overrides）

用户可以通过在 `prompt_overrides` 目录中放置同名的 `.hbs` 文件来覆盖内置模板：

- **macOS**: `~/Library/Application Support/Zed/prompt_overrides/`
- **其他平台**: `~/.local/share/zed/prompt_overrides/`
- **开发模式**: 若仓库中有 `assets/prompts/` 目录，则优先使用该目录

`PromptBuilder::watch_fs_for_template_overrides`（`crates/prompt_store/src/prompts.rs:239-338`）监视该目录的更改，如果存在同名 `.hbs` 文件，则优先使用文件系统版本而非嵌入式版本。

这意味着理论上可以覆盖 `system_prompt.hbs`，但需要将文件放在正确的平台路径下。

#### 参考文件

- `crates/agent/src/thread.rs` — `build_request_messages_until()` 方法
- `crates/prompt_store/src/prompts.rs` — `ProjectContext`、`WorktreeContext`、`RulesFileContext`、`RULES_FILE_NAMES`
- `crates/agent/src/agent.rs` — `build_project_context()`、`load_worktree_rules_file()`
- `crates/agent/src/templates.rs` — `Templates` 初始化与模板注册
- `crates/prompt_store/src/prompts.rs` — `PromptBuilder::watch_fs_for_template_overrides()`
- `crates/paths/src/paths.rs` — `agents_file()`、`prompt_overrides_dir()`
- `crates/agent_settings/src/user_agents_md.rs` — 个人 AGENTS.md 加载

#### 简化归纳

最终发给模型的 system prompt 结构可归纳为三个部分的拼接：

```
最终 System Prompt = 1 + 2 + 3
```

| 部分 | 内容 | 是否必含 | 说明 |
|------|------|---------|------|
| **1** | 核心提示文本 | **必含** | `system_prompt.hbs` 中硬编码的指令，不随设置变化 |
| **2** | 条件性章节 | 视环境/设置 | 工具指引、沙箱说明、模型信息、Skill 目录等，由 `{{#if}}` 控制 |
| **3** | 用户自定义指令 | 视文件存在 | 全局 AGENTS.md + 项目规则文件（**两者可共存**，项目规则优先） |

**第 3 部分的详细规则：**

- 全局 AGENTS.md（`~/.config/zed/AGENTS.md`）存在即渲染，不存在则跳过
- 项目规则文件：每个 worktree 按固定列表 `.rules` → `.cursorrules` → ... → `GEMINI.md` 顺序扫描，**命中第一个文件即停止**，没有则不渲染
- 全局和项目规则**同时存在时同时渲染**，模板中明确标注项目规则优先级更高
- 多 worktree 项目：每个 worktree 独立扫描，各自最多一个规则文件

### Settings 迁移非幂等循环问题

#### 现象

settings.json 中已无任何 deprecated 设置，但每次保存后仍弹出 "Your settings file uses deprecated settings" 提示。

#### 根因

链式迁移中 `m_2025_06_16` + `m_2025_11_25` 双向抵消产生非幂等循环。

`migrate_settings()`（`migrator.rs:159-261`）按顺序链式执行所有迁移，每次迁移的输出是下一次的输入：

```
m_2025_06_16 (tree-sitter):  context_servers 中无 source 的条目 → 插入 "source"
m_2025_06_27 (tree-sitter):  source:"custom" + command 是对象 → 展平 (条件不匹配则跳过)
m_2025_11_25 (JSON):         所有 context_servers 条目 → 删除 source
```

**触发条件**：`context_servers` 中存在一个条目**同时满足**：
- 有 `command` 字段（字符串形式）
- 无 `source` 字段

例如：

```json
{
  "context_servers": {
    "playwright": {
      "enabled": true,
      "command": "C:\\Program Files\\nodejs\\npx.cmd",
      "args": ["@playwright/mcp@latest"]
    }
  }
}
```

**执行流程：**

1. `m_2025_06_16`（`migrations/m_2025_06_16/settings.rs:84-91`）检测到 playwright 条目有 `command` 但无 `source`，在 `{` 后插入 `\n    "source": "custom",`
2. `m_2025_06_27` 检查 `command` 是否为对象（嵌套 `{path, args, env}` 格式），用户的是字符串，跳过
3. `m_2025_11_25` 删除所有 context_servers 条目中的 `source` 字段

理论上双向抵消后应恢复原样。但 `update_value_in_json_text`（`settings_json.rs:8-65`）的移除逻辑存在**文本偏移偏差**：
- `replace_value_in_json_text` 基于 `current`（原地修改中的文本）计算 range
- 编辑被推入 `edits` 后应用到 `migrated_text`（`current_text` 的克隆）
- range 计算和实际文本偏移有细微偏差时，输出 ≠ 输入
- `run_migrations()`（`migrator.rs:117`）最终比较 `text != new_text`，发现不同 → 返回 `Some` → 触发提示

**每次保存的循环**：
```
保存 → should_migrate_settings() 读磁盘 → migrate_settings() 检测到差异 → 弹提示
点击 Backup and Update → write_settings_migration() 写迁移后文件 → 文件内容与原始输入仍不同
下次保存 → should_migrate_settings() 再次检测到差异 → 再次弹提示
```

#### 解决方案

**方案 A**：移除 context_servers 条目的 `command` 字段（仅保留 `enabled` / `settings`），或添加空的 `source` 字段：

```json
{
  "playwright": {
    "enabled": true,
    "source": "custom",       // 主动添加，避免迁移循环
    "command": "C:\\...",
    "args": ["@playwright/mcp@latest"]
  }
}
```

**方案 B**：使用 `workspace::SaveWithoutFormat` 命令（快捷键 `Ctrl-K Ctrl-Shift-S`）保存，避免触发写入后重新读取的检测循环。

**方案 C**：在 `~/.config/zed/settings.json` 中配合 `"format_on_save": "off"`，但仅靠此设置不足以解决问题（因为检测发生在读取时，而非格式化时）。

#### 参考文件

- `crates/migrator/src/migrator.rs` — `migrate_settings()`、`run_migrations()`、`should_migrate_settings()`、`write_settings_migration()`
- `crates/migrator/src/migrations/m_2025_06_16/settings.rs` — `migrate_context_server_settings()`，向无 `source` 的 context_servers 条目插入 `source`
- `crates/migrator/src/migrations/m_2025_06_27/settings.rs` — `flatten_context_server_command()`，展平嵌套 `command` 对象
- `crates/migrator/src/migrations/m_2025_11_25/settings.rs` — `remove_context_server_source()`，删除所有 context_servers 条目中的 `source`
- `crates/settings_json/src/settings_json.rs` — `update_value_in_json_text()`、`replace_value_in_json_text()`
- `crates/zed/src/zed/migrate.rs` — `MigrationBanner` 组件与 `should_migrate_settings()` 检测入口
- `crates/migrator/src/migrations.rs` — `migrate_settings()` 内部函数，JSON 迁移的递归执行

### Agent 编辑审核 UI

#### "Reject All / Keep All" 的意义

Agent 编辑文件后，Zed 会显示 "Reject All" / "Keep All" 按钮，出现在两个位置：

| 位置 | 控制设置 | 可否隐藏 |
|------|---------|---------|
| **编辑器内联工具栏**（单文件 diff 模式） | `agent.single_file_review`（`agent_settings.rs:229`） | 可，设为 `false` |
| **对话卡片底部**（thread view 中的编辑消息） | 硬编码，无可配置设置 | 不可，始终显示 |

编辑已保存到磁盘，但 `ActionLog`（`crates/action_log/src/action_log.rs`）维护了 `diff_base` 快照和 `unreviewed_edits` 列表：

- **Reject 不是撤销，而是基于 diff_base 的版本回退**：用 `diff_base` 中的原始内容替换 agent 写入的内容，重新保存
- **Modified 文件**：替换 agent 内容为原始内容，重新保存
- **Created 文件**：删除文件（纯 AI 创建）或恢复原始内容（有预先存在的内容）
- **Deleted 文件**：从 `diff_base` 恢复内容并保存
- **混合编辑**（用户 + AI 同时修改）：跳过拒绝，避免数据丢失（`action_log.rs` 中有 TODO 注释）
- **Undo 能力**：拒绝后可通过 `undo_reject_toast` 的 "Undo" 按钮恢复 agent 的编辑

#### 隐藏内联审核工具栏

```json
{
  "agent": {
    "single_file_review": false
  }
}
```

设为 `false` 后，agent 编辑后不会在编辑器内显示 diff 工具栏和 hunk 导航，但对话卡片底部的 "Reject All / Keep All" 按钮仍然存在。

#### 参考文件

- `crates/agent_ui/src/agent_diff.rs` — `update_reviewing_editors()`、`single_file_review` 设置读取、工具栏渲染
- `crates/agent_ui/src/conversation_view/thread_view.rs` — 对话卡片底部的 Reject/Keep 按钮
- `crates/agent_ui/src/agent_ui.rs` — `Keep`、`Reject`、`RejectAll`、`KeepAll` 动作定义
- `crates/action_log/src/action_log.rs` — `reject_all_edits()`、`keep_all_edits()`、`undo_last_reject()`、`diff_base` 管理
- `crates/agent_ui/src/ui/undo_reject_toast.rs` — 拒绝后的 Undo toast
- `crates/agent_settings/src/agent_settings.rs` — `single_file_review` 设置字段（line 229）
- `crates/settings_content/src/agent.rs` — `single_file_review` 序列化结构体（line 280）

### 工具权限系统与 YOLO mode

#### 权限判断优先级

`ToolPermissionDecision::from_input()`（`crates/agent/src/tool_permissions.rs:253-365`）按以下优先级判断：

```
1. 硬编码安全规则        → Deny（不可绕过）
2. 无效正则/命令        → Deny
3. always_deny 模式     → Deny
4. always_confirm 模式  → Confirm（弹窗）
5. always_allow 模式    → Allow
6. 工具级 default       → Allow/Deny/Confirm
7. 全局 default         → Allow/Deny/Confirm
```

#### 硬编码安全规则（不可绕过）

定义在 `tool_permissions.rs:25-63`，仅对 terminal 工具生效：

- `rm -rf /`、`rm -rf /*`、`rm -rfv /`、`sudo rm -rf /`
- `rm -rf ~`、`rm -rf ~/`、`rm -rf ~/*`
- `rm -rf $HOME`、`rm -rf ${HOME}`、`rm -rf $HOME/*`
- `rm -rf .`、`rm -rf ./`、`rm -rf ./*`
- `rm -rf ..`、`rm -rf ../`、`rm -rf ../*`
- 含路径穿越的变体（`rm -rf /tmp/../../`）
- 多路径绕过（`rm -rf /tmp /`）

**即使 `default: "allow"` + `always_allow: [{ pattern: ".*" }]` 也无法绕过。**

#### `default: "allow"` 仍然弹窗的原因

全局 `default: "allow"` 仅影响第 7 步。以下情况仍会弹窗：

1. **Shell 替换检查**（`tool_permissions.rs:273-286`）：terminal 命令中的 `$VAR`、`$(...)`、反引号被默认拦截，除非 `is_unconditional_allow_all()` 返回 `true`（即 `default: "allow"` + 无 `always_deny`/`always_confirm` 模式）
2. **工具级 `default` 覆盖**：某个工具设置了 `default: "confirm"` 覆盖了全局默认值
3. **`always_confirm` 模式**：在 profile 或 project 设置中定义了匹配模式
4. **硬编码规则**：如上所述

#### `is_unconditional_allow_all()` 函数

`tool_permissions.rs:433-443`：

```rust
fn is_unconditional_allow_all(rules: &ToolRules, global_default: ToolPermissionMode) -> bool {
    rules.always_deny.is_empty()
        && rules.always_confirm.is_empty()
        && matches!(
            rules.default.unwrap_or(global_default),
            ToolPermissionMode::Allow
        )
}
```

此函数返回 `true` 时，terminal 工具跳过 Shell 替换检查。这是最接近 "YOLO mode" 的机制，但硬编码安全规则仍然生效。

#### 最接近 YOLO 的配置

**没有完全禁用所有权限弹窗的设置。** 最接近的是：

```json
{
  "agent": {
    "tool_permissions": {
      "default": "allow",
      "tools": {
        "terminal": {
          "default": "allow",
          "always_allow": [{ "pattern": ".*" }]
        },
        "write_file": {
          "default": "allow"
        },
        "edit_file": {
          "default": "allow"
        }
      }
    }
  }
}
```

注意：`always_allow` 模式的 `".*"` 匹配所有输入，但**硬编码安全规则仍然优先**。

#### `ToolPermissionMode` 枚举

`crates/settings_content/src/agent.rs:890-902`：

```rust
pub enum ToolPermissionMode {
    Allow,    // 自动允许，不弹窗
    Deny,     // 自动拒绝，返回错误
    Confirm,  // 总是弹窗确认（默认）
}
```

#### `ToolPermissions` 完整结构

`crates/settings_content/src/agent.rs:827-876`：

```rust
pub struct ToolPermissionsContent {
    pub default: Option<ToolPermissionMode>,                              // 全局默认
    pub tools: HashMap<Arc<str>, ToolRulesContent>,                       // 按工具覆盖
}

pub struct ToolRulesContent {
    pub default: Option<ToolPermissionMode>,                              // 工具级默认
    pub always_allow: Option<ExtendingVec<ToolRegexRule>>,                // 正则匹配自动允许
    pub always_deny: Option<ExtendingVec<ToolRegexRule>>,                 // 正则匹配自动拒绝
    pub always_confirm: Option<ExtendingVec<ToolRegexRule>>,              // 正则匹配总是弹窗
}
```

#### 运行时权限解析

`crates/agent/src/thread.rs:6173-6291` — `run_authorization_loop()`：

1. 调用 `check_settings` → `ToolPermissionDecision::from_input()`
2. 返回 `Allow` → 直接放行，不弹窗
3. 返回 `Deny` → 直接返回错误
4. 返回 `Confirm` → 发送 `ToolCallAuthorization` 事件到 UI，显示权限弹窗
5. 弹窗时监视设置变化 — 如果用户修改设置允许，弹窗自动关闭

#### 参考文件

- `crates/agent/src/tool_permissions.rs` — `ToolPermissionDecision::from_input()`、`HARDCODED_SECURITY_RULES`、`is_unconditional_allow_all()`
- `crates/agent/src/thread.rs` — `run_authorization_loop()`、`build_permission_options()`
- `crates/settings_content/src/agent.rs` — `ToolPermissionMode`、`ToolPermissionsContent`、`ToolRulesContent`
- `crates/agent_settings/src/agent_settings.rs` — 编译后的 `ToolPermissions`、`ToolRules` 结构体
- `crates/migrator/src/migrations/m_2026_02_04/settings.rs` — `always_allow_tool_actions` 迁移到 `tool_permissions.default`

### Language Server 安装与加载机制

#### 安装是全局的，启动是 per-worktree 的

**安装位置**：所有 LSP 二进制文件下载到全局目录 `{data_dir}/languages/{server_name}/`。

| 平台 | 路径 |
|------|------|
| Windows | `%LocalAppData%/Zed/languages/{server_name}/` |
| macOS | `~/Library/Application Support/Zed/languages/{server_name}/` |
| Linux | `~/.local/share/zed/languages/{server_name}/` |

**安装一次，全局可用**。不需要为每个项目单独安装。

#### 二进制文件解析顺序

`crates/language/src/language.rs:790-895` 的 `get_language_server_command()` 按三步查找：

```
1. 设置中显式指定路径  → lsp.{name}.binary.path（settings.json）
2. 用户已安装的版本    → 检查 $PATH、rustup、npm global 等
3. 下载缓存版本        → {languages_dir}/{server_name}/ 中的缓存文件
```

如果三步都失败且 `allow_binary_download: true`，则自动下载最新版本。

#### 每个语言的适配器注册

LSP 适配器在 `crates/languages/src/lib.rs` 中全局注册：

- **语言绑定**：每种语言有一个或多个默认适配器（如 Rust → `rust-analyzer`，Python → `BasedPyright + Ruff + PyLsp + Pyright`）
- **全局可用**：某些适配器（`tailwindcss-language-server`、`eslint`、`vtsls`、`typescript-language-server`）注册为全局可用，不绑定到特定语言，通过 `language_servers` 设置启用

#### 每个项目/WrokTree 的 LSP 启动

**每个 worktree 启动独立的 LSP 进程**。两个项目打开 Rust 文件 → 每个 worktree 一个 `rust-analyzer` 进程（除非共享 manifest 根目录）。

启动流程：

```
用户打开文件
  → 检测语言类型
  → adapters_for_language() 按 language_servers 设置过滤
  → start_language_server()
    → 解析二进制路径（settings → PATH → 缓存 → 下载）
    → 启动 LSP 进程
    → 发送 initialize / initialized
    → 打开缓冲区
```

#### `language_servers` 设置

每个语言可配置使用的 LSP 列表（`crates/language/src/language_settings.rs:350-390`）：

```json
{
  "languages": {
    "TypeScript": {
      "language_servers": ["!typescript-language-server", "vtsls", "..."]
    }
  }
}
```

支持的语法：
- `"..."` — 扩展为所有其他注册的 LSP
- `"!server_name"` — 禁用特定服务器
- `"server_name"` — 显式启用特定服务器

默认值 `["..."]` 表示启用该语言注册的所有适配器。

#### 通过 settings.json 显式指定路径

```json
{
  "lsp": {
    "rust-analyzer": {
      "binary": {
        "path": "C:\\tools\\rust-analyzer.exe",
        "arguments": [],
        "env": {}
      }
    }
  }
}
```

显式路径会**完全绕过**下载逻辑。

#### 简化归纳

```
安装：全局一次（{data_dir}/languages/{server_name}/）
发现：按语言注册表 + language_servers 设置过滤
启动：每个 worktree 独立进程
路径：settings.binary.path → PATH 查找 → 全局缓存 → 自动下载
```

#### 参考文件

- `crates/language/src/language.rs` — `LspAdapter`、`LspInstaller` 特质，`get_language_server_command()`
- `crates/language/src/language_registry.rs` — `LanguageRegistry`，适配器注册与发现
- `crates/languages/src/lib.rs` — 所有内置语言与 LSP 适配器的注册入口
- `crates/languages/src/rust.rs` — rust-analyzer 的 `LspInstaller` 实现（GitHub 发布版下载）
- `crates/languages/src/typescript.rs` — TypeScript LSP 的 `LspInstaller` 实现（npm 安装）
- `crates/project/src/lsp_store.rs` — `LocalLspStore`，LSP 生命周期管理
- `crates/project/src/manifest_tree/server_tree.rs` — `LanguageServerTree`，每个 worktree 的 LSP 节点管理
- `crates/language/src/language_settings.rs` — `language_servers` 设置解析
- `crates/settings_content/src/project.rs` — `LspSettings`、`BinarySettings` 结构体
- `crates/paths/src/paths.rs` — `languages_dir()` 路径定义
- `assets/settings/default.json` — 每种语言的默认 `language_servers` 配置

### 编辑预测（Edit Prediction）触发机制

编辑预测（内联补全）由 `Editor::refresh_edit_prediction()`（`crates/editor/src/edit_prediction.rs:231`）驱动。

#### 触发条件

自动触发（`debounce=true`）：

| 操作 | 文件位置 |
|------|---------|
| 输入字符 | `input.rs:526` |
| 换行 | `input.rs:775` |
| 退格/删除 | `editor.rs:4961,4990` |
| 撤销/重做 | `editor.rs:7563,7588` |
| 自动补全菜单关闭 | `completions.rs:928` |
| 诊断导航 | `diagnostics.rs:194` |

手动触发（`user_requested=true`）：

| 操作 | 文件位置 |
|------|---------|
| `editor: show edit prediction`（默认 `alt-\`） | `edit_prediction.rs:333` |
| `editor: toggle edit prediction` 开启时 | `edit_prediction.rs:221` |
| 接受预测后且无活跃预测 | `edit_prediction.rs:485` |
| 部分接受后 | `edit_prediction.rs:544` |

#### 触发前必须满足的条件

`refresh_edit_prediction()`（`edit_prediction.rs:239-276`）中的守卫条件：

1. 编辑器不是多用户跟随者（`self.leader_id.is_some()`）
2. AI 未被禁用（`DisableAiSettings::is_ai_disabled_for_buffer()`）
3. 该位置未被 `edit_predictions_disabled_in` 禁用（检查只读、提供者 `is_enabled()`、文件 glob 匹配）
4. 非手动触发时额外检查：
   - `snippet_stack` 为空
   - 编辑器已聚焦
   - 缓冲区非空

#### 防抖/节流

`EditPredictionStore`（`edit_prediction.rs:2332`）：`THROTTLE_TIMEOUT = 300ms`。`queue_prediction_refresh()` 在最后请求 < 300ms 时等待剩余时间，如排队期间有新请求则丢弃旧请求。

最大待处理数：
- `Zed`/`Mercury`：最多 2 个，需接受跟踪
- `OpenAiCompatibleApi`：最多 2 个，无需接受跟踪
- `Ollama`：最多 1 个，无需接受跟踪

#### 编辑预测模式

`EditPredictionsMode`（`settings_content/src/language.rs:356-364`）：

| 模式 | 行为 | 请求频率 |
|------|------|---------|
| `"eager"`（默认） | 自动显示内联补全 | 每次输入后 300ms 防抖 |
| `"subtle"` | 自动获取但**隐藏预览**，需按修饰键才显示 | 同上，请求仍然发送 |

`Subtle` 模式的具体行为（`edit_prediction.rs:1579-1584`）：
- `preview_requires_modifier = true`
- 内联预览不显示（`edit_prediction.rs:376` — `is_visible || !requires_modifier` 为 false 时跳过跳转）
- Tab 键显示 "Preview" 而非 "Accept"（`edit_prediction.rs:647-648`）
- 菜单中仍然显示预测（`edit_prediction.rs:611`）

**无"仅手动触发、不自动请求"的模式。**

#### OpenAI 兼容模式配置

必须显式设置 `provider`，Zed 不会自动推断：

```json
{
  "edit_predictions": {
    "provider": "open_ai_compatible_api",
    "open_ai_compatible_api": {
      "api_url": "http://localhost:20102",
      "model": "local",
      "max_output_tokens": 8192,
      "prompt_format": "qwen"
    }
  }
}
```

`prompt_format` 可选值：`infer`（默认，按模型名前缀推断）、`zeta`、`codellama`、`starcoder`、`deepseek_coder`、`qwen`、`codegemma`、`codestral`、`glm`。

已知前缀自动识别：`deepseek-coder`、`qwen`、`codellama`、`starcoder`、`codegemma`、`codestral`、`glm`、`zeta2`（`edit_prediction_registry.rs:158-175`）。

API 请求格式：

```http
POST {api_url}
Authorization: Bearer {ZED_OPEN_AI_COMPATIBLE_EDIT_PREDICTION_API_KEY}
Content-Type: application/json

{
  "model": "local",
  "prompt": "...",
  "max_tokens": 8192,
  "stop": [...]
}
```

响应期望 `choices[0].text`（`open_ai_compatible.rs:123-131`）。

#### 如何验证生效

| 方法 | 操作 |
|------|------|
| **状态栏图标** | 右下角出现 `AiOpenAiCompat` 图标（`edit_prediction_button.rs:269`），无指示点 = 已启用 |
| **日志** | `zed: open log`，搜索 `"fim: completion received"`（成功）或 `"custom server error"`（失败） |
| **手动触发** | `alt-\`（`editor: show edit prediction`）绕过防抖立即请求 |
| **API 密钥** | 环境变量 `ZED_OPEN_AI_COMPATIBLE_EDIT_PREDICTION_API_KEY` 或 Zed 凭据存储（`openai-compatible-api-token`） |

#### 响应时间约束与模型选型

编辑预测对模型响应速度有严格要求，不适合推理延迟较高的模型：

| 机制 | 值 | 说明 |
|------|-----|------|
| 防抖间隔 | `THROTTLE_TIMEOUT = 300ms`（`edit_prediction.rs:2332`） | 两次请求之间至少间隔 300ms |
| 超时退避 | `REQUEST_TIMEOUT_BACKOFF`（`edit_prediction.rs:1078`） | 超时后暂停请求一段时间 |
| 日志标记 | `"fim: completion received ({:.2}s)"`（`fim.rs:119`） | 超过 1-2 秒的响应会被标记为慢 |
| 最大待处理 | `OpenAiCompatibleApi` = 2（`edit_prediction.rs:2513`） | 超出则丢弃旧请求，避免堆积 |

**为什么高级模型不适合：**

1. 300ms 防抖意味着每 300ms 可能产生一个新请求
2. 如果模型响应需要 2-10 秒，队列中会堆积大量过期请求
3. `max_pending_predictions = 2` 防止堆积，但只能拿到 2 个请求前的旧结果，光标已移动，补全不匹配当前上下文
4. 日志中 `({:.2}s)` 期望 **0.x 秒**级别的响应，超过 1 秒体验就很差

**编辑预测适合的模型特征：**
- 推理延迟 < 500ms
- 专门为 FIM（Fill-in-the-Middle）微调过的模型（如 `qwen2.5-coder`、`deepseek-coder`、`starcoder` 等）
- 量化到足够小的版本（Q4/Q3/NVFP4），能在本地 GPU 上快速推理

**API 返回 `"all keys exhausted"` 的原因：**
- `api_url` 请求路径为 `POST /v1/completions`（raw completion 格式，非 chat completions）
- 后端返回 `502 Bad Gateway` + `{"error": {"message": "all keys exhausted", "type": "proxy_error"}}`
- 说明后端是一个 API 代理网关，该模型的所有密钥已用尽
- 不代表 Zed 配置有误，请求已正常发出

#### ToggleEditPrediction 动作

`editor: toggle edit prediction`（`edit_prediction.rs:195-207`）切换当前编辑器的 `show_edit_predictions_override`：

- 当前启用 → 关闭，丢弃活跃预测
- 当前关闭 → 开启，立即手动触发一次请求

可绑定快捷键按需开关：

```json
{
  "bindings": {
    "ctrl-shift-e": "editor::ToggleEditPrediction",
    "alt-\\": "editor::ShowEditPrediction"
  }
}
```

#### 参考文件

- `crates/editor/src/edit_prediction.rs` — `refresh_edit_prediction()`、`EditPredictionsMode`、`THROTTLE_TIMEOUT`、`toggle_edit_predictions()`
- `crates/editor/src/input.rs` — 输入字符/换行时触发编辑预测
- `crates/editor/src/actions.rs` — `ToggleEditPrediction` 动作定义
- `crates/editor/src/completions.rs` — 自动补全菜单关闭时触发
- `crates/edit_prediction/src/edit_prediction.rs` — `EditPredictionStore`、`queue_prediction_refresh()`、节流逻辑
- `crates/edit_prediction/src/open_ai_compatible.rs` — `send_custom_server_request()`，API 调用与认证
- `crates/edit_prediction/src/fim.rs` — FIM 提示格式组装
- `crates/zed/src/zed/edit_prediction_registry.rs` — 提供者选择、`prompt_format` 推断
- `crates/edit_prediction_ui/src/edit_prediction_button.rs` — 状态栏按钮与图标
- `crates/settings_content/src/language.rs` — `EditPredictionSettingsContent`、`EditPredictionsMode`、`EditPredictionProvider`
- `crates/language/src/language_settings.rs` — `EditPredictionSettings` 运行时结构体

### Terminal 多 Profile 支持

#### 现状：不支持多终端 Profile

Zed 的终端**没有类似 VSCode 的多 profile 机制**（无法在 cmd/pwsh/bash 之间下拉切换）。

`Shell` 枚举（`crates/util/src/shell.rs:6-23`）仅支持单个值：

```rust
pub enum Shell {
    #[default]
    System,                            // 系统默认 shell
    Program(String),                   // 指定程序，无参数
    WithArguments {
        program: String,
        args: Vec<String>,
        title_override: Option<String>,
    },
}
```

`settings.json` 中只有一个 `terminal.shell` 字段：

```json
{
  "terminal": {
    "shell": "system"
    // 或
    "shell": { "program": "pwsh.exe" }
    // 或
    "shell": { "with_arguments": { "program": "bash", "args": ["--login"] } }
  }
}
```

**所有 new terminal 使用同一个 shell 配置**，无法在同一个面板中混合使用不同 shell。

#### "New Terminal" vs "Spawn Task" 与 Agent 终端

常规终端面板的 `+` 按钮提供两个选项：

| 选项 | 使用哪个 shell | 终端出现在哪 |
|------|---------------|-------------|
| **New Terminal** | `terminal.shell` 配置 | 常规终端面板标签页 |
| **Spawn Task** | 每个 task 独立设置的 `shell` 字段 | 常规终端面板标签页 |

**Agent 面板的 terminal 工具**则不同：它使用**系统默认 shell**（`get_default_system_shell_preferring_bash()`，Windows 上为 Git Bash 或系统 shell），且渲染在 Agent 面板内联，**不在常规终端面板**中。

三种终端来源对比：

| 维度 | 常规面板 `+` → New Terminal | 常规面板 `+` → Spawn Task | Agent 面板 `terminal` 工具 |
|------|---------------------------|--------------------------|--------------------------|
| shell 来源 | `terminal.shell` 配置 | 每个 task 的 `shell` 字段 | 系统默认 shell |
| 终端位置 | 常规终端面板标签页 | 常规终端面板标签页 | Agent 面板内联渲染 |
| 交互性 | 交互式 shell | 运行命令，`use_new_terminal: true` 可保持打开 | 运行命令，输出内联显示 |
| 可配置不同 shell | ❌ 仅一个 | ✅ 每个 task 可不同 | ❌ 固定系统默认 |

#### 变通方案：通过 Task 启动不同 Shell

Task 系统支持每个任务独立设置 `shell` 字段。可在 `tasks.json` 中定义多个任务，各自使用不同 shell：

```json
{
  "tasks": [
    {
      "label": "PowerShell",
      "command": "pwsh.exe",
      "use_new_terminal": true,
      "shell": { "program": "pwsh.exe" }
    },
    {
      "label": "CMD",
      "command": "cmd.exe",
      "use_new_terminal": true,
      "shell": { "program": "cmd.exe" }
    },
    {
      "label": "Git Bash",
      "command": "bash --login",
      "use_new_terminal": true,
      "shell": { "with_arguments": { "program": "C:\\Program Files\\Git\\bin\\bash.exe", "args": ["--login"] } }
    }
  ]
}
```

但 Task 终端本质上是**非交互式**的（运行命令后退出），虽然 `use_new_terminal: true` 可保持打开，但与纯交互式 shell 体验不同。

#### 完整终端设置清单

| 设置 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `shell` | `Shell` enum | `"system"` | shell 程序 |
| `working_directory` | enum | `"current_project_directory"` | 工作目录策略 |
| `dock` | enum | `"bottom"` | 面板停靠位置 |
| `flexible` | bool | `true` | 是否可伸缩 |
| `button` | bool | `true` | 状态栏终端按钮 |
| `env` | object | `{}` | 环境变量 |
| `font_size` | f32? | `null` | 终端字号 |
| `font_family` | string? | `null` | 终端字体 |
| `cursor_shape` | enum | `"block"` | 光标形状 |
| `blinking` | enum | `"terminal_controlled"` | 光标闪烁 |
| `copy_on_select` | bool | `false` | 选中即复制 |
| `option_as_meta` | bool | `false` | Option 键作为 Meta |
| `bell` | enum | `"off"` | 终端铃声 |
| `max_scroll_history_lines` | int | `10000` | 最大滚动行数 |
| `detect_venv` | object | 默认启用 | Python venv 检测 |
| `path_hyperlink_regexes` | array | 内置正则 | 路径超链接匹配规则 |

#### 参考文件

- `crates/util/src/shell.rs` — `Shell` 枚举、`get_system_shell()`、`ShellKind`
- `crates/util/src/shell_builder.rs` — `ShellBuilder` 将 Shell 枚举转为实际命令
- `crates/terminal/src/terminal_settings.rs` — `TerminalSettings` 运行时结构体
- `crates/terminal/src/terminal.rs` — `TerminalBuilder` 解析 Shell 配置
- `crates/terminal_view/src/terminal_panel.rs` — `NewTerminal`、`spawn_task()` 实现
- `crates/settings_content/src/terminal.rs` — `ProjectTerminalSettingsContent` 序列化结构体
- `crates/task/src/task.rs` — `SpawnInTerminal` 结构体，每个任务可独立设置 shell
- `assets/settings/default.json` — 终端默认配置

### settings.json 格式：JSONC 支持

#### 支持 JSONC（JSON with Comments）

Zed 的 `settings.json` **完全支持 JSONC 格式**，即：
- `//` 单行注释
- `/* */` 多行注释
- 尾随逗号（trailing commas）

均由 `serde_json_lenient` 解析器（`crates/settings_json/src/settings_json.rs:743-746`）容忍。

#### 注释在写入时被保留

Zed 修改 settings.json 时**不重新序列化整个文件**，而是通过 `update_value_in_json_text()` 进行**精确范围编辑**（`crates/settings_json/src/settings_json.rs:8-65`）：

```rust
// 找到要修改的 key-value 的文本范围
// 仅替换该范围的文本
// 其余内容（包括注释、格式）原样保留
```

因此注释在程序化写入后**不会被清除**。

#### 通过注释切换终端 shell 配置

**技术上可行，但需要手动操作。** 可以在 settings.json 中预设多种 shell 配置，通过注释/取消注释来切换：

```jsonc
{
  "terminal": {
    // 方案 A: PowerShell 7
    "shell": { "program": "pwsh.exe" }

    // 方案 B: Windows CMD
    // "shell": { "program": "cmd.exe" }

    // 方案 C: Git Bash
    // "shell": {
    //   "with_arguments": {
    //     "program": "C:\\Program Files\\Git\\bin\\bash.exe",
    //     "args": ["--login"]
    //   }
    // }
  }
}
```

**注意**：JSONC 中**不允许同一个 key 出现多次**，所以只能通过注释一块、取消注释另一块来切换。Zed 的 JSON 解析器（`serde_json_lenient`）遇到重复 key 时，后一个会覆盖前一个，因此不能通过"写两个 `shell` 字段，注释掉一个"之外的方式切换。

**没有运行时切换机制**——没有 UI 下拉菜单、快捷键或命令来切换终端 shell。每次切换都需要手动编辑 settings.json 并保存。

#### 参考文件

- `crates/settings_json/src/settings_json.rs` — `parse_json_with_comments()`、`update_value_in_json_text()`、注释保留测试用例
- `crates/settings_json/Cargo.toml` — 依赖 `serde_json_lenient`
- `crates/settings/src/settings_store.rs` — `new_text_for_update()`、`edits_for_update()`、`update_settings_file_inner()`
- `crates/settings/src/settings_file.rs` — `update_settings_file()` 入口
- `crates/settings_content/src/fallible_options.rs` — 用户设置解析使用 `serde_json_lenient`
- `crates/languages/src/json.rs` — JSONC 语言注册
- `crates/languages/src/lib.rs` — JSONC 在语言列表中
- `crates/grammars/src/grammars.rs` — JSONC 语法（tree-sitter-json 原生支持注释）

### Terminal 打开位置设置

#### 设置项：`terminal.dock`

在 `settings.json` 中通过 `terminal.dock` 控制终端面板的停靠位置：

```jsonc
{
  "terminal": {
    "dock": "bottom"  // 可选: "bottom" (默认), "left", "right"
  }
}
```

#### 三个可选位置

| 值 | 效果 | 默认尺寸 |
|---|---|---|
| `"bottom"` | 底部停靠（默认） | `default_height: 320px` |
| `"left"` | 左侧停靠 | `default_width: 640px` |
| `"right"` | 右侧停靠 | `default_width: 640px` |

**注意**：不支持 `"top"` 停靠，也不支持作为标签页嵌入中心编辑区。终端是唯一一个可以停在底部的面板。

#### 类型链

用户配置 → 运行时结构 → 内部枚举：

```
TerminalDockPosition (Left/Bottom/Right)  ← 用户设置
    ↓
TerminalSettings.dock                      ← 运行时解析
    ↓
DockPosition (Left/Bottom/Right)           ← 内部引擎枚举
```

#### 关键代码位置

- `assets/settings/default.json:1850-1859` — 默认值定义
- `crates/settings_content/src/terminal.rs:514-518` — `TerminalDockPosition` 枚举
- `crates/settings_content/src/terminal.rs:139` — `TerminalSettingsContent.dock` 字段
- `crates/terminal/src/terminal_settings.rs:40` — `TerminalSettings.dock` 运行时字段
- `crates/terminal_view/src/terminal_panel.rs:1529-1552` — `Panel` trait 实现（`position()` / `set_position()`）
- `crates/workspace/src/dock.rs:290-294` — `DockPosition` 内部枚举
- `crates/workspace/src/dock.rs:316-324` — `From<TerminalDockPosition> for DockPosition` 转换

#### 运行时修改位置

`TerminalPanel` 实现了 `Panel::set_position()`，当修改位置时，会**直接写回用户的 `settings.json`**，重启后持久化保留。

---

### 界面各部分位置交换

#### 架构概览：固定 3-Dock + 中心编辑器

Zed 的工作区布局是固定的，不能自由拖拽交换：

```
┌────────────────────────────────────────────────┐
│    Left Dock    │   Center (Editor)   │ Right  │
│  (Project,      │                     │ Dock   │
│   Outline,      │                     │ (Agent,│
│   Git, Collab)  │                     │ Collab)│
│                 │                     │        │
├─────────────────┴─────────────────────┴────────┤
│              Bottom Dock (Terminal)             │
└────────────────────────────────────────────────┘
```

- **中心编辑器**：固定不可移动
- **三个 Dock 容器**：`left_dock`, `bottom_dock`, `right_dock`（`crates/workspace/src/workspace.rs:2214`）
- 每个 Dock 可容纳多个 Panel，但同一时间只有一个激活

#### 各 Panel 可停靠位置

| Panel | 可停靠位置 | 代码位置 |
|---|---|---|
| Terminal | **Left / Bottom / Right**（最灵活） | `terminal_panel.rs:1540` |
| Agent | **Left / Right**（不能停 Bottom） | `agent_panel.rs:4968` |
| Project | **Left / Right** | `project_panel.rs:7583` |
| Outline | **Left / Right** | `outline_panel.rs:4971` |
| Git | **Left / Right** | `git_ui/src/git_panel.rs:8092` |
| Collab | **Left / Right** | `collab_panel.rs:3763` |
| Debugger | **任意（无限制）** | `debugger_panel.rs:1554` |

#### 交换方式：不是拖拽，而是通过设置/快捷键

**不支持拖拽 reposition**：Dock 的拖拽手柄（`dragged_dock`）仅用于**调整大小**，不能移动 Panel 到另一个 Dock（`crates/workspace/src/dock.rs:1099`）。

**支持的移动方式**：

1. **`MoveFocusedPanelToNextPosition` 动作**（`crates/workspace/src/workspace.rs:291`）
   - 循环将当前焦点 Panel 移动到下一个合法的 Dock 位置
   - 实现：`crates/workspace/src/dock.rs:127` — `PanelHandle::move_to_next_position()`
   - 遍历 `[Left, Bottom, Right]`，过滤 `position_is_valid()`，移到下一个

2. **设置面板上下文菜单**：状态栏的 Panel 按钮 → 右键菜单 → "Move to Side"

3. **直接修改 settings.json**：每个 Panel 的 `set_position()` 写入对应设置字段

#### Bottom Dock 的额外布局选项

`BottomDockLayout` 枚举控制底部 Dock 的宽度跨越方式（`crates/settings_content/src/workspace.rs:328`）：

| 值 | 效果 |
|---|---|
| `Contained`（默认） | 底部 Dock 仅在编辑器区域内，不与侧边栏齐平 |
| `Full` | 底部 Dock 全宽跨越 |
| `LeftAligned` | 底部 Dock 与左侧对齐 |
| `RightAligned` | 底部 Dock 与右侧对齐 |

设置方式：`workspace.set_bottom_dock_layout`（`crates/workspace/src/workspace.rs:2195`）

---

### 聚焦到界面各部分的直观操作

#### 一、Panel 专用 ToggleFocus 快捷键

每个主要 Panel 都有自己的 `ToggleFocus` 动作，按一次聚焦，再按一次返回编辑器：

| Panel | 动作 | macOS | Windows/Linux |
|---|---|---|---|
| Project Panel | `project_panel::ToggleFocus` | `Cmd-Shift-E` | `Ctrl-Shift-E` |
| Outline Panel | `outline_panel::ToggleFocus` | `Cmd-Shift-B` | `Ctrl-Shift-B` |
| Git Panel | `git_panel::ToggleFocus` | `Ctrl-Shift-G` | `Ctrl-Shift-G` |
| Debug Panel | `debug_panel::ToggleFocus` | `Cmd-Shift-D` | `Ctrl-Shift-D` |
| Agent Panel | `agent::ToggleFocus` | `Cmd-?` | `Ctrl-?` |
| Collab Panel | `collab_panel::ToggleFocus` | `Cmd-Shift-C` | `Ctrl-Shift-C` |
| Terminal Panel | `terminal_panel::ToggleFocus` | — | — |

**核心机制**：`Workspace::toggle_panel_focus::<T: Panel>()`（`crates/workspace/src/workspace.rs:4334`）
- 如果 Panel 未聚焦 → 聚焦到该 Panel 的 `panel_focus_handle`
- 如果 Panel 已聚焦 → 返回编辑器焦点（可选关闭 Panel，取决于 `close_panel_on_toggle` 设置）

#### 二、编辑器 Pane 导航

| 动作 | 效果 | macOS | Windows/Linux |
|---|---|---|---|
| `ActivateNextPane` | 下一个编辑区 | `Cmd-K Cmd-Right` 或 `Ctrl-W W` (Vim) | `Ctrl-K Ctrl-Right` |
| `ActivatePreviousPane` | 上一个编辑区 | `Cmd-K Cmd-Left` 或 `Ctrl-W P` (Vim) | `Ctrl-K Ctrl-Left` |
| `ActivatePane(N)` | 跳到第 N 个编辑区 | `Cmd-1` ~ `Cmd-9` | `Alt-1` ~ `Alt-9` |
| `ActivatePaneLeft/Right/Up/Down` | 方向导航 | `Cmd-K Cmd-{方向}` | `Ctrl-K Ctrl-{方向}` |
| `ActivateLastPane` | 上一个编辑区 | — | — |

Vim 模式额外支持：
- `Ctrl-W H/J/K/L` → 方向导航
- `Space W H/J/K/L` → 方向导航

#### 三、全局焦点循环：`FocusNextPart` / `FocusPreviousPart`

**最通用的方法**，按固定顺序循环聚焦所有 UI 区域：

- 动作：`workspace::FocusNextPart`（`F6`）/ `FocusPreviousPart`（`Shift-F6`）
- 实现：`Workspace::move_part_focus()`（`crates/workspace/src/workspace.rs:8222`）
- 循环顺序：Editor → 各打开的 Panel → Status Bar → Title Bar → 回到 Editor
- 系统适用：
  - macOS: `F6` / `Cmd-F6` / `Shift-F6`
  - Linux: `F6` / `Ctrl-F6` / `Shift-F6`
  - Windows: `F6` / `Ctrl-F6` / `Shift-F6`

#### 四、Terminal Panel 内部 Pane 导航

当焦点在 Terminal Panel 内时，`ActivateNextPane` 等动作会在**终端 Dock 内部的多个终端间**切换（`crates/terminal_view/src/terminal_panel.rs:1407-1431`），而不是切换编辑器 Pane。

#### 五、Debugger 子面板聚焦

Debugger Panel 内部有更精细的聚焦动作：

| 动作 | 聚焦目标 |
|---|---|
| `debugger::FocusConsole` | 控制台 |
| `debugger::FocusVariables` | 变量面板 |
| `debugger::FocusBreakpointList` | 断点列表 |
| `debugger::FocusFrames` | 调用栈 |
| `debugger::FocusModules` | 模块面板 |
| `debugger::FocusLoadedSources` | 已加载源码 |
| `debugger::FocusTerminal` | 调试终端 |

#### 六、关键代码位置

- `crates/zed_actions/src/lib.rs` — 所有 `ToggleFocus` 动作定义
- `crates/workspace/src/workspace.rs:4334` — `toggle_panel_focus()` 核心实现
- `crates/workspace/src/workspace.rs:8222` — `move_part_focus()` 全局焦点循环
- `crates/workspace/src/workspace.rs:248-364` — 所有 workspace 动作定义
- `crates/workspace/src/workspace.rs:5179-5188` — `activate_next/previous_pane`
- `assets/keymaps/default-macos.json` — macOS 默认快捷键
- `assets/keymaps/default-linux.json` — Linux 默认快捷键
- `assets/keymaps/default-windows.json` — Windows 默认快捷键
- `assets/keymaps/vim.json` — Vim 模式快捷键

---

### 全面板 Toggle/Focus 动作对照表

#### 所有实现 `Panel` trait 的面板

Zed 中共有 9 个实现了 `Panel` trait 的面板，每个都有自己的 `ToggleFocus` 动作（按一次聚焦，再按一次回到编辑器）：

| # | 面板名称 | Panel 结构体 | ToggleFocus 动作 | 默认快捷键 (macOS) | 默认快捷键 (Win/Linux) |
|---|---|---|---|---|---|
| 1 | Project Panel | `ProjectPanel` | `project_panel::ToggleFocus` | `Cmd-Shift-E` | `Ctrl-Shift-E` |
| 2 | Outline Panel | `OutlinePanel` | `outline_panel::ToggleFocus` | `Cmd-Shift-B` | `Ctrl-Shift-B` |
| 3 | Git Panel | `GitPanel` | `git_panel::ToggleFocus` | `Ctrl-Shift-G` | `Ctrl-Shift-G` |
| 4 | Debug Panel | `DebugPanel` | `debug_panel::ToggleFocus` | `Cmd-Shift-D` | `Ctrl-Shift-D` |
| 5 | Agent Panel | `AgentPanel` | `agent::ToggleFocus` | `Cmd-?` | `Ctrl-?` / `Ctrl-Shift-/` |
| 6 | Collab Panel | `CollabPanel` | `collab_panel::ToggleFocus` | `Cmd-Shift-C` | `Ctrl-Shift-C` |
| 7 | Terminal Panel | `TerminalPanel` | `terminal_panel::ToggleFocus` | **无默认快捷键** | **无默认快捷键** |
| 8 | Project Search | `ProjectSearch` | `project_search::ToggleFocus` | `Cmd-Shift-F` (部署) / `Escape` (关闭) | `Ctrl-Shift-F` (部署) / `Escape` (关闭) |
| 9 | Buffer Search | (内嵌条) | `buffer_search::Deploy` / `Dismiss` | `Cmd-F` (部署) / `Escape` (关闭) | `Ctrl-F` (部署) / `Escape` (关闭) |

**注意**：
- Terminal Panel 没有默认快捷键，可通过状态栏按钮或 Dock 级别快捷键（`Cmd-J` 切换底部 Dock）操作
- Git Panel 没有 `Toggle` 动作，只有 `ToggleFocus`
- Agent Panel 的 `agent::ToggleFocus` 有别名 `assistant::ToggleFocus`（已弃用）

#### Dock 级别动作（开关整个 Dock 容器）

| 动作 | 效果 | macOS | Windows/Linux |
|---|---|---|---|
| `workspace::ToggleLeftDock` | 开关左侧 Dock | `Cmd-B` | `Ctrl-B` |
| `workspace::ToggleRightDock` | 开关右侧 Dock | `Cmd-Alt-B` / `Cmd-R` | `Ctrl-Alt-B` |
| `workspace::ToggleBottomDock` | 开关底部 Dock | `Cmd-J` | `Ctrl-J` |
| `workspace::ToggleAllDocks` | 开关所有 Dock | `Alt-Cmd-Y` | `Ctrl-Shift-Y` (Win) / `Ctrl-Alt-Y` (Linux) |
| `workspace::CloseActiveDock` | 关闭当前焦点所在 Dock | `Cmd-W` | `Ctrl-F4` / `Ctrl-W` |
| `workspace::CloseAllDocks` | 关闭所有 Dock | — | — |
| `workspace::FocusNextPart` | 循环聚焦下一区域 | `F6` / `Cmd-F6` | `F6` / `Ctrl-F6` |
| `workspace::FocusPreviousPart` | 循环聚焦上一区域 | `Shift-F6` | `Shift-F6` |
| `workspace::MoveFocusedPanelToNextPosition` | 移动当前 Panel 到下一 Dock 位置 | **无默认快捷键** | **无默认快捷键** |

---

### 各面板聚焦后可用动作详解

#### 1. Project Panel（`context: "ProjectPanel"`）

**ToggleFocus**: `project_panel::ToggleFocus` → `Cmd-Shift-E` / `Ctrl-Shift-E`

**聚焦后可用的动作**：

| 类别 | 动作 | 快捷键 | 说明 |
|---|---|---|---|
| 导航 | `ExpandSelectedEntry` | `Right` | 展开选中条目 |
| 导航 | `CollapseSelectedEntry` | `Left` | 折叠选中条目 |
| 导航 | `ExpandAllEntries` | `Cmd-Right` | 展开所有条目 |
| 导航 | `CollapseAllEntries` | `Cmd-Left` | 折叠所有条目 |
| 导航 | `SelectParent` | — | 选择父节点 |
| 导航 | `SelectNextDirectory` | — | 选择下一个目录 |
| 导航 | `SelectPrevDirectory` | — | 选择上一个目录 |
| 导航 | `SelectNextGitEntry` | — | 选择下一个 Git 变更条目 |
| 导航 | `SelectPrevGitEntry` | — | 选择上一个 Git 变更条目 |
| 导航 | `menu::SelectNext` | `Shift-Down` | 下一个条目 |
| 导航 | `menu::SelectPrevious` | `Shift-Up` | 上一个条目 |
| 文件 | `NewFile` | `Cmd-N` | 新建文件 |
| 文件 | `NewDirectory` | `Alt-Cmd-N` | 新建目录 |
| 文件 | `Duplicate` | `Cmd-D` | 复制文件 |
| 文件 | `Rename` | `Enter` / `F2` | 重命名 |
| 文件 | `Trash` | `Backspace` / `Delete` | 移到回收站（需确认） |
| 文件 | `Trash` (skip confirm) | `Cmd-Backspace` | 直接移到回收站 |
| 文件 | `Delete` | `Cmd-Delete` / `Cmd-Alt-Backspace` | 删除（需确认） |
| 文件 | `RevealInFileManager` | `Alt-Cmd-R` | 在文件管理器中显示 |
| 文件 | `Open` | `Space`（`not_editing` 时） | 打开文件 |
| 文件 | `OpenSplitVertical` | — | 垂直分屏打开 |
| 文件 | `OpenSplitHorizontal` | — | 水平分屏打开 |
| 文件 | `workspace::CopyPath` | `Cmd-Alt-C` | 复制绝对路径 |
| 文件 | `workspace::CopyRelativePath` | `Alt-Cmd-Shift-C` | 复制相对路径 |
| 文件 | `workspace::OpenWithSystem` | `Ctrl-Shift-Enter` | 用系统默认程序打开 |
| 编辑 | `Cut` / `Copy` / `Paste` | `Cmd-X` / `Cmd-C` / `Cmd-V` | 剪切/复制/粘贴 |
| 编辑 | `Undo` / `Redo` | `Cmd-Z` / `Cmd-Shift-Z` | 撤销/重做 |
| 搜索 | `NewSearchInDirectory` | `Cmd-Alt-Shift-F` | 在当前目录新建搜索 |
| Git | `CompareMarkedFiles` | `Alt-D` | 比较标记的文件 |
| 视图 | `ToggleHideGitIgnore` | — | 切换显示/隐藏 .gitignore 文件 |
| 视图 | `ToggleHideHidden` | — | 切换显示/隐藏隐藏文件 |
| 视图 | `ScrollUp` / `ScrollDown` | — | 滚动 |
| 视图 | `ScrollCursorCenter` / `Top` / `Bottom` | — | 滚动到光标位置 |

---

#### 2. Outline Panel（`context: "OutlinePanel"`）

**ToggleFocus**: `outline_panel::ToggleFocus` → `Cmd-Shift-B` / `Ctrl-Shift-B`

**聚焦后可用的动作**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `ExpandSelectedEntry` | `Right` | 展开选中条目 |
| `CollapseSelectedEntry` | `Left` | 折叠选中条目 |
| `ExpandAllEntries` | — | 展开所有条目 |
| `CollapseAllEntries` | — | 折叠所有条目 |
| `OpenSelectedEntry` | `Space` | 打开选中条目 |
| `RevealInFileManager` | `Alt-Cmd-R` | 在文件管理器中显示 |
| `menu::SelectNext` | `Shift-Down` | 选择下一个 |
| `menu::SelectPrevious` | `Shift-Up` | 选择上一个 |
| `menu::Cancel` | `Escape` | 取消 |
| `workspace::CopyPath` | `Cmd-Alt-C` | 复制路径 |
| `workspace::CopyRelativePath` | `Alt-Cmd-Shift-C` | 复制相对路径 |
| `editor::OpenExcerpts` | `Alt-Enter` | 打开摘要 |
| `editor::OpenExcerptsSplit` | `Cmd-Alt-Enter` | 分屏打开摘要 |
| `ToggleActiveEditorPin` | — | 固定/取消固定编辑器 |
| `FoldDirectory` / `UnfoldDirectory` | — | 折叠/展开目录 |
| `ScrollUp` / `ScrollDown` | — | 滚动 |

---

#### 3. Git Panel（`context: "GitPanel"` + 子 context: `ChangesList`, `CommitEditor`, `GitBranchSelector`）

**ToggleFocus**: `git_panel::ToggleFocus` → `Ctrl-Shift-G`

**标签切换**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `git_panel::ActivateChangesTab` | `Cmd-1` | 切换到 Changes 标签 |
| `git_panel::ActivateHistoryTab` | `Cmd-2` | 切换到 History 标签 |

**ChangesList 聚焦时（`GitPanel && ChangesList && !GitBranchSelector`）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `git_panel::PreviousEntry` | `Up` | 上一个条目 |
| `git_panel::NextEntry` | `Down` | 下一个条目 |
| `git_panel::FirstEntry` | `Cmd-Up` | 第一个条目 |
| `git_panel::LastEntry` | `Cmd-Down` | 最后一个条目 |
| `CollapseSelectedEntry` | `Left` | 折叠 |
| `ExpandSelectedEntry` | `Right` | 展开 |
| `menu::Confirm` | `Enter` | 确认 |
| `git::ToggleStaged` | `Cmd-Alt-Y` / `Space` | 切换暂存状态 |
| `git::StageRange` | `Shift-Space` | 暂存选中范围 |
| `git::StageFile` | `Cmd-Y` | 暂存文件 |
| `git::UnstageFile` | `Cmd-Shift-Y` | 取消暂存文件 |
| `git::RestoreFile` | `Backspace` / `Delete`（需确认） | 还原文件 |
| `git::RestoreFile` (skip confirm) | `Cmd-Backspace` / `Cmd-Delete` | 直接还原 |
| `git_panel::FocusEditor` | `Alt-Down` / `Tab` / `Shift-Tab` | 聚焦编辑器 |
| `git_panel::ToggleFocus` | `Escape` | 切换聚焦（返回编辑器） |

**CommitEditor 聚焦时（`GitPanel && CommitEditor`）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `git::Cancel` | `Escape` | 取消提交 |
| `git::Commit` | `Cmd-Enter` | 提交 |
| `git::Amend` | `Cmd-Shift-Enter` | 修改上次提交 |
| `git::GenerateCommitMessage` | `Alt-Tab` | 生成提交信息 |
| `git_panel::FocusChanges` | `Tab` | 聚焦到 Changes 列表 |

**通用 Git 操作（`GitPanel` context）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `git::Fetch` | `Ctrl-G Ctrl-G` | 拉取 |
| `git::Push` | `Ctrl-G Up` | 推送 |
| `git::Pull` | `Ctrl-G Down` | 拉取最新 |
| `git::PullRebase` | `Ctrl-G Shift-Down` | 拉取并 rebase |
| `git::ForcePush` | `Ctrl-G Shift-Up` | 强制推送 |
| `git::Diff` | `Ctrl-G D` | 查看差异 |
| `git::RestoreTrackedFiles` | `Ctrl-G Backspace` | 还原已跟踪文件 |
| `git::TrashUntrackedFiles` | `Ctrl-G Shift-Backspace` | 删除未跟踪文件 |
| `git::StageAll` | `Cmd-Ctrl-Y` | 暂存所有 |
| `git::UnstageAll` | `Cmd-Ctrl-Shift-Y` | 取消暂存所有 |
| `git::Commit` | `Cmd-Enter` | 提交 |
| `git::Amend` | `Cmd-Shift-Enter` | 修改上次提交 |

---

#### 4. Terminal Panel（`context: "Terminal"`）

**ToggleFocus**: `terminal_panel::ToggleFocus` → **无默认快捷键**

**聚焦后可用的动作**：

| 类别 | 动作 | 快捷键 | 说明 |
|---|---|---|---|
| 编辑 | `terminal::Copy` | `Cmd-C` | 复制选中文本 |
| 编辑 | `terminal::Paste` | `Cmd-V` | 粘贴 |
| 编辑 | `terminal::PasteText` | `Ctrl-Cmd-V` | 粘贴纯文本 |
| 编辑 | `editor::SelectAll` | `Cmd-A` | 全选 |
| 搜索 | `buffer_search::Deploy` | `Cmd-F` | 在终端中搜索 |
| 终端 | `terminal::Clear` | `Cmd-K` | 清屏 |
| 终端 | `terminal::RerunTask` | `Cmd-Alt-R` | 重新运行任务 |
| 终端 | `terminal::ToggleViMode` | `Ctrl-Shift-Space` | 切换 Vi 模式 |
| 终端 | `terminal::ShowCharacterPalette` | `Ctrl-Cmd-Space` | 显示字符面板 |
| 滚动 | `terminal::ScrollLineUp` | `Shift-Up` | 向上滚动一行 |
| 滚动 | `terminal::ScrollLineDown` | `Shift-Down` | 向下滚动一行 |
| 滚动 | `terminal::ScrollPageUp` | `Shift-PageUp` / `Cmd-Up` | 向上滚动一页 |
| 滚动 | `terminal::ScrollPageDown` | `Shift-PageDown` / `Cmd-Down` | 向下滚动一页 |
| 滚动 | `terminal::ScrollToTop` | `Shift-Home` / `Cmd-Home` | 滚动到顶部 |
| 滚动 | `terminal::ScrollToBottom` | `Shift-End` / `Cmd-End` | 滚动到底部 |
| 分屏 | `pane::SplitRight` | `Cmd-D` | 向右分屏 |
| 分屏 | `pane::SplitUp` / `Down` / `Left` / `Right` | `Ctrl-Alt-方向键` | 方向分屏 |
| 新建 | `workspace::NewTerminal` | `Cmd-N` | 新建终端 |
| 内联 | `assistant::InlineAssist` | `Ctrl-Enter` | AI 内联辅助 |
| Agent | `agent::AddSelectionToThread` | `Cmd->` | 将选择添加到 Agent 线程 |
| 发送按键 | `SendKeystroke ctrl-u` | `Cmd-Backspace` | 发送 Ctrl-U（清行） |
| 发送按键 | `SendKeystroke ctrl-a` | `Cmd-Left` | 发送 Ctrl-A（行首） |
| 发送按键 | `SendKeystroke ctrl-e` | `Cmd-Right` | 发送 Ctrl-E（行尾） |
| 发送按键 | `SendKeystroke ctrl-k` | `Cmd-Delete` | 发送 Ctrl-K（删到行尾） |
| 发送文本 | `SendText` | `Alt-Delete` | 发送删除词文本 |
| 发送文本 | Terminal.app 风格词导航 | `Alt-Left` / `Alt-Right` | 按词移动 |

**Terminal Panel 内部 Pane 导航**（当焦点在 Terminal Dock 内时）：

| 动作 | 说明 |
|---|---|
| `ActivateNextPane` | 切换到终端 Dock 内的下一个终端 |
| `ActivatePreviousPane` | 切换到终端 Dock 内的上一个终端 |
| `ActivatePane(N)` | 切换到终端 Dock 内的第 N 个终端 |
| `ActivatePaneDown` | 向下方向切换终端 |

---

#### 5. Agent Panel（`context: "AgentPanel"` + 子 context: `> Markdown`, `> Terminal`）

**ToggleFocus**: `agent::ToggleFocus` → `Cmd-?` / `Ctrl-?`

**聚焦后可用的动作**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `agent::NewThread` | `Cmd-N` | 新建线程 |
| `agent::NewTerminalThread` | — | 新建终端线程 |
| `agent::OpenSettings` | `Cmd-Alt-C` | 打开 Agent 设置 |
| `agent::ToggleOptionsMenu` | `Cmd-Alt-M` | 切换选项菜单 |
| `agent::ToggleNewThreadMenu` | `Cmd-Alt-Shift-N` | 切换新建线程菜单 |
| `agent::ManageSkills` | — | 管理 Skills |
| `agent::OpenActiveThreadAsMarkdown` | — | 以 Markdown 打开当前线程 |
| `agent::IncreaseFontSize` | — | 增大字体 |
| `agent::DecreaseFontSize` | — | 减小字体 |
| `agent::ResetFontSize` | — | 重置字体大小 |
| `agent::ToggleZoom` | — | 切换缩放 |
| `agent::ToggleTerminalThreadSearch` | — | 切换终端线程搜索 |
| `agent::ReauthenticateAgent` | — | 重新认证 |
| `agent::LogoutAgent` | — | 登出 |
| `agents_sidebar::ToggleThreadSwitcher` | `Ctrl-Tab` | 切换线程切换器 |
| `agents_sidebar::ToggleThreadSwitcher` (select_last) | `Ctrl-Shift-Tab` | 切换线程切换器（选最后一个） |
| `project_panel::ToggleFocus` | `Cmd-Shift-E` | 聚焦 Project Panel |

**消息编辑器子动作**（`message_editor.rs`）：

| 动作 | 说明 |
|---|---|
| `agent::Chat` | 发送消息 |
| `agent::SendImmediately` | 立即发送 |
| `agent::ChatWithFollow` | 发送并跟进 |
| `agent::Cancel` | 取消 |
| `agent::PasteRaw` | 粘贴原始内容 |

**线程视图子动作**（`thread_view.rs`）：

| 动作 | 说明 |
|---|---|
| `agent::ToggleFastMode` | 切换快速模式 |
| `agent::ToggleThinkingMode` | 切换思考模式 |
| `agent::CycleThinkingEffort` | 循环思考力度 |
| `agent::ToggleModelSelector` | 切换模型选择器 |

---

#### 6. Collab Panel（`context: "CollabPanel"` + 子 context: `editing` / `not_editing`）

**ToggleFocus**: `collab_panel::ToggleFocus` → `Cmd-Shift-C` / `Ctrl-Shift-C`

**聚焦后可用的动作**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `collab_panel::Remove` | `Ctrl-Backspace`（`not_editing`） | 移除频道 |
| `menu::Confirm` | `Space`（`not_editing`） | 确认 |
| `collab_panel::MoveChannelUp` | `Alt-Up` | 频道上移 |
| `collab_panel::MoveChannelDown` | `Alt-Down` | 频道下移 |
| `collab_panel::OpenSelectedChannelNotes` | `Alt-Enter` | 打开选中频道笔记 |
| `collab_panel::ToggleSelectedChannelFavorite` | `Shift-Enter` | 切换收藏 |
| `collab_panel::CollapseSelectedChannel` | — | 折叠频道 |
| `collab_panel::ExpandSelectedChannel` | — | 展开频道 |
| `collab_panel::StartMoveChannel` | — | 开始移动频道 |
| `collab_panel::MoveSelected` | — | 移动选中项 |
| `collab_panel::InsertSpace` | `Space`（`editing` 时） | 插入空格 |
| `collab_panel::Secondary` | — | 次要操作 |

---

#### 7. Debugger Panel（`context: "DebugPanel"` + 子 context: `BreakpointList`, `VariableList`, `Workspace && debugger_session`）

**ToggleFocus**: `debug_panel::ToggleFocus` → `Cmd-Shift-D` / `Ctrl-Shift-D`

**调试控制（`Workspace && debugger_session`）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `debugger::Start` | — | 开始调试 |
| `debugger::Continue` | — | 继续执行 |
| `debugger::Pause` | `F6`（会话中） | 暂停 |
| `debugger::Stop` | `Shift-F5` | 停止 |
| `debugger::Restart` | — | 重启 |
| `debugger::RerunSession` | `Shift-Cmd-F5` | 重新运行会话 |
| `debugger::StepOver` | `F7` | 单步跳过 |
| `debugger::StepInto` | `Ctrl-F11` | 单步进入 |
| `debugger::StepOut` | `Shift-F11` | 单步跳出 |
| `debugger::StepBack` | — | 后退一步 |
| `debugger::Detach` | — | 分离调试器 |
| `debugger::ToggleIgnoreBreakpoints` | — | 切换忽略断点 |
| `debugger::ClearAllBreakpoints` | — | 清除所有断点 |

**子面板聚焦**：

| 动作 | 快捷键 | 聚焦目标 |
|---|---|---|
| `debugger::FocusConsole` | — | 控制台 |
| `debugger::FocusVariables` | — | 变量面板 |
| `debugger::FocusBreakpointList` | — | 断点列表 |
| `debugger::FocusFrames` | — | 调用栈 |
| `debugger::FocusModules` | — | 模块面板 |
| `debugger::FocusLoadedSources` | — | 已加载源码 |
| `debugger::FocusTerminal` | — | 调试终端 |

**断点列表（`BreakpointList`）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `ToggleEnableBreakpoint` | `Space` | 启用/禁用断点 |
| `UnsetBreakpoint` | `Backspace` | 移除断点 |
| `PreviousBreakpointProperty` | `Left` | 上一个断点属性 |
| `NextBreakpointProperty` | `Right` | 下一个断点属性 |

**变量列表（`VariableList`）**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| Collapse/Expand | `Left` / `Right` | 折叠/展开变量 |
| `EditVariable` | `Enter` | 编辑变量值 |
| `CopyVariableValue` | `Cmd-C` | 复制变量值 |
| `CopyVariableName` | `Cmd-Alt-C` | 复制变量名 |
| `RemoveWatch` | `Delete` / `Backspace` | 移除监视 |
| `AddWatch` | `Alt-Enter` | 添加监视 |

---

#### 8. Project Search（`context: "ProjectSearchBar"` / `"ProjectSearchView"`）

**ToggleFocus**: `project_search::ToggleFocus` → `Cmd-Shift-F` / `Ctrl-Shift-F`

**搜索栏聚焦时**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `project_search::ToggleFocus` | `Escape` | 关闭搜索 |
| `project_search::FocusSearch` | `Cmd-Shift-F` | 聚焦搜索框 |
| `project_search::ToggleFilters` | `Cmd-Shift-J` | 切换过滤器 |
| `project_search::ToggleAllSearchResults` | `Cmd-Shift-Enter` | 切换全选结果 |
| `project_search::ToggleReplace` | `Cmd-Shift-H` | 切换替换模式 |
| `project_search::ToggleRegex` | `Alt-Cmd-G` / `Alt-Cmd-X` | 切换正则模式 |
| `project_search::OpenTextFinder` | `Alt-Cmd-F` | 打开文本查找器 |
| `project_search::NextField` | — | 下一个输入框 |
| `project_search::SearchInNew` | `Cmd-Enter` | 在新搜索中搜索 |
| `search::PreviousHistoryQuery` | `Up` | 上一条搜索历史 |
| `search::NextHistoryQuery` | `Down` | 下一条搜索历史 |

**替换模式额外**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `search::ReplaceNext` | `Enter` | 替换下一个 |
| `search::ReplaceAll` | `Cmd-Enter` | 全部替换 |

---

#### 9. Buffer Search（`context: "BufferSearchBar"`）

**Deploy**: `buffer_search::Deploy` → `Cmd-F` / `Ctrl-F`

**搜索栏聚焦时**：

| 动作 | 快捷键 | 说明 |
|---|---|---|
| `buffer_search::Dismiss` | `Escape` | 关闭搜索 |
| `buffer_search::FocusEditor` | `Tab` | 聚焦编辑器 |
| `search::FocusSearch` | `Cmd-F` | 聚焦搜索框 |
| `search::SelectNextMatch` | `Enter` | 选择下一个匹配 |
| `search::SelectPreviousMatch` | `Shift-Enter` | 选择上一个匹配 |
| `search::SelectAllMatches` | `Alt-Enter` | 全选所有匹配 |
| `search::ToggleReplace` | `Cmd-Alt-F` | 切换替换模式 |
| `search::ToggleWholeWord` | — | 切换全词匹配 |
| `search::ToggleCaseSensitive` | — | 切换大小写敏感 |
| `search::ToggleRegex` | — | 切换正则模式 |
| `search::ToggleSelection` | `Cmd-Alt-L` | 切换选中范围搜索 |
| `search::CycleMode` | — | 循环搜索模式 |
| `search::NextHistoryQuery` | — | 下一条搜索历史 |
| `search::PreviousHistoryQuery` | — | 上一条搜索历史 |
| `search::ReplaceAll` | — | 全部替换 |
| `search::ReplaceNext` | — | 替换下一个 |

---

### 全局焦点循环（`FocusNextPart` / `FocusPreviousPart`）

**最通用的 UI 聚焦方式**，按固定顺序遍历所有主要区域：

```
Editor → 各打开的 Panel → Status Bar → Title Bar → 回到 Editor
```

- 动作：`workspace::FocusNextPart`（`F6` / `Cmd-F6`）/ `workspace::FocusPreviousPart`（`Shift-F6`）
- 实现：`Workspace::move_part_focus()`（`crates/workspace/src/workspace.rs:8222`）
- 适用于所有平台的通用聚焦方式，适合无障碍访问或键盘重度用户

---

### 关键代码位置

- `crates/zed_actions/src/lib.rs` — 所有 panel 动作命名空间定义（`project_panel`, `agent`, `git_panel`, `debug_panel` 等）
- `crates/workspace/src/workspace.rs:248-364` — 所有 workspace 级别动作定义
- `crates/workspace/src/workspace.rs:4334` — `toggle_panel_focus()` 核心实现
- `crates/workspace/src/workspace.rs:8222` — `move_part_focus()` 全局焦点循环
- `crates/workspace/src/dock.rs:36` — `Panel` trait 定义
- `crates/workspace/src/dock.rs:127` — `PanelHandle::move_to_next_position()` 面板移动
- `crates/workspace/src/dock.rs:1034` — Dock → 动作映射（`toggle_action()`）
- `assets/keymaps/default-macos.json` — macOS 默认快捷键全集
- `assets/keymaps/default-linux.json` — Linux 默认快捷键全集
- `assets/keymaps/default-windows.json` — Windows 默认快捷键全集
- `assets/keymaps/vim.json` — Vim 模式快捷键

---

## 参考资料

<!-- 待补充 -->
