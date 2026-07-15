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

type Room struct {
    Name        string
    Description string
    Exits       map[string]*Room   // key 是方向 "north"/"south"/"east"/"west"
}


// 返回指定方向的房间指针，如果该方向没有出口，返回 nil
func (r *Room) GetExit(direction string) *Room {
	_, ok := r.Exits[direction]
	if ok {return r.Exits[direction]}
	return nil
}
// 打印房间的名字、描述、以及所有可用出口
func (r *Room) Describe() {
    // TODO: 你来实现
    // 提示：用 for range 遍历 r.Exits

    // 输出格式示例：
    //   【村庄广场】
    fmt.Println(r.Name,"\n")
    //   这里是新手村的中心，人来人往。
    fmt.Println(r.Description,"\n")

    line := ""
    for k := range r.Exits{ line += k+" "}
    //   出口：north south east
    fmt.Println("出口: ", line)
}

func main() {

r2 := &Room{
	Name: "西边房间",
	Description: "一个乱七八糟的仓库",
}
r3 := &Room{
	Name: "东边央房间",
	Description: "酒馆",
}
r4 := &Room{
	Name: "南边央房间",
	Description: "停尸间",
}



r1 := &Room{
	Name: "中央房间",
	Description: "大家聚集的地方",
	Exits: map[string]*Room{
		"west": r2,
		"east": r3,
		"south": r4,
	},
}
r1.Describe()

// === 行走试炼 ===
current := r1   // current 的类型是 *Room（指针）
fmt.Println("\n=== 行走试炼 ===")
fmt.Printf("你站在：%s\n", current.Name)

// TODO 1：尝试向北走（r1 没有 north 出口）
//   用 current.GetExit("north") 获取下一个房间
//   如果返回 nil → 打印 "北边是一堵墙，无法通行。"
//   否则 → 让 current 指向新房间，并打印 "你移动到了：xxx"
if current.GetExit("north") == nil { fmt.Println("北边是一堵墙，无法通行。")}

// TODO 2：尝试向西走（r1 有 west 出口，通向 r2 "西边房间"）
//   同样用 GetExit；成功后 current 指向 r2，打印移动信息
//   然后调用 current.Describe() 查看新房间
if current.GetExit("west") != nil { current = current.GetExit("west")}
current.Describe()

// TODO 3（思考，写代码后回答）：
//   为什么 current 必须是 *Room（指针），而不是 Room（值）？
//   如果 current 是 Room 值类型，移动后，原来的 r1 本身会跟着变吗？
}
