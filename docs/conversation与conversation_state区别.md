# conversation、group_info 与 conversation_state 的区别

这三张表现在按职责拆开：

- `conversations` 存会话本身的公共信息。
- `group_info` 存群聊专属信息，以及群成员 uid 快照。
- `conversation_state` 存某个用户对某个会话的个人状态。

可以简单记成：

```text
conversations      = 这个聊天房间本身
group_info         = 群聊房间的群资料 + 成员名单
conversation_state = 某个用户看到这个房间时的个人状态
```

## conversations：会话本身

`conversations` 表描述所有成员看到都一样的事实：

- `conv_id`
- `type`：`c2c` 或 `group`
- `peer_key`：单聊双方 uid 的去重键
- `last_msg_id`
- `last_msg_at`

C2C 成员关系来自 `peer_key`，例如 `1001_1002` 表示这个单聊属于用户 1001 和 1002。

## group_info：群聊信息和群成员

群聊成员不再由 `conversation_state` 判断，而是放在 `group_info.members`：

```json
[1001, 1002, 1003]
```

所以群聊成员判断和消息 fanout 的来源是：

```text
group_info.members
```

成员变更时，必须同时维护：

1. `group_info.members`
2. `group_info.member_count`
3. 对应用户的 `conversation_state`

这三类数据要放在同一个事务里更新，避免出现“人在群里但没有会话状态”或“退出群了但会话列表还在”的脏状态。

## conversation_state：用户个人状态

`conversation_state` 不表示成员关系，只表示某个用户对某个会话的个人状态：

```text
uid = 1001, conv_id = conv_1, unread = 0, pinned = true, muted = false
uid = 1002, conv_id = conv_1, unread = 5, pinned = false, muted = false
```

这些字段表示：

- 这个用户在该会话有多少未读。
- 这个用户是否置顶了该会话。
- 这个用户是否开启免打扰。
- 这个用户读到了哪一条消息。

## 发消息时怎么配合

用户发一条消息时：

1. `dao.IsMember` 判断成员资格。
   - C2C 看 `conversations.peer_key`
   - 群聊看 `group_info.members`
2. `messages` 新增一条消息。
3. `conversations.last_msg_id` 和 `conversations.last_msg_at` 更新。
4. `conversation_state.unread` 给除发送者外的会话状态加 1。
5. `dao.ListMembers` 取成员列表做 fanout。

也就是说：

- `messages` 记录消息本体。
- `conversations` 记录会话最新公共状态。
- `group_info` 记录群成员。
- `conversation_state` 记录每个用户自己的未读和已读状态。

## 拉会话列表时怎么用

客户端拉会话列表时，先查当前用户的 `conversation_state`，拿到自己有哪些会话状态，再批量查 `conversations` 拼公共信息。

列表页展示的是：

```text
会话公共信息 + 我的个人状态
```
