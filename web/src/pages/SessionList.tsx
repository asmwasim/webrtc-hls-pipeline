import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { listSessions, createSession, type Session } from '../lib/api';

export function SessionList() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [title, setTitle] = useState('');
  const [error, setError] = useState('');

  const load = async () => {
    try {
      const data = await listSessions();
      setSessions(data);
    } catch (e: any) {
      setError(e.message);
    }
  };

  useEffect(() => { load(); }, []);

  const handleCreate = async () => {
    if (!title.trim()) return;
    try {
      await createSession(title.trim());
      setTitle('');
      load();
    } catch (e: any) {
      setError(e.message);
    }
  };

  return (
    <div style={{ maxWidth: 800, margin: '0 auto', padding: 24 }}>
      <h1>Streaming Sessions</h1>

      <div style={{ display: 'flex', gap: 8, marginBottom: 24 }}>
        <input
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleCreate()}
          placeholder="New session title..."
          style={{ flex: 1, padding: '8px 12px', borderRadius: 4, border: '1px solid #555', background: '#1a1a1a', color: '#fff' }}
        />
        <button onClick={handleCreate} style={{ padding: '8px 20px', background: '#646cff', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}>
          Create
        </button>
      </div>

      {error && <p style={{ color: '#e53935' }}>{error}</p>}

      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        {sessions.map((s) => (
          <div key={s.id} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: 12, border: '1px solid #333', borderRadius: 8 }}>
            <div>
              <strong>{s.title}</strong>
              <span style={{ marginLeft: 12, padding: '2px 8px', borderRadius: 4, fontSize: 12, background: s.status === 'live' ? '#e53935' : s.status === 'ended' ? '#555' : '#ff9800', color: '#fff' }}>
                {s.status}
              </span>
            </div>
            <div style={{ display: 'flex', gap: 8 }}>
              {s.status === 'waiting' && (
                <Link to={`/teach/${s.id}`} style={{ padding: '6px 14px', background: '#646cff', color: '#fff', borderRadius: 4, textDecoration: 'none' }}>
                  Teach
                </Link>
              )}
              {s.status === 'live' && (
                <>
                  <Link to={`/teach/${s.id}`} style={{ padding: '6px 14px', background: '#646cff', color: '#fff', borderRadius: 4, textDecoration: 'none' }}>
                    Teach
                  </Link>
                  <Link to={`/watch/${s.id}`} style={{ padding: '6px 14px', background: '#333', color: '#fff', borderRadius: 4, textDecoration: 'none' }}>
                    Watch
                  </Link>
                </>
              )}
              {s.status === 'ended' && (
                <Link to={`/watch/${s.id}`} style={{ padding: '6px 14px', background: '#333', color: '#fff', borderRadius: 4, textDecoration: 'none' }}>
                  Replay
                </Link>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
