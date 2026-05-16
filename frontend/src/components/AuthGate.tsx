import { useState, useEffect } from 'react'
import { api } from '../api'

export default function AuthGate({ onSuccess }: { onSuccess: () => void }) {
  const [mode, setMode] = useState<'loading' | 'login' | 'bootstrap'>('loading')
  const [secret, setSecret] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    fetch('/api/admin/bootstrap-status')
      .then(r => r.json())
      .then(data => {
        if (data.needs_bootstrap) setMode('bootstrap')
        else setMode('login')
      })
      .catch(() => setMode('login'))
  }, [])

  const handleLogin = async () => {
    setLoading(true); setError('')
    try {
      const res = await api.login(secret)
      if (res.status === 'ok') {
        localStorage.setItem('admin_secret', secret)
        onSuccess()
      } else {
        setError(res.error || 'Login failed')
      }
    } catch { setError('Connection failed') }
    setLoading(false)
  }

  const handleBootstrap = async () => {
    if (secret.length < 4) { setError('At least 4 characters'); return }
    if (secret !== confirm) { setError('Passwords do not match'); return }
    setLoading(true); setError('')
    try {
      const res = await fetch('/api/admin/bootstrap', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ admin_secret: secret }),
      }).then(r => r.json())
      if (res.status === 'ok') {
        localStorage.setItem('admin_secret', secret)
        onSuccess()
      } else {
        setError(res.error || 'Bootstrap failed')
      }
    } catch { setError('Connection failed') }
    setLoading(false)
  }

  if (mode === 'loading') return <div className="min-h-screen flex items-center justify-center"><p className="text-gray-400">Loading...</p></div>

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="bg-white rounded-lg shadow-md p-8 w-96">
        <h1 className="text-2xl font-bold text-center mb-2">Atom2API</h1>
        <p className="text-gray-500 text-center text-sm mb-6">
          {mode === 'bootstrap' ? 'First-time setup — create admin secret' : 'Admin Dashboard'}
        </p>

        <input
          type="password"
          value={secret}
          onChange={e => setSecret(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && (mode === 'bootstrap' ? handleBootstrap() : handleLogin())}
          placeholder={mode === 'bootstrap' ? 'Create admin secret' : 'Admin Secret'}
          className="w-full border rounded px-3 py-2 mb-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />

        {mode === 'bootstrap' && (
          <input
            type="password"
            value={confirm}
            onChange={e => setConfirm(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && handleBootstrap()}
            placeholder="Confirm admin secret"
            className="w-full border rounded px-3 py-2 mb-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        )}

        {error && <p className="text-red-500 text-sm mb-3">{error}</p>}

        <button
          onClick={mode === 'bootstrap' ? handleBootstrap : handleLogin}
          disabled={loading}
          className="w-full bg-gray-900 text-white rounded py-2 text-sm hover:bg-gray-800 disabled:opacity-50"
        >
          {loading ? 'Please wait...' : mode === 'bootstrap' ? 'Set Admin Secret' : 'Login'}
        </button>
      </div>
    </div>
  )
}
