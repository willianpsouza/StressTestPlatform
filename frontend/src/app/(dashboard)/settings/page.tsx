'use client'

import { useEffect, useState } from 'react'
import { api } from '@/lib/api'
import { useAuthStore } from '@/lib/store'
import { User } from '@/types'

interface BucketInfo {
  name: string
  test_name: string
  test_id: string
}

export default function SettingsPage() {
  const user = useAuthStore((s) => s.user)

  return (
    <div className="max-w-2xl space-y-8">
      <h1 className="text-2xl font-bold text-gray-900">Configuracoes</h1>
      <ProfileSection user={user} />
      <PasswordSection />
      {user?.role === 'ROOT' && <GrafanaTokenSection />}
      <InfluxDBSection />
    </div>
  )
}

function ProfileSection({ user }: { user: User | null }) {
  const [name, setName] = useState(user?.name || '')
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const setUser = useAuthStore((s) => s.setUser)

  useEffect(() => {
    if (user) setName(user.name)
  }, [user])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)

    const res = await api.put<User>('/auth/me', { name })
    if (res.success && res.data) {
      setUser(res.data)
      setSuccess('Perfil atualizado com sucesso')
    } else {
      setError(res.error?.message || 'Falha ao atualizar perfil')
    }
    setLoading(false)
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Perfil</h2>

      <form onSubmit={handleSubmit} className="space-y-4">
        {success && <div className="p-3 text-sm text-green-700 bg-green-50 rounded-lg">{success}</div>}
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Email</label>
          <input type="email" value={user?.email || ''} disabled
            className="w-full px-3 py-2 border border-gray-200 rounded-lg bg-gray-50 text-gray-500" />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Nome</label>
          <input type="text" value={name} onChange={(e) => setName(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <button type="submit" disabled={loading}
          className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
          {loading ? 'Salvando...' : 'Salvar Perfil'}
        </button>
      </form>
    </div>
  )
}

