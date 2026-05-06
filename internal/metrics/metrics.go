package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	StreamsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "streams_active",
		Help: "Number of currently active streams",
	})

	ViewersActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "viewers_active",
		Help: "Number of active viewers",
	})

	ChatMessagesTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "chat_messages_total",
		Help: "Total number of chat messages sent",
	})

	FFmpegRestartsTotal = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "ffmpeg_restarts_total",
		Help: "Total number of FFmpeg process restarts",
	})

	WebSocketConnections = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "websocket_connections",
		Help: "Current number of WebSocket connections",
	})

	SegmentLagSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:    "segment_lag_seconds",
		Help:    "Time since last HLS segment was written",
		Buckets: []float64{0.5, 1, 2, 3, 5, 10},
	})
)

func Register() {
	prometheus.MustRegister(
		StreamsActive,
		ViewersActive,
		ChatMessagesTotal,
		FFmpegRestartsTotal,
		WebSocketConnections,
		SegmentLagSeconds,
	)
}
