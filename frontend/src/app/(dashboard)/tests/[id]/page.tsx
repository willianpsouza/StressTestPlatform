'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { Test, TestExecution } from '@/types'

export default function TestDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [test, setTest] = useState<Test | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [vus, setVus] = useState('')
  const [duration, setDuration] = useState('')
  const [running, setRunning] = useState(false)

  useEffect(() => {
    api.get<Test>(`/tests/${params.id}`).then((res) => {
      if (res.success && res.data) {
        setTest(res.data)
        setVus(String(res.data.default_vus))
        setDuration(res.data.default_duration)
      }
    })
    loadExecutions()
  }, [params.id])

  const loadExecutions = () => {
    api.get<TestExecution[]>(`/executions?test_id=${params.id}&page_size=10`).then((res) => {
      if (res.success && res.data) setExecutions(res.data)
    })
  }

  const handleRun = async () => {
    setRunning(true)
    const res = await api.post<TestExecution>('/executions', {
      test_id: params.id,
      vus: parseInt(vus) || 1,
      duration: duration || '30s',
    })
    if (res.success) {
      loadExecutions()
    }
    setRunning(false)
  }

  const handleDelete = async () => {
    if (!confirm('Tem certeza?')) return
    const res = await api.delete(`/tests/${params.id}`)
    if (res.success) router.push('/tests')
  }

  if (!test) return <div className="text-gray-400">Carregando...</div>

  const grafanaUrl = `/grafana/d/k6-metrics?var-bucket=${test.influxdb_bucket}`

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{test.name}</h1>
          <p className="text-sm text-gray-500">Dominio: {test.domain_name} | Script: {test.script_filename}</p>
          {test.description && <p className="mt-1 text-gray-500">{test.description}</p>}
        </div>
        <div className="flex space-x-2">
          <a href={grafanaUrl} target="_blank" rel="noopener noreferrer"
            className="px-4 py-2 bg-orange-50 text-orange-600 text-sm font-medium rounded-lg hover:bg-orange-100">
            Ver no Grafana
          </a>
          <Link href={`/tests/${test.id}/edit`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Editar
          </Link>
          <button onClick={handleDelete}
            className="px-4 py-2 bg-red-50 text-red-600 text-sm font-medium rounded-lg hover:bg-red-100">
            Excluir
          </button>
        </div>
      </div>

      {/* Run Test */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Executar Teste</h2>
        <div className="flex items-end space-x-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">VUs</label>
            <input type="number" value={vus} onChange={(e) => setVus(e.target.value)}
              className="w-24 px-3 py-2 border border-gray-300 rounded-lg" min="1" max="20" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Duracao</label>
            <input type="text" value={duration} onChange={(e) => setDuration(e.target.value)}
              className="w-32 px-3 py-2 border border-gray-300 rounded-lg" placeholder="30s" />
          </div>
          <button onClick={handleRun} disabled={running}
            className="px-6 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700 disabled:opacity-50">
            {running ? 'Iniciando...' : 'Executar'}
          </button>
          <Link href={`/schedules/new?test_id=${test.id}`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Agendar
          </Link>
        </div>
      </div>

      {/* Execution History */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Historico de Execucoes</h2>
          <button onClick={loadExecutions} className="text-sm text-primary-600 hover:text-primary-700">Atualizar</button>
        </div>
        {executions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">Nenhuma execucao</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duracao</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Data</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Acoes</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {executions.map((exec) => (
                  <tr key={exec.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4">
                      <span className={cn('px-2 py-1 text-xs font-medium rounded-full', statusColors[exec.status])}>
                        {exec.status}
                      </span>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.duration}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{formatDate(exec.created_at)}</td>
                    <td className="px-6 py-4 text-sm">
                      <Link href={`/executions/${exec.id}`} className="text-primary-600 hover:text-primary-700">
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
