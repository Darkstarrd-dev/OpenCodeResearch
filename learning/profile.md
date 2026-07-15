# 学习者画像

- 角色：Go 语言学习者
- 当前项目：终端纯文字 MUD 游戏（GoMud，模块 C:\OpenCodeResearch\GoMud）
- 已掌握：变量、if/for 控制流、Slice/Map、函数、指针(&/*)、Struct、Method、组合(内嵌)、Interface(鸭子类型)
- 偏好：中文交流；沉浸式 MUD / 跑团氛围教学；每次只聚焦一个具体模块；不直接给全代码，需自己填核心逻辑
- 诊断等级：入门稳健级
- 近期薄弱点（2026-07-14 观察）：
  - 各类型的零值（曾误以为指针零值是 false、map 缺失返回 0）
  - 方法集规则（值类型 T 仅含值接收者方法；*T 含值+指针接收者方法）
  - 值拷贝 vs 引用类型（误以为值拷贝的 struct 不持有其 map 字段指向的底层数据）
  - 曾误以为不能对 nil slice 直接 append（实际安全）
- 进度：
  - 已完成 第一幕(Entity) / 第二幕(组合) / 第三幕(Room+移动)
  - 第四幕(Usable 接口)已讲解概念、暂缓实现
  - 2026-07-14 起做"从头过一遍 Go 概念"巩固：#1 变量/零值【已点亮】、#2 if/for【已点亮】、#3 Slice/Map【进行中，array vs slice 与 error vs panic 已讲】
