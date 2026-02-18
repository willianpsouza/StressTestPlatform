'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { TestExecution } from '@/types'

export default function ExecutionsPage() {
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [loading, setLoading] = useState(true)
  const [statusFilter, setStatusFilter] = useState('')

  const loadExecutions = () => {
    const params = new URLSearchParams({ page_size: '50' })
    if (statusFilter) params.set('status', statusFilter)

    api.get<TestExecution[]>(`/executions?${params}`).then((res) => {
      if (res.success && res.data) setExecutions(res.data)
      setLoading(false)
    })
  }

  useEffect(() => {
    loadExecutions()
  }, [statusFilter])

  // Auto-refresh if any execution is RUNNING or PENDING
  useEffect(() => {
    const hasActive = executions.some((e) => e.status === 'RUNNING' || e.status === 'PENDING')
    if (!hasActive) return

    const interval = setInterval(loadExecutions, 3000)
    return () => clearInterval(interval)
  }, [executions])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Executions</h1>
        <div className="flex items-center space-x-3">
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2 border border-gray-300 rounded-lg text-sm"
          >
            <option value="">All Statuses</option>
            <option value="PENDING">PENDING</option>
            <option value="RUNNING">RUNNING</option>
            <option value="COMPLETED">COMPLETED</option>
            <option value="FAILED">FAILED</option>
            <option value="CANCELLED">CANCELLED</option>
            <option value="TIMEOUT">TIMEOUT</option>
          </select>
          <button onClick={loadExecutions} className="text-sm text-primary-600 hover:text-primary-700">
            Refresh
          </button>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        {loading ? (
          <div className="p-8 text-center text-gray-400">Loading...</div>
        ) : executions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">No executions found</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Test</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duration</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Start</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">End</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Actions</th>
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
                    <td className="px-6 py-4 text-sm font-medium text-gray-900">{exec.test_name || '-'}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.duration}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {exec.started_at ? formatDate(exec.started_at) : '-'}
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">
                      {exec.completed_at ? formatDate(exec.completed_at) : '-'}
                    </td>
                    <td className="px-6 py-4 text-sm">
                      <Link href={`/executions/${exec.id}`} className="text-primary-600 hover:text-primary-700">
                        Details
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
