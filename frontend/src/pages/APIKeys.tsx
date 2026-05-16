import { useEffect, useState } from 'react'
import { api } from '../api'
import { APIKey } from '../types'
import { Trash2, Copy, Plus } from 'lucide-react'

export default function APIKeys() {
  const [keys, setKeys] = useState<APIKey[]>([])
  const [showAdd, setShowAdd] = useState(false)
  const [name, setName] = useState('')
  const [quota, setQuota] = useState(0)
  const [newKey, setNewKey] = useState('')
  const [msg, setMsg] = useState('')

  const load = () => api.apiKeys().then(setKeys).catch(() => {})
  useEffect(() => { load() }, [])

  const handleCreate = async () => {
    setMsg('')
    try {
      const res = await api.createAPIKey({ name, quota_limit: quota })
      if (res.error) setMsg(`Error: ${res.error}`)
      else { setNewKey(res.key); setMsg('API key created. Copy it now — it won\'t be shown again.'); setShowAdd(false); setName(''); setQuota(0); load() }
    } catch { setMsg('Create failed') }
  }

  const handleDelete = async (id: number) => {
    if (!confirm('Delete this API key?')) return
    await api.deleteAPIKey(id)
    load()
  }

  const copyKey = (key: string) => navigator.clipboard.writeText(key)

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold">API Keys</h1>
        <button onClick={() => setShowAdd(!showAdd)} className="flex items-center gap-1 px-3 py-1.5 bg-gray-900 text-white rounded text-sm hover:bg-gray-800">
          <Plus size={14} /> Create Key
        </button>
      </div>

      {msg && <div className="mb-3 text-sm text-blue-700 bg-blue-50 rounded px-3 py-2">{msg}</div>}
      {newKey && (
        <div className="mb-3 bg-yellow-50 border border-yellow-200 rounded px-4 py-3 flex items-center justify-between">
          <code className="text-sm font-mono">{newKey}</code>
          <button onClick={() => { copyKey(newKey); setNewKey('') }} className="text-blue-600 text-sm hover:underline">Copy & Dismiss</button>
        </div>
      )}

      {showAdd && (
        <div className="bg-white rounded-lg border shadow-sm p-4 mb-4">
          <div className="flex gap-3 mb-3">
            <input value={name} onChange={e => setName(e.target.value)} placeholder="Key name" className="border rounded px-3 py-1.5 text-sm flex-1" />
            <input type="number" value={quota || ''} onChange={e => setQuota(Number(e.target.value))} placeholder="Quota (0=unlimited)" className="border rounded px-3 py-1.5 text-sm w-48" />
          </div>
          <button onClick={handleCreate} className="px-4 py-1.5 bg-green-600 text-white rounded text-sm hover:bg-green-700">Create</button>
        </div>
      )}

      <div className="bg-white rounded-lg border shadow-sm overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-600">
            <tr>
              <th className="text-left px-4 py-2">Name</th>
              <th className="text-left px-4 py-2">Key</th>
              <th className="text-left px-4 py-2">Enabled</th>
              <th className="text-left px-4 py-2">Used / Quota</th>
              <th className="text-left px-4 py-2">Created</th>
              <th className="text-right px-4 py-2">Actions</th>
            </tr>
          </thead>
          <tbody>
            {keys.length === 0 ? (
              <tr><td colSpan={6} className="text-center py-8 text-gray-400">No API keys. Requests are open to everyone.</td></tr>
            ) : keys.map(k => (
              <tr key={k.id} className="border-t">
                <td className="px-4 py-2">{k.name || '-'}</td>
                <td className="px-4 py-2 font-mono text-xs">{k.key.slice(0, 20)}...</td>
                <td className="px-4 py-2">{k.enabled ? <span className="text-green-600">Yes</span> : <span className="text-red-600">No</span>}</td>
                <td className="px-4 py-2">{k.quota_used} / {k.quota_limit || '∞'}</td>
                <td className="px-4 py-2 text-gray-500">{new Date(k.created_at).toLocaleDateString()}</td>
                <td className="px-4 py-2 text-right">
                  <button onClick={() => copyKey(k.key)} className="text-gray-500 hover:text-gray-700 mr-2" title="Copy"><Copy size={14} /></button>
                  <button onClick={() => handleDelete(k.id)} className="text-red-600 hover:text-red-800" title="Delete"><Trash2 size={14} /></button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
