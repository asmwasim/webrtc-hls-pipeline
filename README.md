# WebRTC-HLS Pipeline

A self-hosted live streaming microservice that ingests WebRTC from browsers, transcodes to adaptive-bitrate HLS via FFmpeg, and serves to viewers — with real-time chat, recording, and observability built in.

Built as the streaming backbone for a SaaS LMS platform where teachers go live from any browser and students watch with no plugins or installs.

## How It Works

```
Browser (teacher)                Server                    Browser (student)
     │                              │                           │
     │  getUserMedia + WHIP offer   │                           │
     │ ────────────────────────────▶│                           │
     │  SDP answer                  │                           │
     │ ◀────────────────────────────│                           │
     │                              │                           │
     │  H.264 RTP over WebRTC      │                           │
     │ ═══════════════════════════▶ │                           │
     │                              │  FFmpeg: 720p/480p/360p  │
     │                              │  ────▶ HLS segments      │
     │                              │                           │
     │                              │  master.m3u8 + .ts files │
     │                              │ ◀─────────────────────────│
     │                              │         HLS.js            │
     │                              │                           │
     │  WebSocket chat ◀───────────▶│◀─────────── WebSocket ───│
```

## Quick Start

**Prerequisites**: Docker, Docker Compose, Node.js 18+

```bash
# Start backend services
docker compose up -d

# Run database migrations
docker compose up migrate

# Generate test tokens
go run ./scripts/generate-token -role teacher -username "Ms. Smith"
go run ./scripts/generate-token -role student -username "Alex"

# Start frontend dev server
cd web && npm install && npm run dev
```

Open http://localhost:5173, paste a teacher token, create a session, and go live.

## API

All endpoints are prefixed with `/api/v1`. Authenticated via `Authorization: Bearer <jwt>`.

### Sessions

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/sessions` | Teacher | Create session |
| `GET` | `/sessions` | Any | List sessions |
| `GET` | `/sessions/{id}` | Any | Get session |
| `POST` | `/sessions/{id}/end` | Teacher | End stream |

### Streaming

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `POST` | `/sessions/{id}/whip` | Teacher | WHIP signaling (SDP exchange) |
| `DELETE` | `/sessions/{id}/whip` | Teacher | Teardown WebRTC connection |
| `GET` | `/sessions/{id}/watch` | Any | Get HLS playlist URL |

### Chat

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `WS` | `/sessions/{id}/chat` | Any | Real-time WebSocket chat |
| `GET` | `/sessions/{id}/chat/history` | Any | Paginated chat history |

### Recording

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/sessions/{id}/recording` | Any | Get recording status/URL |

### System

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| `GET` | `/health` | None | Health check (Postgres + Redis) |
| `GET` | `/metrics` | None | Prometheus metrics |

HLS segments are served at `/hls/{sessionID}/master.m3u8`.

## Project Structure

```
cmd/server/              Go entrypoint, router, graceful shutdown
internal/
  auth/                  JWT validation middleware
  chat/                  WebSocket hub, Redis fan-out, Postgres persistence
  config/                Environment-based configuration
  events/                Redis Pub/Sub event publisher
  hls/                   Segment serving, CORS, VOD marking
  httputil/              Shared HTTP response helpers
  metrics/               Prometheus metric definitions
  recording/             Background MP4 concatenation worker
  session/               Session CRUD and lifecycle
  transcode/             FFmpeg subprocess manager
  whip/                  WHIP handler, Pion WebRTC, track management
web/                     React + Vite + TypeScript frontend
  src/pages/             Teacher dashboard, student viewer, session list
  src/components/        Video player (HLS.js), chat panel, stream controls
  src/lib/               WHIP client, WebSocket client, API helpers
migrations/              PostgreSQL schema migrations
scripts/generate-token/  CLI to mint test JWTs
configs/                 Prometheus + Grafana provisioning
```

## Configuration

Environment variables (with defaults):

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server listen port |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/streaming?sslmode=disable` | PostgreSQL connection |
| `REDIS_URL` | `redis://localhost:6379` | Redis connection |
| `JWT_SECRET` | `dev-secret-change-in-production` | JWT signing key |
| `SEGMENT_DIR` | `./segments` | HLS segment output directory |
| `MINIO_URL` | _(empty)_ | MinIO endpoint (optional) |
| `MINIO_BUCKET` | `recordings` | MinIO bucket for recordings |

## Docker Compose Profiles

```bash
docker compose up                              # Core: server, Postgres, Redis
docker compose --profile monitoring up         # + Prometheus + Grafana (localhost:3000)
docker compose --profile storage up            # + MinIO (localhost:9001)
docker compose --profile monitoring --profile storage up  # Everything
```

Grafana ships with a pre-provisioned streaming dashboard (admin/admin).

## Tech Stack

| Layer | Choice |
|-------|--------|
| Ingestion | Pion WebRTC (Go) via WHIP protocol |
| Transcoding | FFmpeg — 3 ABR renditions (720p/480p/360p) |
| Delivery | HLS over HTTP, HLS.js player |
| Database | PostgreSQL with multi-tenant `tenant_id` |
| Events | Redis Pub/Sub |
| Auth | JWT (HS256) with role-based guards |
| Frontend | React + Vite + TypeScript |
| Observability | Prometheus + Grafana + zerolog |

## Architecture

See [ARCHITECTURE.md](ARCHITECTURE.md) for system diagrams, data flow, database schema, and the SaaS scaling path.
