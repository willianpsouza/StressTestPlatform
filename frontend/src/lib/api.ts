import { ApiResponse } from '@/types'

const API_BASE = process.env.NEXT_PUBLIC_API_URL || '/api/v1'

class ApiClient {
  private getToken(): string | null {
    if (typeof window === 'undefined') return null
    return localStorage.getItem('access_token')
  }

  private async request<T>(
    path: string,
    options: RequestInit = {}
  ): Promise<ApiResponse<T>> {
    const token = this.getToken()
    const headers: Record<string, string> = {
      ...(options.headers as Record<string, string>),
    }

    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    // Only set Content-Type for non-FormData bodies
    if (!(options.body instanceof FormData)) {
      headers['Content-Type'] = 'application/json'
    }

    const res = await fetch(`${API_BASE}${path}`, {
      ...options,
      headers,
    })

    if (res.status === 204) {
      return { success: true } as ApiResponse<T>
    }

    const data = await res.json()

    if (!data.success && res.status === 401) {
      // Try refresh
      const refreshed = await this.tryRefresh()
      if (refreshed) {
        headers['Authorization'] = `Bearer ${this.getToken()}`
        const retryRes = await fetch(`${API_BASE}${path}`, { ...options, headers })
        if (retryRes.status === 204) return { success: true } as ApiResponse<T>
        return retryRes.json()
      } else {
        if (typeof window !== 'undefined') {
          localStorage.removeItem('access_token')
          localStorage.removeItem('refresh_token')
          window.location.href = '/login'
        }
      }
    }

    return data
  }

  private async tryRefresh(): Promise<boolean> {
    const refreshToken = typeof window !== 'undefined' ? localStorage.getItem('refresh_token') : null
    if (!refreshToken) return false

    try {
      const res = await fetch(`${API_BASE}/auth/refresh`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: refreshToken }),
      })
      const data = await res.json()
      if (data.success && data.data) {
        localStorage.setItem('access_token', data.data.access_token)
        localStorage.setItem('refresh_token', data.data.refresh_token)
        return true
      }
    } catch {}
    return false
  }

  get<T>(path: string) {
    return this.request<T>(path)
  }

  post<T>(path: string, body?: unknown) {
    return this.request<T>(path, {
      method: 'POST',
      body: body instanceof FormData ? body : JSON.stringify(body),
    })
  }

  put<T>(path: string, body?: unknown) {
    return this.request<T>(path, {
      method: 'PUT',
      body: body instanceof FormData ? body : JSON.stringify(body),
    })
  }

  delete<T>(path: string) {
    return this.request<T>(path, { method: 'DELETE' })
  }
}

export const api = new ApiClient()
