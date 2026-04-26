import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { login, getMe } from '../api/client'

export default function LoginPage({ onLogin }) {
  const navigate = useNavigate()
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      const { access_token } = await login(email, password)
      sessionStorage.setItem('token', access_token)
      const user = await getMe()
      onLogin(user)
      const home = user.role === 'developer' ? '/library' : '/documents'
      navigate(user.must_change_password ? '/change-password' : home)
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="login-page">
      <form className="login-form" onSubmit={handleSubmit}>
        <h1>DocManager</h1>
        <p className="subtitle">Войдите в систему</p>
        {error && <div className="error-banner">{error}</div>}
        <label>
          Email
          <input type="text" value={email} onChange={(e) => setEmail(e.target.value.trim())} required autoFocus placeholder="admin@example.com" />
        </label>
        <label>
          Пароль
          <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        </label>
        <button type="submit" className="btn-login" disabled={busy}>{busy ? 'Вход...' : 'Войти'}</button>
      </form>
    </div>
  )
}
