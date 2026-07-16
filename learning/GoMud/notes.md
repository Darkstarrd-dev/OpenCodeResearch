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

## 历史脉络教学（2026-07-16 会话起）

### #1 变量与内存【已点亮】
- **起源**：1950s 之前，程序员靠插线/拨开关设定数据；汇编用 `MOV AX, 5` 但寄存器极少且名字写死
- **痛点**：人记不住内存地址，程序变大后无法管理
- **解决**：1957 Fortran 引入"变量"——给内存块起人能看懂的名字，编译器负责名字→地址的映射
- **核心洞察**：变量最初只解决一件事——可读性、可维护性。后面所有特性都是叠加的设计选择
- **叠加层**：
  - 类型（1960s）→ 决定占用空间 + 解读方式（同一块内存按 int vs string 读出不同结果）
  - 对齐 padding（1970s+）→ CPU 按 8 字节格子读，编译器在字段间塞空白对齐，牺牲空间换性能
  - struct 字段按声明顺序排列，编译器可能自动重排（Go 社区经验：按大小从大到小排可省 padding）
- **string 底层结构**（Go 设计）：
  - 16 字节 = 指针(8) + 长度(8)，实际字符数据存在别处（紧凑，不对齐凑整）
  - 指针前导零省略（0x7ff6... 实际是 0x00007ff6...）
  - 长度存的是**字节数**不是字符个数；`len(string)` 返回字节数 O(1)；`utf8.RuneCountInString` 才数字符
  - 不可变：不能 `s[0]='x'`，`s += "x"` 是创建新 string
  - **不需要 cap**：因为不可变、不能 append；对比 slice 有 cap 是因为可 append 需要知道剩余空间
  - vs C 的 `\0` 结尾：Go 用指针+长度，O(1) 求长、内容可含任意字节；C 要遍历 O(n)、不能含 `\0`
- **rune**：本质 int32，一个人类可见的 Unicode 字符；汉字 UTF-8 占 3 字节，英文 1 字节，emoji 4 字节
- **unsafe.Sizeof**：返回类型占用的字节数（含 padding）；`unsafe.Offsetof` 返回字段在 struct 中的偏移量

### #2 控制流 if/for【已点亮】
- **起源**：ENIAC（1945）没有代码，程序=物理接线（电缆+拨开关）
  - 数据：20个累加器，每个10位十进制（BCD编码，每位0-9用4个二进制位表示），无小数点
  - 计算：电缆连线决定哪个累加器的数据流入算术单元
  - 循环：Master Programmer 部件，计数器+电脉冲回传实现重复
  - 条件判断：比较器电路检查符号位，正/负触发不同电脉冲走不同线路
  - ENIAC 用途：计算火炮弹道表，比人工快约1400倍（30秒 vs 12小时/条弹道）
- **存储程序（1945 冯·诺依曼）**：代码也存内存，跟数据放一起，CPU 逐条读取执行
  - 指令指针（PC）= ENIAC Master Programmer 的精神继承，记住"执行到第几条"
  - 译码器 = 固定电路，把机器码数字翻译成电路通断（0001→LOAD通路，0010→ADD通路...）
  - 本质没变：仍然是开关电路储存状态、电平做判断，只是从"物理接线"变成"内存里的数字控制电路"
- **机器码**：纯数字指令，CPU 直接读懂（如 0001 0001 = LOAD R1）
- **汇编语言（1950s）**：给机器码起助记符（LOAD/SUB/JZ/JMP），汇编器翻译成机器码
  - 条件跳转 = ENIAC 符号位检查的直接继承：CPU 有标志位（ZF零/SF符号/CF进位），运算后自动设置
  - `if hp <= 0` 编译后 = 减法 → 检查 SF/ZF → 条件跳转
