import { useEffect, useState } from 'react'
import { api } from '../api'
import { Save } from 'lucide-react'

export default function Settings() {
  const [settings, setSettings] = useState<Record<string, string>>({})
  const [msg, setMsg] = useState('')

  useEffect(() => { api.settings().then(setSettings).catch(() => {}) }, [])

  const handleSave = async () => {
    setMsg('')
    try {
      await api.updateSettings(settings)
      setMsg('Settings saved')
    } catch { setMsg('Save failed') }
  }

  const update = (key: string, val: string) => setSettings(s => ({ ...s, [key]: val }))

  return (
    <div>
      <h1 className="text-xl font-bold mb-4">Settings</h1>
      {msg && <div className="mb-3 text-sm text-green-700 bg-green-50 rounded px-3 py-2">{msg}</div>}

      <div className="bg-white rounded-lg border shadow-sm p-6 space-y-4 max-w-2xl">
        <Field label="Admin Secret" desc="Password for admin dashboard. Leave empty to disable auth.">
          <input type="password" value={settings.admin_secret || ''} onChange={e => update('admin_secret', e.target.value)} className="border rounded px-3 py-1.5 text-sm w-full" />
        </Field>
        <Field label="Default Model" desc="Default model shown in /v1/models">
          <select value={settings.default_model || 'deepseek-v4-flash'} onChange={e => update('default_model', e.target.value)} className="border rounded px-3 py-1.5 text-sm w-full">
            <option value="deepseek-v4-flash">deepseek-v4-flash</option>
            <option value="Qwen/Qwen3.6-35B-A3B">Qwen/Qwen3.6-35B-A3B</option>
            <option value="Qwen/Qwen3-32B">Qwen/Qwen3-32B</option>
            <option value="Qwen/Qwen3-30B-A3B">Qwen/Qwen3-30B-A3B</option>
            <option value="Qwen/Qwen3-Coder-480B-A35B-Instruct">Qwen/Qwen3-Coder-480B-A35B-Instruct</option>
          </select>
        </Field>
        <Field label="Rate Limit (RPM)" desc="Max requests per minute. 0 = unlimited.">
          <input type="number" value={settings.rate_limit_rpm || '0'} onChange={e => update('rate_limit_rpm', e.target.value)} className="border rounded px-3 py-1.5 text-sm w-full" />
        </Field>
        <Field label="Auto Refresh Tokens" desc="Automatically refresh AtomCode tokens every 6 hours.">
          <select value={settings.auto_refresh_tokens || 'true'} onChange={e => update('auto_refresh_tokens', e.target.value)} className="border rounded px-3 py-1.5 text-sm w-full">
            <option value="true">Enabled</option>
            <option value="false">Disabled</option>
          </select>
        </Field>

        <button onClick={handleSave} className="flex items-center gap-2 px-4 py-2 bg-gray-900 text-white rounded text-sm hover:bg-gray-800">
          <Save size={14} /> Save Settings
        </button>
      </div>
    </div>
  )
}

function Field({ label, desc, children }: { label: string; desc: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="block text-sm font-medium mb-1">{label}</label>
      <p className="text-xs text-gray-500 mb-2">{desc}</p>
      {children}
    </div>
  )
}
