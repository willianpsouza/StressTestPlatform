'use client'

import { useState } from 'react'
import { useRouter } from 'next/navigation'
import { api } from '@/lib/api'
import { Domain } from '@/types'

export default function NewDomainPage() {
  const router = useRouter()
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    const res = await api.post<Domain>('/domains', { name, description: description || undefined })

    if (res.success) {
      router.push('/domains')
    } else {
      setError(res.error?.message || 'Failed to create domain')
    }
    setLoading(false)
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Novo Dominio</h1>

      <form onSubmit={handleSubmit} className="space-y-4 bg-white p-6 rounded-xl shadow-sm border border-gray-200">
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Descricao</label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" rows={3} />
        </div>

        <div className="flex space-x-3">
          <button type="submit" disabled={loading}
            className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
            {loading ? 'Criando...' : 'Criar'}
          </button>
          <button type="button" onClick={() => router.back()}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Cancelar
          </button>
        </div>
      </form>
    </div>
  )
}
