export interface User {
  id: string
  email: string
  name: string
  role: 'ROOT' | 'USER'
  status: 'ACTIVE' | 'INACTIVE' | 'SUSPENDED'
  grafana_user_id?: number
  grafana_username?: string
  last_login_at?: string
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  expires_at: string
  user: User
}

export interface Domain {
  id: string
  user_id: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface Test {
  id: string
  domain_id: string
  user_id: string
  name: string
  description?: string
  script_filename: string
  script_size_bytes: number
  default_vus: number
  default_duration: string
  created_at: string
  updated_at: string
  domain_name?: string
  user_name?: string
  user_email?: string
}

export interface TestExecution {
  id: string
  test_id: string
  user_id: string
  schedule_id?: string
  vus: number
  duration: string
  status: 'PENDING' | 'RUNNING' | 'COMPLETED' | 'FAILED' | 'CANCELLED' | 'TIMEOUT'
  started_at?: string
  completed_at?: string
  exit_code?: number
  stdout?: string
  stderr?: string
  metrics_summary?: Record<string, unknown>
  error_message?: string
  created_at: string
  updated_at: string
  test_name?: string
  domain_name?: string
  user_name?: string
  user_email?: string
}

export interface Schedule {
  id: string
  test_id: string
  user_id: string
  schedule_type: 'ONCE' | 'RECURRING'
  cron_expression?: string
  next_run_at?: string
  vus: number
  duration: string
  status: 'ACTIVE' | 'PAUSED' | 'COMPLETED' | 'CANCELLED'
  last_run_at?: string
  run_count: number
  created_at: string
  updated_at: string
  test_name?: string
  domain_name?: string
}

export interface ApiResponse<T> {
  success: boolean
  data?: T
  error?: {
    code: string
    message: string
    details?: Record<string, string>
  }
  meta?: {
    total: number
    page: number
    page_size: number
    total_pages: number
  }
}

export interface DashboardStats {
  total_tests: number
  running_now: number
  completed_today: number
  failed_today: number
  total_executions: number
}

export interface ServiceStatus {
  name: string
  status: 'ok' | 'warning' | 'error'
  message?: string
}
