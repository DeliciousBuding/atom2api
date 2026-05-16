import { Routes, Route, Navigate } from 'react-router-dom'
import { useState, useEffect } from 'react'
import Layout from './components/Layout'
import AuthGate from './components/AuthGate'
import Dashboard from './pages/Dashboard'
import Tokens from './pages/Tokens'
import APIKeys from './pages/APIKeys'
import Usage from './pages/Usage'
import Settings from './pages/Settings'

export default function App() {
  const [authed, setAuthed] = useState(false)

  useEffect(() => {
    const s = localStorage.getItem('admin_secret')
    if (s) setAuthed(true)
  }, [])

  if (!authed) return <AuthGate onSuccess={() => setAuthed(true)} />

  return (
    <Layout onLogout={() => { localStorage.removeItem('admin_secret'); setAuthed(false) }}>
      <Routes>
        <Route path="/" element={<Dashboard />} />
        <Route path="/tokens" element={<Tokens />} />
        <Route path="/api-keys" element={<APIKeys />} />
        <Route path="/usage" element={<Usage />} />
        <Route path="/settings" element={<Settings />} />
        <Route path="*" element={<Navigate to="/" />} />
      </Routes>
    </Layout>
  )
}
