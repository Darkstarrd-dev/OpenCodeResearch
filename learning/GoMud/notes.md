# GoMud 学习笔记

## 第一幕：创世的火种 — Entity 基础
- 定义 `Entity{Name, HP, MaxHP}`
- 指针接收者实现 `IsAlive() / TakeDamage() / Heal()`
- 关键：需要修改字段 → 指针接收者；溢出保护（HP 不低于 0、不超过 MaxHP）

## 第二幕：万物初生 — 组合(内嵌)
- `Player / Monster / NPC` 内嵌 `Entity`
- 字段提升 & 方法提升：`p.HP` 与 `p.Entity.HP` 等价且指向同一内存；`p.TakeDamage()` 直接可用
- 战斗场景模拟：玩家 vs 哥布林（AttackPower 反击）

## 第三幕：世界的骨架 — Room（已完成）
- `Room{Name, Description, Exits map[string]*Room}`
- `GetExit(direction) *Room`：逗号 ok 惯用法，无出口返回 nil
- `Describe()`：遍历 Exits 拼接出口字符串
- 行走试炼：`current *Room` 在房间间移动
- 关键事实：Exits 用指针 *Room 形成引用关系；nil map range 安全；current 用指针=共享可变状态+效率

## 第四幕：万物的交互 — Usable 接口（已讲解，暂缓实现）
- `type Usable interface { Use(target *Entity) string }`
- `Potion`（治疗药水，有 Charges）与 `Scroll`（火焰卷轴）用指针接收者实现 Use
- 背包 `[]Usable`：接口切片
- 方法集规则：值类型 T 仅含值接收者方法；*T 含值+指针接收者方法

## 概念巩固复盘（2026-07-14 会话：从头过一遍 Go）
- #1 变量/零值【已点亮】：
  - 五种零值：int→0, bool→false, string→"", 指针→nil, map→nil
  - 为什么重要：①安全（不读内存垃圾）②nil 是"无"的天然哨兵（GetExit 返回 nil=墙）
    ③int 零值 0 让"刚造出 HP 未设=死"天然成立（IsAlive）④nil map 可读不可写（Describe 安全；写入 panic）⑤struct 全字段自动归零
  - 旁注：C 局部变量不初始化是"刻意哲学"（信任程序员/不为未要求的付费），代价是未初始化 bug；Go 反过来默认安全
  - 旁注：nil 源自拉丁文 nihil（nothing），经 ALGOL/Lisp 入计算机；非 "Not In List"
- #2 if/for【已点亮】：
  - `for k := range m` 拿 key；`for k,v := range m` 拿 key+value；`_` 是空白标识符（占位不用）
- #3 Slice/Map【进行中】：
  - slice 零值 nil，len=0 cap=0；**可对 nil slice 直接 append（安全）**——Act4 背包 `var bag []Usable; append` 依赖此
  - error vs panic：error=预期可恢复失败（返回 error 值，if err!=nil 处理）；
    panic=意外不可恢复崩溃（打印调用栈，如 nil map 写入/越界/空指针）；recover() 在 defer 接住
  - array `[N]T`：长度固定且是类型一部分；值类型，赋值/传参整体拷贝；元素可改；元素类型固定（同质），
    但元素类型可为任意类型（含 slice/map/struct/数组）；混合类型用 `any`(=interface{}) 需类型断言
  - slice `[]T`：引用类型，头(指针+len+cap)共享底层数组；赋值/传参只拷头
  - 关系：slice 是 array 的视图；array 是 slice 的地基
