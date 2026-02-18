'use client'

import { useEffect, useState, useCallback } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { TestExecution } from '@/types'

export default function ExecutionDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [exec, setExec] = useState<TestExecution | null>(null)
  const [cancelling, setCancelling] = useState(false)
  const [showStdout, setShowStdout] = useState(false)
  const [showStderr, setShowStderr] = useState(false)

  const loadExecution = useCallback(() => {
    api.get<TestExecution>(`/executions/${params.id}`).then((res) => {
      if (res.success && res.data) setExec(res.data)
    })
  }, [params.id])

  useEffect(() => {
    loadExecution()
  }, [loadExecution])

  // Polling every 3s while RUNNING/PENDING
  useEffect(() => {
    if (!exec || (exec.status !== 'RUNNING' && exec.status !== 'PENDING')) return

    const interval = setInterval(loadExecution, 3000)
    return () => clearInterval(interval)
  }, [exec?.status, loadExecution])

  const handleCancel = async () => {
    if (!confirm('Cancelar esta execucao?')) return
    setCancelling(true)
    const res = await api.post(`/executions/${params.id}/cancel`)
    if (res.success) loadExecution()
    setCancelling(false)
  }

  if (!exec) return <div className="text-gray-400">Carregando...</div>

  const isActive = exec.status === 'RUNNING' || exec.status === 'PENDING'
  const metrics = exec.metrics_summary as Record<string, unknown> | undefined

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">Execucao</h1>
          <p className="text-sm text-gray-500">
            Teste: {exec.test_name || exec.test_id} | Criado em: {formatDate(exec.created_at)}
          </p>
        </div>
        <div className="flex space-x-2">
          {isActive && (
            <button onClick={handleCancel} disabled={cancelling}
              className="px-4 py-2 bg-red-50 text-red-600 text-sm font-medium rounded-lg hover:bg-red-100 disabled:opacity-50">
              {cancelling ? 'Cancelando...' : 'Cancelar'}
            </button>
          )}
          <Link href={`/tests/${exec.test_id}`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Ver Teste
          </Link>
          <button onClick={() => router.back()}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Voltar
          </button>
        </div>
      </div>

      {/* Status & Info */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-6">
        <InfoCard label="Status">
          <span className={cn('px-3 py-1 text-sm font-medium rounded-full', statusColors[exec.status])}>
            {exec.status}
            {isActive && <span className="ml-1 animate-pulse">...</span>}
          </span>
        </InfoCard>
        <InfoCard label="VUs" value={String(exec.vus)} />
        <InfoCard label="Duracao" value={exec.duration} />
        <InfoCard label="Exit Code" value={exec.exit_code !== undefined ? String(exec.exit_code) : '-'} />
      </div>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-6">
        <InfoCard label="Inicio" value={exec.started_at ? formatDate(exec.started_at) : 'Aguardando...'} />
        <InfoCard label="Fim" value={exec.completed_at ? formatDate(exec.completed_at) : '-'} />
      </div>

      {/* Error Message */}
      {exec.error_message && (
        <div className="bg-red-50 border border-red-200 rounded-xl p-4 mb-6">
          <h3 className="text-sm font-semibold text-red-800 mb-1">Erro</h3>
          <p className="text-sm text-red-700">{exec.error_message}</p>
        </div>
      )}

      {/* Metrics Summary */}
      {metrics && Object.keys(metrics).length > 0 && (
        <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-6">
          <h2 className="text-lg font-semibold text-gray-900 mb-4">Metricas</h2>
          <MetricsSummary metrics={metrics} />
        </div>
      )}

      {/* Stdout */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 mb-6">
        <button onClick={() => setShowStdout(!showStdout)}
          className="w-full px-6 py-4 flex items-center justify-between hover:bg-gray-50">
          <h2 className="text-lg font-semibold text-gray-900">Stdout</h2>
          <svg className={cn('w-5 h-5 text-gray-400 transition-transform', showStdout && 'rotate-180')}
            fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {showStdout && (
          <div className="px-6 pb-4">
            <LogViewer content={exec.stdout || 'Sem output'} />
          </div>
        )}
      </div>

      {/* Stderr */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        <button onClick={() => setShowStderr(!showStderr)}
          className="w-full px-6 py-4 flex items-center justify-between hover:bg-gray-50">
          <h2 className="text-lg font-semibold text-gray-900">Stderr</h2>
          <svg className={cn('w-5 h-5 text-gray-400 transition-transform', showStderr && 'rotate-180')}
            fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>
        {showStderr && (
          <div className="px-6 pb-4">
            <LogViewer content={exec.stderr || 'Sem output'} />
          </div>
        )}
      </div>
    </div>
  )
}

function InfoCard({ label, value, children }: { label: string; value?: string; children?: React.ReactNode }) {
  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
      <p className="text-xs font-medium text-gray-500 uppercase mb-1">{label}</p>
      {children || <p className="text-lg font-semibold text-gray-900">{value}</p>}
    </div>
  )
}

function LogViewer({ content }: { content: string }) {
  return (
    <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 text-xs font-mono overflow-x-auto max-h-96 whitespace-pre-wrap">
      {content}
    </pre>
  )
}

function MetricsSummary({ metrics }: { metrics: Record<string, unknown> }) {
  // K6 metrics_summary usually contains things like http_reqs, http_req_duration, etc.
  const formatMetricValue = (value: unknown): string => {
    if (typeof value === 'number') {
      return value % 1 === 0 ? String(value) : value.toFixed(2)
    }
    if (typeof value === 'object' && value !== null) {
      const obj = value as Record<string, unknown>
      // K6 trend metrics have avg, min, max, p(90), p(95), etc.
      const parts: string[] = []
      if ('avg' in obj) parts.push(`avg: ${formatMetricValue(obj.avg)}`)
      if ('min' in obj) parts.push(`min: ${formatMetricValue(obj.min)}`)
      if ('max' in obj) parts.push(`max: ${formatMetricValue(obj.max)}`)
      if ('p(90)' in obj) parts.push(`p90: ${formatMetricValue(obj['p(90)'])}`)
      if ('p(95)' in obj) parts.push(`p95: ${formatMetricValue(obj['p(95)'])}`)
      if ('count' in obj) parts.push(`count: ${formatMetricValue(obj.count)}`)
      if ('rate' in obj) parts.push(`rate: ${formatMetricValue(obj.rate)}`)
      if (parts.length > 0) return parts.join(' | ')
      return JSON.stringify(value)
    }
    return String(value)
  }

  return (
    <div className="grid grid-cols-1 gap-3">
      {Object.entries(metrics).map(([key, value]) => (
        <div key={key} className="flex items-start justify-between py-2 border-b border-gray-100 last:border-0">
          <span className="text-sm font-medium text-gray-700 font-mono">{key}</span>
          <span className="text-sm text-gray-500 text-right max-w-md">{formatMetricValue(value)}</span>
        </div>
      ))}
    </div>
  )
}
