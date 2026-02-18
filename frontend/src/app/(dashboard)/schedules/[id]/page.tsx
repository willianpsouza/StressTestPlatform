'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { Schedule } from '@/types'

export default function ScheduleDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [schedule, setSchedule] = useState<Schedule | null>(null)

  const loadSchedule = () => {
    api.get<Schedule>(`/schedules/${params.id}`).then((res) => {
      if (res.success && res.data) setSchedule(res.data)
    })
  }

  useEffect(() => {
    loadSchedule()
  }, [params.id])

  const handlePause = async () => {
    const res = await api.post(`/schedules/${params.id}/pause`)
    if (res.success) loadSchedule()
  }

  const handleResume = async () => {
    const res = await api.post(`/schedules/${params.id}/resume`)
    if (res.success) loadSchedule()
  }

  const handleDelete = async () => {
    if (!confirm('Excluir este agendamento?')) return
    const res = await api.delete(`/schedules/${params.id}`)
    if (res.success) router.push('/schedules')
  }

  if (!schedule) return <div className="text-gray-400">Carregando...</div>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Agendamento</h1>
          <p className="text-sm text-gray-500">Teste: {schedule.test_name || schedule.test_id}</p>
        </div>
        <div className="flex space-x-2">
          {schedule.status === 'ACTIVE' && (
            <button onClick={handlePause}
              className="px-4 py-2 bg-yellow-50 text-yellow-700 text-sm font-medium rounded-lg hover:bg-yellow-100">
              Pausar
            </button>
          )}
          {schedule.status === 'PAUSED' && (
            <button onClick={handleResume}
              className="px-4 py-2 bg-green-50 text-green-700 text-sm font-medium rounded-lg hover:bg-green-100">
              Retomar
            </button>
          )}
          <button onClick={handleDelete}
            className="px-4 py-2 bg-red-50 text-red-600 text-sm font-medium rounded-lg hover:bg-red-100">
            Excluir
          </button>
          <Link href={`/tests/${schedule.test_id}`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Ver Teste
          </Link>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Status</p>
            <span className={cn('px-3 py-1 text-sm font-medium rounded-full', statusColors[schedule.status])}>
              {schedule.status}
            </span>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Tipo</p>
            <p className="text-sm font-semibold text-gray-900">
              {schedule.schedule_type === 'ONCE' ? 'Unico' : 'Recorrente'}
            </p>
          </div>

          {schedule.cron_expression && (
            <div>
              <p className="text-xs font-medium text-gray-500 uppercase mb-1">Expressao Cron</p>
              <p className="text-sm font-mono text-gray-900">{schedule.cron_expression}</p>
            </div>
          )}

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">VUs</p>
            <p className="text-sm font-semibold text-gray-900">{schedule.vus}</p>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Duracao</p>
            <p className="text-sm font-semibold text-gray-900">{schedule.duration}</p>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Proxima Execucao</p>
            <p className="text-sm text-gray-900">{schedule.next_run_at ? formatDate(schedule.next_run_at) : '-'}</p>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Ultima Execucao</p>
            <p className="text-sm text-gray-900">{schedule.last_run_at ? formatDate(schedule.last_run_at) : '-'}</p>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Total de Execucoes</p>
            <p className="text-sm font-semibold text-gray-900">{schedule.run_count}</p>
          </div>

          <div>
            <p className="text-xs font-medium text-gray-500 uppercase mb-1">Criado em</p>
            <p className="text-sm text-gray-900">{formatDate(schedule.created_at)}</p>
          </div>
        </div>
      </div>
    </div>
  )
}
