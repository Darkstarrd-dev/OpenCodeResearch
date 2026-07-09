---
# ═══════════════════════════════════════════════════════════════
# OpenCode Slash Command 完整配置模板（含问答详解）
# ═══════════════════════════════════════════════════════════════
#
# 定义方式支持三种：
#   1. JSON 配置：opencode.jsonc 中 command 字段
#   2. Markdown 文件：.opencode/command/<name>.md（文件名即命令名）
#   3. MCP 自动注册：MCP 服务器的 prompts 自动成为命令
#
# 注：同名命令可覆盖内置命令（如 /init, /review）

# ─────────────────────────────────────────────
# 必填字段
# ─────────────────────────────────────────────

# template 是发送给 LLM 的 prompt 模板。
# 注意：它并不是"原封不动"发送给 LLM 的，而是经过以下预处理流程：
#   1. 参数替换：$1, $2, $ARGUMENTS 替换为用户输入的实际值
#   2. Bash 注入：!`command` 模式执行命令并捕获输出替换回模板
#   3. 文件引用：@filepath 模式读取文件内容嵌入模板
#   4. 最终拼接后的文本才发送给 LLM
template: ""

# ─────────────────────────────────────────────
# 可选字段
# ─────────────────────────────────────────────

# description 在 TUI 中显示给用户看。
# 用户在 TUI 按 Ctrl+P 或输入 / 时弹出命令面板，
# description 显示在命令名旁边作为辅助说明。
description: ""

# agent 指定此命令由哪个 agent 执行。
# 即使当前会话用的是其他 agent，此命令也会临时切换到指定 agent 执行。
# 执行完后自动切回当前会话的 agent。
# 如果不指定，则使用当前会话的默认 agent。
# 切换的是 agent 配置（system prompt、可用工具、temperature 等），
# 不是永久改变会话的 agent。
agent: ""

# model 覆盖此命令使用的模型，优先级最高。
# 模型优先级（从高到低）：
#   1. 命令的 model 字段（最高）
#   2. 命令指定 agent 的默认 model
#   3. 调用时传入的 model
#   4. 会话最近使用的 model
# 注意：model 和 agent 不绑定，可只指定一个。
model: ""

# subtask 强制以 subagent 模式运行。
# 行为逻辑：
#   - 不设此字段（默认）：如果 agent 的 mode 是 "subagent" 则自动 subtask，
#     否则在主会话中执行
#   - true：强制在独立子会话中运行，不污染主上下文
#   - false：强制不在子会话中运行（即使 agent 是 subagent 模式）
#
# 典型场景：长分析、复杂任务、不希望结果塞满主会话上下文时设为 true
#
# 常见问题：subtask 内部能不能再派发 subagent？
# 答：可以。subtask 只控制命令本身是否在子会话中执行，
# 不影响子会话内 LLM 的 task tool 调用能力。
# LLM 可以自主调用 task tool 继续派发 subagent，形成嵌套：
#   主会话 → /analyze (subtask:true)
#       └─ subagent A
#            └─ task tool → subagent B
# 每层 subagent 都是独立会话，父级只能看到最终结果。
# 推荐嵌套 2~3 层以内。
subtask: false

# ─────────────────────────────────────────────
# 模板正文（下列内容不属于 frontmatter，属于 template）
# ─────────────────────────────────────────────

# 模板中支持以下特殊语法：

# ── 参数系统 ──
# $ARGUMENTS — 替换为用户输入的所有原始参数文本
#   例如：/cmd foo bar → $ARGUMENTS = "foo bar"
# $1, $2, $3 ... — 位置参数
#   例如：/cmd a b c → $1=a, $2=b, $3=c
# 关键行为：
#   - 最后一个 $N 会吞掉所有剩余参数，用空格拼接
#   - 如：模板中只有 $1 $2 两个占位符，用户输入 /cmd a b c d
#     → $1=a, $2=b c d（因为 $2 是最后一个）
#   - 参数数量不足时，缺失的占位符替换为空字符串
#   - 参数支持双引号/单引号包裹：
#     /cmd "hello world" foo → 第一个参数是 "hello world"（作为一个整体）

