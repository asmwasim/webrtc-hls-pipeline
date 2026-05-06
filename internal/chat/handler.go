package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"

	"github.com/asmwasim/webrtc-hls-pipeline/internal/auth"
	"github.com/asmwasim/webrtc-hls-pipeline/internal/metrics"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type incomingMessage struct {
	Content string `json:"message"`
	Type    string `json:"type"`
}

func HandleWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		claims := auth.GetClaims(r.Context())
		if claims == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Error().Err(err).Msg("websocket upgrade failed")
			return
		}

		userID, _ := uuid.Parse(claims.UserID)
		tenantID, _ := uuid.Parse(claims.TenantID)

		client := &Client{
			UserID:   userID,
			Username: claims.Username,
			TenantID: tenantID,
			Send:     make(chan []byte, 64),
		}

		room := hub.GetOrCreateRoom(sessionID)
		room.AddClient(client)
		metrics.WebSocketConnections.Inc()

		go writePump(conn, client)
		go readPump(conn, client, hub, sessionID)
	}
}

func readPump(conn *websocket.Conn, client *Client, hub *Hub, sessionID uuid.UUID) {
	defer func() {
		room := hub.GetOrCreateRoom(sessionID)
		room.RemoveClient(client)
		metrics.WebSocketConnections.Dec()
		conn.Close()
	}()

	conn.SetReadLimit(4096)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var incoming incomingMessage
		if err := json.Unmarshal(data, &incoming); err != nil {
			continue
		}

		if incoming.Content == "" {
			continue
		}

		msgType := incoming.Type
		if msgType == "" {
			msgType = "message"
		}

		msg := &Message{
			ID:        uuid.New(),
			SessionID: sessionID,
			TenantID:  client.TenantID,
			UserID:    client.UserID,
			Username:  client.Username,
			Content:   incoming.Content,
			Type:      msgType,
			CreatedAt: time.Now().UTC(),
		}

		if err := hub.Publish(context.Background(), msg); err != nil {
			log.Error().Err(err).Msg("failed to publish chat message")
		}
	}
}

func writePump(conn *websocket.Conn, client *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.Close()
	}()

	for {
		select {
		case msg, ok := <-client.Send:
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func HandleHistory(repo *Repository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := uuid.Parse(chi.URLParam(r, "sessionID"))
		if err != nil {
			http.Error(w, `{"error":"invalid session id"}`, http.StatusBadRequest)
			return
		}

		before := time.Now().UTC()
		if b := r.URL.Query().Get("before"); b != "" {
			if t, err := time.Parse(time.RFC3339, b); err == nil {
				before = t
			}
		}

		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil {
				limit = n
			}
		}

		messages, err := repo.GetHistory(r.Context(), sessionID, before, limit)
		if err != nil {
			http.Error(w, `{"error":"failed to fetch history"}`, http.StatusInternalServerError)
			return
		}

		if messages == nil {
			messages = []*Message{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}
}
