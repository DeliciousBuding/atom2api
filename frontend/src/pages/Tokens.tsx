import { useEffect, useState } from 'react'
import { api } from '../api'
import { Token } from '../types'
import { RefreshCw, Trash2, Download, Plus } from 'lucide-react'

export default function Tokens() {
  const [tokens, setTokens] = useState<Token[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [label, setLabel] = useState('')
  const [accessToken, setAccessToken] = useState('')
  const [refreshToken, setRefreshToken] = useState('')
  const [msg, setMsg] = useState('')

  const load = () => api.tokens().then(setTokens).catch(() => {})
  useEffect(() => { load() }, [])

  const handleImportEnv = async () => {
    setMsg('')
    try {
      const res = await api.importEnv()
      setMsg(res.error ? `Error: ${res.error}` : res.imported > 0 ? `Imported ${res.imported} token(s) from ATOMCODE_TOKENS` : 'No new tokens found in ATOMCODE_TOKENS env')
      load()
    } catch { setMsg('Import failed') }
  }

  const handleAdd = async () => {
    if (!accessToken) return
    setMsg('')
    try {
      const res = await api.addToken({ label, access_token: accessToken, refresh_token: refreshToken })
      if (res.error) setMsg(`Error: ${res.error}`)
      else { setMsg('Token added'); setShowAdd(false); setLabel(''); setAccessToken(''); setRefreshToken(''); load() }
    } catch { setMsg('Add failed') }
  }

  const handleRefresh = async (id: number) => {
    setMsg('')
    try {
      const res = await api.refreshToken(id)
      setMsg(res.error ? `Error: ${res.error}` : 'Token refreshed')
      load()
    } catch { setMsg('Refresh failed') }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this token?')) return
    await api.deleteToken(id)
    load()
  }

  const statusBadge = (s: string) => {
    const cls = s === 'active' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
    return <span className={`px-2 py-0.5 rounded text-xs font-medium ${cls}`}>{s}</span>
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold">Tokens</h1>
        <div className="flex gap-2">
          <button onClick={handleImportEnv} className="flex items-center gap-1 px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
            <Download size={14} /> Import from Env
          </button>
          <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-1 px-3 py-1.5 bg-gray-900 text-white rounded text-sm hover:bg-gray-800">
            <Plus size={14} /> Add Token
          </button>
        </div>
      </div>

      {msg && <div className="mb-3 text-sm text-blue-700 bg-blue-50 rounded px-3 py-2">{msg}</div>}

      {showAdd && (
        <div className="bg-white rounded-lg border shadow-sm p-4 mb-4">
          <div className="grid grid-cols-3 gap-3 mb-3">
            <input value={label} onChange={e => setLabel(e.target.value)} placeholder="Label (optional)" className="border rounded px-3 py-1.5 text-sm" />
            <input value={accessToken} onChange={e => setAccessToken(e.target.value)} placeholder="Access Token" className="border rounded px-3 py-1.5 text-sm" />
            <input value={refreshToken} onChange={e => setRefreshToken(e.target.value)} placeholder="Refresh Token (optional)" className="border rounded px-3 py-1.5 text-sm" />
          </div>
          <button onClick={handleAdd} className="px-4 py-1.5 bg-green-600 text-white rounded text-sm hover:bg-green-700">Save</button>
        </div>
      )}

      <div className="bg-white rounded-lg border shadow-sm overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-600">
            <tr>
              <th className="text-left px-4 py-2">ID</th>
              <th className="text-left px-4 py-2">Label</th>
              <th className="text-left px-4 py-2">Token</th>
              <th className="text-left px-4 py-2">Status</th>
              <th className="text-left px-4 py-2">Last Used</th>
              <th className="text-right px-4 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {tokens.length === 0 ? (
              <tr><td colSpan={6} className="text-center py-8 text-gray-400">No tokens. Add manually or set ATOMCODE_TOKENS env and click Import.</td></tr>
            ) : tokens.map(t => (
              <tr key={t.id} className="border-t">
                <td className="px-4 py-2">{t.id}</td>
                <td className="px-4 py-2">{t.label || '-'}</td>
                <td className="px-4 py-2 font-mono text-xs">{t.access_token}</td>
                <td className="px-4 py-2">{statusBadge(t.status)}</td>
                <td className="px-4 py-2 text-gray-500">{t.last_used_at ? new Date(t.last_used_at).toLocaleString() : 'Never'}</td>
                <td className="px-4 py-2 text-right">
                  <button onClick={() => handleRefresh(t.id)} className="text-blue-600 hover:text-blue-800 mr-2" title="Refresh"><RefreshCw size={14} /></button>
                  <button onClick={() => handleDelete(t.id)} className="text-red-600 hover:text-red-800" title="Delete"><Trash2 size={14} /></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