- **输入介质演化**：拨开关 → 打孔卡片/纸带（光电传感器读孔，有孔=1无孔=0）→ 磁带/磁鼓 → 内存(RAM)
  - 打孔卡片最早用于织布机（1725 Bouchon / 1804 Jacquard），IBM 从1890年用于计算机
  - 每一步都是同一方向：更快输入、更方便改、更密集存；底层永远是二值
- **结构化编程（1968 Dijkstra）**：论文"Go To Considered Harmful"，用三种基本结构替代任意跳转
  - ① 顺序 ② 选择(if) ③ 循环(for) — 不需要 JUMP 地址，编译器算跳转目标
- **Go 的极简设计**：只有 `for` 一个循环关键字，覆盖 while/无限循环/do-while/range 四种形式
  - `for cond { }` = while（先判断后执行，最少0次）
  - `for { body; if !cond { break } }` = do-while（先执行后判断，最少1次）
  - `for { }` = 无限循环
  - `for k,v := range m { }` = 遍历
  - 设计哲学：能简化就简化，一个关键字能做的事不用两个（C 砍 elseif，Go 砍 while/do-while）
- **演化链**：ENIAC插线 → 存储程序+机器码 → 汇编 → 高级语言 if/for；每层不变的是底层开关电路，变的是人控制电路的方式从物理操作→文字表达

### #3 函数【已点亮】
- **起源**：1950s 汇编时代，同样逻辑写10遍只换数字，改一处要改10处
- **抽象本质**：把重复指令打包 + 起名字 + 传不同数据进去复用
- **机器码层面实现**：
  - CALL 指令：把返回地址压入栈，跳到函数地址
  - RET 指令：从栈弹出返回地址，跳回调用者
  - 函数栈帧：返回地址 + 参数副本 + 局部变量 + 预留返回值空间
  - 嵌套调用：后调用的先返回 = 后进先出(LIFO) = 栈
- **栈的便签模型**：
  - 调用函数 = 在楼梯间贴便签（压栈）
  - 函数返回 = 撕便签（弹栈）
  - 后贴的先撕 = LIFO
  - 返回值 = 撕便签前把结果抄到上一张便签的预留格子
- **值类型参数**：传进去的是拷贝（副本），改副本不影响原值
- **演化链**：重复指令 → 打包成函数 → CALL/RET + 栈帧 → 高级语言的 func

### #4 多返回值 & 命名返回值【已点亮】
- **痛点**：函数经常需要同时返回"结果"+"状态"（值是否存在、是否出错、是否有余数）
- **各语言对比**：
  - C：用指针参数塞结果（`f(a,b,&q,&r)`）→ 容易出错
  - Java：包装对象（`new Result(a,b)`）→ 太重
  - Python：返回元组 → 灵活但类型不安全
  - Go：栈上直接多开格子 → 类型安全 + 零开销
- **Go 多返回值底层**：调用者预留 N 个格子，函数返回时填 N 个值，跟单返回值唯一的区别就是多填格子
- **命名返回值**：`(q int, r int)` = 函数开头就在栈帧上开好格子，函数体内可直接赋值，裸 return 自动填回
  - 未命名：`func f() (int, int)` → 必须 `return a, b`
  - 命名：`func f() (q int, r int)` → 可以 `q=a; r=b; return`（裸 return）
- **`_` 空白标识符**：丢弃不需要的返回值，函数仍返回所有值，只是调用者选择不接收
- **逗号 ok 模式**：Go 标准库最经典的多返回值应用——用第二个返回值消除歧义
  - map 读取 `v, ok := m[k]`：ok 区分"值是零值"和"key 不存在"
  - 类型断言 `v, ok := i.(T)`：ok 表示接口是否是目标类型
  - channel 接收 `v, ok := <-ch`：ok 表示 channel 是否关闭
  - 全部是同一思路：值 + 状态，两个返回值消除歧义
- **Go 为什么能做多返回值**：无历史包袱 + Ken Thompson（C/Unix 创造者之一）懂底层痛点
  - Go 设计哲学：不造新概念，用最直接的底层机制解决问题
