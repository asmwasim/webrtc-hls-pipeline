import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { getWatchURL } from '../lib/api';
import { VideoPlayer } from '../components/VideoPlayer';
import { ChatPanel } from '../components/ChatPanel';

export function StudentViewer() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const [hlsUrl, setHlsUrl] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (!sessionId) return;

    const fetchURL = async () => {
      try {
        const data = await getWatchURL(sessionId);
        setHlsUrl(data.hls_url);
      } catch (e: any) {
        setError(e.message);
      }
    };

    fetchURL();
  }, [sessionId]);

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>
      <h1>Live Stream</h1>
      {error && <p style={{ color: '#e53935' }}>{error}</p>}

      <div style={{ display: 'flex', gap: 16 }}>
        <div style={{ flex: 2 }}>
          {hlsUrl ? (
            <VideoPlayer src={hlsUrl} />
          ) : (
            <div style={{ background: '#000', borderRadius: 8, height: 400, display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#888' }}>
              Loading stream...
            </div>
          )}
        </div>
        <div style={{ flex: 1, minHeight: 400 }}>
          {sessionId && <ChatPanel sessionId={sessionId} />}
        </div>
      </div>
    </div>
  );
}
