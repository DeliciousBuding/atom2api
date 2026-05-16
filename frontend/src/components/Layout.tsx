import { NavLink, useLocation } from 'react-router-dom'
import { LayoutDashboard, Key, Shield, BarChart3, Settings, LogOut } from 'lucide-react'

const nav = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard },
  { to: '/tokens', label: 'Tokens', icon: Key },
  { to: '/api-keys', label: 'API Keys', icon: Shield },
  { to: '/usage', label: 'Usage', icon: BarChart3 },
  { to: '/settings', label: 'Settings', icon: Settings },
]

export default function Layout({ children, onLogout }: { children: React.ReactNode; onLogout: () => void }) {
  const location = useLocation()
  return (
    <div className="flex h-screen">
      <aside className="w-56 bg-gray-900 text-white flex flex-col">
        <div className="p-4 font-bold text-lg border-b border-gray-700">Atom2API</div>
        <nav className="flex-1 p-2 space-y-1">
          {nav.map(n => (
            <NavLink
              key={n.to}
              to={n.to}
              end={n.to === '/'}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded text-sm ${isActive ? 'bg-gray-700 text-white' : 'text-gray-300 hover:bg-gray-800'}`
              }
            >
              <n.icon size={18} />
              {n.label}
            </NavLink>
          ))}
        </nav>
        <button onClick={onLogout} className="flex items-center gap-3 px-3 py-3 text-gray-400 hover:text-white text-sm border-t border-gray-700">
          <LogOut size={18} /> Logout
        </button>
      </aside>
      <main className="flex-1 overflow-auto p-6 bg-gray-50">
        {children}
      </main>
    </div>
  )
}
