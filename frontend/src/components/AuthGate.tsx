import { useState } from 'react'
import { api } from '../api'

export default function AuthGate({ onSuccess }: { onSuccess: () => void }) {
  const [secret, setSecret] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleLogin = async () => {
    setLoading(true)
    setError('')
    try {
      const res = await api.login(secret)
      if (res.status === 'ok') {
        localStorage.setItem('admin_secret', secret)
        onSuccess()
      } else {
        setError(res.error || 'Login failed')
      }
    } catch {
      setError('Connection failed')
    }
    setLoading(false)
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-gray-50">
      <div className="bg-white rounded-lg shadow-md p-8 w-96">
        <h1 className="text-2xl font-bold text-center mb-2">Atom2API</h1>
        <p className="text-gray-500 text-center text-sm mb-6">Admin Dashboard</p>
        <input
          type="password"
          value={secret}
          onChange={e => setSecret(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && handleLogin()}
          placeholder="Admin Secret"
          className="w-full border rounded px-3 py-2 mb-3 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        {error && <p className="text-red-500 text-sm mb-3">{error}</p>}
        <button
          onClick={handleLogin}
          disabled={loading}
          className="w-full bg-gray-900 text-white rounded py-2 text-sm hover:bg-gray-800 disabled:opacity-50"
        >
          {loading ? 'Logging in...' : 'Login'}
        </button>
      </div>
    </div>
  )
}
