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
- #3 Slice/Map 深入（2026-07-15 会话新增）：
  - **slice 共享底层数组**：从同一 array 切出的多个 slice 看同一块内存；改一个 slice 的元素，array 和其他重叠区域的 slice 跟着变
  - **append 两种命运**：
    - cap 够 → 原地往后填，指针不动，**可能踩到相邻 slice 的数据**（共享底层的危险）
    - cap 不够 → 分配新数组（cap<256 翻倍，≥256 增 25%），拷贝旧数据，指针跳走，和老家分道扬镳
    - 所以 `bag = append(bag, x)` 必须赋值回 bag（指针可能变）
  - **切片语法 `a[low:high]`**：左闭右开，取 low 到 high-1；len=high-low；cap=从切点到底层数组末尾的格子数
  - **三段切片 `a[low:high:max]`**：可手动限制 cap=max-low；或用 `make([]T, len, cap)` 指定
  - **len vs cap**：len=现在装了多少；cap=最多能装多少不搬家；cap 是 append 的缓冲区，有缓冲区就不用每次分配新数组
  - **Map 从头梳理**：
    - 三种声明：字面量 `map[string]int{...}`（可写）、`make(map[string]int)`（可写）、`var m map[string]int`（nil，可读不可写，写入 panic）
    - 基本操作：写入 `m[k]=v`、读取 `m[k]`、删除 `delete(m,k)`、长度 `len(m)`
    - 逗号 ok 惯用法：`v, ok := m[k]`；ok 只回答"key 在不在"，不管 value 是 nil 还是有值
    - `m["north"]=nil` 算加了一个元素（key 存在，value=nil），ok=true；未写入的 key ok=false
    - make：内置函数，只用于 slice/map/channel，帮底层结构搭好；array/struct 不需要 make（值类型声明即分配）
  - **指针类型 `*T` 从头梳理**：
    - `*T` 在类型位置 = "指向 T 的指针"这个类型名（存地址），不是操作
    - `*变量` 在变量前 = 解引用操作（取值）；`&变量` = 取地址操作
    - `var r Room` = Room 本人（全字段归零，不是 nil）；`var rp *Room` = 空白纸条（nil）
    - `rp = &r` = 抄下 r 的地址；`*rp` = 拨打号码找到 r 本人；`rp.Name` = Go 自动解引用
    - nil 是指针的零值（一根没系任何东西的线），不是"非指针"
    - struct 零值不是 nil，是全字段归零的实例；`r == nil` 编译报错（值类型不能和 nil 比）
  - **方法接收者规则**：
    - 只有两种：值接收者 `(e Entity)` vs 指针接收者 `(e *Entity)`，无第三种变体
    - 选哪种：要改字段→必须指针；不改→看 struct 大小，大用指针省拷贝，小无所谓
    - Go 社区习惯：同一 struct 的方法统一用指针（避免混用踩方法集坑、省拷贝、未来改字段不用改签名）
    - 值接收者适合：小且不可变的 struct（如数学坐标 Point），纯计算不改字段
    - 方法集规则：`(e Entity)` 方法属于 Entity 和 *Entity（*e 自动解引用）；`(e *Entity)` 方法属于 *Entity，Entity 调时 Go 自动加 &（前提值能取地址，临时值不行）
    - 混用值/指针接收者的 struct 塞进接口可能编译报错（第四幕会踩到）
