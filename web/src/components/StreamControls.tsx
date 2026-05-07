interface Props {
  isLive: boolean;
  onGoLive: () => void;
  onEndStream: () => void;
  onToggleMute: () => void;
  onToggleCamera: () => void;
  isMuted: boolean;
  isCameraOff: boolean;
}

export function StreamControls({
  isLive,
  onGoLive,
  onEndStream,
  onToggleMute,
  onToggleCamera,
  isMuted,
  isCameraOff,
}: Props) {
  return (
    <div style={{ display: 'flex', gap: 8, padding: '12px 0' }}>
      {!isLive ? (
        <button
          onClick={onGoLive}
          style={{ padding: '8px 24px', background: '#e53935', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontWeight: 600 }}
        >
          Go Live
        </button>
      ) : (
        <>
          <button
            onClick={onToggleMute}
            style={{ padding: '8px 16px', background: isMuted ? '#ff9800' : '#333', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
          >
            {isMuted ? 'Unmute' : 'Mute'}
          </button>
          <button
            onClick={onToggleCamera}
            style={{ padding: '8px 16px', background: isCameraOff ? '#ff9800' : '#333', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
          >
            {isCameraOff ? 'Camera On' : 'Camera Off'}
          </button>
          <button
            onClick={onEndStream}
            style={{ padding: '8px 24px', background: '#e53935', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer', fontWeight: 600 }}
          >
            End Stream
          </button>
        </>
      )}
    </div>
  );
}
