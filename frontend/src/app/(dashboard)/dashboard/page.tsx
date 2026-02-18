'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { DashboardStats, TestExecution, ServiceStatus } from '@/types'

const serviceLabels: Record<string, string> = {
  postgres: 'PostgreSQL',
  redis: 'Redis',
  influxdb: 'InfluxDB',
  grafana: 'Grafana',
  k6: 'K6 Engine',
}

const serviceIcons: Record<string, string> = {
  postgres: 'M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4',
  redis: 'M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01',
  influxdb: 'M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z',
  grafana: 'M11 3.055A9.001 9.001 0 1020.945 13H11V3.055z M20.488 9H15V3.512A9.025 9.025 0 0120.488 9z',
  k6: 'M13 10V3L4 14h7v7l9-11h-7z',
}

export default function DashboardPage() {
  const [stats, setStats] = useState<DashboardStats | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [services, setServices] = useState<ServiceStatus[]>([])
  const [loading, setLoading] = useState(true)

  const fetchData = async () => {
    const [statsRes, execsRes, servicesRes] = await Promise.all([
      api.get<DashboardStats>('/dashboard/stats'),
      api.get<TestExecution[]>('/dashboard/executions?page_size=20'),
      api.get<ServiceStatus[]>('/services/status'),
    ])

    if (statsRes.success && statsRes.data) setStats(statsRes.data)
    if (execsRes.success && execsRes.data) setExecutions(execsRes.data)
    if (servicesRes.success && servicesRes.data) setServices(servicesRes.data)
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

      {/* Services Status */}
      {services.length > 0 && (
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 mb-6">
          {services.map((svc) => (
            <div
              key={svc.name}
              className={cn(
                'flex items-center space-x-3 p-3 rounded-xl border',
                svc.status === 'ok' && 'bg-green-50 border-green-200',
                svc.status === 'warning' && 'bg-yellow-50 border-yellow-200',
                svc.status === 'error' && 'bg-red-50 border-red-200',
              )}
              title={svc.message || ''}
            >
              <div className={cn(
                'flex-shrink-0 w-9 h-9 rounded-lg flex items-center justify-center',
                svc.status === 'ok' && 'bg-green-100',
                svc.status === 'warning' && 'bg-yellow-100',
                svc.status === 'error' && 'bg-red-100',
              )}>
                <svg className={cn(
                  'w-5 h-5',
                  svc.status === 'ok' && 'text-green-600',
                  svc.status === 'warning' && 'text-yellow-600',
                  svc.status === 'error' && 'text-red-600',
                )} fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                    d={serviceIcons[svc.name] || 'M5 13l4 4L19 7'} />
                </svg>
              </div>
              <div className="min-w-0">
                <p className="text-xs font-semibold text-gray-700 truncate">
                  {serviceLabels[svc.name] || svc.name}
                </p>
                <div className="flex items-center space-x-1">
                  <span className={cn(
                    'inline-block w-2 h-2 rounded-full',
                    svc.status === 'ok' && 'bg-green-500',
                    svc.status === 'warning' && 'bg-yellow-500',
                    svc.status === 'error' && 'bg-red-500',
                  )} />
                  <span className={cn(
                    'text-xs',
                    svc.status === 'ok' && 'text-green-700',
                    svc.status === 'warning' && 'text-yellow-700',
                    svc.status === 'error' && 'text-red-700',
                  )}>
                    {svc.status === 'ok' ? 'Conectado' : svc.status === 'warning' ? 'Atencao' : 'Offline'}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

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
