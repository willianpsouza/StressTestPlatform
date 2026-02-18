'use client'

import { useEffect, useState } from 'react'
import { useRouter, useSearchParams } from 'next/navigation'
import { api } from '@/lib/api'
import { Domain, Test } from '@/types'

export default function NewTestPage() {
  const router = useRouter()
  const searchParams = useSearchParams()
  const [domains, setDomains] = useState<Domain[]>([])
  const [domainId, setDomainId] = useState(searchParams.get('domain_id') || '')
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [defaultVus, setDefaultVus] = useState('1')
  const [defaultDuration, setDefaultDuration] = useState('30s')
  const [script, setScript] = useState<File | null>(null)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    api.get<Domain[]>('/domains').then((res) => {
      if (res.success && res.data) setDomains(res.data)
    })
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!script) { setError('Script is required'); return }
    setError('')
    setLoading(true)

    const formData = new FormData()
    formData.append('domain_id', domainId)
    formData.append('name', name)
    formData.append('description', description)
    formData.append('default_vus', defaultVus)
    formData.append('default_duration', defaultDuration)
    formData.append('script', script)

    const res = await api.post<Test>('/tests', formData)

    if (res.success) {
      router.push('/tests')
    } else {
      setError(res.error?.message || 'Failed to create test')
    }
    setLoading(false)
  }

  return (
    <div className="max-w-lg">
      <h1 className="text-2xl font-bold text-gray-900 mb-6">Novo Teste</h1>

      <form onSubmit={handleSubmit} className="space-y-4 bg-white p-6 rounded-xl shadow-sm border border-gray-200">
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Dominio</label>
          <select value={domainId} onChange={(e) => setDomainId(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required>
            <option value="">Selecione...</option>
            {domains.map((d) => <option key={d.id} value={d.id}>{d.name}</option>)}
          </select>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Descricao</label>
          <textarea value={description} onChange={(e) => setDescription(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" rows={2} />
        </div>

        <div className="grid grid-cols-2 gap-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">VUs Padrao</label>
            <input type="number" value={defaultVus} onChange={(e) => setDefaultVus(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" min="1" max="20" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Duracao Padrao</label>
            <input type="text" value={defaultDuration} onChange={(e) => setDefaultDuration(e.target.value)}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" placeholder="30s" />
          </div>
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Script K6 (.js)</label>
          <input type="file" accept=".js" onChange={(e) => setScript(e.target.files?.[0] || null)}
            className="w-full text-sm text-gray-500 file:mr-4 file:py-2 file:px-4 file:rounded-lg file:border-0 file:text-sm file:font-medium file:bg-primary-50 file:text-primary-700 hover:file:bg-primary-100" required />
        </div>

        <div className="flex space-x-3">
          <button type="submit" disabled={loading}
            className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
            {loading ? 'Criando...' : 'Criar Teste'}
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
