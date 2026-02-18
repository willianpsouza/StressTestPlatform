'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { DashboardStats, TestExecution } from '@/types'

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [loading, setLoading] = useState(true)

  const fetchData = async () => {
    const [statsRes, execsRes] = await Promise.all([
      api.get<DashboardStats>('/dashboard/stats'),
      api.get<TestExecution[]>('/dashboard/executions?page_size=20'),
    ])

    if (statsRes.success && statsRes.data) setStats(statsRes.data)
    if (execsRes.success && execsRes.data) setExecutions(execsRes.data)
    setLoading(false)
  }

  useEffect(() => {
    fetchData()
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [])

  if (loading) {
    return <div className="text-gray-400">Carregando...</div>
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dashboard</h1>
        <p className="mt-1 text-lg text-primary-600 font-semibold">
          {process.env.NEXT_PUBLIC_PROJECT_NAME || 'BR-IDNF'}
        </p>
      </div>

      {/* Stats Cards */}
      {stats && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <StatCard label="Total Testes" value={stats.total_tests} />
          <StatCard label="Executando Agora" value={stats.running_now} color="text-blue-600" />
          <StatCard label="Completos Hoje" value={stats.completed_today} color="text-green-600" />
          <StatCard label="Falhos Hoje" value={stats.failed_today} color="text-red-600" />
        </div>
      )}

      {/* Executions Table */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Execucoes Recentes</h2>
          <button
            onClick={fetchData}
            className="text-sm text-primary-600 hover:text-primary-700"
          >
            Atualizar
          </button>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className="bg-gray-50">
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Teste</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Dominio</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Usuario</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duracao</th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Data</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-200">
              {executions.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-6 py-8 text-center text-gray-400">
                    Nenhuma execucao encontrada
                  </td>
                </tr>
              ) : (
                executions.map((exec) => (
                  <tr key={exec.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">{exec.test_name}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.domain_name}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.user_name}</td>
                    <td className="px-6 py-4">
                      <span className={cn('px-2 py-1 text-xs font-medium rounded-full', statusColors[exec.status])}>
                        {exec.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.duration}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{formatDate(exec.created_at)}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  )
}

function StatCard({ label, value, color = 'text-gray-900' }: { label: string; value: number; color?: string }) {
  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
      <p className="text-sm text-gray-500">{label}</p>
      <p className={cn('text-3xl font-bold mt-1', color)}>{value}</p>
    </div>
  )
}
