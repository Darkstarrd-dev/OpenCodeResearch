# 错题本

## M1（第二幕前）：误以为内嵌后字段不再能通过 Entity 访问
- 错误：认为 Entity 嵌入后变成 Player 直接字段，不能再通过 Entity 访问
- 正确：内嵌是"提升"不是"删除"。p.HP 与 p.Entity.HP 等价且指向同一内存；方法同样被提升
- 巩固：写了字段提升 + 方法提升测试代码验证 ✅ 已掌握

## M2（第三幕）：指针与整数 0 比较
- 错误：r.Exits[direction] != 0，类型不匹配（*Room vs int）
- 正确：指针零值是 nil；检查 map key 用 v, ok := m[k] 逗号 ok 惯用法，或 m[k] != nil
- 巩固：已用逗号 ok 重写 GetExit 并理解 ✅ 已掌握

## M3（第三幕）：struct 字面量中 map 字段缺类型
- 错误：Exits: { "north": r2 }
- 正确：Exits: map[string]*Room{ "north": r2 }
- 巩固：后续代码中正确写出类型 ✅ 已掌握

## M4（第三幕）：零值概念混淆
- 错误：认为"指针类型的零值是 false""map 缺失 key 返回 0"
- 正确：每种类型有独立零值——int→0、bool→false、string→""、指针/map/slice/interface→nil。
  map 缺失 key 返回的是 value 类型的零值（map[string]*Room 缺失返回 nil，不是 0）
- 巩固：已用逗号 ok 惯用法绕开零值比较；#1 复盘已讲"为什么重要" ✅

## M5（第三幕）：误以为值拷贝的 Room 不持有对其他 Room 的引用
- 错误：认为"值拷贝的房间，copy 并不持有与其他 room 沟通的指针"
- 正确：拷贝 struct 不会拷贝其引用类型字段指向的底层数据。Room.Exits 是 map（引用类型），
  值拷贝的 Room 仍通过同一份底层 map 引用其他房间
- 巩固：已纠正；current 用指针的真正理由是"共享可变状态 + 效率"；#3 复盘再强化（slice/map 是引用类型）

## M6（第四幕前）：接口 append 取地址理由误解
- 错误：认为 append 写 &Potion 是"避免复制两份新的装入"
- 正确：无论 &Potion 还是 Potion 存进接口都会拷贝进接口内部。能否存入只看方法集：
  Use 用指针接收者时，只有 *Potion 的方法集含 Use，故必须 &Potion{...}；写 Potion{} 会编译报错
- 巩固：已讲解方法集规则，待实现时验证

## M7（概念巩固 #3）：误以为不能对 nil slice 直接 append
- 错误：认为 `var s []int` 后不能直接 append（答"不行"）
- 正确：append 对 nil slice 安全，会分配底层数组；Act4 背包 `var bag []Usable; bag = append(bag, ...)` 正是靠此，无需先 make
- 巩固：已纠正 ✅
