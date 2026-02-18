'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { api } from '@/lib/api'
import { Test, Schedule } from '@/types'

export default function NewSchedulePage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [tests, setTests] = useState<Test[]>([])
  const [testId, setTestId] = useState(searchParams.get('test_id') || '')
  const [scheduleType, setScheduleType] = useState<'ONCE' | 'RECURRING'>('ONCE')
  const [cronExpression, setCronExpression] = useState('')
  const [nextRunAt, setNextRunAt] = useState('')
  const [vus, setVus] = useState('1')
  const [duration, setDuration] = useState('30s')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.get<Test[]>('/tests').then((res) => {
      if (res.success && res.data) {
        setTests(res.data)
        // Pre-fill VUs/duration from selected test
        if (testId) {
          const test = res.data.find((t) => t.id === testId)
          if (test) {
            setVus(String(test.default_vus))
            setDuration(test.default_duration)
          }
        }
      }
    })
  }, [])

  const handleTestChange = (id: string) => {
    setTestId(id)
    const test = tests.find((t) => t.id === id)
    if (test) {
      setVus(String(test.default_vus))
      setDuration(test.default_duration)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const body: Record<string, unknown> = {
      test_id: testId,
      schedule_type: scheduleType,
      vus: parseInt(vus) || 1,
      duration: duration || '30s',
    }

    if (scheduleType === 'RECURRING') {
      if (!cronExpression) {
        setError('Cron expression is required for recurring schedules')
        setLoading(false)
        return
      }
      body.cron_expression = cronExpression
    } else {
      if (!nextRunAt) {
        setError('Date/time is required for one-time schedules')
        setLoading(false)
        return
      }
      body.next_run_at = new Date(nextRunAt).toISOString()
    }

    const res = await api.post<Schedule>('/schedules', body)
    if (res.success) {
      router.push('/schedules')
    } else {
      setError(res.error?.message || 'Failed to create schedule')
    }
    setLoading(false)
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">New Schedule</h1>

      <form onSubmit={handleSubmit} className="space-y-4 bg-white p-6 rounded-xl shadow-sm border border-gray-200">
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Test</label>
          <select value={testId} onChange={(e) => handleTestChange(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required>
            <option value="">Select...</option>
            {tests.map((t) => <option key={t.id} value={t.id}>{t.name} ({t.domain_name})</option>)}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Type</label>
          <div className="flex space-x-4">
            <label className="flex items-center">
              <input type="radio" value="ONCE" checked={scheduleType === 'ONCE'}
                onChange={() => setScheduleType('ONCE')} className="mr-2" />
              <span className="text-sm text-gray-700">Once</span>
            </label>
            <label className="flex items-center">
              <input type="radio" value="RECURRING" checked={scheduleType === 'RECURRING'}
                onChange={() => setScheduleType('RECURRING')} className="mr-2" />
              <span className="text-sm text-gray-700">Recurring</span>
            </label>
          </div>
        </div>

        {scheduleType === 'ONCE' ? (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Date/Time</label>
            <input type="datetime-local" value={nextRunAt} onChange={(e) => setNextRunAt(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
          </div>
        ) : (
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Cron Expression</label>
            <input type="text" value={cronExpression} onChange={(e) => setCronExpression(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono"
              placeholder="*/30 * * * *" required />
            <p className="mt-1 text-xs text-gray-400">Format: minute hour day month day_of_week</p>
          </div>
        )}

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">VUs</label>
            <input type="number" value={vus} onChange={(e) => setVus(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg" min="1" max="20" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Duration</label>
            <input type="text" value={duration} onChange={(e) => setDuration(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg" placeholder="30s" />
          </div>
        </div>

        <div className="flex space-x-3">
          <button type="submit" disabled={loading}
            className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
            {loading ? 'Creating...' : 'Create Schedule'}
          </button>
          <button type="button" onClick={() => router.back()}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Cancel
          </button>
        </div>
      </form>
    </div>
  )
}
