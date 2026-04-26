import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { changePassword, getMe } from '../api/client'

export default function ChangePasswordPage({ user, setUser }) {
  const navigate = useNavigate()
  const forced = user.must_change_password
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    if (newPassword !== confirm) {
      setError('Новый пароль и подтверждение не совпадают')
      return
    }
    if (newPassword.length < 8) {
      setError('Пароль должен быть не короче 8 символов')
      return
    }
    setBusy(true)
    try {
      await changePassword(forced ? '' : oldPassword, newPassword)
      const fresh = await getMe()
      setUser(fresh)
      const home = fresh.role === 'developer' ? '/library' : '/documents'
      navigate(home)
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
        <p className="subtitle">
          {forced ? 'Сменить пароль (обязательно при первом входе)' : 'Сменить пароль'}
        </p>
        {error && <div className="error-banner">{error}</div>}
        {!forced && (
          <label>
            Текущий пароль
            <input type="password" value={oldPassword} onChange={(e) => setOldPassword(e.target.value)} required autoFocus />
          </label>
        )}
        <label>
          Новый пароль
          <input type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} required minLength={8} placeholder="минимум 8 символов" autoFocus={forced} />
        </label>
        <label>
          Подтвердите новый пароль
          <input type="password" value={confirm} onChange={(e) => setConfirm(e.target.value)} required minLength={8} />
        </label>
        <button type="submit" className="btn-login" disabled={busy}>{busy ? 'Сохранение...' : 'Сохранить'}</button>
      </form>
    </div>
  )
}
