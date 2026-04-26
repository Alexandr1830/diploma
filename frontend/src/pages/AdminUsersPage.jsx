import { useState, useEffect, useId } from 'react'
import { Link } from 'react-router-dom'
import { adminListUsers, adminSetActive, adminResetPassword, adminUpdateUser } from '../api/client'
import { Label } from '../components/ui/Label'
import { SelectNative } from '../components/ui/SelectNative'

const ROLE_LABELS = {
  writer: 'Писатель',
  reviewer: 'Ревьюер',
  developer: 'Разработчик',
  admin: 'Администратор',
}

const ROLE_OPTIONS = ['writer', 'reviewer', 'developer', 'admin']

export default function AdminUsersPage() {
  const [users, setUsers] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState(null) // редактируемый пользователь или null

  const reload = () => {
    setLoading(true)
    adminListUsers()
      .then((data) => setUsers(data || []))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false))
  }

  useEffect(() => { reload() }, [])

  const handleToggleActive = async (u) => {
    const action = u.is_active ? 'деактивировать' : 'активировать'
    if (!confirm(`${action[0].toUpperCase()}${action.slice(1)} пользователя ${u.name}?`)) return
    try {
      await adminSetActive(u.id, !u.is_active)
      reload()
    } catch (e) {
      alert(`Ошибка: ${e.message}`)
    }
  }

  const handleResetPassword = async (u) => {
    const newPassword = prompt(`Новый пароль для ${u.name} (минимум 8 символов):`)
    if (!newPassword) return
    if (newPassword.length < 8) {
      alert('Пароль должен быть не короче 8 символов')
      return
    }
    try {
      await adminResetPassword(u.id, newPassword)
      alert(`Пароль сброшен. Передайте пользователю: ${newPassword}\nПри следующем входе он будет обязан сменить его.`)
      reload()
    } catch (e) {
      alert(`Ошибка: ${e.message}`)
    }
  }

  if (loading) return <div className="loading-screen">Загрузка...</div>
  if (error) return <div className="error-screen">Ошибка: {error}</div>

  return (
    <div className="page">
      <div className="page-header">
        <h1>Пользователи</h1>
        <Link to="/admin/users/new" className="btn btn-primary">Создать пользователя</Link>
      </div>

      <table className="data-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Имя</th>
            <th>Email</th>
            <th>Роль</th>
            <th>Статус</th>
            <th>Смена пароля</th>
            <th>Действия</th>
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id} className={u.is_active ? '' : 'row-inactive'}>
              <td>{u.id}</td>
              <td>{u.name}</td>
              <td>{u.email}</td>
              <td>{ROLE_LABELS[u.role] || u.role}</td>
              <td>
                <span className={`badge ${u.is_active ? 'status-approved' : 'status-archived'}`}>
                  {u.is_active ? 'Активен' : 'Заблокирован'}
                </span>
              </td>
              <td>
                {u.must_change_password
                  ? <span className="badge status-needs_revision">Требуется</span>
                  : <span className="text-muted">—</span>}
              </td>
              <td className="actions">
                <button className="btn btn-sm btn-secondary" onClick={() => setEditing(u)}>
                  Изменить
                </button>
                <button className="btn btn-sm btn-secondary" onClick={() => handleResetPassword(u)}>
                  Сбросить пароль
                </button>
                <button
                  className={`btn btn-sm ${u.is_active ? 'btn-danger' : 'btn-primary'}`}
                  onClick={() => handleToggleActive(u)}
                >
                  {u.is_active ? 'Деактивировать' : 'Активировать'}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>

      {editing && (
        <EditUserModal
          user={editing}
          onClose={() => setEditing(null)}
          onSaved={() => { setEditing(null); reload() }}
        />
      )}
    </div>
  )
}

// EditUserModal — редактирование имени, email и роли пользователя.
// Пароль и активность меняются другими кнопками в строке таблицы.
function EditUserModal({ user, onClose, onSaved }) {
  const [name, setName] = useState(user.name)
  const [email, setEmail] = useState(user.email)
  const [role, setRole] = useState(user.role)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const roleId = useId()

  const handleSubmit = async (e) => {
    e.preventDefault()
    setError('')
    setBusy(true)
    try {
      await adminUpdateUser(user.id, { name: name.trim(), email: email.trim(), role })
      onSaved()
    } catch (err) {
      setError(err.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={(e) => e.stopPropagation()}>
        <h2>Изменить пользователя</h2>
        <form onSubmit={handleSubmit}>
          {error && <div className="error-banner">{error}</div>}
          <label>
            Имя
            <input type="text" value={name} required onChange={(e) => setName(e.target.value)} />
          </label>
          <label>
            Email
            <input type="email" value={email} required onChange={(e) => setEmail(e.target.value)} />
          </label>
          <div className="form-field">
            <Label htmlFor={roleId}>Роль</Label>
            <SelectNative id={roleId} value={role} onChange={(e) => setRole(e.target.value)}>
              {ROLE_OPTIONS.map((r) => (
                <option key={r} value={r}>{ROLE_LABELS[r]}</option>
              ))}
            </SelectNative>
          </div>
          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose} disabled={busy}>
              Отмена
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy}>
              {busy ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
