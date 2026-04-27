# conversation 与 user_conv 的区别

`conversation` 和 `user_conv` 是聊天系统里两张职责不同的表：

- `conversation` 存会话本身的客观信息。
- `user_conv` 存某个用户和这个会话之间的关系，以及这个用户自己的状态。

可以简单记成：

```text
conversation = 这个房间本身
user_conv    = 某个用户在这个房间里的会员卡 + 个人设置
```

## conversation：会话本身

`conversation` 表描述的是所有成员看到都一样的事实。

例如一个群聊会话：

```text
conv_id     = conv_1
type        = group
name        = 周末去哪玩
avatar      = xxx
owner_id    = 1001
last_msg_id = msg_100
last_msg_at = 2026-04-27 20:30:00
```

这些字段表示：

- 这个会话是什么类型：单聊还是群聊。
- 群聊的名称、头像、群主是谁。
- 这个会话最后一条消息是哪条。
- 这个会话最后活跃时间是什么时候。

不管 A、B、C 哪个成员打开这个会话，`conversation` 里的这些客观信息都是一样的。

## user_conv：用户在会话里的个人状态

`user_conv` 表描述的是“某个用户”和“某个会话”的关系。

如果一个群聊有 A、B、C 三个成员，那么 `conversation` 只有一行，但 `user_conv` 会有三行：

```text
uid = 1001, conv_id = conv_1, unread = 0, pinned = true,  muted = false
uid = 1002, conv_id = conv_1, unread = 5, pinned = false, muted = false
uid = 1003, conv_id = conv_1, unread = 2, pinned = false, muted = true
```

这些字段表示：

- 这个用户是不是会话成员。
- 这个用户在该会话有多少未读。
- 这个用户是否置顶了该会话。
- 这个用户是否对该会话开启了免打扰。
- 这个用户读到了哪一条消息。

这些状态是每个用户独立的。A 置顶会话，不影响 B；C 开免打扰，也不影响 A 和 B。

## 发消息时两张表怎么配合

用户发一条消息时，通常会同时影响三类数据：

1. `messages` 新增一条消息。
2. `conversation.last_msg_id` 和 `conversation.last_msg_at` 更新，让会话列表能显示最新消息。
3. `user_conv.unread` 给除发送者以外的成员加 1。

也就是说：

- `messages` 记录消息本体。
- `conversation` 记录会话最新状态。
- `user_conv` 记录每个成员自己的未读状态。

## 拉会话列表时怎么用

客户端拉会话列表时，通常会把 `conversation` 和 `user_conv` 关联起来：

- 从 `conversation` 拿会话名称、头像、类型、最后消息。
- 从 `user_conv` 拿当前登录用户的未读数、置顶、免打扰、已读位置。

所以列表页展示的是两张表组合后的结果：会话的公共信息，加上“我自己”的个人状态。