function PasswordSection() {
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')

    if (newPassword !== confirmPassword) {
      setError('As senhas nao coincidem')
      return
    }
    if (newPassword.length < 8) {
      setError('A senha deve ter pelo menos 8 caracteres')
      return
    }

    setLoading(true)
    const res = await api.post('/auth/change-password', {
      current_password: currentPassword,
      new_password: newPassword,
    })

    if (res.success) {
      setSuccess('Senha alterada com sucesso')
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } else {
      setError(res.error?.message || 'Falha ao alterar senha')
    }
    setLoading(false)
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">Alterar Senha</h2>

      <form onSubmit={handleSubmit} className="space-y-4">
        {success && <div className="p-3 text-sm text-green-700 bg-green-50 rounded-lg">{success}</div>}
        {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Senha Atual</label>
          <input type="password" value={currentPassword} onChange={(e) => setCurrentPassword(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Nova Senha</label>
          <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <div>
          <label className="block text-sm font-medium text-gray-700 mb-1">Confirmar Nova Senha</label>
          <input type="password" value={confirmPassword} onChange={(e) => setConfirmPassword(e.target.value)}
            className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent" required />
        </div>

        <button type="submit" disabled={loading}
          className="px-4 py-2 bg-primary-600 text-white text-sm font-medium rounded-lg hover:bg-primary-700 disabled:opacity-50">
          {loading ? 'Alterando...' : 'Alterar Senha'}
        </button>
      </form>
    </div>
  )
}

function GrafanaTokenSection() {
  const [token, setToken] = useState('')
  const [currentToken, setCurrentToken] = useState('')
  const [success, setSuccess] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadingCurrent, setLoadingCurrent] = useState(true)

  useEffect(() => {
    api.get<Record<string, string>>('/settings').then((res) => {
      if (res.success && res.data) {
        setCurrentToken(res.data.grafana_token || '')
      }
      setLoadingCurrent(false)
    })
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setSuccess('')
    setLoading(true)

    const res = await api.put('/settings', { grafana_token: token })
    if (res.success) {
      setSuccess('Grafana token atualizado com sucesso')
      setCurrentToken(token.length > 8 ? token.slice(0, 4) + '...' + token.slice(-4) : token)
      setToken('')
    } else {
      setError(res.error?.message || 'Falha ao atualizar token')
    }
    setLoading(false)
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
      <div className="flex items-center space-x-3 mb-4">
        <div className="w-10 h-10 rounded-lg bg-orange-100 flex items-center justify-center">
          <svg className="w-5 h-5 text-orange-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
              d="M11 3.055A9.001 9.001 0 1020.945 13H11V3.055z M20.488 9H15V3.512A9.025 9.025 0 0120.488 9z" />
          </svg>
        </div>
        <div>
          <h2 className="text-lg font-semibold text-gray-900">Grafana API Token</h2>
          <p className="text-sm text-gray-500">
            Configure o token de servico para integracao com o Grafana.
          </p>
        </div>
      </div>

      {loadingCurrent ? (
        <div className="text-gray-400 text-sm">Carregando...</div>
      ) : (
        <>
          {currentToken && (
            <div className="mb-4 p-3 bg-gray-50 rounded-lg flex items-center justify-between">
              <div>
                <p className="text-xs font-medium text-gray-500 uppercase">Token atual</p>
                <p className="text-sm font-mono text-gray-700">{currentToken}</p>
              </div>
              <span className="px-2 py-1 text-xs font-medium rounded-full bg-green-100 text-green-800">
                Configurado
              </span>
            </div>
          )}

          {!currentToken && (
            <div className="mb-4 p-3 bg-yellow-50 border border-yellow-200 rounded-lg">
              <p className="text-sm text-yellow-800">
                Nenhum token configurado. O dashboard mostrara o Grafana como &quot;Atencao&quot;.
              </p>
              <p className="text-xs text-yellow-600 mt-1">
                Para gerar um token: acesse Grafana &gt; Administration &gt; Service accounts &gt; Add token.
              </p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {success && <div className="p-3 text-sm text-green-700 bg-green-50 rounded-lg">{success}</div>}
            {error && <div className="p-3 text-sm text-red-700 bg-red-50 rounded-lg">{error}</div>}

            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                {currentToken ? 'Novo Token' : 'Token'}
              </label>
              <input
                type="password"
                value={token}
                onChange={(e) => setToken(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-primary-500 focus:border-transparent font-mono"
                placeholder="glsa_..."
                required
              />
            </div>

            <button type="submit" disabled={loading}
              className="px-4 py-2 bg-orange-600 text-white text-sm font-medium rounded-lg hover:bg-orange-700 disabled:opacity-50">
              {loading ? 'Salvando...' : 'Salvar Token'}
            </button>
          </form>
        </>
      )}
    </div>
  )
}

function InfluxDBSection() {
  const [buckets, setBuckets] = useState<BucketInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [clearing, setClearing] = useState<string | null>(null)

  useEffect(() => {
    api.get<BucketInfo[]>('/influxdb/buckets').then((res) => {
      if (res.success && res.data) setBuckets(res.data)
      setLoading(false)
    })
  }, [])

  const handleClear = async (bucketName: string) => {
    if (!confirm(`Limpar todos os dados do bucket "${bucketName}"? Esta acao nao pode ser desfeita.`)) return
    setClearing(bucketName)
    await api.post(`/influxdb/buckets/${bucketName}/clear`)
    setClearing(null)
  }

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6">
      <h2 className="text-lg font-semibold text-gray-900 mb-4">InfluxDB Buckets</h2>
      <p className="text-sm text-gray-500 mb-4">Buckets de metricas associados aos seus testes.</p>

      {loading ? (
        <div className="text-gray-400 text-sm">Carregando...</div>
      ) : buckets.length === 0 ? (
        <div className="text-gray-400 text-sm">Nenhum bucket encontrado</div>
      ) : (
        <div className="space-y-3">
          {buckets.map((b) => (
            <div key={b.name} className="flex items-center justify-between py-3 px-4 bg-gray-50 rounded-lg">
              <div>
                <p className="text-sm font-medium text-gray-900">{b.name}</p>
                <p className="text-xs text-gray-500">Teste: {b.test_name}</p>
              </div>
              <button
                onClick={() => handleClear(b.name)}
                disabled={clearing === b.name}
                className="px-3 py-1 bg-red-50 text-red-600 text-xs font-medium rounded-lg hover:bg-red-100 disabled:opacity-50"
              >
                {clearing === b.name ? 'Limpando...' : 'Limpar Dados'}
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
