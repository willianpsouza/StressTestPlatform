'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/utils'
import { Test } from '@/types'

export default function TestsPage() {
  const [tests, setTests] = useState<Test[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<Test[]>('/tests').then((res) => {
      if (res.success && res.data) setTests(res.data)
      setLoading(false)
    })
  }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Tests</h1>
        <Link href="/tests/new" className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700">
          New Test
        </Link>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        {loading ? (
          <div className="p-8 text-center text-gray-400">Loading...</div>
        ) : tests.length === 0 ? (
          <div className="p-8 text-center text-gray-400">No tests found</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Name</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Domain</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Script</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duration</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Created at</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {tests.map((test) => (
                  <tr key={test.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-primary-600">
                      <Link href={`/tests/${test.id}`}>{test.name}</Link>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.domain_name}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.script_filename}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.default_vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{test.default_duration}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{formatDate(test.created_at)}</td>
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
