'use client'

import { useEffect, useState, useCallback } from 'react'
import {
  AreaChart,
  Area,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface VarItem {
  __text: string
  __value: string
}

interface StatsData {
  requests: number
  failures: number
  peak_rps: number
  error_rate: number
  avg_response: number
  p90: number
  p95: number
  max_response: number
  vus_max: number
  req_per_vu: number
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const TIME_RANGES = [
  { label: 'Last Hour', seconds: 3600, interval: 5 },
  { label: 'Last 24 Hours', seconds: 86400, interval: 60 },
  { label: 'A Week', seconds: 604800, interval: 300 },
  { label: '2 Weeks', seconds: 1209600, interval: 600 },
  { label: 'A Month', seconds: 2592000, interval: 1800 },
  { label: '2 Months', seconds: 5184000, interval: 3600 },
  { label: '6 Months', seconds: 15552000, interval: 7200 },
]

const CHART_COLORS = {
  primary: '#6366f1',
  secondary: '#22d3ee',
  tertiary: '#f59e0b',
  danger: '#ef4444',
  success: '#10b981',
}

interface ChartDef {
  title: string
  endpoint: string
  fields: { key: string; color: string; label: string }[]
  unit?: string
  type: 'area' | 'line'
}

const CHARTS: ChartDef[] = [
  {
    title: 'Requests',
    endpoint: '/grafana/ts/requests',
    fields: [{ key: 'requests', color: CHART_COLORS.primary, label: 'Requests' }],
    type: 'area',
  },
  {
    title: 'Errors',
    endpoint: '/grafana/ts/errors',
    fields: [{ key: 'errors', color: CHART_COLORS.danger, label: 'Errors' }],
    type: 'area',
  },
  {
    title: 'Response Time',
    endpoint: '/grafana/ts/response-histogram',
    fields: [{ key: 'avg_response', color: CHART_COLORS.tertiary, label: 'Avg Response' }],
    unit: 'ms',
    type: 'line',
  },
  {
    title: 'Virtual Users',
    endpoint: '/grafana/ts/vus',
    fields: [{ key: 'vus', color: CHART_COLORS.success, label: 'VUs' }],
    type: 'area',
  },
  {
    title: 'Percentiles (P90 / P95)',
    endpoint: '/grafana/ts/percentiles',
    fields: [
      { key: 'median', color: CHART_COLORS.success, label: 'Median' },
      { key: 'p90', color: CHART_COLORS.tertiary, label: 'P90' },
      { key: 'p95', color: CHART_COLORS.danger, label: 'P95' },
    ],
    unit: 'ms',
    type: 'line',
  },
  {
    title: 'Requests per Second',
    endpoint: '/grafana/ts/rps',
    fields: [{ key: 'rps', color: CHART_COLORS.secondary, label: 'RPS' }],
    type: 'area',
  },
  {
    title: 'Iterations',
    endpoint: '/grafana/ts/iterations',
    fields: [{ key: 'iterations', color: CHART_COLORS.primary, label: 'Iterations' }],
    type: 'area',
  },
  {
    title: 'Requests per VU',
    endpoint: '/grafana/ts/req-per-vu',
    fields: [{ key: 'req_per_vu', color: CHART_COLORS.success, label: 'Req/VU' }],
    type: 'line',
  },
]

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function formatAxisTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit' })
}

function formatTooltipTime(iso: string): string {
  const d = new Date(iso)
  return d.toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  })
}

function formatNumber(n: number): string {
  if (n == null || Number.isNaN(n)) return '0'
  if (Number.isInteger(n)) return n.toLocaleString()
  return n.toFixed(2)
}

function buildParams(domain: string, test: string, range: typeof TIME_RANGES[number]): string {
  const now = new Date()
  const from = new Date(now.getTime() - range.seconds * 1000)
  const params = new URLSearchParams({
    domain,
    test,
    from: from.toISOString(),
    to: now.toISOString(),
    interval: String(range.interval),
  })
  return params.toString()
}

// ---------------------------------------------------------------------------
// Components
// ---------------------------------------------------------------------------

function StatCard({ label, value, unit }: { label: string; value: number; unit?: string }) {
  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
      <p className="text-xs font-medium text-gray-500 uppercase tracking-wide">{label}</p>
      <p className="mt-1 text-xl font-semibold text-gray-900 font-mono">
        {formatNumber(value)}
        {unit && <span className="text-sm text-gray-500 ml-1">{unit}</span>}
      </p>
    </div>
  )
}

