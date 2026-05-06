import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useState } from 'react';
import { SessionList } from './pages/SessionList';
import { TeacherDashboard } from './pages/TeacherDashboard';
import { StudentViewer } from './pages/StudentViewer';

function TokenSetup({ onSave }: { onSave: () => void }) {
  const [token, setToken] = useState('');

  const handleSave = () => {
    localStorage.setItem('token', token.trim());
    onSave();
  };

  return (
    <div style={{ maxWidth: 500, margin: '100px auto', padding: 24 }}>
      <h1>Setup</h1>
      <p style={{ color: '#888', marginBottom: 16 }}>
        Paste a JWT token generated with <code>go run ./scripts/generate-token</code>
      </p>
      <textarea
        value={token}
        onChange={(e) => setToken(e.target.value)}
        placeholder="Paste JWT token here..."
        rows={4}
        style={{ width: '100%', padding: 12, borderRadius: 4, border: '1px solid #555', background: '#1a1a1a', color: '#fff', fontFamily: 'monospace', fontSize: 12 }}
      />
      <button
        onClick={handleSave}
        disabled={!token.trim()}
        style={{ marginTop: 12, padding: '8px 24px', background: '#646cff', color: '#fff', border: 'none', borderRadius: 4, cursor: 'pointer' }}
      >
        Save & Continue
      </button>
    </div>
  );
}

export default function App() {
  const [hasToken, setHasToken] = useState(() => !!localStorage.getItem('token'));

  if (!hasToken) {
    return <TokenSetup onSave={() => setHasToken(true)} />;
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<SessionList />} />
        <Route path="/teach/:sessionId" element={<TeacherDashboard />} />
        <Route path="/watch/:sessionId" element={<StudentViewer />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </BrowserRouter>
  );
}
