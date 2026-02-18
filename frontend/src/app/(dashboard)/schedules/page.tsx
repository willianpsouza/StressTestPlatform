'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { Schedule } from '@/types'

export default function SchedulesPage() {
  const [schedules, setSchedules] = useState<Schedule[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<Schedule[]>('/schedules?page_size=50').then((res) => {
      if (res.success && res.data) setSchedules(res.data)
      setLoading(false)
    })
  }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Agendamentos</h1>
        <Link href="/schedules/new"
          className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700">
          Novo Agendamento
        </Link>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        {loading ? (
          <div className="p-8 text-center text-gray-400">Carregando...</div>
        ) : schedules.length === 0 ? (
          <div className="p-8 text-center text-gray-400">Nenhum agendamento encontrado</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Teste</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Tipo</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Cron</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duracao</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Proxima Exec.</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Execucoes</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Acoes</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {schedules.map((s) => (
                  <tr key={s.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">{s.test_name || '-'}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{s.schedule_type}</td>
                    <td className="px-6 py-4 text-sm text-gray-500 font-mono">{s.cron_expression || '-'}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{s.vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{s.duration}</td>
                    <td className="px-6 py-4">
                      <span className={cn('px-2 py-1 text-xs font-medium rounded-full', statusColors[s.status])}>
                        {s.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {s.next_run_at ? formatDate(s.next_run_at) : '-'}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{s.run_count}</td>
                    <td className="px-6 py-4 text-sm">
                      <Link href={`/schedules/${s.id}`} className="text-primary-600 hover:text-primary-700">
                        Detalhes
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  )
}
