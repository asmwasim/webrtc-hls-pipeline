package events

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	StreamStarted  = "stream.started"
	StreamEnded    = "stream.ended"
	RecordingReady = "recording.ready"
)

type Publisher struct {
	rdb *redis.Client
}

func NewPublisher(rdb *redis.Client) *Publisher {
	return &Publisher{rdb: rdb}
}

func (p *Publisher) Publish(ctx context.Context, channel string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		log.Error().Err(err).Str("channel", channel).Msg("failed to marshal event")
		return
	}

	if err := p.rdb.Publish(ctx, channel, data).Err(); err != nil {
		log.Error().Err(err).Str("channel", channel).Msg("failed to publish event")
	}
}
