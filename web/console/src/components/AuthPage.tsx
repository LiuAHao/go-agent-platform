import { useState } from 'react'

type AuthPageProps = {
  mode: 'login' | 'register'
  onLogin: (email: string, password: string) => Promise<void>
  onNavigateHome: () => void
  onNavigateLogin: () => void
  onNavigateRegister: () => void
}

export function AuthPage(props: AuthPageProps) {
  const { mode, onLogin, onNavigateHome, onNavigateLogin, onNavigateRegister } = props
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')

  const isLogin = mode === 'login'

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    setError('')

    if (!isLogin) {
      setError('当前后端还没有开放注册接口，注册页已预留，登录功能可正常使用。')
      return
    }

    setPending(true)
    try {
      await onLogin(email, password)
    } catch (currentError) {
      setError(currentError instanceof Error ? currentError.message : '登录失败，请稍后重试。')
    } finally {
      setPending(false)
    }
  }

  return (
    <div className="auth-route-shell">
      <div className="auth-route-panel">
        <button className="auth-back-link" onClick={onNavigateHome} type="button">
          返回首页
        </button>

        <div className="auth-card">
          <div className="auth-logo">A</div>
          <h1>{isLogin ? '登录平台' : '注册账号'}</h1>
          <p>
            {isLogin
              ? '登录后进入控制台，创建 Agent、安装平台能力并开始对话。'
              : '注册入口已预留，当前版本暂未接入后端注册接口。'}
          </p>

          <div className="auth-switch">
            <button className={isLogin ? 'active' : ''} onClick={onNavigateLogin} type="button">
              登录
            </button>
            <button className={!isLogin ? 'active' : ''} onClick={onNavigateRegister} type="button">
              注册
            </button>
          </div>

          <form className="auth-form" onSubmit={handleSubmit}>
            <label className="auth-field">
              <span>邮箱地址</span>
              <input onChange={(event) => setEmail(event.target.value)} placeholder="请输入登录邮箱" type="email" value={email} />
            </label>
            <label className="auth-field">
              <span>{isLogin ? '登录密码' : '设置密码'}</span>
              <input onChange={(event) => setPassword(event.target.value)} placeholder={isLogin ? '请输入密码' : '注册接口接入后可用'} type="password" value={password} />
            </label>

            {!isLogin ? (
              <div className="auth-notice">当前只开放登录链路，注册页先用于展示完整产品入口。</div>
            ) : null}

            <button disabled={pending || !isLogin} type="submit">
              {isLogin ? (pending ? '登录中...' : '登录并进入控制台') : '注册接口待接入'}
            </button>
          </form>

          {error ? <p className="auth-error">{error}</p> : null}
        </div>
      </div>
    </div>
  )
}
