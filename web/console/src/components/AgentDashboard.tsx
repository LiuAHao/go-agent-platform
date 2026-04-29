import { useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  executeTask,
  fetchMessages,
  fetchSessions,
  installSkill,
  installTool,
  uninstallSkill,
  uninstallTool,
  type AgentItem,
  type CatalogResponse,
  type CreateAgentPayload,
  type CreateModelPayload,
  type CreateSkillPayload,
  type CreateToolPayload,
  type MessageItem,
  type ModelItem,
  type SessionItem,
  type SkillItem,
  type ToolItem,
  type UserInfo,
} from '../api'
import type { ViewKey } from '../App'
import { AppIcon, type AppIconKey } from './AppIcon'

type DashboardProps = {
  activeAgent: AgentItem | null
  agents: AgentItem[]
  currentView: ViewKey
  error: string
  loading: boolean
  models: ModelItem[]
  onCreateAgent: (payload: CreateAgentPayload) => Promise<void>
  onCreateModel: (payload: CreateModelPayload) => Promise<void>
  onCreateSkill: (payload: CreateSkillPayload) => Promise<void>
  onCreateTool: (payload: CreateToolPayload) => Promise<void>
  onRefresh: () => Promise<void>
  skillCatalog: CatalogResponse<SkillItem>
  token: string
  toolCatalog: CatalogResponse<ToolItem>
  user: UserInfo | null
}

export function AgentDashboard(props: DashboardProps) {
  switch (props.currentView) {
    case 'skills':
      return <SkillsPage {...props} />
    case 'mcp':
      return <McpPage {...props} />
    case 'models':
      return <ModelsPage {...props} />
    case 'settings':
      return <SettingsPage user={props.user} />
    case 'agent-chat':
      return <AgentChatPage {...props} />
    default:
      return <CreateAgentPage {...props} />
  }
}

