package hls

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

func MarkAsVOD(segmentDir string, sessionID string) error {
	dir := filepath.Join(segmentDir, sessionID)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".m3u8") || entry.Name() == "master.m3u8" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			log.Error().Err(err).Str("file", path).Msg("failed to read playlist for VOD marking")
			continue
		}

		content := string(data)
		if strings.Contains(content, "#EXT-X-ENDLIST") {
			continue
		}

		content = strings.TrimRight(content, "\n") + "\n#EXT-X-ENDLIST\n"

		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			log.Error().Err(err).Str("file", path).Msg("failed to write VOD playlist")
			continue
		}

		log.Info().Str("file", entry.Name()).Msg("marked playlist as VOD")
	}

	return nil
}
