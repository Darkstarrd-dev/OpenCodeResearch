package main

import "fmt"

// Entity —— 万物之基，所有生灵共享的生命属性
type Entity struct {
    Name  string
    HP    int
    MaxHP int
}

// IsAlive —— 判断该实体是否还活着
// 💡 思考：这个方法只读取不改，但为了风格统一，我们用指针接收者
func (e *Entity) IsAlive() bool {
    // TODO: 你来实现
    // 返回 true 如果 e.HP > 0，否则 false
    return e.HP > 0
}

// TakeDamage —— 扣减生命值，HP 不能低于 0
func (e *Entity) TakeDamage(dmg int) {
    // TODO: 你来实现
    // 提示：
    //   1. e.HP -= dmg
    //   2. 但如果 e.HP < 0，要把 e.HP 设为 0
    //   你可以用 if 判断，也可以想想有没有更简洁的写法
    e.HP -= dmg
    if e.HP < 0 { e.HP = 0 }


}

// Heal —— 恢复生命值，HP 不能超过 MaxHP
func (e *Entity) Heal(amount int) {
    // TODO: 你来实现
    // 提示：
    //   1. e.HP += amount
    //   2. 但如果 e.HP > e.MaxHP，要把 e.HP 设为 e.MaxHP
    e.HP += amount
    if e.HP > e.MaxHP { e.HP = e.MaxHP}
}

type Player struct{
	Entity
	Level int
	Exp int
}



type Monster struct {
    Entity         // 内嵌
    AttackPower int
}

type NPC struct {
    Entity          // 内嵌
    Dialogue string
}

func main() {
    // 测试你的代码
    hero := &Entity{Name: "勇者", HP: 100, MaxHP: 100}

    hero.TakeDamage(30)
    fmt.Printf("%s 受到攻击！当前 HP: %d/%d\n", hero.Name, hero.HP, hero.MaxHP)

    hero.TakeDamage(80)
    fmt.Printf("%s 受到致命攻击！当前 HP: %d/%d\n", hero.Name, hero.HP, hero.MaxHP)
    fmt.Printf("是否存活: %v\n", hero.IsAlive())

    hero.Heal(50)
    fmt.Printf("%s 饮下药水！当前 HP: %d/%d\n", hero.Name, hero.HP, hero.MaxHP)
    fmt.Printf("是否存活: %v\n", hero.IsAlive())

    // 在 main 函数中添加：
    p := &Player{
        Entity: Entity{Name: "勇者", HP: 100, MaxHP: 100},
        Level: 1,
        Exp:   0,
    }

    fmt.Println("--- 字段提升测试 ---")
    fmt.Printf("p.Name        = %s\n", p.Name)
    fmt.Printf("p.Entity.Name = %s\n", p.Entity.Name)
    fmt.Printf("两者相同吗？%v\n", p.Name == p.Entity.Name)

    fmt.Println("--- 方法提升测试 ---")
    p.TakeDamage(25)
    fmt.Printf("p.HP = %d (通过提升的字段)\n", p.HP)
    fmt.Printf("p.Entity.HP = %d (通过内嵌字段)\n", p.Entity.HP)
    fmt.Printf("p.IsAlive() = %v\n", p.IsAlive())

    b := &Player{
        Entity: Entity{Name: "勇者", HP: 100, MaxHP: 100},
        Level: 1,
        Exp:   0,
    }

    m := &Monster{
        Entity: Entity{Name: "哥布林", HP: 50, MaxHP: 50},
        AttackPower: 15,
    }

    m.TakeDamage(40)
    fmt.Printf("%s 攻击 %s,造成40点伤害\n", b.Name,m.Name)
    fmt.Printf("%s 当前 HP: %d/%d\n", m.Name, m.HP, m.MaxHP)
    fmt.Printf("是否存活: %v\n", m.IsAlive())
    b.TakeDamage(m.AttackPower)
    fmt.Printf("%s 反击 %s！造成 %d 点伤害\n", m.Name, b.Name, m.AttackPower)
    fmt.Printf("%s 当前 HP: %d/%d\n", b.Name, b.HP, b.MaxHP)
    fmt.Printf("是否存活: %v\n", b.IsAlive())

    c := &NPC {
    	Entity: Entity{Name: "村长", HP: 999, MaxHP: 999},
     	Dialogue: "欢迎来到新手村，年轻的勇者啊。",
    }
    fmt.Printf("%s: %s\n", c.Name, c.Dialogue)

}
