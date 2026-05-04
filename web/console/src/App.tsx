import { useEffect, useMemo, useState } from 'react'
import {
  createAgent,
  createModel,
  createSkill,
  createTool,
  deleteSession,
  fetchAgents,
  fetchMe,
  fetchModels,
  fetchSkills,
  fetchTools,
  login,
  type AgentItem,
  type CatalogResponse,
  type CreateAgentPayload,
  type CreateModelPayload,
  type CreateSkillPayload,
  type CreateToolPayload,
  type ModelItem,
  type SkillItem,
  type ToolItem,
  type UserInfo,
} from './api'
import { AgentDashboard } from './components/AgentDashboard'
import { AuthPage } from './components/AuthPage'
import { LandingPage } from './components/LandingPage'
import { Sidebar } from './components/Sidebar'

export type ViewKey = 'create-agent' | 'agent-chat' | 'skills' | 'mcp' | 'models' | 'settings'
type RouteKey = 'landing' | 'login' | 'register' | 'console'

const TOKEN_KEY = 'go-agent-platform-token'

function routeToPath(route: RouteKey) {
  switch (route) {
    case 'login':
      return '/login'
    case 'register':
      return '/register'
    case 'console':
      return '/app'
    default:
      return '/'
  }
}

function resolveRoute(pathname: string, hasToken: boolean): RouteKey {
  if (pathname === '/login') return 'login'
  if (pathname === '/register') return 'register'
  if (pathname.startsWith('/app')) return hasToken ? 'console' : 'login'
  return 'landing'
}

