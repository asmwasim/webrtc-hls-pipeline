# Architecture

## System Overview

WebRTC-HLS-Pipeline is a self-hosted live streaming microservice designed as the first component of a SaaS LMS platform. Teachers stream from their browser or mobile device via WebRTC; the server transcodes the feed in real time into adaptive-bitrate HLS; students watch through a standard HLS player.

```
┌──────────────┐     WHIP/SDP      ┌──────────────────────────────────────┐
│  Teacher UI  │ ─────────────────▶ │             Go Server                │
│  (Browser)   │ ◀───────────────── │                                      │
│  getUserMedia│     SDP Answer     │  ┌──────┐  ┌────────┐  ┌─────────┐  │
└──────┬───────┘                    │  │ WHIP │  │Transcode│  │   HLS   │  │
       │                            │  │Handler│─▶│ Manager │─▶│ Handler │  │
       │ WebRTC (H.264 + Opus)      │  └──────┘  └────┬───┘  └────┬────┘  │
       │                            │                  │           │       │
       ▼                            │           FFmpeg │    .m3u8  │ .ts   │
┌──────────────┐                    │           subprocess    segments     │
│  Pion WebRTC │ ──RTP──▶ stdin ───▶│                  │           │       │
│  (server)    │                    │  ┌──────┐  ┌─────┴──┐  ┌────┴────┐  │
└──────────────┘                    │  │ Chat │  │Recording│  │ Metrics │  │
                                    │  │ Hub  │  │ Worker  │  │         │  │
                                    │  └──┬───┘  └────┬───┘  └─────────┘  │
                                    └─────┼───────────┼───────────────────┘
                                          │           │
                               ┌──────────┼───────────┼──────────┐
                               │          ▼           ▼          │
                               │  ┌─────────┐  ┌───────────┐    │
                               │  │  Redis   │  │ PostgreSQL│    │
                               │  │ Pub/Sub  │  │           │    │
                               │  └─────────┘  └───────────┘    │
                               └─────────────────────────────────┘

┌──────────────┐     HLS (HTTP)     ┌──────────────────────────────────────┐
│  Student UI  │ ◀───────────────── │  /hls/{sessionID}/master.m3u8       │
│  (HLS.js)    │                    │  /hls/{sessionID}/stream_*.ts       │
│              │ ◀──── WebSocket ──▶│  /api/v1/sessions/{id}/chat         │
└──────────────┘                    └──────────────────────────────────────┘
```

## Data Flow

### 1. Stream Ingestion (WHIP)

1. Teacher opens browser, grants camera/mic access via `getUserMedia`
2. Browser creates an SDP offer and POSTs it to `POST /api/v1/sessions/{id}/whip`
3. Server creates a Pion PeerConnection, sets the remote description, creates an SDP answer
4. ICE candidates are gathered; the completed answer is returned as `201 Created`
5. WebRTC connection is established; Pion receives H.264 video and Opus audio RTP tracks

### 2. Transcoding Pipeline

1. When both video and audio tracks are ready, the transcode manager spawns an FFmpeg subprocess
2. H.264 RTP packets are extracted via `pion/webrtc/v4/pkg/media/h264writer` and piped to FFmpeg's stdin
3. Audio RTP packets are drained (read and discarded) to prevent WebRTC backpressure — audio encoding via a separate FFmpeg input is planned for a future iteration
4. FFmpeg produces 3 ABR renditions:
   - **720p** at 2500 kbps
   - **480p** at 1000 kbps
   - **360p** at 500 kbps
5. Output: HLS segments (2-second `.ts` files) + per-variant `.m3u8` playlists + `master.m3u8`
6. Segments are written to `./segments/{session_id}/`
7. Live HLS window keeps the last 5 segments; older segments are deleted by FFmpeg's `delete_segments` flag

### 3. HLS Delivery

1. Student opens the viewer page and fetches `GET /api/v1/sessions/{id}/watch` to get the playlist URL
2. HLS.js loads `master.m3u8`, discovers available renditions, and begins adaptive playback
3. The Go server serves `.m3u8` and `.ts` files with appropriate cache headers:
   - Playlists: `no-cache` (must refetch for live edge)
   - Segments: `max-age=3600` (immutable once written)

### 4. Real-time Chat

1. Client opens a WebSocket connection to `/api/v1/sessions/{id}/chat`
2. Messages are published to Redis Pub/Sub channel `chat:{session_id}`
3. Each server instance subscribes to active session channels and fans out to local WebSocket clients
4. Messages are persisted to PostgreSQL asynchronously
5. Chat history is available via `GET /api/v1/sessions/{id}/chat/history` (cursor-based pagination)
6. When the last client disconnects from a room, the room is cleaned up and the Redis subscription is cancelled

### 5. Recording & VOD

1. When a stream ends (PeerConnection disconnects), the recording worker is enqueued
2. All variant playlists are rewritten with `#EXT-X-ENDLIST` for instant VOD playback
3. Background worker concatenates the highest-quality variant segments (`stream_0_*.ts`) into a single MP4 via FFmpeg with `-movflags +faststart`
4. Recording status and MP4 URL are tracked in the `recordings` table
5. A `recording.ready` event is published to Redis on completion

