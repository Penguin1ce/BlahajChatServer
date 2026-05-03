# Kafka 基础知识

> 项目里实际用到的 Kafka 知识点收敛在这里。不是 Kafka 全功能手册，只覆盖"看本项目代码 + 面试聊 Kafka"够用的那部分。

## 一、核心概念

### Topic / Partition / Offset

| 概念 | 解释 |
|---|---|
| **Topic** | 消息按主题分类的逻辑队列，类似一张表名 |
| **Partition** | Topic 内的分区，是真正的存储单元和并发单元；同一 partition 内消息严格有序，跨 partition 不保证 |
| **Offset** | partition 内每条消息的递增序号；consumer 通过 commit offset 记录"已经读到哪了" |

```
Topic: chat.events
├── partition-0  msg(0,1,2,3,...)
├── partition-1  msg(0,1,2,3,...)
└── partition-2  msg(0,1,2,3,...)
```

**关键事实**：Kafka 的"有序"是 partition 级别的，不是 topic 级别的。要保证一组消息有序，必须让它们落到同一个 partition。

### Broker / Cluster / KRaft

- 一个 Kafka 进程是一个 **Broker**
- 多个 Broker 组成 **Cluster**
- 每个 partition 可以有多副本（leader + follower），leader 故障时 follower 顶上
- ≥ 3.0 版本默认 **KRaft 模式**（不再依赖 ZooKeeper），节点同时担任 controller 和 broker

## 二、Producer 端

### 写入路径

```
Producer ──▶ partition leader ──▶ 落盘 ──▶ 同步到 ISR(in-sync replicas) ──▶ ack
```

### 必须知道的几个参数

| 参数 | 取值 | 说明 |
|---|---|---|
| **`acks`** | `0` | 不等 ack，吞吐最高，可能丢 |
| | `1` | leader 落盘就 ack，leader 故障会丢 |
| | `all`（推荐） | 所有 ISR 副本都落盘才 ack |
| **`Key`** | `[]byte` | 决定消息进哪个 partition；同 Key → 同 partition |
| **`Balancer`** | `Hash` | Key 不为空时按 hash 选 partition（顺序保证依赖这个） |
| | `RoundRobin` | Key 为空时轮询 |
| | `Sticky`（默认） | 同一批消息黏在同分区，提升吞吐 |
| **`retries`** | `int` | 重试次数；与幂等 producer 配合避免重复 |
| **`enable.idempotence`** | `bool` | 单 partition 内 producer 严格不重复（用 PID + seq） |

### 顺序保证的链条

**同 Key → 同 partition → partition 内有序 → consumer 按 offset 读 → 最终对消费者也有序**

任何一环断了顺序就不保证。比如：
- 用了非 Hash balancer → 同 Key 也会乱跑
- 多个 producer 同时往同 partition 写 → 网络延迟可能让先发的后到（不开幂等的话）

## 三、Consumer 端

### Consumer Group

- 多个 consumer 实例可以组成同一个 **Group**
- **同 Group 内**：每个 partition 只会被分配给一个 consumer 处理（Kafka 来负载均衡）
- **不同 Group**：互不影响，各自独立从某个 offset 消费

```
Topic 有 3 个 partition
                                          
Group-A（2 个 consumer）            Group-B（1 个 consumer）
├── consumer-1 ◀── partition-0     consumer-1 ◀── partition-0
│              ◀── partition-1                ◀── partition-1
└── consumer-2 ◀── partition-2                ◀── partition-2

(同一个 partition 的消息，Group-A 和 Group-B 各自都会消费一遍)
```

### Offset 提交策略

| 策略 | 行为 | 风险 |
|---|---|---|
| **auto-commit**（默认） | 定期把"读到"的 offset 提交 | 读到但还没处理就提交 → crash 后**丢消息** |
| **manual commit** | 业务处理完后手动提交 | 处理完但 commit 失败 → 重启后**重复消费** |

业内主流做法：**manual commit + 业务幂等**，对应 at-least-once 语义。

### Rebalance

- Group 成员加入/退出（包括 crash、扩缩容、心跳超时）会触发 partition 重新分配
- 期间整个 Group 暂停消费
- 频繁 rebalance 是性能杀手，常见原因：消费太慢导致心跳超时、`max.poll.interval.ms` 设置不当

## 四、投递语义

| 语义 | 实现方式 | 用什么场景 |
|---|---|---|
| **at-most-once** | 处理前先 commit | 容忍丢、但不能重（统计、采样） |
| **at-least-once** | 处理后再 commit + 业务幂等 | 大多数业务（消息推送、订单事件） |
| **exactly-once** | 事务 producer + 事务 consumer | 资金、计费等强一致场景，复杂度高 |

记住：**Kafka 自己最自然支持的是 at-least-once**，业务层做幂等比追求 exactly-once 划算得多。

## 五、Kafka 适合做什么

| 场景 | 为什么合适 |
|---|---|
| 流量削峰 | producer 写入快，consumer 按自己的速率消化 |
| 异步解耦 | 上游不知道下游是谁，下游可以多个 |
| 事件总线 / 广播 | 多 Group 全量消费同一 Topic，每个消费者拿到全量 |
| 日志聚合 | 业务日志 → Kafka → ES / 监控 / 数仓 |
| Change Data Capture | binlog → Kafka → 下游索引/缓存/数仓更新 |

## 六、和其它 MQ 的对比

| | Kafka | RabbitMQ | Redis Pub/Sub |
|---|---|---|---|
| 持久化 | 默认落盘 | 可配置 | 不持久化 |
| 消费者离线 | 上线后从 offset 续 | queue 留着等 | 直接丢 |
| 强项 | 高吞吐、顺序、回放 | 路由灵活、低延迟 | 极简、无状态广播 |
| 弱项 | 路由能力弱 | 吞吐不如 Kafka | 不可靠 |

## 七、运维相关（简要）

- **Topic 自动创建**：`auto.create.topics.enable=true`，开发方便，生产建议关掉显式建
- **副本因子**：`replication.factor`，生产至少 3，单机开发用 1
- **分区数**：建好之后只能加不能减；**预估好同 Key 的并发**，分区太少会成为瓶颈
- **保留时长**：`retention.ms` 默认 7 天；超过会被删除（不是消费过就删，是按时间）
- **消费滞后（lag）**：`current offset - committed offset`，监控的核心指标

## 八、推荐阅读

- 官方文档：<https://kafka.apache.org/documentation/>
- 《深入理解 Kafka》朱忠华（中文）
- Confluent 博客：<https://www.confluent.io/blog/>
