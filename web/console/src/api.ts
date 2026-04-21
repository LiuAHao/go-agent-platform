export const API_BASE = 'http://localhost:8081/api/v1'

export type LoginResponse = {
  token: string
  user: UserInfo
}

export type UserInfo = {
  id: string
  email: string
  name: string
}

export type MeResponse = {
  user: UserInfo
}

export type AgentItem = {
  id: string
  workspace_id: string
  name: string
  description: string
  system_prompt: string
  model: string
  skill_policy: string[]
  tool_policy: string[]
  runtime_policy: string
  published_version_id?: string
  created_by: string
  created_at: string
  updated_at: string
}

export type ToolItem = {
  id: string
  workspace_id: string
  name: string
  scope: 'platform' | 'personal'
  description: string
  schema: Record<string, unknown>
  config: Record<string, unknown>
  requires_approval: boolean
  enabled: boolean
  created_by: string
  created_at: string
}

export type SkillItem = {
  id: string
  workspace_id: string
  name: string
  slug: string
  scope: 'platform' | 'personal'
  description: string
  version: string
  entry: string
  schema: Record<string, unknown>
  config: Record<string, unknown>
  enabled: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export type ModelItem = {
  id: string
  workspace_id: string
  name: string
  provider: string
  api_base_url: string
  api_key: string
  model_key: string
  description: string
  context_window: number
  max_output_tokens: number
  capabilities: string[]
  enabled: boolean
  is_default: boolean
  created_by: string
  created_at: string
  updated_at: string
}

export type SessionItem = {
  id: string
  workspace_id: string
  agent_id: string
  created_by: string
  title: string
  created_at: string
  updated_at: string
}

export type MessageItem = {
  id: string
  session_id: string
  role: 'user' | 'assistant' | 'agent' | 'system' | 'tool'
  content: string
  trace_id: string
  created_at: string
}

export type TaskItem = {
  id: string
  trace_id: string
  workspace_id: string
  agent_id: string
  session_id: string
  model: string
  reasoning: string
  prompt: string
  status: string
  result?: string
  error?: string
  created_by: string
  approval_id?: string
  created_at: string
  updated_at: string
}

export type CatalogResponse<T> = {
  platform_items: T[]
  my_items: T[]
}

export type CreateAgentPayload = {
  name: string
  description: string
  system_prompt: string
  model: string
  skill_policy: string[]
  tool_policy: string[]
  runtime_policy: string
}

export type CreateToolPayload = {
  name: string
  scope: 'platform' | 'personal'
  description: string
  schema: Record<string, unknown>
  config: Record<string, unknown>
  requires_approval: boolean
}

export type CreateSkillPayload = {
  name: string
  slug: string
  scope: 'platform' | 'personal'
  description: string
  version: string
  entry: string
  schema: Record<string, unknown>
  config: Record<string, unknown>
}

export type CreateModelPayload = {
  name: string
  provider: string
  api_base_url: string
  api_key: string
  model_key: string
  description: string
  context_window: number
  max_output_tokens: number
  capabilities: string[]
  is_default: boolean
}

export type ExecuteTaskPayload = {
  agent_id: string
  session_id?: string
  session_title?: string
  model?: string
  reasoning?: string
  prompt: string
  async?: boolean
  tool_calls?: Array<{ tool_id: string; input: Record<string, unknown> }>
}

export async function login(email: string, password: string): Promise<LoginResponse> {
  const response = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '登录失败'))
  }

  return response.json()
}

export async function fetchMe(token: string): Promise<MeResponse> {
  const response = await fetch(`${API_BASE}/me`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取用户信息失败'))
  }

  return response.json()
}

export async function fetchAgents(token: string): Promise<AgentItem[]> {
  const response = await fetch(`${API_BASE}/agents`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取 Agent 列表失败'))
  }

  const payload = (await response.json()) as { items?: AgentItem[] }
  return payload.items ?? []
}

export async function createAgent(token: string, payload: CreateAgentPayload): Promise<AgentItem> {
  const response = await fetch(`${API_BASE}/agents`, {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '创建 Agent 失败'))
  }

  return response.json()
}

export async function fetchTools(token: string): Promise<CatalogResponse<ToolItem>> {
  const response = await fetch(`${API_BASE}/tools`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取 MCP 列表失败'))
  }

  return response.json()
}

export async function createTool(token: string, payload: CreateToolPayload): Promise<ToolItem> {
  const response = await fetch(`${API_BASE}/tools`, {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '创建 MCP 失败'))
  }

  return response.json()
}

export async function installTool(token: string, toolID: string) {
  const response = await fetch(`${API_BASE}/tools/${toolID}/install`, {
    method: 'POST',
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '安装 MCP 失败'))
  }
}

export async function uninstallTool(token: string, toolID: string) {
  const response = await fetch(`${API_BASE}/tools/${toolID}/install`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '移除 MCP 失败'))
  }
}

export async function fetchSkills(token: string): Promise<CatalogResponse<SkillItem>> {
  const response = await fetch(`${API_BASE}/skills`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取 Skill 列表失败'))
  }

  return response.json()
}

export async function createSkill(token: string, payload: CreateSkillPayload): Promise<SkillItem> {
  const response = await fetch(`${API_BASE}/skills`, {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '创建 Skill 失败'))
  }

  return response.json()
}

export async function installSkill(token: string, skillID: string) {
  const response = await fetch(`${API_BASE}/skills/${skillID}/install`, {
    method: 'POST',
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '安装 Skill 失败'))
  }
}

export async function uninstallSkill(token: string, skillID: string) {
  const response = await fetch(`${API_BASE}/skills/${skillID}/install`, {
    method: 'DELETE',
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '移除 Skill 失败'))
  }
}

export async function fetchModels(token: string): Promise<ModelItem[]> {
  const response = await fetch(`${API_BASE}/models`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取模型列表失败'))
  }

  const payload = (await response.json()) as { items?: ModelItem[] }
  return payload.items ?? []
}

export async function createModel(token: string, payload: CreateModelPayload): Promise<ModelItem> {
  const response = await fetch(`${API_BASE}/models`, {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '创建模型失败'))
  }

  return response.json()
}

export async function fetchSessions(token: string, agentID: string): Promise<SessionItem[]> {
  const response = await fetch(`${API_BASE}/sessions?agent_id=${encodeURIComponent(agentID)}`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取会话列表失败'))
  }

  const payload = (await response.json()) as { items?: SessionItem[] }
  return payload.items ?? []
}

export async function fetchMessages(token: string, sessionID: string): Promise<MessageItem[]> {
  const response = await fetch(`${API_BASE}/sessions/${sessionID}/messages`, {
    headers: authHeaders(token),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '读取消息失败'))
  }

  const payload = (await response.json()) as { items?: MessageItem[] }
  return payload.items ?? []
}

export async function executeTask(token: string, payload: ExecuteTaskPayload): Promise<TaskItem> {
  const response = await fetch(`${API_BASE}/tasks`, {
    method: 'POST',
    headers: authHeaders(token),
    body: JSON.stringify(payload),
  })

  if (!response.ok) {
    throw new Error(await readError(response, '发送消息失败'))
  }

  return response.json()
}

function authHeaders(token: string) {
  return {
    Authorization: `Bearer ${token}`,
    'Content-Type': 'application/json',
  }
}

async function readError(response: Response, fallback: string) {
  try {
    const payload = (await response.json()) as { error?: string }
    return payload.error ?? fallback
  } catch {
    return fallback
  }
}
