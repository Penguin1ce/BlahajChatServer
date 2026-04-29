package ws

import (
	"sync"

	"BlahajChatServer/internal/zlog"
)

// Envelope 是 Hub 内部投递单元：已序列化好的帧 + 目标用户列表。
// 目标为空视为无效包（暂不支持全局广播）。
type Envelope struct {
	Targets []uint64
	Data    []byte
}

type Hub struct {
	clients    map[uint64]map[*Client]struct{}
	register   chan *Client
	unregister chan *Client
	broadcast  chan *Envelope
	mu         sync.RWMutex
}

// GlobalHub 由 InitHub 赋值，handler / service 层共享一个实例。
var GlobalHub *Hub

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[uint64]map[*Client]struct{}),
		register:   make(chan *Client, 64),
		unregister: make(chan *Client, 64),
		broadcast:  make(chan *Envelope, 256),
	}
}

// InitHub 在 main 里调用：初始化全局 Hub 并起一个 Run goroutine。
func InitHub() {
	GlobalHub = NewHub()
	go GlobalHub.Run()
}

// Run 负责串行处理注册/注销/广播，避免 clients map 的并发写。
// 直接 SendToUser 走的是 RLock + 写 send chan，与 Run 互不阻塞。
func (h *Hub) Run() {
	for {
		select {
		case c := <-h.register:
			// 添加客户端
			h.addClient(c)
		case c := <-h.unregister:
			// 注销客户端
			h.removeClient(c)
		case env := <-h.broadcast:
			if env == nil {
				continue
			}
			for _, uid := range env.Targets {
				h.SendToUser(uid, env.Data)
			}
		}
	}
}

func (h *Hub) addClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		set = make(map[*Client]struct{})
		h.clients[c.userID] = set
	}
	set[c] = struct{}{}
	zlog.Infof("Hub 注册连接 uid=%d conn=%s online=%d", c.userID, c.connID, len(set))
}

func (h *Hub) removeClient(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.clients[c.userID]
	if !ok {
		return
	}
	if _, exists := set[c]; !exists {
		return
	}
	delete(set, c)
	if len(set) == 0 {
		delete(h.clients, c.userID)
	}
	c.close()
	zlog.Infof("Hub 注销连接 uid=%d conn=%s remain=%d", c.userID, c.connID, len(set))
}

// Register 由 handler 在 Upgrade 成功后调用。
func (h *Hub) Register(c *Client) {
	h.register <- c
}

// Unregister 由 readPump/writePump 退出时调用。重复调用安全。
func (h *Hub) Unregister(c *Client) {
	if c.closed.Load() {
		return
	}
	h.unregister <- c
}

// Broadcast 把一份已经序列化好的帧投递给一组用户。
// broadcast 队列满时降级为异步直投，避免业务层直接操作裸 channel。
func (h *Hub) Broadcast(env *Envelope) bool {
	if h == nil || env == nil || len(env.Targets) == 0 || len(env.Data) == 0 {
		return false
	}
	select {
	case h.broadcast <- env:
	default:
		zlog.Warnf("Hub broadcast 缓冲满 targets=%d", len(env.Targets))
		go h.SendToUsers(env.Targets, env.Data)
	}
	return true
}

// SendToUser 把同一份数据投递给该用户所有在线连接。
// send 通道满视为该连接落后太多，踢掉（异步 Unregister）。
func (h *Hub) SendToUser(uid uint64, data []byte) {
	h.mu.RLock()
	set := h.clients[uid]
	if len(set) == 0 {
		h.mu.RUnlock()
		return
	}
	targets := make([]*Client, 0, len(set))
	for c := range set {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		select {
		case c.send <- data:
		default:
			zlog.Warnf("下发 send 缓冲已满，踢掉 uid=%d conn=%s", c.userID, c.connID)
			go h.Unregister(c)
		}
	}
}

// SendToUsers 批量下发。
func (h *Hub) SendToUsers(uids []uint64, data []byte) {
	for _, uid := range uids {
		h.SendToUser(uid, data)
	}
}

// OnlineCount 在线用户数（按 uid 去重）。调试用。
func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