export function App() {
  const [token, setToken] = useState<string | null>(() => window.localStorage.getItem(TOKEN_KEY))
  const [route, setRoute] = useState<RouteKey>(() => resolveRoute(window.location.pathname, Boolean(window.localStorage.getItem(TOKEN_KEY))))
  const [booting, setBooting] = useState(() => Boolean(window.localStorage.getItem(TOKEN_KEY)))
  const [currentView, setCurrentView] = useState<ViewKey>('create-agent')
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [user, setUser] = useState<UserInfo | null>(null)
  const [agents, setAgents] = useState<AgentItem[]>([])
  const [skillCatalog, setSkillCatalog] = useState<CatalogResponse<SkillItem>>({ platform_items: [], my_items: [] })
  const [toolCatalog, setToolCatalog] = useState<CatalogResponse<ToolItem>>({ platform_items: [], my_items: [] })
  const [models, setModels] = useState<ModelItem[]>([])
  const [activeAgentId, setActiveAgentId] = useState('')
  const [loading, setLoading] = useState(false)
  const [loadError, setLoadError] = useState('')

  function navigate(nextRoute: RouteKey, replace = false) {
    const nextPath = routeToPath(nextRoute)
    if (window.location.pathname !== nextPath) {
      window.history[replace ? 'replaceState' : 'pushState']({}, '', nextPath)
    }
    setRoute(nextRoute)
  }

  useEffect(() => {
    function handlePopState() {
      setRoute(resolveRoute(window.location.pathname, Boolean(window.localStorage.getItem(TOKEN_KEY))))
    }

    window.addEventListener('popstate', handlePopState)
    return () => window.removeEventListener('popstate', handlePopState)
  }, [])

  useEffect(() => {
    if (!token) {
      setBooting(false)
      if (route === 'console') {
        navigate('login', true)
      }
      return
    }

    let cancelled = false

    async function loadAll() {
      setBooting(true)
      setLoading(true)
      try {
        const [me, agentItems, skills, tools, modelItems] = await Promise.all([
          fetchMe(token),
          fetchAgents(token),
          fetchSkills(token),
          fetchTools(token),
          fetchModels(token),
        ])

        if (cancelled) return

        setUser(me.user)
        setAgents(agentItems)
        setSkillCatalog(skills)
        setToolCatalog(tools)
        setModels(modelItems)
        setActiveAgentId((current) => current || agentItems[0]?.id || '')
        setCurrentView((current) => {
          if (current === 'settings' || current === 'skills' || current === 'mcp' || current === 'models') return current
          return agentItems.length > 0 ? 'agent-chat' : 'create-agent'
        })
        setLoadError('')
      } catch (error) {
        if (cancelled) return
        const message = error instanceof Error ? error.message : '读取平台数据失败。'
        setLoadError(message)
        window.localStorage.removeItem(TOKEN_KEY)
        setToken(null)
        setUser(null)
        navigate('login', true)
      } finally {
        if (!cancelled) {
          setLoading(false)
          setBooting(false)
        }
      }
    }

    loadAll()
    return () => {
      cancelled = true
    }
  }, [token])

  const activeAgent = useMemo(
    () => agents.find((item) => item.id === activeAgentId) ?? null,
    [activeAgentId, agents],
  )

  async function refreshAll() {
    if (!token) return
    const [agentItems, skills, tools, modelItems] = await Promise.all([
      fetchAgents(token),
      fetchSkills(token),
      fetchTools(token),
      fetchModels(token),
    ])
    setAgents(agentItems)
    setSkillCatalog(skills)
    setToolCatalog(tools)
    setModels(modelItems)
    setActiveAgentId((current) => {
      if (current && agentItems.some((item) => item.id === current)) return current
      return agentItems[0]?.id ?? ''
    })
    if (agentItems.length > 0 && currentView === 'create-agent') {
      setCurrentView('agent-chat')
    }
  }

  async function handleLogin(email: string, password: string) {
    const result = await login(email, password)
    window.localStorage.setItem(TOKEN_KEY, result.token)
    setToken(result.token)
    navigate('console')
  }

  function handleLogout() {
    window.localStorage.removeItem(TOKEN_KEY)
    setToken(null)
    setUser(null)
    setAgents([])
    setSkillCatalog({ platform_items: [], my_items: [] })
    setToolCatalog({ platform_items: [], my_items: [] })
    setModels([])
    setActiveAgentId('')
    setCurrentView('create-agent')
    setLoadError('')
    navigate('landing')
  }

  async function handleCreateAgent(payload: CreateAgentPayload) {
    if (!token) throw new Error('请先登录。')
    const created = await createAgent(token, payload)
    await refreshAll()
    setActiveAgentId(created.id)
    setCurrentView('agent-chat')
    navigate('console')
  }

  async function handleCreateSkill(payload: CreateSkillPayload) {
    if (!token) throw new Error('请先登录。')
    await createSkill(token, payload)
    await refreshAll()
  }

  async function handleCreateTool(payload: CreateToolPayload) {
    if (!token) throw new Error('请先登录。')
    await createTool(token, payload)
    await refreshAll()
  }

  async function handleCreateModel(payload: CreateModelPayload) {
    if (!token) throw new Error('请先登录。')
    await createModel(token, payload)
    await refreshAll()
  }

  async function handleDeleteSession(sessionID: string) {
    if (!token) throw new Error('请先登录。')
    await deleteSession(token, sessionID)
  }

  function handleSelectAgent(agentID: string) {
    setActiveAgentId(agentID)
    setCurrentView('agent-chat')
    navigate('console')
  }

  if (booting) {
    return <div className="boot-screen">正在连接平台...</div>
  }

  if (route === 'landing') {
    return (
      <LandingPage
        isLoggedIn={Boolean(token)}
        onEnterApp={() => navigate('console')}
        onNavigateLogin={() => navigate('login')}
        onNavigateRegister={() => navigate('register')}
      />
    )
  }

  if (route === 'login' || route === 'register') {
    return (
      <AuthPage
        mode={route === 'login' ? 'login' : 'register'}
        onLogin={handleLogin}
        onNavigateHome={() => navigate('landing')}
        onNavigateLogin={() => navigate('login')}
        onNavigateRegister={() => navigate('register')}
      />
    )
  }

  if (!token) {
    return null
  }

  return (
    <div className={`desktop-shell ${sidebarCollapsed ? 'sidebar-collapsed' : ''}`}>
      <Sidebar
        activeAgentId={activeAgentId}
        agents={agents}
        collapsed={sidebarCollapsed}
        currentView={currentView}
        onChangeAgent={handleSelectAgent}
        onChangeView={setCurrentView}
        onLogout={handleLogout}
        onToggleCollapsed={() => setSidebarCollapsed((value) => !value)}
        user={user}
      />

      <main className="desktop-main">
        <header className="desktop-menubar" />
        <section className="desktop-content">
          <AgentDashboard
            activeAgent={activeAgent}
            agents={agents}
            currentView={currentView}
            error={loadError}
            loading={loading}
            models={models}
            onCreateAgent={handleCreateAgent}
            onCreateModel={handleCreateModel}
            onCreateSkill={handleCreateSkill}
            onCreateTool={handleCreateTool}
            onDeleteSession={handleDeleteSession}
            onRefresh={refreshAll}
            skillCatalog={skillCatalog}
            token={token}
            toolCatalog={toolCatalog}
            user={user}
          />
        </section>
      </main>
    </div>
  )
}