## Database Schema

```
┌──────────────────┐     ┌────────────────────┐     ┌──────────────────┐
│     sessions     │     │   chat_messages     │     │    recordings    │
├──────────────────┤     ├────────────────────┤     ├──────────────────┤
│ id          UUID │◀────│ session_id    UUID  │     │ id          UUID │
│ tenant_id   UUID │     │ tenant_id     UUID  │     │ session_id  UUID │──▶ sessions.id
│ teacher_id  UUID │     │ user_id       UUID  │     │ tenant_id   UUID │
│ title       TEXT │     │ username      TEXT  │     │ status      TEXT │
│ status      TEXT │     │ message       TEXT  │     │ mp4_url     TEXT │
│ started_at  TS   │     │ type          TEXT  │     │ created_at  TS   │
│ ended_at    TS   │     │ created_at    TS    │     │ completed_at TS  │
│ hls_url     TEXT │     └────────────────────┘     └──────────────────┘
│ mp4_url     TEXT │
│ created_at  TS   │
│ updated_at  TS   │
└──────────────────┘
```

All tables include `tenant_id` for multi-tenant isolation.

## Authentication & Authorization

- JWT (HS256) tokens carry `tenant_id`, `user_id`, `username`, and `role` claims
- `Authenticate` middleware validates the token and injects claims into the request context
- `RequireRole("teacher")` middleware gates stream creation, WHIP signaling, and stream termination
- Students can view sessions, watch streams, and participate in chat
- The `scripts/generate-token/` CLI mints test tokens for development

## Event System

Redis Pub/Sub is used for inter-service event communication:

| Event | Trigger | Payload |
|-------|---------|---------|
| `stream.started` | WebRTC connection established | `session_id`, `tenant_id`, `teacher_id` |
| `stream.ended` | Teacher disconnects | `session_id`, `tenant_id` |
| `recording.ready` | MP4 concatenation complete | `session_id`, `tenant_id`, `mp4_url` |
| `chat:{session_id}` | Chat message sent | Full message JSON |

This design allows future LMS services (notifications, analytics, billing) to subscribe to these events without coupling to the streaming server.

## Observability

- **Structured logging**: zerolog with JSON output, request-level middleware
- **Prometheus metrics**: streams active, WebSocket connections, chat messages, segment lag, FFmpeg restarts
- **Grafana dashboard**: pre-provisioned via Docker Compose (`monitoring` profile) with auto-configured datasource

## Deployment

### Single-node (current)

```
docker compose up                              # core services
docker compose --profile monitoring up         # + Prometheus & Grafana
docker compose --profile storage up            # + MinIO
```

All components (Go server, FFmpeg, Postgres, Redis) run on a single machine. Target: 5-10 concurrent streams, ~200 viewers each.

### SaaS Scaling Path

The architecture is designed for horizontal scaling with minimal changes:

1. **CDN for HLS delivery**: Segments are immutable once written — put a CDN (CloudFront, Bunny) in front of `/hls/*` endpoints. Playlists remain short-lived. This offloads 90%+ of bandwidth from the origin.

2. **Media server fleet**: Move FFmpeg transcoding to dedicated worker nodes. The Go API server becomes stateless; FFmpeg workers pull jobs from a queue (Redis Streams or SQS). Segments are written to object storage (S3/MinIO) instead of local disk.

3. **Horizontal API scaling**: The Go server is already stateless except for in-memory WebRTC PeerConnections. Use consistent hashing or sticky sessions to route WHIP signaling to the node holding the PeerConnection. Chat and recording are already decoupled via Redis.

4. **Database scaling**: Add read replicas for session listing and chat history. Shard by `tenant_id` when needed. The schema is multi-tenant-ready.

5. **Multi-region**: Deploy media server fleets in multiple regions. Use GeoDNS to route teachers to the nearest ingestion point. CDN handles viewer distribution globally.

### Key Design Decisions for Scale

- **`tenant_id` on every table**: Enables row-level isolation, per-tenant billing queries, and future sharding
- **Redis Pub/Sub for events**: Decouples producers from consumers; future services subscribe without code changes
- **FFmpeg as subprocess**: Can be replaced with hardware-accelerated transcoding (NVENC, VA-API) by changing CLI flags — no code changes
- **HLS over HTTP**: Standard protocol supported by every CDN and player — no proprietary lock-in
- **Stateless recording worker**: Reads segments from disk/object storage, writes MP4 — can scale independently

## Technology Stack

| Component | Technology | Rationale |
|-----------|-----------|-----------|
| WebRTC | Pion (Go) | Native Go, WHIP support, no CGo |
| Transcoding | FFmpeg | Industry standard, ABR HLS output |
| HTTP | Chi router | Lightweight, stdlib-compatible |
| Database | PostgreSQL | Reliable, JSON support, pgx driver |
| Cache/Events | Redis | Pub/Sub for chat + events, low latency |
| Auth | JWT (HS256) | Stateless, standard, no session store |
| Frontend | React + Vite | Fast dev cycle, TypeScript |
| Player | HLS.js | Adaptive bitrate, Safari native fallback |
| Metrics | Prometheus + Grafana | De facto standard for Go services |
| Container | Docker Compose | Single-command dev setup |
