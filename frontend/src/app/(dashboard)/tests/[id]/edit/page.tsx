'use client'

import { useEffect, useState } from 'react'
import { useParams, useRouter } from 'next/navigation'
import { api } from '@/lib/api'
import { Test } from '@/types'

export default function EditTestPage() {
  const params = useParams()
  const router = useRouter()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [defaultVus, setDefaultVus] = useState('1')
  const [defaultDuration, setDefaultDuration] = useState('30s')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.get<Test>(`/tests/${params.id}`).then((res) => {
      if (res.success && res.data) {
        setName(res.data.name)
        setDescription(res.data.description || '')
        setDefaultVus(String(res.data.default_vus))
        setDefaultDuration(res.data.default_duration)
      }
    })
  }, [params.id])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const res = await api.put<Test>(`/tests/${params.id}`, {
      name,
      description: description || undefined,
      default_vus: parseInt(defaultVus),
      default_duration: defaultDuration,
    })
    if (res.success) {
      router.push(`/tests/${params.id}`)
    } else {
      setError(res.error?.message || 'Failed to update')
    }
    setLoading(false)
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Edit Test</h1>

      <form onSubmit={handleSubmit} className="space-y-4 bg-white p-6 rounded-xl shadow-sm border border-gray-200">
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Description</label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" rows={2} />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Default VUs</label>
            <input type="number" value={defaultVus} onChange={(e) => setDefaultVus(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg" min="1" max="20" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Default Duration</label>
            <input type="text" value={defaultDuration} onChange={(e) => setDefaultDuration(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg" placeholder="30s" />
          </div>
        </div>

        <div className="flex space-x-3">
          <button type="submit" disabled={loading}
            className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
            {loading ? 'Saving...' : 'Save'}
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