function MetricChart({ def, data, loading }: { def: ChartDef; data: Record<string, unknown>[]; loading: boolean }) {
  const isEmpty = !loading && data.length === 0
  const ChartComponent = def.type === 'area' ? AreaChart : LineChart

  return (
    <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4">
      <h3 className="text-sm font-semibold text-gray-700 mb-3">{def.title}</h3>
      {loading ? (
        <div className="flex items-center justify-center h-72 text-sm text-gray-400">Loading...</div>
      ) : isEmpty ? (
        <div className="flex items-center justify-center h-72 text-sm text-gray-400">No data</div>
      ) : (
        <ResponsiveContainer width="100%" height={288}>
          <ChartComponent data={data} margin={{ top: 4, right: 8, left: 0, bottom: 0 }}>
            <CartesianGrid strokeDasharray="3 3" stroke="#e5e7eb" />
            <XAxis
              dataKey="time"
              tickFormatter={formatAxisTime}
              tick={{ fontSize: 11 }}
              stroke="#9ca3af"
            />
            <YAxis tick={{ fontSize: 11 }} stroke="#9ca3af" width={52} />
            <Tooltip
              labelFormatter={formatTooltipTime}
              formatter={(value: number) => [
                `${formatNumber(value)}${def.unit ? ` ${def.unit}` : ''}`,
              ]}
              contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e5e7eb' }}
            />
            {def.fields.length > 1 && <Legend wrapperStyle={{ fontSize: 12 }} />}
            {def.fields.map((f) =>
              def.type === 'area' ? (
                <Area
                  key={f.key}
                  type="monotone"
                  dataKey={f.key}
                  name={f.label}
                  stroke={f.color}
                  fill={f.color}
                  fillOpacity={0.15}
                  strokeWidth={2}
                  dot={false}
                  connectNulls={false}
                />
              ) : (
                <Line
                  key={f.key}
                  type="monotone"
                  dataKey={f.key}
                  name={f.label}
                  stroke={f.color}
                  strokeWidth={2}
                  dot={false}
                  connectNulls={false}
                />
              )
            )}
          </ChartComponent>
        </ResponsiveContainer>
      )}
    </div>
  )
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export default function ReportPage() {
  // Selector state
  const [domains, setDomains] = useState<VarItem[]>([])
  const [tests, setTests] = useState<VarItem[]>([])
  const [selectedDomain, setSelectedDomain] = useState('')
  const [selectedTest, setSelectedTest] = useState('')
  const [selectedRange, setSelectedRange] = useState(0) // index into TIME_RANGES
  const [loadingDomains, setLoadingDomains] = useState(true)
  const [loadingTests, setLoadingTests] = useState(false)

  // Report state
  const [generated, setGenerated] = useState(false)
  const [stats, setStats] = useState<StatsData | null>(null)
  const [chartData, setChartData] = useState<Record<string, Record<string, unknown>[]>>({})
  const [loadingStats, setLoadingStats] = useState(false)
  const [loadingCharts, setLoadingCharts] = useState<Record<string, boolean>>({})

  // Fetch domains on mount
  useEffect(() => {
    fetch('/metrics-api/grafana/variables/domains')
      .then((res) => res.json())
      .then((data: VarItem[]) => setDomains(data))
      .catch(() => {})
      .finally(() => setLoadingDomains(false))
  }, [])

  // Fetch tests when domain changes
  useEffect(() => {
    if (!selectedDomain) {
      setTests([])
      setSelectedTest('')
      return
    }
    setLoadingTests(true)
    setSelectedTest('')
    fetch(`/metrics-api/grafana/variables/tests?domain=${encodeURIComponent(selectedDomain)}`)
      .then((res) => res.json())
      .then((data: VarItem[]) => setTests(data))
      .catch(() => setTests([]))
      .finally(() => setLoadingTests(false))
  }, [selectedDomain])

  // Generate report
  const generate = useCallback(() => {
    if (!selectedDomain || !selectedTest) return
    const range = TIME_RANGES[selectedRange]
    const qs = buildParams(selectedDomain, selectedTest, range)

    setGenerated(true)
    setStats(null)
    setChartData({})

    // Fetch stats (endpoint returns an array, take first element)
    setLoadingStats(true)
    fetch(`/metrics-api/grafana/stats?${qs}`)
      .then((res) => res.json())
      .then((data: StatsData[]) => setStats(Array.isArray(data) ? data[0] ?? null : data))
      .catch(() => setStats(null))
      .finally(() => setLoadingStats(false))

    // Fetch each chart in parallel
    for (const chart of CHARTS) {
      setLoadingCharts((prev) => ({ ...prev, [chart.endpoint]: true }))
      fetch(`/metrics-api${chart.endpoint}?${qs}`)
        .then((res) => res.json())
        .then((data: Record<string, unknown>[]) => {
          setChartData((prev) => ({ ...prev, [chart.endpoint]: data }))
        })
        .catch(() => {
          setChartData((prev) => ({ ...prev, [chart.endpoint]: [] }))
        })
        .finally(() => {
          setLoadingCharts((prev) => ({ ...prev, [chart.endpoint]: false }))
        })
    }
  }, [selectedDomain, selectedTest, selectedRange])

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Report</h1>
        <p className="mt-1 text-sm text-gray-500">Generate metric charts for a specific domain, test and time range</p>
      </div>

      {/* Selectors */}
      <div className="bg-white rounded-xl shadow-sm border border-gray-200 p-4 mb-6">
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
          {/* Domain */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Domain</label>
            <select
              value={selectedDomain}
              onChange={(e) => setSelectedDomain(e.target.value)}
              disabled={loadingDomains}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
            >
              <option value="">Select a domain...</option>
              {domains.map((d) => (
                <option key={d.__value} value={d.__value}>
                  {d.__text}
                </option>
              ))}
            </select>
          </div>

          {/* Test */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Test</label>
            <select
              value={selectedTest}
              onChange={(e) => setSelectedTest(e.target.value)}
              disabled={!selectedDomain || loadingTests}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
            >
              <option value="">{loadingTests ? 'Loading...' : 'Select a test...'}</option>
              {tests.map((t) => (
                <option key={t.__value} value={t.__value}>
                  {t.__text}
                </option>
              ))}
            </select>
          </div>

          {/* Time Range */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Time Range</label>
            <select
              value={selectedRange}
              onChange={(e) => setSelectedRange(Number(e.target.value))}
              className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm shadow-sm focus:border-primary-500 focus:ring-1 focus:ring-primary-500"
            >
              {TIME_RANGES.map((r, i) => (
                <option key={r.label} value={i}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>

          {/* Generate button */}
          <div className="flex items-end">
            <button
              onClick={generate}
              disabled={!selectedDomain || !selectedTest}
              className="w-full rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Generate Report
            </button>
          </div>
        </div>
      </div>

      {/* Report content */}
      {generated && (
        <>
          {/* Stats summary */}
          {loadingStats ? (
            <div className="text-sm text-gray-400 mb-6">Loading stats...</div>
          ) : stats ? (
            <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-5 gap-4 mb-6">
              <StatCard label="Total Requests" value={stats.requests} />
              <StatCard label="Failures" value={stats.failures} />
              <StatCard label="Error Rate" value={stats.error_rate} unit="%" />
              <StatCard label="Avg Response" value={stats.avg_response} unit="ms" />
              <StatCard label="P90" value={stats.p90} unit="ms" />
              <StatCard label="P95" value={stats.p95} unit="ms" />
              <StatCard label="Max Response" value={stats.max_response} unit="ms" />
              <StatCard label="Peak RPS" value={stats.peak_rps} />
              <StatCard label="VUs Max" value={stats.vus_max} />
              <StatCard label="Req / VU" value={stats.req_per_vu} />
            </div>
          ) : null}

          {/* Charts */}
          <div className="grid grid-cols-1 gap-4">
            {CHARTS.map((def) => (
              <MetricChart
                key={def.endpoint}
                def={def}
                data={chartData[def.endpoint] || []}
                loading={loadingCharts[def.endpoint] || false}
              />
            ))}
          </div>
        </>
      )}

      {/* Empty state */}
      {!generated && (
        <div className="text-center py-12 text-gray-400">
          Select a domain, test and time range, then click Generate Report
        </div>
      )}
    </div>
  )
}
