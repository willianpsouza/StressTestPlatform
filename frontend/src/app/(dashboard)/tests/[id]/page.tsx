'use client'

import { useEffect, useState, useCallback } from 'react'
import { useParams, useRouter } from 'next/navigation'
import Link from 'next/link'
import dynamic from 'next/dynamic'
import { api } from '@/lib/api'
import { cn, formatDate, statusColors } from '@/lib/utils'
import { Test, TestExecution } from '@/types'

const MonacoEditor = dynamic(() => import('@monaco-editor/react'), { ssr: false })

export default function TestDetailPage() {
  const params = useParams()
  const router = useRouter()
  const [test, setTest] = useState<Test | null>(null)
  const [executions, setExecutions] = useState<TestExecution[]>([])
  const [vus, setVus] = useState('')
  const [duration, setDuration] = useState('')
  const [running, setRunning] = useState(false)

  // Script editor state
  const [scriptContent, setScriptContent] = useState<string | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)
  const [saving, setSaving] = useState(false)
  const [saveMsg, setSaveMsg] = useState<{ type: 'ok' | 'error'; text: string } | null>(null)
  const [editorDirty, setEditorDirty] = useState(false)
  const [originalContent, setOriginalContent] = useState('')

  useEffect(() => {
    api.get<Test>(`/tests/${params.id}`).then((res) => {
      if (res.success && res.data) {
        setTest(res.data)
        setVus(String(res.data.default_vus))
        setDuration(res.data.default_duration)
      }
    })
    loadExecutions()
  }, [params.id])

  const loadExecutions = () => {
    api.get<TestExecution[]>(`/executions?test_id=${params.id}&page_size=10`).then((res) => {
      if (res.success && res.data) setExecutions(res.data)
    })
  }

  const loadScript = useCallback(async () => {
    const res = await api.get<{ content: string }>(`/tests/${params.id}/script/content`)
    if (res.success && res.data) {
      setScriptContent(res.data.content)
      setOriginalContent(res.data.content)
      setEditorDirty(false)
    }
  }, [params.id])

  const handleToggleEditor = () => {
    if (!editorOpen && scriptContent === null) {
      loadScript()
    }
    setEditorOpen(!editorOpen)
    setSaveMsg(null)
  }

  const handleSaveScript = async () => {
    if (scriptContent === null) return
    setSaving(true)
    setSaveMsg(null)

    const res = await api.put<Test>(`/tests/${params.id}/script/content`, {
      content: scriptContent,
    })

    if (res.success && res.data) {
      setTest(res.data)
      setOriginalContent(scriptContent)
      setEditorDirty(false)
      setSaveMsg({ type: 'ok', text: 'Script salvo com sucesso' })
    } else {
      setSaveMsg({ type: 'error', text: res.error?.message || 'Falha ao salvar' })
    }
    setSaving(false)
  }

  const handleEditorChange = (value: string | undefined) => {
    const v = value || ''
    setScriptContent(v)
    setEditorDirty(v !== originalContent)
    setSaveMsg(null)
  }

  const handleRun = async () => {
    setRunning(true)
    const res = await api.post<TestExecution>('/executions', {
      test_id: params.id,
      vus: parseInt(vus) || 1,
      duration: duration || '30s',
    })
    if (res.success) {
      loadExecutions()
    }
    setRunning(false)
  }

  const handleDelete = async () => {
    if (!confirm('Tem certeza?')) return
    const res = await api.delete(`/tests/${params.id}`)
    if (res.success) router.push('/tests')
  }

  if (!test) return <div className="text-gray-400">Carregando...</div>

  const grafanaUrl = `/grafana/d/k6-metrics?var-domain=${encodeURIComponent(test.domain_name || '')}&var-test=${encodeURIComponent(test.name)}`

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900">{test.name}</h1>
          <p className="text-sm text-gray-500">Dominio: {test.domain_name} | Script: {test.script_filename}</p>
          {test.description && <p className="mt-1 text-gray-500">{test.description}</p>}
        </div>
        <div className="flex space-x-2">
          <a href={grafanaUrl} target="_blank" rel="noopener noreferrer"
            className="px-4 py-2 bg-orange-50 text-orange-600 text-sm font-medium rounded-lg hover:bg-orange-100">
            Ver no Grafana
          </a>
          <Link href={`/tests/${test.id}/edit`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Editar
          </Link>
          <button onClick={handleDelete}
            className="px-4 py-2 bg-red-50 text-red-600 text-sm font-medium rounded-lg hover:bg-red-100">
            Excluir
          </button>
        </div>
      </div>

      {/* Run Test */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-6 mb-6">
        <h2 className="text-lg font-semibold text-gray-900 mb-4">Executar Teste</h2>
        <div className="flex items-end space-x-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">VUs</label>
            <input type="number" value={vus} onChange={(e) => setVus(e.target.value)}
              className="w-24 px-3 py-2 border border-gray-300 rounded-lg" min="1" max="20" />
          </div>
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Duracao</label>
            <input type="text" value={duration} onChange={(e) => setDuration(e.target.value)}
              className="w-32 px-3 py-2 border border-gray-300 rounded-lg" placeholder="30s" />
          </div>
          <button onClick={handleRun} disabled={running}
            className="px-6 py-2 bg-green-600 text-white text-sm font-medium rounded-lg hover:bg-green-700 disabled:opacity-50">
            {running ? 'Iniciando...' : 'Executar'}
          </button>
          <Link href={`/schedules/new?test_id=${test.id}`}
            className="px-4 py-2 bg-gray-100 text-gray-700 text-sm font-medium rounded-lg hover:bg-gray-200">
            Agendar
          </Link>
        </div>
      </div>

      {/* Script Editor */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 mb-6">
        <button
          onClick={handleToggleEditor}
          className="w-full px-6 py-4 flex items-center justify-between hover:bg-gray-50 rounded-t-xl"
        >
          <div className="flex items-center space-x-3">
            <svg className="w-5 h-5 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
            </svg>
            <h2 className="text-lg font-semibold text-gray-900">Editor de Script</h2>
            {editorDirty && (
              <span className="px-2 py-0.5 text-xs font-medium rounded-full bg-yellow-100 text-yellow-800">
                Nao salvo
              </span>
            )}
          </div>
          <svg className={cn('w-5 h-5 text-gray-400 transition-transform', editorOpen && 'rotate-180')}
            fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
          </svg>
        </button>

        {editorOpen && (
          <div className="border-t border-gray-200">
            {/* Toolbar */}
            <div className="flex items-center justify-between px-6 py-3 bg-gray-50 border-b border-gray-200">
              <div className="flex items-center space-x-3">
                <span className="text-xs font-medium text-gray-500 uppercase">
                  {test.script_filename} ({test.script_size_bytes} bytes)
                </span>
                {saveMsg && (
                  <span className={cn(
                    'text-xs font-medium',
                    saveMsg.type === 'ok' ? 'text-green-600' : 'text-red-600'
                  )}>
                    {saveMsg.text}
                  </span>
                )}
              </div>
              <div className="flex items-center space-x-2">
                <button
                  onClick={loadScript}
                  className="px-3 py-1.5 text-xs font-medium text-gray-600 bg-white border border-gray-300 rounded-lg hover:bg-gray-50"
                >
                  Recarregar
                </button>
                <button
                  onClick={handleSaveScript}
                  disabled={saving || !editorDirty}
                  className={cn(
                    'px-4 py-1.5 text-xs font-medium rounded-lg',
                    editorDirty
                      ? 'bg-primary-600 text-white hover:bg-primary-700'
                      : 'bg-gray-200 text-gray-400 cursor-not-allowed',
                    saving && 'opacity-50'
                  )}
                >
                  {saving ? 'Salvando...' : 'Salvar Script'}
                </button>
              </div>
            </div>

            {/* Monaco Editor */}
            {scriptContent === null ? (
              <div className="p-8 text-center text-gray-400">Carregando script...</div>
            ) : (
              <MonacoEditor
                height="500px"
                language="javascript"
                theme="vs-dark"
                value={scriptContent}
                onChange={handleEditorChange}
                options={{
                  minimap: { enabled: false },
                  fontSize: 13,
                  lineNumbers: 'on',
                  scrollBeyondLastLine: false,
                  automaticLayout: true,
                  tabSize: 2,
                  wordWrap: 'on',
                  formatOnPaste: true,
                  formatOnType: true,
                  suggestOnTriggerCharacters: true,
                  quickSuggestions: true,
                  bracketPairColorization: { enabled: true },
                  padding: { top: 12 },
                }}
              />
            )}
          </div>
        )}
      </div>

      {/* Execution History */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200">
        <div className="px-6 py-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-gray-900">Historico de Execucoes</h2>
          <button onClick={loadExecutions} className="text-sm text-primary-600 hover:text-primary-700">Atualizar</button>
        </div>
        {executions.length === 0 ? (
          <div className="p-8 text-center text-gray-400">Nenhuma execucao</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50">
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Status</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">VUs</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Duracao</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Data</th>
                  <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase">Acoes</th>
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
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.vus}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{exec.duration}</td>
                    <td className="px-6 py-4 text-sm text-gray-500">{formatDate(exec.created_at)}</td>
                    <td className="px-6 py-4 text-sm">
                      <Link href={`/executions/${exec.id}`} className="text-primary-600 hover:text-primary-700">
                        Detalhes
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
