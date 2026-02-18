'use client'

import { useEffect, useState } from 'react'
import { cn, formatDate } from '@/lib/utils'
import { ExecutionListItem, ExecutionStats } from '@/types'

// Which direction is "better" for each metric
// 'higher' = higher is better (green up), 'lower' = lower is better (green down)
const metrics: { label: string; key: keyof ExecutionStats; unit: string; better: 'higher' | 'lower' }[] = [
  { label: 'Requests Made', key: 'requests', unit: '', better: 'higher' },
  { label: 'HTTP Failures', key: 'failures', unit: '', better: 'lower' },
  { label: 'Peak RPS', key: 'peak_rps', unit: '', better: 'higher' },
  { label: 'Error Rate', key: 'error_rate', unit: '%', better: 'lower' },
  { label: 'Avg Response Time', key: 'avg_response', unit: ' ms', better: 'lower' },
  { label: 'Max Response Time', key: 'max_response', unit: ' ms', better: 'lower' },
  { label: 'P90 Response', key: 'p90', unit: ' ms', better: 'lower' },
  { label: 'P95 Response', key: 'p95', unit: ' ms', better: 'lower' },
  { label: 'VUs Max', key: 'vus_max', unit: '', better: 'higher' },
  { label: 'Req per VU', key: 'req_per_vu', unit: '', better: 'higher' },
]

function formatValue(value: number, unit: string): string {
  if (unit === '%') return `${value.toFixed(2)}%`
  if (unit === ' ms') return `${value.toFixed(1)} ms`
  if (Number.isInteger(value)) return value.toLocaleString()
  return value.toFixed(1)
}

function formatExecLabel(item: ExecutionListItem): string {
  const date = formatDate(item.created_at)
  return `${item.test_name} — ${item.domain_name} — ${item.vus} VUs — ${date}`
}

export default function AnalyticsPage() {
  const [executions, setExecutions] = useState<ExecutionListItem[]>([])
  const [execA, setExecA] = useState<string>('')
  const [execB, setExecB] = useState<string>('')
  const [statsA, setStatsA] = useState<ExecutionStats | null>(null)
  const [statsB, setStatsB] = useState<ExecutionStats | null>(null)
  const [loadingList, setLoadingList] = useState(true)
  const [loadingA, setLoadingA] = useState(false)
  const [loadingB, setLoadingB] = useState(false)

  // Fetch execution list
  useEffect(() => {
    fetch('/metrics-api/executions/list')
      .then((res) => res.json())
      .then((data) => setExecutions(data))
      .catch(() => {})
      .finally(() => setLoadingList(false))
  }, [])

  // Fetch stats for exec A
  useEffect(() => {
    if (!execA) { setStatsA(null); return }
    setLoadingA(true)
    fetch(`/metrics-api/executions/${execA}/stats`)
      .then((res) => res.json())
      .then((data) => setStatsA(data))
      .catch(() => setStatsA(null))
      .finally(() => setLoadingA(false))
  }, [execA])

  // Fetch stats for exec B
  useEffect(() => {
    if (!execB) { setStatsB(null); return }
    setLoadingB(true)
    fetch(`/metrics-api/executions/${execB}/stats`)
      .then((res) => res.json())
      .then((data) => setStatsB(data))
      .catch(() => setStatsB(null))
      .finally(() => setLoadingB(false))
  }, [execB])

  const computeDelta = (a: number, b: number): { pct: number; improved: boolean } | null => {
    if (a === 0 && b === 0) return null
    if (a === 0) return { pct: 100, improved: false }
    const pct = ((b - a) / Math.abs(a)) * 100
    return { pct, improved: false } // direction set per-metric below
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Analytics</h1>
        <p className="mt-1 text-sm text-gray-500">Comparacao de Execucoes K6</p>
      </div>

      {/* Selectors */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-8">
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Execucao A (base)</label>
          <select
            value={execA}
            onChange={(e) => setExecA(e.target.value)}
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
            disabled={loadingList}
          >
            <option value="">Selecione uma execucao...</option>
            {executions.map((item) => (
              <option key={item.id} value={item.id} disabled={item.id === execB}>
                {formatExecLabel(item)}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Execucao B (comparar)</label>
          <select
            value={execB}
            onChange={(e) => setExecB(e.target.value)}
            className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
            disabled={loadingList}
          >
            <option value="">Selecione uma execucao...</option>
            {executions.map((item) => (
              <option key={item.id} value={item.id} disabled={item.id === execA}>
                {formatExecLabel(item)}
              </option>
            ))}
          </select>
        </div>
      </div>

      {/* Loading state */}
      {(loadingA || loadingB) && (
        <div className="text-sm text-gray-400 mb-4">Carregando metricas...</div>
      )}

      {/* Comparison Table */}
      {statsA && statsB && (
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Metrica</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Exec A</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Exec B</th>
                  <th className="px-6 py-3 text-right text-xs font-medium text-gray-500 uppercase">Delta</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {metrics.map((m) => {
                  const valA = statsA[m.key]
                  const valB = statsB[m.key]
                  const delta = computeDelta(valA, valB)

                  let deltaColor = 'text-gray-400'
                  let arrow = ''
                  if (delta && Math.abs(delta.pct) > 0.01) {
                    const isUp = delta.pct > 0
                    const isImprovement = m.better === 'higher' ? isUp : !isUp
                    deltaColor = isImprovement ? 'text-green-600' : 'text-red-600'
                    arrow = isUp ? '\u2191' : '\u2193'
                  }

                  return (
                    <tr key={m.key} className="hover:bg-gray-50">
                      <td className="px-6 py-3 text-sm font-medium text-gray-900">{m.label}</td>
                      <td className="px-6 py-3 text-sm text-gray-700 text-right font-mono">
                        {formatValue(valA, m.unit)}
                      </td>
                      <td className="px-6 py-3 text-sm text-gray-700 text-right font-mono">
                        {formatValue(valB, m.unit)}
                      </td>
                      <td className={cn('px-6 py-3 text-sm text-right font-mono font-semibold', deltaColor)}>
                        {delta && Math.abs(delta.pct) > 0.01
                          ? `${arrow} ${Math.abs(delta.pct).toFixed(1)}%`
                          : '—'}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Empty state */}
      {!loadingA && !loadingB && (!statsA || !statsB) && (execA || execB) && (
        <div className="text-center py-12 text-gray-400">
          Selecione duas execucoes para comparar
        </div>
      )}

      {!execA && !execB && !loadingList && (
        <div className="text-center py-12 text-gray-400">
          Selecione duas execucoes acima para ver a comparacao
        </div>
      )}
    </div>
  )
}
