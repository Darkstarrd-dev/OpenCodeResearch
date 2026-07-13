以下是四个 agent 基于 `agent.ts` 权限定义的对比表（生效结果，`*` 通配 + 各 agent 覆盖项合并后；带 `*`/`?` 表示按路径/条件区分）：

| 工具 / 权限域 | build | plan | explore | general |
|---|---|---|---|---|
| read（含 `*.env` 询问） | allow | allow | allow | allow |
| edit（write / edit / apply_patch） | allow | deny（仅 `.opencode/plans/*.md` 与全局 plans `*.md` 允许） | deny | allow |
| glob | allow | allow | allow | allow |
| grep | allow | allow | allow | allow |
| list | allow | allow | allow | allow |
| bash（shell） | allow | allow | allow | allow |
| task（派生子 agent） | allow | deny（general 拒绝） | deny | allow |
| external_directory | ask（白名单 allow） | ask（白名单 + plans 允许） | ask（白名单 allow） | ask（白名单 allow） |
| todowrite | allow | allow | deny | deny |
| question | allow | allow | deny | deny |
| webfetch | allow | allow | allow | allow |
| websearch | allow | allow | allow | allow |
| lsp | allow | allow | deny（`*`:deny） | allow |
| skill | allow | allow | deny（`*`:deny） | allow |
| doom_loop（循环护栏） | ask | ask | ask | ask |
| plan_enter（进入计划态） | allow | deny | deny | deny |
| plan_exit（退出计划态） | deny | allow | deny | deny |

说明：
- build/plan/general 的基集 `*`=allow，未列出的工具默认 allow；explore 的 `*`=deny，未列出的默认 deny。
- `external_directory` 的 `ask` 指非白名单目录需询问，白名单（truncate 临时目录、skills、references 等）直接 allow。
- read 对所有 agent 都是 `*`:allow，但 `*.env` / `*.env.*` 类文件为 ask（plan/explore 同此规则）。
- 最终还要叠加用户 `cfg.permission`（可进一步收紧或放开）。
