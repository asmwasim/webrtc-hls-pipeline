import { useEffect, useRef, useState } from 'react';
import { useParams } from 'react-router-dom';
import { WHIPClient } from '../lib/whip';
import { endSession } from '../lib/api';
import { StreamControls } from '../components/StreamControls';
import { ChatPanel } from '../components/ChatPanel';

export function TeacherDashboard() {
  const { sessionId } = useParams<{ sessionId: string }>();
  const videoRef = useRef<HTMLVideoElement>(null);
  const whipRef = useRef<WHIPClient | null>(null);
  const streamRef = useRef<MediaStream | null>(null);

  const [isLive, setIsLive] = useState(false);
  const [isMuted, setIsMuted] = useState(false);
  const [isCameraOff, setIsCameraOff] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    const startPreview = async () => {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({
          video: { width: 1280, height: 720 },
          audio: true,
        });
        streamRef.current = stream;
        if (videoRef.current) {
          videoRef.current.srcObject = stream;
        }
      } catch (e: any) {
        setError('Camera access denied: ' + e.message);
      }
    };
    startPreview();

    return () => {
      streamRef.current?.getTracks().forEach((t) => t.stop());
      whipRef.current?.disconnect();
    };
  }, []);

  const handleGoLive = async () => {
    if (!streamRef.current || !sessionId) return;
    try {
      const whip = new WHIPClient();
      await whip.connect(sessionId, streamRef.current);
      whipRef.current = whip;
      setIsLive(true);
    } catch (e: any) {
      setError('Failed to go live: ' + e.message);
    }
  };

  const handleEndStream = async () => {
    if (!sessionId) return;
    whipRef.current?.disconnect();
    try {
      await endSession(sessionId);
    } catch {
      // best effort
    }
    setIsLive(false);
  };

  const handleToggleMute = () => {
    const audioTrack = streamRef.current?.getAudioTracks()[0];
    if (audioTrack) {
      audioTrack.enabled = !audioTrack.enabled;
      setIsMuted(!audioTrack.enabled);
    }
  };

  const handleToggleCamera = () => {
    const videoTrack = streamRef.current?.getVideoTracks()[0];
    if (videoTrack) {
      videoTrack.enabled = !videoTrack.enabled;
      setIsCameraOff(!videoTrack.enabled);
    }
  };

  return (
    <div style={{ maxWidth: 1200, margin: '0 auto', padding: 24 }}>
      <h1>Teacher Dashboard</h1>
      {error && <p style={{ color: '#e53935' }}>{error}</p>}

      <div style={{ display: 'flex', gap: 16 }}>
        <div style={{ flex: 2 }}>
          <video
            ref={videoRef}
            autoPlay
            playsInline
            muted
            style={{ width: '100%', borderRadius: 8, background: '#000' }}
          />
          <StreamControls
            isLive={isLive}
            onGoLive={handleGoLive}
            onEndStream={handleEndStream}
            onToggleMute={handleToggleMute}
            onToggleCamera={handleToggleCamera}
            isMuted={isMuted}
            isCameraOff={isCameraOff}
          />
        </div>
        <div style={{ flex: 1, minHeight: 400 }}>
          {sessionId && <ChatPanel sessionId={sessionId} />}
        </div>
      </div>
    </div>
  );
}
