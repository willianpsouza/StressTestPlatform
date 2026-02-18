'use client'

import { useEffect } from 'react'
import { useRouter } from 'next/navigation'
import { api } from '@/lib/api'
import { useAuthStore } from '@/lib/store'
import { User } from '@/types'
import Sidebar from '@/components/layout/Sidebar'
import Header from '@/components/layout/Header'

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter()
  const { user, setUser, isAuthenticated } = useAuthStore()

  useEffect(() => {
    const token = localStorage.getItem('access_token')
    if (!token) {
      router.push('/login')
      return
    }

    // Fetch current user if not loaded
    if (!user) {
      api.get<User>('/auth/me').then((res) => {
        if (res.success && res.data) {
          setUser(res.data)
        } else {
          router.push('/login')
        }
      })
    }

    // Session keep-alive beacon every 30s
    const interval = setInterval(() => {
      api.get('/auth/me')
    }, 30000)

    return () => clearInterval(interval)
  }, [user, setUser, router])

  if (!isAuthenticated && !user) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="text-gray-400">Carregando...</div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <Sidebar />
      <div className="lg:pl-64">
        <Header />
        <main className="p-6">{children}</main>
      </div>
    </div>
  )
}
