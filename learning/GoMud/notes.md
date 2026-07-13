# GoMUD 学习笔记

## 项目概述
用 Go 语言从零构建终端纯文字 MUD 游戏，通过项目驱动学习 Go 核心概念。

---

## 第一幕 · 创世的火种 — Entity 基础结构体 [已完成]

### 核心知识点

#### 1. 指针接收者 vs 值接收者
```
需要修改实例状态 → 必须用指针接收者 (e *Entity)
只需读取实例状态 → 值或指针都行，但建议统一风格
一个 struct 的某些方法用了指针接收者 → 所有方法都统一用指针接收者
```

#### 2. 已实现的 Entity 结构体
```go
type Entity struct {
    Name  string
    HP    int
    MaxHP int
}

func (e *Entity) IsAlive() bool        // HP > 0
func (e *Entity) TakeDamage(dmg int)   // HP -= dmg, 不低于 0
func (e *Entity) Heal(amount int)      // HP += amount, 不超过 MaxHP
```

---

## 第二幕 · 万物初生 — Struct 组合与内嵌 [已完成]

### 核心知识点

#### 1. Go 内嵌（Embedding）机制
```
type Player struct {
    Entity     // 内嵌：不是继承，是组合
    Level int
    Exp   int
}
```

#### 2. 字段提升（Field Promotion）
- 内嵌的 `Entity` 字段（Name, HP, MaxHP）被"提升"到外层 struct
- `p.Name` 和 `p.Entity.Name` 完全等价，指向同一块内存
- 两种写法都能编译通过，简写更推荐

#### 3. 方法提升（Method Promotion）
- 内嵌的 `Entity` 方法（IsAlive, TakeDamage, Heal）被"提升"到外层 struct
- `p.TakeDamage(20)` 和 `p.Entity.TakeDamage(20)` 完全等价
- 调用提升的方法时，实际操作的是内嵌的 Entity 实例

#### 4. 三种生灵已定义
```go
type Player struct {
    Entity
    Level int
    Exp   int
}

type Monster struct {
    Entity
    AttackPower int
}

type NPC struct {
    Entity
    Dialogue string
}
```

#### 5. 调用方式选择
```go
monster.TakeDamage(40)        // 最佳：简洁，走封装方法
monster.Entity.TakeDamage(40) // 能用但冗余
monster.HP -= 40              // 最差：绕过 TakeDamage 的溢出保护
```

---

## 第三幕 · 世界的骨架 — 房间与出口系统 [待开始]

### 预告
- Room struct：Name, Description, Exits map[string]*Room
- 方法：GetExit(direction string) *Room, Describe()
- 核心思考题：为什么 Exits 的值必须是 *Room 而不是 Room？
  - 答案预告：不用指针会拷贝副本，房间之间无法互相引用，形成不了双向连接的地图

### 尚未涉及的概念（后续幕）
- [ ] Interface 实战（Usable 接口：药水/卷轴）
- [ ] 游戏循环与玩家输入（bufio.Scanner / fmt.Scanln）
- [ ] 房间内实体管理（Map 遍历、增删）
- [ ] 战斗系统（Interface 多态：Attacker 接口）
- [ ] 物品系统（Interface：Usable / Pickable）
- [ ] 存档系统（文件 I/O / JSON 序列化）

---

## 代码文件
- **路径**: `C:\OpenCodeResearch\GoMud\main.go`
- **模块名**: GoMud
- **当前状态**: 包含 Entity + Player/Monster/NPC 定义 + 测试代码（120 行）
