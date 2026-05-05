import axios from 'axios'
import { useAuthStore } from './stores/auth'

const api = axios.create({
  baseURL: '/admin/api/v1',
  timeout: 30000,
})

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

api.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401) {
      useAuthStore.getState().logout()
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)

export default api

// 类型定义
export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user: {
    id: string
    email: string
    name: string
    role: string
  }
}

export interface UserItem {
  id: string
  email: string
  name: string
  role: string
  status: string
  created_at: string
  last_login_at: string | null
}

export interface SkillItem {
  id: string
  name: string
  slug: string
  description: string
  version: string
  scope: string
  category: string
  download_count: number
  status: string
  created_at: string
}

export interface ToolItem {
  id: string
  name: string
  slug: string
  description: string
  version: string
  scope: string
  category: string
  download_count: number
  status: string
  created_at: string
}

export interface StatsOverview {
  total_users: number
  total_skills: number
  total_tools: number
  total_sessions: number
  total_messages: number
}

export interface DailyStats {
  date: string
  new_users: number
  new_sessions: number
  new_messages: number
}

// API 方法
export const authApi = {
  login: (data: LoginRequest) => api.post<LoginResponse>('/auth/login', data),
}

export const usersApi = {
  list: (params?: { page?: number; page_size?: number }) =>
    api.get<{ items: UserItem[]; total: number }>('/users', { params }),
  get: (id: string) => api.get<UserItem>(`/users/${id}`),
  update: (id: string, data: Partial<UserItem>) => api.put(`/users/${id}`, data),
  delete: (id: string) => api.delete(`/users/${id}`),
}

export const skillsApi = {
  list: (params?: { page?: number; page_size?: number; scope?: string }) =>
    api.get<{ items: SkillItem[]; total: number }>('/skills', { params }),
  get: (id: string) => api.get<SkillItem>(`/skills/${id}`),
  create: (data: Partial<SkillItem>) => api.post('/skills', data),
  update: (id: string, data: Partial<SkillItem>) => api.put(`/skills/${id}`, data),
  delete: (id: string) => api.delete(`/skills/${id}`),
}

export const toolsApi = {
  list: (params?: { page?: number; page_size?: number; scope?: string }) =>
    api.get<{ items: ToolItem[]; total: number }>('/tools', { params }),
  get: (id: string) => api.get<ToolItem>(`/tools/${id}`),
  create: (data: Partial<ToolItem>) => api.post('/tools', data),
  update: (id: string, data: Partial<ToolItem>) => api.put(`/tools/${id}`, data),
  delete: (id: string) => api.delete(`/tools/${id}`),
}

export const statsApi = {
  overview: () => api.get<StatsOverview>('/stats/overview'),
  daily: (params?: { days?: number }) =>
    api.get<DailyStats[]>('/stats/daily', { params }),
}