# ── Bash 命令注入 ──
# 语法：!`command`
# 作用：在模板中执行 bash 命令，捕获 stdout 输出，替换到此处
# 处理流程：
#   1. 正则匹配所有 !`...` 模式
#   2. 所有匹配命令通过 Promise.all 并行执行
#   3. 全部完成后，将输出文本替换回模板
#   4. 最终拼接后的 prompt 才发送给 LLM
#
# 例子：
#   !`git log --oneline -5`
#   !`npm test 2>&1 | tail -20`
#   !`opencode run --agent build --prompt "analyze src/main.ts"`
#
# 问答汇总：
#
# Q: bash 执行完的结果会作为 prompt 给当前命令指定的 agent 推理吗？
# A: 是的。bash 输出替换回模板后，和其他文本一起组成最终 prompt
#    发送给 agent。bash 不是替代 LLM，而是给 LLM 提供实时数据。
#
# Q: 可否设置 bash command 的 timeout？
# A: 不能。源码中用的是 Bun Shell 的 $ 函数，没有传入 timeout 参数，
#    也没有任何 timeout 配置暴露给用户。命令会无限等待直到完成。
#
# Q: 不设置 timeout 的话，是否等待所有 bash 都有返回后才完成 prompt 拼接？
# A: 是的。Promise.all 会阻塞等待所有命令完成，然后才替换模板、发送给 LLM。
#    整个 command() 函数是 async 的，上游用 await 等待。
#
# Q: 用户点击"停止"按钮能否中断正在执行的 bash？
# A: 结果会被丢弃（HTTP 请求被中止），但正在执行的 bash 子进程
#    不会自动 kill，可能成为孤儿进程继续在后台运行到自然结束。
#    因为 Bun Shell 的 $ 没有传 AbortSignal。
#
# Q: bash 中打开 opencode TUI 能否控制 TUI？
# A: 不能。原因有三：
#    1. !`cmd` 是非交互式捕获，没有 TTY，TUI 依赖 TTY 无法初始化
#    2. 即使启动，stdin/stdout 被 $ 接管，用户无法交互
#    3. 两个 TUI 不能嵌套，会争夺同一个终端的控制权
#
# Q: 用 !`opencode run --agent xxx --prompt "yyy"` 变相调用其他 agent 可行吗？
# A: 理论上可行，但存在风险：
#    - 无 timeout，如果卡住则整个 slash command 死锁
#    - 阻塞主进程，期间 TUI 无响应
#    - 如果 opencode 又调用了 slash command，可能形成循环
#    - agent 的完整输出可能很大，撑爆 token 窗口
#    推荐做法：用 template 引导 LLM 用 task tool 自主 dispatch subagent，
#    框架原生支持、有超时控制、错误处理和结果返回，比 bash 绕道安全得多。

# ── 文件引用 ──
# 语法：@filepath
# 作用：自动读取文件内容嵌入 prompt
# 例子：@src/components/Button.tsx
# 注意：路径相对于项目根目录，支持相对路径

# ── 能力边界总览 ──
# ✅ 自定义 prompt 模板
# ✅ 注入参数（$ARGUMENTS, $1, $2...）
# ✅ 执行 bash 并注入输出（!`cmd`）
# ✅ 引用文件内容（@filepath）
# ✅ 指定执行 agent
# ✅ 覆盖模型
# ✅ 强制 subagent
# ✅ 跨项目共享（~/.config/opencode/command/）
# ✅ 子目录命名空间（command/foo/bar.md → /foo/bar）
# ✅ 覆盖内置命令（同名覆盖）
# ✅ MCP 自动注册
# ❌ 条件/循环逻辑（纯模板，无编程能力）
# ❌ 命令内调用另一个命令（无 @command:xxx 或 $CALL:xxx 语法）
# ❌ 多 agent 并行 subtask 后汇总（需引导 LLM 用 task tool 自主 dispatch）
# ❌ bash timeout 配置
# ❌ 自动 kill 残留 bash 子进程
# ❌ TTY/TUI 交互（!`cmd` 是非交互式文本捕获）
# ❌ 直接返回 bash 输出给用户（bash 输出嵌入 prompt，LLM 二次处理）
# ❌ 多步骤工作流（单次 prompt 发送）
---

# 在这里写你的 prompt 模板内容
# 示例：
# 分析以下代码变更：
# !`git diff HEAD~1`
# 重点关注：$ARGUMENTS
# 代码文件：@src/main.ts