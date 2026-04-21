type LandingPageProps = {
  isLoggedIn: boolean
  onEnterApp: () => void
  onNavigateLogin: () => void
  onNavigateRegister: () => void
}

const steps = [
  {
    index: '01',
    title: '平台发布能力',
    description: 'Skill 与 MCP 由平台集中审核与供给，避免每个用户重复配置底层繁琐能力。',
    tone: 'neutral',
  },
  {
    index: '02',
    title: '安装至我的资源',
    description: '在平台资源库中浏览，将需要的 Skill 和 MCP 一键安装到“我的资源”列表中。',
    tone: 'blue',
  },
  {
    index: '03',
    title: '创建与能力装配',
    description: '创建一个或多个 Agent。为每个业务角色分别绑定不同模型与已安装能力。',
    tone: 'violet',
  },
  {
    index: '04',
    title: '独立聊天页执行',
    description: '进入专属对话页，Agent 会按需自动调用已装配的 Skill 与 MCP。',
    tone: 'green',
  },
]

const featureCards = [
  {
    title: '统一供给，降低门槛',
    description: '平台负责维护核心能力，普通用户不需要重复搭建 Skill 与 MCP 服务，就能直接装配和使用。',
    size: 'large',
  },
  {
    title: '多 Agent 并行管理',
    description: '一个账号可以维护多个 Agent，每个 Agent 绑定自己的模型、技能和 MCP 组合。',
    size: 'small',
  },
  {
    title: '从创建到对话闭环',
    description: '首页负责表达平台价值，登录后进入控制台，完成创建、装配和聊天执行。',
    size: 'small',
  },
]

export function LandingPage(props: LandingPageProps) {
  const { isLoggedIn, onEnterApp, onNavigateLogin, onNavigateRegister } = props

  return (
    <div className="landing-shell">
      <header className="landing-navbar">
        <div className="landing-brand">
          <div className="landing-brand-mark" aria-hidden="true" />
          <span>公共 Agent 平台</span>
        </div>

        <div className="landing-navbar-actions">
          {isLoggedIn ? (
            <button className="landing-primary-button" onClick={onEnterApp} type="button">
              进入控制台
            </button>
          ) : (
            <>
              <button className="landing-nav-button" onClick={onNavigateLogin} type="button">
                登录
              </button>
              <button className="landing-primary-button" onClick={onNavigateRegister} type="button">
                注册
              </button>
            </>
          )}
        </div>
      </header>

      <main className="landing-main">
        <section className="landing-hero">
          <div className="landing-hero-badge">全新发布 · Skill 与 MCP 统一供给</div>
          <h1>
            统一提供能力，
            <br />
            按需装配 Agent
          </h1>
          <p>
            面向公共用户的 Agent 平台。平台负责统一提供 Skill 与 MCP，您只需选择安装、配置模型，即可把能力装配到自己的 Agent
            上，直接进入对话执行。
          </p>
          <div className="landing-hero-actions">
            {isLoggedIn ? (
              <button className="landing-primary-button landing-primary-button-strong" onClick={onEnterApp} type="button">
                进入控制台
              </button>
            ) : (
              <>
                <button className="landing-primary-button landing-primary-button-strong" onClick={onNavigateLogin} type="button">
                  登录
                </button>
                <button className="landing-secondary-button" onClick={onNavigateRegister} type="button">
                  注册
                </button>
              </>
            )}
          </div>
        </section>

        <section className="landing-section">
          <div className="landing-section-head">
            <h2>平台怎么工作？</h2>
            <p>围绕平台供给、用户安装、Agent 配置和聊天执行四个阶段展开。</p>
          </div>

          <div className="landing-step-grid">
            {steps.map((step) => (
              <article key={step.index} className="landing-step-card">
                <div className={`landing-step-index is-${step.tone}`}>{step.index}</div>
                <h3>{step.title}</h3>
                <p>{step.description}</p>
              </article>
            ))}
          </div>
        </section>

        <section className="landing-section">
          <div className="landing-section-head">
            <h2>核心特性</h2>
            <p>保留平台的极简风格，但把多 Agent 编排、能力装配和对话执行表达得更明确。</p>
          </div>

          <div className="landing-feature-grid">
            {featureCards.map((card) => (
              <article key={card.title} className={`landing-feature-card ${card.size === 'large' ? 'is-large' : ''}`}>
                <strong>{card.title}</strong>
                <p>{card.description}</p>
              </article>
            ))}
          </div>
        </section>
      </main>
    </div>
  )
}
