import { useEffect, useState } from 'react'
import { api } from '../api'
import { UsageLog } from '../types'

export default function Usage() {
  const [logs, setLogs] = useState<UsageLog[]>([])
  const [total, setTotal] = useState(0)
  const [model, setModel] = useState('')
  const [page, setPage] = useState(0)
  const limit = 20

  const load = () => {
    api.usage({ limit, offset: page * limit, model: model || undefined }).then(res => {
      setLogs(res.logs || [])
      setTotal(res.total || 0)
    }).catch(() => {})
  }
  useEffect(() => { load() }, [page, model])

  const statusColor = (code: number) => code >= 400 ? 'text-red-600' : code === 200 ? 'text-green-600' : 'text-gray-600'

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl font-bold">Usage Logs</h1>
        <div className="flex gap-2">
          <select value={model} onChange={e => { setModel(e.target.value); setPage(0) }} className="border rounded px-2 py-1 text-sm">
            <option value="">All Models</option>
            <option value="deepseek-v4-flash">deepseek-v4-flash</option>
            <option value="Qwen/Qwen3.6-35B-A3B">Qwen3.6-35B</option>
            <option value="Qwen/Qwen3-32B">Qwen3-32B</option>
          </select>
        </div>
      </div>

      <div className="bg-white rounded-lg border shadow-sm overflow-hidden">
        <table className="w-full text-sm">
          <thead className="bg-gray-50 text-gray-600">
            <tr>
              <th className="text-left px-4 py-2">Time</th>
              <th className="text-left px-4 py-2">Model</th>
              <th className="text-left px-4 py-2">Stream</th>
              <th className="text-right px-4 py-2">Tokens</th>
              <th className="text-right px-4 py-2">Duration</th>
              <th className="text-left px-4 py-2">Status</th>
              <th className="text-left px-4 py-2">Error</th>
            </tr>
          </thead>
          <tbody>
            {logs.length === 0 ? (
              <tr><td colSpan={7} className="text-center py-8 text-gray-400">No usage logs yet.</td></tr>
            ) : logs.map(l => (
              <tr key={l.id} className="border-t">
                <td className="px-4 py-2 text-gray-500 text-xs">{new Date(l.created_at).toLocaleString()}</td>
                <td className="px-4 py-2 font-mono text-xs">{l.model}</td>
                <td className="px-4 py-2">{l.is_stream ? 'Yes' : 'No'}</td>
                <td className="px-4 py-2 text-right">{l.total_tokens || '-'}</td>
                <td className="px-4 py-2 text-right">{l.duration_ms}ms</td>
                <td className={`px-4 py-2 font-medium ${statusColor(l.status_code)}`}>{l.status_code}</td>
                <td className="px-4 py-2 text-red-500 text-xs max-w-48 truncate">{l.error_message || '-'}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <div className="flex items-center justify-between mt-3 text-sm text-gray-500">
        <span>Total: {total}</span>
        <div className="flex gap-2">
          <button onClick={() => setPage(Math.max(0, page - 1))} disabled={page === 0} className="px-3 py-1 border rounded disabled:opacity-50">Prev</button>
          <span className="px-3 py-1">Page {page + 1}</span>
          <button onClick={() => setPage(page + 1)} disabled={(page + 1) * limit >= total} className="px-3 py-1 border rounded disabled:opacity-50">Next</button>
        </div>
      </div>
    </div>
  )
}
