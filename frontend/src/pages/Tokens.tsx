import { useEffect, useState } from 'react'
import { api } from '../api'
import { Token } from '../types'
import { RefreshCw, Trash2, Download, Plus, Activity, BarChart3, Power } from 'lucide-react'

export default function Tokens() {
  const [tokens, setTokens] = useState<Token[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [showImportTOML, setShowImportTOML] = useState(false)
  const [label, setLabel] = useState('')
  const [accessToken, setAccessToken] = useState('')
  const [refreshToken, setRefreshToken] = useState('')
  const [tomlText, setTomlText] = useState('')
  const [msg, setMsg] = useState('')
  const [busyIds, setBusyIds] = useState<Set<number>>(new Set())
  const [batchTesting, setBatchTesting] = useState(false)

  const load = () => api.tokens().then(setTokens).catch(() => {})
  useEffect(() => { load() }, [])

  const setBusy = (id: number, busy: boolean) => {
    setBusyIds(prev => {
      const next = new Set(prev)
      if (busy) next.add(id); else next.delete(id)
      return next
    })
  }

  const handleImportEnv = async () => {
    setMsg('')
    try {
      const res = await api.importEnv()
      setMsg(res.error ? `Error: ${res.error}` : res.imported > 0 ? `Imported ${res.imported} token(s) from env` : 'No new tokens in ATOMCODE_TOKENS')
      load()
    } catch { setMsg('Import failed') }
  }

  const handleImportTOML = async () => {
    if (!tomlText.trim()) return
    setMsg('')
    try {
      const res = await api.importTOML(tomlText)
      const errPart = res.errors?.length ? ` (${res.errors.length} skipped)` : ''
      setMsg(res.error ? `Error: ${res.error}` : `Imported ${res.imported} token(s)${errPart}`)
      setTomlText('')
      setShowImportTOML(false)
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
    setBusy(id, true); setMsg('')
    try {
      const res = await api.refreshToken(id)
      setMsg(res.error ? `Error: ${res.error}` : 'Refreshed')
      load()
    } catch { setMsg('Refresh failed') }
    setBusy(id, false)
  }

  const handleTest = async (id: number) => {
    setBusy(id, true); setMsg('')
    try {
      const res = await api.testToken(id)
      setMsg(res.ok ? `Test OK (${res.latency_ms}ms)` : `Test failed: ${res.error}`)
      load()
    } catch { setMsg('Test failed') }
    setBusy(id, false)
  }

  const handleQuota = async (id: number) => {
    setBusy(id, true); setMsg('')
    try {
      const res = await api.fetchQuota(id)
      if (res.error) setMsg(`Quota: ${res.error}`)
      else setMsg(`Quota: ${JSON.stringify(res).slice(0, 200)}`)
      load()
    } catch { setMsg('Quota fetch failed') }
    setBusy(id, false)
  }

  const handleToggleEnabled = async (t: Token) => {
    setBusy(t.id, true)
    try {
      await api.toggleTokenEnabled(t.id, !t.enabled)
      load()
    } catch { setMsg('Toggle failed') }
    setBusy(t.id, false)
  }

  const handleBatchTest = async () => {
    setBatchTesting(true); setMsg('')
    try {
      const res = await api.batchTestTokens()
      const ok = res.results?.filter((r: any) => r.ok).length || 0
      setMsg(`Batch test: ${ok}/${res.total} healthy`)
      load()
    } catch { setMsg('Batch test failed') }
    setBatchTesting(false)
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this token?')) return
    await api.deleteToken(id)
    load()
  }

  const statusBadge = (t: Token) => {
    if (!t.enabled) return <span className="px-2 py-0.5 rounded text-xs font-medium bg-gray-100 text-gray-600">disabled</span>
    const cls = t.status === 'active' ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
    return <span className={`px-2 py-0.5 rounded text-xs font-medium ${cls}`}>{t.status}</span>
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold">Tokens</h1>
        <div className="flex gap-2">
          <button onClick={handleBatchTest} disabled={batchTesting || tokens.length === 0} className="flex items-center gap-1 px-3 py-1.5 bg-purple-600 text-white rounded text-sm hover:bg-purple-700 disabled:opacity-50">
            <Activity size={14} /> {batchTesting ? 'Testing...' : 'Batch Test'}
          </button>
          <button onClick={handleImportEnv} className="flex items-center gap-1 px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
            <Download size={14} /> Import Env
          </button>
          <button onClick={() => setShowImportTOML(!showImportTOML)} className="flex items-center gap-1 px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-700">
            <Download size={14} /> Paste TOML
          </button>
          <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-1 px-3 py-1.5 bg-gray-900 text-white rounded text-sm hover:bg-gray-800">
            <Plus size={14} /> Add Token
          </button>
        </div>
      </div>

      {msg && <div className="mb-3 text-sm text-blue-700 bg-blue-50 rounded px-3 py-2 whitespace-pre-wrap break-all">{msg}</div>}

      {showImportTOML && (
        <div className="bg-white rounded-lg border shadow-sm p-4 mb-4">
          <div className="text-sm text-gray-600 mb-2">Paste contents of one or more <code className="bg-gray-100 px-1 rounded">~/.atomcode/auth.toml</code> files. Separate multiple with blank lines or <code className="bg-gray-100 px-1 rounded">---</code>.</div>
          <textarea
            value={tomlText}
            onChange={e => setTomlText(e.target.value)}
            placeholder={'access_token = "..."\nrefresh_token = "..."\n[user]\nusername = "..."\nemail = "..."'}
            className="w-full border rounded px-3 py-2 text-sm font-mono mb-3 h-48"
          />
          <button onClick={handleImportTOML} className="px-4 py-1.5 bg-green-600 text-white rounded text-sm hover:bg-green-700">Import</button>
        </div>
      )}

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
              <th className="text-left px-4 py-2">Latency</th>
              <th className="text-left px-4 py-2">Last Used</th>
              <th className="text-right px-4 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {tokens.length === 0 ? (
              <tr><td colSpan={7} className="text-center py-8 text-gray-400">No tokens. Click Paste TOML or Add Token.</td></tr>
            ) : tokens.map(t => {
              const busy = busyIds.has(t.id)
              return (
                <tr key={t.id} className={`border-t ${!t.enabled ? 'opacity-60' : ''}`}>
                  <td className="px-4 py-2">{t.id}</td>
                  <td className="px-4 py-2">{t.label || '-'}</td>
                  <td className="px-4 py-2 font-mono text-xs">{t.access_token}</td>
                  <td className="px-4 py-2">{statusBadge(t)}</td>
                  <td className="px-4 py-2 text-gray-500 text-xs">{t.test_latency_ms > 0 ? `${t.test_latency_ms}ms` : '-'}</td>
                  <td className="px-4 py-2 text-gray-500 text-xs">{t.last_used_at ? new Date(t.last_used_at).toLocaleString() : 'Never'}</td>
                  <td className="px-4 py-2 text-right whitespace-nowrap">
                    <button onClick={() => handleTest(t.id)} disabled={busy} className="text-purple-600 hover:text-purple-800 mr-2 disabled:opacity-30" title="Test"><Activity size={14} /></button>
                    <button onClick={() => handleQuota(t.id)} disabled={busy} className="text-amber-600 hover:text-amber-800 mr-2 disabled:opacity-30" title="Quota"><BarChart3 size={14} /></button>
                    <button onClick={() => handleRefresh(t.id)} disabled={busy} className="text-blue-600 hover:text-blue-800 mr-2 disabled:opacity-30" title="Refresh"><RefreshCw size={14} /></button>
                    <button onClick={() => handleToggleEnabled(t)} disabled={busy} className={`mr-2 disabled:opacity-30 ${t.enabled ? 'text-gray-600 hover:text-gray-800' : 'text-green-600 hover:text-green-800'}`} title={t.enabled ? 'Disable' : 'Enable'}><Power size={14} /></button>
                    <button onClick={() => handleDelete(t.id)} className="text-red-600 hover:text-red-800" title="Delete"><Trash2 size={14} /></button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </div>
  )
}
