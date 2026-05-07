package chat

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
)

type Message struct {
	ID        uuid.UUID `json:"id"`
	SessionID uuid.UUID `json:"session_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	Content   string    `json:"message"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type Hub struct {
	mu       sync.RWMutex
	rooms    map[uuid.UUID]*Room
	rdb      *redis.Client
	repo     *Repository
	ctx      context.Context
	cancel   context.CancelFunc
}

type Room struct {
	sessionID uuid.UUID
	clients   map[*Client]bool
	mu        sync.RWMutex
	onEmpty   func()
	cancel    context.CancelFunc
}

type Client struct {
	UserID   uuid.UUID
	Username string
	TenantID uuid.UUID
	Send     chan []byte
}

func NewHub(rdb *redis.Client, repo *Repository) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	return &Hub{
		rooms:  make(map[uuid.UUID]*Room),
		rdb:    rdb,
		repo:   repo,
		ctx:    ctx,
		cancel: cancel,
	}
}

func (h *Hub) Stop() {
	h.cancel()
}

func (h *Hub) GetOrCreateRoom(sessionID uuid.UUID) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()

	if room, exists := h.rooms[sessionID]; exists {
		return room
	}

	roomCtx, roomCancel := context.WithCancel(h.ctx)
	room := &Room{
		sessionID: sessionID,
		clients:   make(map[*Client]bool),
		cancel:    roomCancel,
	}
	room.onEmpty = func() {
		h.removeRoom(sessionID)
	}
	h.rooms[sessionID] = room

	go h.subscribeRedis(roomCtx, sessionID, room)

	return room
}

func (h *Hub) Publish(ctx context.Context, msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	metrics.ChatMessagesTotal.Inc()

	channel := "chat:" + msg.SessionID.String()
	if err := h.rdb.Publish(ctx, channel, data).Err(); err != nil {
		return err
	}

	go func() {
		if err := h.repo.Insert(context.Background(), msg); err != nil {
			log.Error().Err(err).Str("session_id", msg.SessionID.String()).Msg("failed to persist chat message")
		}
	}()

	return nil
}

func (h *Hub) removeRoom(sessionID uuid.UUID) {
	h.mu.Lock()
	room, exists := h.rooms[sessionID]
	if exists {
		delete(h.rooms, sessionID)
	}
	h.mu.Unlock()

	if exists && room.cancel != nil {
		room.cancel()
	}
}

func (h *Hub) subscribeRedis(ctx context.Context, sessionID uuid.UUID, room *Room) {
	channel := "chat:" + sessionID.String()
	sub := h.rdb.Subscribe(ctx, channel)
	defer sub.Close()

	ch := sub.Channel()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			room.Broadcast([]byte(msg.Payload))
		}
	}
}

func (r *Room) AddClient(client *Client) {
	r.mu.Lock()
	r.clients[client] = true
	r.mu.Unlock()
}

func (r *Room) RemoveClient(client *Client) {
	r.mu.Lock()
	delete(r.clients, client)
	empty := len(r.clients) == 0
	r.mu.Unlock()
	close(client.Send)

	if empty && r.onEmpty != nil {
		r.onEmpty()
	}
}

func (r *Room) Broadcast(data []byte) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for client := range r.clients {
		select {
		case client.Send <- data:
		default:
		}
	}
}

func (r *Room) ClientCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.clients)
}
