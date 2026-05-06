package transcode

import (
	"bytes"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type ffmpegLogger struct {
	sessionID uuid.UUID
	buf       bytes.Buffer
}

func (l *ffmpegLogger) Write(p []byte) (int, error) {
	l.buf.Write(p)

	for {
		line, err := l.buf.ReadBytes('\n')
		if err != nil {
			l.buf.Write(line)
			break
		}

		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}

		log.Warn().
			Str("session_id", l.sessionID.String()).
			Str("source", "ffmpeg").
			Msg(string(trimmed))
	}

	return len(p), nil
}
