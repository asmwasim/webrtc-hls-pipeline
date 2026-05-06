import { useEffect, useRef } from 'react';
import Hls from 'hls.js';

interface Props {
  src: string;
}

export function VideoPlayer({ src }: Props) {
  const videoRef = useRef<HTMLVideoElement>(null);
  const hlsRef = useRef<Hls | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    if (!video || !src) return;

    if (Hls.isSupported()) {
      const hls = new Hls({
        lowLatencyMode: true,
        liveSyncDurationCount: 3,
      });
      hls.loadSource(src);
      hls.attachMedia(video);
      hls.on(Hls.Events.MANIFEST_PARSED, () => {
        video.play().catch(() => {});
      });
      hlsRef.current = hls;

      return () => {
        hls.destroy();
      };
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src;
      const onLoaded = () => video.play().catch(() => {});
      video.addEventListener('loadedmetadata', onLoaded);

      return () => {
        video.removeEventListener('loadedmetadata', onLoaded);
        video.src = '';
      };
    }
  }, [src]);

  return (
    <video
      ref={videoRef}
      controls
      playsInline
      muted
      style={{ width: '100%', maxHeight: '70vh', background: '#000' }}
    />
  );
}
