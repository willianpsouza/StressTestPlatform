'use client'

import { useRouter } from 'next/navigation'
import { useAuthStore } from '@/lib/store'
import { api } from '@/lib/api'

export default function Header() {
  const router = useRouter()
  const { user, logout } = useAuthStore()

  const handleLogout = async () => {
    const refreshToken = localStorage.getItem('refresh_token')
    await api.post('/auth/logout', { refresh_token: refreshToken })
    logout()
    router.push('/login')
  }

  return (
    <header className="sticky top-0 z-10 bg-white border-b border-gray-200">
      <div className="flex items-center justify-between h-16 px-6">
        <div className="flex items-center space-x-4">
          <span className="text-sm text-gray-500">
            Project: <span className="font-semibold text-primary-600">{process.env.NEXT_PUBLIC_PROJECT_NAME || 'BR-IDNF'}</span>
          </span>
        </div>
        <div className="flex items-center space-x-4">
          <span className="text-sm text-gray-600">{user?.name}</span>
          <span className="px-2 py-0.5 text-xs font-medium bg-gray-100 text-gray-600 rounded">{user?.role}</span>
          <button
            onClick={handleLogout}
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Logout
          </button>
        </div>
      </div>
    </header>
  )
}