function CreateAgentPage(props: DashboardProps) {
  const { error, loading, models, onCreateAgent, skillCatalog, toolCatalog } = props
  const [name, setName] = useState('')
  const [instruction, setInstruction] = useState('')
  const [model, setModel] = useState('')
  const [selectedSkills, setSelectedSkills] = useState<string[]>([])
  const [selectedTools, setSelectedTools] = useState<string[]>([])
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    if (!model && models.length > 0) {
      setModel(models.find((item) => item.is_default)?.model_key ?? models[0]?.model_key ?? '')
    }
  }, [model, models])

  async function submit() {
    if (!name.trim()) {
      setMessage('请先填写 Agent 名称。')
      return
    }
    if (!model.trim()) {
      setMessage('请先选择模型。')
      return
    }

    setSaving(true)
    setMessage('')
    try {
      await onCreateAgent({
        name: name.trim(),
        description: instruction.trim(),
        system_prompt: instruction.trim(),
        model: model.trim(),
        skill_policy: selectedSkills,
        tool_policy: selectedTools,
        runtime_policy: 'default',
      })
      setName('')
      setInstruction('')
      setSelectedSkills([])
      setSelectedTools([])
      setMessage('Agent 已创建。')
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '创建 Agent 失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page-shell">
      <PageHero icon="create-agent" title="新建 Agent" subtitle="只保留必要配置。先说明 Agent 做什么，再选择模型、Skill 与 MCP 就可以开始使用。" />
      <div className="editor-panel">
        <Section title="基础信息" hint={loading ? '正在读取资源...' : '零基础用户只需要完成名称、说明和模型选择。'}>
          <div className="form-grid form-grid-two">
            <label className="field">
              <span>Agent 名称</span>
              <input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：行业情报 Agent" type="text" />
            </label>
            <label className="field">
              <span>模型</span>
              <select value={model} onChange={(e) => setModel(e.target.value)}>
                <option value="">请选择模型</option>
                {models.map((item) => (
                  <option key={item.id} value={item.model_key}>
                    {item.name} · {item.model_key}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <label className="field">
            <span>Agent 说明</span>
            <textarea
              value={instruction}
              onChange={(e) => setInstruction(e.target.value)}
              placeholder="直接描述这个 Agent 的职责、风格和边界。这里会同时写入 Agent 描述和系统提示词。"
              rows={5}
            />
          </label>
        </Section>

        <Section title="能力装配" hint="平台资源先安装到“我的”列表，再勾选给当前 Agent 使用。">
          <div className="form-grid form-grid-two">
            <SelectionPanel
              title="我的 Skill"
              items={skillCatalog.my_items}
              selected={selectedSkills}
              emptyText="当前没有可用 Skill，可先到“技能与应用”页安装平台 Skill 或创建个人 Skill。"
              onToggle={(id) => toggleSelection(id, selectedSkills, setSelectedSkills)}
              renderMeta={(item) => item.description || item.entry || '暂无描述'}
            />
            <SelectionPanel
              title="我的 MCP"
              items={toolCatalog.my_items}
              selected={selectedTools}
              emptyText="当前没有可用 MCP，可先到“MCP 配置”页安装平台 MCP 或创建个人 MCP。"
              onToggle={(id) => toggleSelection(id, selectedTools, setSelectedTools)}
              renderMeta={(item) => item.description || '暂无描述'}
            />
          </div>
        </Section>

        {error ? <div className="notice error">{error}</div> : null}
        {message ? <div className={`notice ${message.includes('已') ? 'success' : 'error'}`}>{message}</div> : null}

        <div className="panel-actions">
          <button className="primary-button" disabled={saving} onClick={submit} type="button">
            {saving ? '创建中...' : '创建 Agent'}
          </button>
        </div>
      </div>
    </div>
  )
}

function SkillsPage(props: DashboardProps) {
  const { error, onCreateSkill, onRefresh, skillCatalog, token } = props
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [description, setDescription] = useState('')
  const [scope, setScope] = useState<'platform' | 'personal'>('personal')
  const [creationMode, setCreationMode] = useState<'upload' | 'generate'>('upload')
  const [version, setVersion] = useState('0.1.0')
  const [entry, setEntry] = useState('bundle/SKILL.md')
  const [folderFiles, setFolderFiles] = useState<Array<{ path: string; content: string }>>([])
  const [folderLabel, setFolderLabel] = useState('')
  const [generateIntent, setGenerateIntent] = useState('')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const myIDs = useMemo(() => new Set(skillCatalog.my_items.map((item) => item.id)), [skillCatalog.my_items])

  async function submit() {
    if (!name.trim()) {
      setMessage('请先填写 Skill 名称。')
      return
    }
    if (scope === 'personal' && creationMode === 'upload' && folderFiles.length === 0) {
      setMessage('请先选择一个本地 Skill 文件夹。')
      return
    }
    if (scope === 'personal' && creationMode === 'generate') {
      setMessage('AI 生成 Skill 需要后端生成接口，当前先保留页面入口。')
      return
    }
    setSaving(true)
    setMessage('')
    try {
      await onCreateSkill({
        name: name.trim(),
        slug: resolveSkillSlug(slug, name),
        scope,
        description: description.trim(),
        version: version.trim() || '0.1.0',
        entry: scope === 'personal' ? entry.trim() || 'bundle/SKILL.md' : entry.trim(),
        schema: {},
        config:
          scope === 'personal'
            ? {
                creation_mode: creationMode,
                files: folderFiles,
                folder_label: folderLabel,
                generate_intent: generateIntent.trim(),
              }
            : {},
      })
      setName('')
      setSlug('')
      setDescription('')
      setCreationMode('upload')
      setVersion('0.1.0')
      setEntry('bundle/SKILL.md')
      setFolderFiles([])
      setFolderLabel('')
      setGenerateIntent('')
      setScope('personal')
      setMessage('Skill 已创建。')
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '创建 Skill 失败')
    } finally {
      setSaving(false)
    }
  }

  async function handleInstall(item: SkillItem, installed: boolean) {
    setMessage('')
    try {
      if (installed) {
        await uninstallSkill(token, item.id)
      } else {
        await installSkill(token, item.id)
      }
      await onRefresh()
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '更新 Skill 失败')
    }
  }

  async function handleFolderChange(files: FileList | null) {
    if (!files || files.length === 0) {
      setFolderFiles([])
      setFolderLabel('')
      return
    }

    const parsedFiles = await Promise.all(
      Array.from(files).map(async (file) => ({
        path: file.webkitRelativePath || file.name,
        content: await file.text(),
      })),
    )

    setFolderFiles(parsedFiles)
    const firstPath = parsedFiles[0]?.path || ''
    setFolderLabel(firstPath.includes('/') ? firstPath.split('/')[0] : '已选择文件夹')
    if (!slug.trim() && !name.trim()) {
      const inferredName = firstPath.split('/')[0] || '我的 Skill'
      setName(inferredName)
      setSlug(resolveSkillSlug('', inferredName))
    }
  }

  function handleDownload(item: SkillItem) {
    const bundle = {
      id: item.id,
      name: item.name,
      slug: item.slug,
      description: item.description,
      version: item.version,
      entry: item.entry,
      config: item.config,
    }
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = `${item.slug || item.name}.skill.json`
    anchor.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="page-shell">
      <PageHero icon="skills" title="技能与应用" subtitle="平台统一提供 Skill。个人 Skill 只保留最少字段，复杂配置放到高级设置或后续管理里。" />
      <div className="editor-panel">
        <CatalogSection
          title="平台 Skill"
          items={skillCatalog.platform_items}
          emptyTitle="当前还没有平台 Skill"
          emptyText="平台 Skill 创建后会出现在这里，用户可以安装到“我的 Skill”。"
          renderItem={(item) => (
            <CatalogItem
              title={item.name}
              secondary={`${item.version || '未标记版本'}${item.entry ? ` · ${item.entry}` : ''}`}
              actionLabel={myIDs.has(item.id) ? '移除' : '安装'}
              onAction={() => handleInstall(item, myIDs.has(item.id))}
            >
              {item.description || '暂无描述'}
            </CatalogItem>
          )}
        />

        <CatalogSection
          title="我的 Skill"
          items={skillCatalog.my_items}
          emptyTitle="当前还没有我的 Skill"
          emptyText="这里包含你自己创建的 Skill，以及从平台安装下来的 Skill。"
          renderItem={(item) => (
            <CatalogItem
              title={item.name}
              secondary={item.scope === 'platform' ? '已安装的平台 Skill' : `个人 Skill${item.entry ? ` · ${item.entry}` : ''}`}
              actionLabel={item.scope === 'personal' ? '下载草稿' : undefined}
              onAction={item.scope === 'personal' ? () => handleDownload(item) : undefined}
            >
              {item.description || '暂无描述'}
            </CatalogItem>
          )}
        />

        <ResourceForm
          title="创建 Skill"
          hint="零基础模式下只先填名称、描述和 Slug。个人 Skill 支持上传本地文件夹；AI 生成入口先保留。"
          fields={
            <>
              <div className="form-grid form-grid-three">
                <label className="field">
                  <span>Skill 名称</span>
                  <input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：行业研究" type="text" />
                </label>
                <label className="field">
                  <span>Slug</span>
                  <input value={slug} onChange={(e) => setSlug(e.target.value)} placeholder="例如：industry-research" type="text" />
                </label>
                <label className="field">
                  <span>发布范围</span>
                  <select value={scope} onChange={(e) => setScope(e.target.value as 'platform' | 'personal')}>
                    <option value="personal">我的 Skill</option>
                    <option value="platform">平台 Skill</option>
                  </select>
                </label>
              </div>

              <label className="field">
                <span>描述</span>
                <textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="描述这个 Skill 的能力边界。" rows={4} />
              </label>

              {scope === 'personal' ? (
                <>
                  <div className="mode-tabs" role="tablist" aria-label="Skill 创建方式">
                    <button
                      className={`mode-tab ${creationMode === 'upload' ? 'active' : ''}`}
                      onClick={() => setCreationMode('upload')}
                      type="button"
                    >
                      本地上传文件夹
                    </button>
                    <button
                      className={`mode-tab ${creationMode === 'generate' ? 'active' : ''}`}
                      onClick={() => setCreationMode('generate')}
                      type="button"
                    >
                      AI 生成 Skill
                    </button>
                  </div>

                  {creationMode === 'upload' ? (
                    <label className="field">
                      <span>上传 Skill 文件夹</span>
                      <div className="upload-card">
                        <input
                          className="upload-input"
                          id="skill-folder-upload"
                          {...({ webkitdirectory: 'true', directory: 'true' } as Record<string, string>)}
                          multiple
                          onChange={(e) => handleFolderChange(e.target.files)}
                          type="file"
                        />
                        <label className="upload-trigger" htmlFor="skill-folder-upload">
                          选择文件夹
                        </label>
                        <div className="upload-filename">{folderLabel || '暂未选择文件夹'}</div>
                        <strong>{folderLabel || '选择本地 Skill 文件夹'}</strong>
                        <small>{folderFiles.length > 0 ? `已读取 ${folderFiles.length} 个文件，创建后可在列表中下载草稿。` : '建议包含 SKILL.md、README.md 等文件。'}</small>
                      </div>
                    </label>
                  ) : (
                    <label className="field">
                      <span>生成说明</span>
                      <textarea
                        value={generateIntent}
                        onChange={(e) => setGenerateIntent(e.target.value)}
                        placeholder="描述希望平台生成怎样的 Skill，例如目标、输入输出和适用场景。"
                        rows={4}
                      />
                    </label>
                  )}

                  <details className="advanced-panel">
                    <summary>高级设置</summary>
                    <div className="advanced-panel-body">
                      <div className="form-grid form-grid-two">
                        <label className="field">
                          <span>版本</span>
                          <input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="例如：0.1.0" type="text" />
                        </label>
                        <label className="field">
                          <span>入口标识</span>
                          <input value={entry} onChange={(e) => setEntry(e.target.value)} placeholder="例如：bundle/SKILL.md" type="text" />
                        </label>
                      </div>
                    </div>
                  </details>
                </>
              ) : (
                <details className="advanced-panel">
                  <summary>高级设置</summary>
                  <div className="advanced-panel-body">
                    <div className="form-grid form-grid-two">
                      <label className="field">
                        <span>版本</span>
                        <input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="例如：0.1.0" type="text" />
                      </label>
                      <label className="field">
                        <span>入口标识</span>
                        <input value={entry} onChange={(e) => setEntry(e.target.value)} placeholder="例如：builtin/research" type="text" />
                      </label>
                    </div>
                  </div>
                </details>
              )}
            </>
          }
          message={message}
          error={error}
          saving={saving}
          submitText="创建 Skill"
          onSubmit={submit}
        />
      </div>
    </div>
  )
}

function McpPage(props: DashboardProps) {
  const { error, onCreateTool, onRefresh, token, toolCatalog } = props
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [scope, setScope] = useState<'platform' | 'personal'>('personal')
  const [requiresApproval, setRequiresApproval] = useState(false)
  const [bindingText, setBindingText] = useState('{\n  \n}')
  const [schemaText, setSchemaText] = useState('{}')
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const myIDs = useMemo(() => new Set(toolCatalog.my_items.map((item) => item.id)), [toolCatalog.my_items])

  async function submit() {
    if (!name.trim()) {
      setMessage('请先填写 MCP 名称。')
      return
    }

    setSaving(true)
    setMessage('')
    try {
      await onCreateTool({
        name: name.trim(),
        scope,
        description: description.trim(),
        schema: JSON.parse(schemaText),
        config: JSON.parse(bindingText),
        requires_approval: requiresApproval,
      })
      setName('')
      setDescription('')
      setScope('personal')
      setRequiresApproval(false)
      setBindingText('{\n  \n}')
      setSchemaText('{}')
      setMessage('MCP 已创建。')
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '创建 MCP 失败')
    } finally {
      setSaving(false)
    }
  }

  async function handleInstall(item: ToolItem, installed: boolean) {
    setMessage('')
    try {
      if (installed) {
        await uninstallTool(token, item.id)
      } else {
        await installTool(token, item.id)
      }
      await onRefresh()
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '更新 MCP 失败')
    }
  }

  return (
    <div className="page-shell">
      <PageHero icon="mcp" title="MCP 配置" subtitle="先填名称、描述和绑定 JSON 就能建立 MCP 连接，复杂配置统一收进高级设置。" />
      <div className="editor-panel">
        <CatalogSection
          title="平台 MCP"
          items={toolCatalog.platform_items}
          emptyTitle="当前还没有平台 MCP"
          emptyText="平台 MCP 创建后会出现在这里，用户可以安装到“我的 MCP”。"
          renderItem={(item) => (
            <CatalogItem
              title={item.name}
              secondary={item.requires_approval ? '需要审批' : '直接调用'}
              actionLabel={myIDs.has(item.id) ? '移除' : '安装'}
              onAction={() => handleInstall(item, myIDs.has(item.id))}
            >
              {item.description || '暂无描述'}
            </CatalogItem>
          )}
        />

        <CatalogSection
          title="我的 MCP"
          items={toolCatalog.my_items}
          emptyTitle="当前还没有我的 MCP"
          emptyText="这里包含你自己创建的 MCP，以及从平台安装下来的 MCP。"
          renderItem={(item) => (
            <CatalogItem title={item.name} secondary={item.scope === 'platform' ? '已安装的平台 MCP' : item.requires_approval ? '个人 MCP · 需要审批' : '个人 MCP'}>
              {item.description || '暂无描述'}
            </CatalogItem>
          )}
        />

        <ResourceForm
          title="创建 MCP"
          hint="基础模式只填名称、描述和绑定 JSON。平台或审批类字段放到高级设置里。"
          fields={
            <>
              <div className="form-grid form-grid-two">
                <label className="field">
                  <span>MCP 名称</span>
                  <input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：browser-search" type="text" />
                </label>
                <label className="field">
                  <span>描述</span>
                  <input value={description} onChange={(e) => setDescription(e.target.value)} placeholder="例如：连接浏览器搜索并返回结构化结果" type="text" />
                </label>
              </div>

              <label className="field">
                <span>绑定 JSON</span>
                <textarea value={bindingText} onChange={(e) => setBindingText(e.target.value)} placeholder="填写连接所需的基础 JSON 配置。" rows={8} />
              </label>

              <details className="advanced-panel">
                <summary>高级设置</summary>
                <div className="advanced-panel-body">
                  <div className="form-grid form-grid-two">
                    <label className="field">
                      <span>发布范围</span>
                      <select value={scope} onChange={(e) => setScope(e.target.value as 'platform' | 'personal')}>
                        <option value="personal">我的 MCP</option>
                        <option value="platform">平台 MCP</option>
                      </select>
                    </label>
                    <label className="toggle-field">
                      <span>调用审批</span>
                      <div className="toggle-inline">
                        <input checked={requiresApproval} onChange={(e) => setRequiresApproval(e.target.checked)} type="checkbox" />
                        <strong>{requiresApproval ? '需要审批' : '直接调用'}</strong>
                      </div>
                    </label>
                  </div>

                  <label className="field">
                    <span>Schema JSON（可选）</span>
                    <textarea value={schemaText} onChange={(e) => setSchemaText(e.target.value)} rows={5} />
                  </label>
                </div>
              </details>
            </>
          }
          message={message}
          error={error}
          saving={saving}
          submitText="创建 MCP"
          onSubmit={submit}
        />
      </div>
    </div>
  )
}

