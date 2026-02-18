'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import { api } from '@/lib/api'
import { cn, formatDate } from '@/lib/utils'
import { Domain, Test, K6Overview } from '@/types'

export default function DomainDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [domain, setDomain] = useState<Domain | null>(null)
  const [tests, setTests] = useState<Test[]>([])
  const [k6Stats, setK6Stats] = useState<K6Overview | null>(null)

  useEffect(() => {
    api.get<Domain>(`/domains/${params.id}`).then((res) => {
      if (res.success && res.data) {
        setDomain(res.data)
        // Fetch K6 metrics for this domain
        fetch(`/metrics-api/dashboard/domain?name=${encodeURIComponent(res.data.name)}`)
          .then(r => r.ok ? r.json() : null)
          .then(data => { if (data) setK6Stats(data) })
          .catch(() => {})
      }
    })
    api.get<Test[]>(`/tests?domain_id=${params.id}`).then((res) => {
      if (res.success && res.data) setTests(res.data)
    })
  }, [params.id])

  const handleDelete = async () => {
    if (!confirm('Are you sure you want to delete this domain?')) return
    const res = await api.delete(`/domains/${params.id}`)
    if (res.success) router.push('/domains')
  }

  if (!domain) return <div className="text-gray-400">Loading...</div>

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{domain.name}</h1>
          {domain.description && <p className="mt-1 text-gray-500">{domain.description}</p>}
          <p className="mt-1 text-sm text-gray-400">Created at {formatDate(domain.created_at)}</p>
        </div>
        <div className="flex space-x-2">
          <Link href={`/domains/${domain.id}/edit`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Edit
          </Link>
          <button onClick={handleDelete}
            className="px-4 py-2 bg-red-50 text-red-600 text-sm font-medium rounded-lg hover:bg-red-100">
            Delete
          </button>
        </div>
      </div>

      {/* K6 Metrics */}
      {k6Stats && k6Stats.total_data_points > 0 && (
        <div className="mb-6">
          <h2 className="text-sm font-semibold text-gray-500 uppercase tracking-wider mb-3">K6 Metrics</h2>
          <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
              <p className="text-xs text-gray-500">Total Requests</p>
              <p className="text-2xl font-bold text-gray-900 mt-1">{k6Stats.total_requests.toLocaleString()}</p>
            </div>
            <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
              <p className="text-xs text-gray-500">Error Rate</p>
              <p className={cn('text-2xl font-bold mt-1', k6Stats.error_rate > 5 ? 'text-red-600' : k6Stats.error_rate > 1 ? 'text-yellow-600' : 'text-green-600')}>
                {k6Stats.error_rate}%
              </p>
            </div>
            <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
              <p className="text-xs text-gray-500">Avg Response</p>
              <p className={cn('text-2xl font-bold mt-1', k6Stats.avg_response_ms > 500 ? 'text-red-600' : k6Stats.avg_response_ms > 200 ? 'text-yellow-600' : 'text-green-600')}>
                {k6Stats.avg_response_ms} ms
              </p>
            </div>
            <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
              <p className="text-xs text-gray-500">P95 Response</p>
              <p className={cn('text-2xl font-bold mt-1', k6Stats.p95_response_ms > 500 ? 'text-red-600' : k6Stats.p95_response_ms > 200 ? 'text-yellow-600' : 'text-green-600')}>
                {k6Stats.p95_response_ms} ms
              </p>
            </div>
          </div>
        </div>
      )}

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Tests ({tests.length})</h2>
          <Link href={`/tests/new?domain_id=${domain.id}`}
            className="px-3 py-1.5 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700">
            New Test
          </Link>
        </div>
        {tests.length === 0 ? (
          <div className="p-8 text-center text-gray-400">No tests in this domain</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Script</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duration</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {tests.map((test) => (
                  <tr key={test.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-primary-600">
                      <Link href={`/tests/${test.id}`}>{test.name}</Link>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.script_filename}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.default_vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.default_duration}</td>
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
