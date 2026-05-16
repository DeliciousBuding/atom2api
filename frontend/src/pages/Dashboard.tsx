import { useEffect, useState } from 'react'
import { api } from '../api'
import { Stats } from '../types'
import StatCard from '../components/StatCard'

export default function Dashboard() {
  const [stats, setStats] = useState<Stats | null>(null)
  useEffect(() => { api.stats().then(setStats).catch(() => {}) }, [])

  if (!stats) return <div className="text-gray-500">Loading...</div>

  return (
    <div>
      <h1 className="text-xl font-bold mb-4">Dashboard</h1>
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-4 mb-6">
        <StatCard label="Tokens" value={stats.tokens_total} />
        <StatCard label="Active" value={stats.tokens_active} color="text-green-600" />
        <StatCard label="Errors" value={stats.tokens_error} color="text-red-600" />
        <StatCard label="Requests" value={stats.total_requests} />
        <StatCard label="Total Tokens" value={stats.total_tokens.toLocaleString()} />
        <StatCard label="API Keys" value={stats.api_keys_count} />
      </div>
      <div className="bg-white rounded-lg shadow-sm border p-4">
        <h2 className="font-semibold mb-3">Quick Start</h2>
        <pre className="bg-gray-100 rounded p-3 text-sm overflow-auto">{`# Use Atom2API as OpenAI-compatible endpoint
curl http://localhost:8080/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -d '{
    "model": "deepseek-v4-flash",
    "messages": [{"role": "user", "content": "Hello!"}],
    "stream": true
  }'

# Python OpenAI SDK
from openai import OpenAI
client = OpenAI(base_url="http://localhost:8080/v1", api_key="any")
resp = client.chat.completions.create(
    model="deepseek-v4-flash",
    messages=[{"role": "user", "content": "Hello!"}]
)`}</pre>
      </div>
    </div>
  )
}