function ModelsPage(props: DashboardProps) {
  const { error, models, onCreateModel } = props
  const [name, setName] = useState('')
  const [provider, setProvider] = useState('')
  const [apiBaseURL, setAPIBaseURL] = useState('')
  const [apiKey, setAPIKey] = useState('')
  const [modelKey, setModelKey] = useState('')
  const [description, setDescription] = useState('')
  const [contextWindow, setContextWindow] = useState('')
  const [maxOutputTokens, setMaxOutputTokens] = useState('')
  const [isDefault, setIsDefault] = useState(false)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')

  async function submit() {
    if (!name.trim() || !modelKey.trim()) {
      setMessage('请先填写模型名称和官方模型名称。')
      return
    }
    if (!apiBaseURL.trim() || !apiKey.trim()) {
      setMessage('请先填写 API URL 和 API Key。')
      return
    }

    setSaving(true)
    setMessage('')
    try {
      await onCreateModel({
        name: name.trim(),
        provider: provider.trim(),
        api_base_url: apiBaseURL.trim(),
        api_key: apiKey.trim(),
        model_key: modelKey.trim(),
        description: description.trim(),
        context_window: Number(contextWindow) || 0,
        max_output_tokens: Number(maxOutputTokens) || 0,
        capabilities: [],
        is_default: isDefault,
      })
      setName('')
      setProvider('')
      setAPIBaseURL('')
      setAPIKey('')
      setModelKey('')
      setDescription('')
      setContextWindow('')
      setMaxOutputTokens('')
      setIsDefault(false)
      setMessage('模型已创建。')
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '创建模型失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="page-shell">
      <PageHero icon="models" title="模型配置" subtitle="基础模式只填模型名称、官方模型名、API URL 和 API Key，其余参数统一放到高级设置。" />
      <div className="editor-panel">
        <CatalogSection
          title="已配置模型"
          items={models}
          emptyTitle="当前还没有模型"
          emptyText="请先创建至少一个模型，然后在新建 Agent 或对话页中选择。"
          renderItem={(item) => (
            <CatalogItem title={item.name} secondary={`${item.model_key}${item.provider ? ` · ${item.provider}` : ''}${item.is_default ? ' · 默认模型' : ''}`}>
              {item.description || item.api_base_url || '暂无描述'}
            </CatalogItem>
          )}
        />

        <ResourceForm
          title="创建模型"
          hint="先完成模型接入必须字段。窗口大小、输出上限等细节可以稍后再补。"
          fields={
            <>
              <div className="form-grid form-grid-two">
                <label className="field">
                  <span>模型名称</span>
                  <input value={name} onChange={(e) => setName(e.target.value)} placeholder="例如：DeepSeek Chat" type="text" />
                </label>
                <label className="field">
                  <span>官方模型名称</span>
                  <input value={modelKey} onChange={(e) => setModelKey(e.target.value)} placeholder="例如：deepseek-chat" type="text" />
                </label>
              </div>

              <div className="form-grid form-grid-two">
                <label className="field">
                  <span>API URL</span>
                  <input value={apiBaseURL} onChange={(e) => setAPIBaseURL(e.target.value)} placeholder="例如：https://api.deepseek.com/v1" type="text" />
                </label>
                <label className="field">
                  <span>API Key</span>
                  <input value={apiKey} onChange={(e) => setAPIKey(e.target.value)} placeholder="请输入对应模型服务的 API Key" type="password" />
                </label>
              </div>

              <details className="advanced-panel">
                <summary>高级设置</summary>
                <div className="advanced-panel-body">
                  <label className="field">
                    <span>描述（可选）</span>
                    <textarea value={description} onChange={(e) => setDescription(e.target.value)} placeholder="描述这个模型适合的任务。" rows={3} />
                  </label>

                  <div className="form-grid form-grid-three">
                    <label className="field">
                      <span>Provider（可选）</span>
                      <input value={provider} onChange={(e) => setProvider(e.target.value)} placeholder="例如：deepseek" type="text" />
                    </label>
                    <label className="field">
                      <span>上下文窗口</span>
                      <input value={contextWindow} onChange={(e) => setContextWindow(e.target.value)} placeholder="例如：128000" type="number" />
                    </label>
                    <label className="field">
                      <span>最大输出 Token</span>
                      <input value={maxOutputTokens} onChange={(e) => setMaxOutputTokens(e.target.value)} placeholder="例如：4096" type="number" />
                    </label>
                  </div>

                  <label className="toggle-field">
                    <span>默认模型</span>
                    <div className="toggle-inline">
                      <input checked={isDefault} onChange={(e) => setIsDefault(e.target.checked)} type="checkbox" />
                      <strong>{isDefault ? '设为默认' : '普通模型'}</strong>
                    </div>
                  </label>
                </div>
              </details>
            </>
          }
          message={message}
          error={error}
          saving={saving}
          submitText="创建模型"
          onSubmit={submit}
        />
      </div>
    </div>
  )
}

function SettingsPage({ user }: { user: UserInfo | null }) {
  return (
    <div className="page-shell">
      <PageHero icon="settings" title="系统设置" subtitle="这里只保留当前平台已有的真实账户信息，不引入额外配置概念。" />
      <div className="editor-panel">
        <Section title="当前账户" hint="登录信息来自 /api/v1/me。">
          <div className="form-grid form-grid-three">
            <div className="field readonly">
              <span>姓名</span>
              <input readOnly type="text" value={user?.name ?? '未登录'} />
            </div>
            <div className="field readonly">
              <span>邮箱</span>
              <input readOnly type="text" value={user?.email ?? '未登录'} />
            </div>
            <div className="field readonly">
              <span>用户 ID</span>
              <input readOnly type="text" value={user?.id ?? '未登录'} />
            </div>
          </div>
        </Section>
      </div>
    </div>
  )
}

function AgentChatPage(props: DashboardProps) {
  const { activeAgent, error, models, token } = props
  const [sessions, setSessions] = useState<SessionItem[]>([])
  const [activeSessionId, setActiveSessionId] = useState('')
  const [messages, setMessages] = useState<MessageItem[]>([])
  const [model, setModel] = useState('')
  const [reasoning, setReasoning] = useState('standard')
  const [prompt, setPrompt] = useState('')
  const [sending, setSending] = useState(false)
  const [message, setMessage] = useState('')

  useEffect(() => {
    setModel(activeAgent?.model ?? models.find((item) => item.is_default)?.model_key ?? models[0]?.model_key ?? '')
  }, [activeAgent?.id, activeAgent?.model, models])

  useEffect(() => {
    if (!activeAgent) {
      setSessions([])
      setActiveSessionId('')
      setMessages([])
      return
    }

    let cancelled = false

    async function loadSessions() {
      try {
        const items = await fetchSessions(token, activeAgent.id)
        if (cancelled) return
        setSessions(items)
        setActiveSessionId((current) => (current && items.some((item) => item.id === current) ? current : items[0]?.id ?? ''))
      } catch (currentError) {
        if (!cancelled) {
          setMessage(currentError instanceof Error ? currentError.message : '读取会话失败')
        }
      }
    }

    loadSessions()
    return () => {
      cancelled = true
    }
  }, [activeAgent, token])

  useEffect(() => {
    if (!activeSessionId) {
      setMessages([])
      return
    }

    let cancelled = false

    async function loadMessages() {
      try {
        const items = await fetchMessages(token, activeSessionId)
        if (!cancelled) {
          setMessages(items)
        }
      } catch (currentError) {
        if (!cancelled) {
          setMessage(currentError instanceof Error ? currentError.message : '读取消息失败')
        }
      }
    }

    loadMessages()
    return () => {
      cancelled = true
    }
  }, [activeSessionId, token])

  async function submit() {
    if (!activeAgent) {
      setMessage('请先选择一个 Agent。')
      return
    }
    if (!prompt.trim()) {
      setMessage('请输入内容。')
      return
    }

    setSending(true)
    setMessage('')
    const currentPrompt = prompt.trim()
    try {
      const task = await executeTask(token, {
        agent_id: activeAgent.id,
        session_id: activeSessionId || undefined,
        session_title: currentPrompt.slice(0, 32),
        model: model || undefined,
        reasoning,
        prompt: currentPrompt,
        async: false,
      })
      const nextSessions = await fetchSessions(token, activeAgent.id)
      setSessions(nextSessions)
      setActiveSessionId(task.session_id)
      if (task.session_id) {
        setMessages(await fetchMessages(token, task.session_id))
      }
      setPrompt('')
    } catch (currentError) {
      setMessage(currentError instanceof Error ? currentError.message : '发送消息失败')
    } finally {
      setSending(false)
    }
  }

  if (!activeAgent) {
    return (
      <div className="page-shell">
        <PageHero icon="chat" title="选择一个 Agent" subtitle="左侧选择 Agent 后，这里会进入它的聊天页面。" />
      </div>
    )
  }

  const emptyState = sessions.length === 0 && messages.length === 0

  return (
    <div className="page-shell chat-page-shell">
      <div className="chat-shell chat-shell-full">
        {emptyState ? (
          <div className="chat-empty-state">
            <PageHero icon="person" title={activeAgent.name} subtitle={activeAgent.description || '还没有聊天记录，可以直接开始对话。'} />
          </div>
        ) : (
          <div className="chat-header">
            <div>
              <h1>{activeAgent.name}</h1>
              <p>{activeAgent.description || '当前 Agent 已加载，可以继续对话。'}</p>
            </div>
            {sessions.length > 0 ? (
              <div className="session-strip">
                {sessions.map((item) => (
                  <button key={item.id} className={`session-chip ${activeSessionId === item.id ? 'active' : ''}`} onClick={() => setActiveSessionId(item.id)} type="button">
                    {item.title || '新对话'}
                  </button>
                ))}
              </div>
            ) : null}
          </div>
        )}

        {messages.length > 0 ? (
          <div className="message-list message-list-full">
            {messages.map((item) => (
              <div key={item.id} className={`message-bubble ${item.role === 'user' ? 'user' : 'assistant'}`}>
                <div className="message-role">{item.role === 'user' ? '你' : activeAgent.name}</div>
                <div className="message-content">{item.content}</div>
              </div>
            ))}
          </div>
        ) : null}

        <div className="composer-shell composer-shell-docked">
          <textarea
            className="composer-input"
            value={prompt}
            onChange={(e) => setPrompt(e.target.value)}
            placeholder={`向 ${activeAgent.name} 发送消息，Agent 会按已绑定的 Skill 与 MCP 进行调用。`}
            rows={4}
          />
          <div className="composer-toolbar">
            <div className="composer-group">
              <label className="toolbar-select">
                <span>模型</span>
                <select value={model} onChange={(e) => setModel(e.target.value)}>
                  <option value="">默认模型</option>
                  {models.map((item) => (
                    <option key={item.id} value={item.model_key}>
                      {item.name}
                    </option>
                  ))}
                </select>
              </label>
              <label className="toolbar-select">
                <span>思考深度</span>
                <select value={reasoning} onChange={(e) => setReasoning(e.target.value)}>
                  <option value="standard">标准</option>
                  <option value="deep">深度思考</option>
                </select>
              </label>
            </div>
            <button className="send-button" disabled={sending} onClick={submit} type="button">
              {sending ? '发送中...' : '发送'}
            </button>
          </div>
        </div>

        {error ? <div className="notice error">{error}</div> : null}
        {message ? <div className="notice error">{message}</div> : null}
      </div>
    </div>
  )
}

function PageHero(props: { icon: AppIconKey; title: string; subtitle: string }) {
  return (
    <div className="page-hero">
      <div className="hero-mark">
        <AppIcon kind={props.icon} />
      </div>
      <h1>{props.title}</h1>
      <p>{props.subtitle}</p>
    </div>
  )
}

function Section(props: { title: string; hint: string; children: ReactNode }) {
  return (
    <div className="panel-section">
      <div className="panel-section-head">
        <h3>{props.title}</h3>
        <span>{props.hint}</span>
      </div>
      {props.children}
    </div>
  )
}

function ResourceForm(props: {
  title: string
  hint: string
  fields: ReactNode
  message: string
  error: string
  saving: boolean
  submitText: string
  onSubmit: () => void
}) {
  return (
    <Section title={props.title} hint={props.hint}>
      {props.fields}
      {props.error ? <div className="notice error">{props.error}</div> : null}
      {props.message ? <div className={`notice ${props.message.includes('已') ? 'success' : 'error'}`}>{props.message}</div> : null}
      <div className="panel-actions">
        <button className="primary-button" disabled={props.saving} onClick={props.onSubmit} type="button">
          {props.saving ? '提交中...' : props.submitText}
        </button>
      </div>
    </Section>
  )
}

function CatalogSection<T>(props: { title: string; items: T[]; emptyTitle: string; emptyText: string; renderItem: (item: T) => ReactNode }) {
  return (
    <Section title={props.title} hint="保持简洁，只展示真实资源。">
      <div className="list-panel">
        {props.items.length > 0 ? (
          props.items.map((item, index) => <div key={index}>{props.renderItem(item)}</div>)
        ) : (
          <div className="empty-panel">
            <strong>{props.emptyTitle}</strong>
            <p>{props.emptyText}</p>
          </div>
        )}
      </div>
    </Section>
  )
}

function CatalogItem(props: { title: string; secondary: string; children: ReactNode; actionLabel?: string; onAction?: () => void }) {
  return (
    <div className="list-item">
      <div>
        <strong>{props.title}</strong>
        <p>{props.children}</p>
        <div className="list-secondary">{props.secondary}</div>
      </div>
      {props.actionLabel && props.onAction ? (
        <button className="secondary-button" onClick={props.onAction} type="button">
          {props.actionLabel}
        </button>
      ) : null}
    </div>
  )
}

function SelectionPanel<T extends { id: string; name: string }>(props: {
  title: string
  items: T[]
  selected: string[]
  emptyText: string
  onToggle: (id: string) => void
  renderMeta: (item: T) => string
}) {
  return (
    <div className="field">
      <span>{props.title}</span>
      <div className="check-list">
        {props.items.length > 0 ? (
          props.items.map((item) => (
            <label key={item.id} className="check-item">
              <input checked={props.selected.includes(item.id)} onChange={() => props.onToggle(item.id)} type="checkbox" />
              <div>
                <strong>{item.name}</strong>
                <small>{props.renderMeta(item)}</small>
              </div>
            </label>
          ))
        ) : (
          <div className="inline-empty">{props.emptyText}</div>
        )}
      </div>
    </div>
  )
}

function toggleSelection(value: string, items: string[], setter: (items: string[]) => void) {
  setter(items.includes(value) ? items.filter((item) => item !== value) : [...items, value])
}

function resolveSkillSlug(slug: string, name: string) {
  const trimmed = slug.trim()
  if (trimmed) {
    return trimmed
  }
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9\u4e00-\u9fa5]+/g, '-')
    .replace(/^-+|-+$/g, '')
}
