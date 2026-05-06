import { useState, useEffect, useRef } from 'react';
import { ChatSocket, type ChatMessage } from '../lib/ws';

interface Props {
  sessionId: string;
}

export function ChatPanel({ sessionId }: Props) {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const socketRef = useRef<ChatSocket | null>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const socket = new ChatSocket();
    socketRef.current = socket;

    socket.onMessage((msg) => {
      setMessages((prev) => [...prev, msg]);
    });

    socket.connect(sessionId);

    return () => socket.disconnect();
  }, [sessionId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = () => {
    if (!input.trim()) return;
    socketRef.current?.send(input.trim());
    setInput('');
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', border: '1px solid #333', borderRadius: 8 }}>
      <div style={{ padding: '8px 12px', borderBottom: '1px solid #333', fontWeight: 600 }}>
        Chat
      </div>
      <div style={{ flex: 1, overflowY: 'auto', padding: 12 }}>
        {messages.map((msg) => (
          <div key={msg.id} style={{ marginBottom: 8 }}>
            {msg.type === 'hand_raise' ? (
              <em>{msg.username} raised hand</em>
            ) : (
              <>
                <strong>{msg.username}: </strong>
                <span>{msg.message}</span>
              </>
            )}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
      <div style={{ display: 'flex', borderTop: '1px solid #333', padding: 8, gap: 8 }}>
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSend()}
          placeholder="Type a message..."
          style={{ flex: 1, padding: '6px 10px', borderRadius: 4, border: '1px solid #555', background: '#1a1a1a', color: '#fff' }}
        />
        <button onClick={handleSend} style={{ padding: '6px 16px', borderRadius: 4, background: '#646cff', color: '#fff', border: 'none', cursor: 'pointer' }}>
          Send
        </button>
      </div>
    </div>
  );
}
