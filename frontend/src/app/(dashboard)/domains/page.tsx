'use client'

import { useEffect, useState } from 'react'
import Link from 'next/link'
import { api } from '@/lib/api'
import { formatDate } from '@/lib/utils'
import { Domain } from '@/types'

export default function DomainsPage() {
  const [domains, setDomains] = useState<Domain[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api.get<Domain[]>('/domains').then((res) => {
      if (res.success && res.data) setDomains(res.data)
      setLoading(false)
    })
  }, [])

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Dominios</h1>
        <Link href="/domains/new" className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700">
          Novo Dominio
        </Link>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        {loading ? (
          <div className="p-8 text-center text-gray-400">Carregando...</div>
        ) : domains.length === 0 ? (
          <div className="p-8 text-center text-gray-400">Nenhum dominio encontrado</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Nome</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Descricao</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Criado em</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Acoes</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200">
                {domains.map((domain) => (
                  <tr key={domain.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 text-sm font-medium text-primary-600">
                      <Link href={`/domains/${domain.id}`}>{domain.name}</Link>
                    </td>
                    <td className="px-6 py-4 text-sm text-gray-500">{domain.description || '-'}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{formatDate(domain.created_at)}</td>
                    <td className="px-6 py-4 text-sm space-x-2">
                      <Link href={`/domains/${domain.id}/edit`} className="text-primary-600 hover:text-primary-700">Editar</Link>
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
