import { useState, useId } from 'react'
import { useNavigate, Link } from 'react-router-dom'
import { adminCreateUser } from '../api/client'
import { Label } from '../components/ui/Label'
import { SelectNative } from '../components/ui/SelectNative'

const ROLES = [
  { value: 'writer',    label: 'Писатель (Writer)' },
  { value: 'reviewer',  label: 'Ревьюер (Reviewer)' },
  { value: 'developer', label: 'Разработчик (Developer)' },
  { value: 'admin',     label: 'Администратор (Admin)' },
]

export default function CreateUserPage() {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState('writer')
  const roleId = useId()
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await adminCreateUser({ name, email, password, role })
      alert(`Пользователь создан.\nПри первом входе он будет обязан сменить пароль.`)
      navigate('/admin/users')
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="page">
      <div className="page-header">
        <h1>Создать пользователя</h1>
        <Link to="/admin/users" className="btn btn-secondary">Назад</Link>
      </div>

      <form className="form-card" onSubmit={handleSubmit}>
        {error && <div className="error-banner">{error}</div>}
        <label>
          Имя
          <input type="text" value={name} onChange={(e) => setName(e.target.value)} required autoFocus />
        </label>
        <label>
          Email
          <input type="email" value={email} onChange={(e) => setEmail(e.target.value.trim())} required />
        </label>
        <label>
          Временный пароль
          <input type="text" value={password} onChange={(e) => setPassword(e.target.value)} required minLength={8} placeholder="минимум 8 символов" />
          <span className="hint">Пользователь будет обязан сменить его при первом входе.</span>
        </label>
        <div className="form-field">
          <Label htmlFor={roleId}>Роль</Label>
          <SelectNative id={roleId} value={role} onChange={(e) => setRole(e.target.value)}>
            {ROLES.map(r => <option key={r.value} value={r.value}>{r.label}</option>)}
          </SelectNative>
        </div>
        <button type="submit" disabled={busy} className="btn btn-primary">
          {busy ? 'Создание...' : 'Создать'}
        </button>
      </form>
    </div>
  )
}
