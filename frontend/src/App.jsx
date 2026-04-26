import { useState, useEffect } from 'react'
import { Routes, Route, Navigate, Link, useNavigate } from 'react-router-dom'
import { getMe } from './api/client'
import LoginPage from './pages/LoginPage'
import ChangePasswordPage from './pages/ChangePasswordPage'
import AdminUsersPage from './pages/AdminUsersPage'
import CreateUserPage from './pages/CreateUserPage'
import DocumentsListPage from './pages/DocumentsListPage'
import CreateDocumentPage from './pages/CreateDocumentPage'
import DocumentPage from './pages/DocumentPage'
import LibraryPage from './pages/LibraryPage'
import LibraryDocumentPage from './pages/LibraryDocumentPage'
import './App.css'

function NavBar({ user, onLogout }) {
  return (
    <nav className="navbar">
      <div className="navbar-brand">DocManager</div>
      <div className="navbar-links">
        {user.role !== 'developer' && <Link to="/documents">Documents</Link>}
        <Link to="/library">Library</Link>
        {user.role === 'admin' && <Link to="/admin/users">Users</Link>}
      </div>
      <div className="navbar-user">
        <span>{user.name} ({user.role})</span>
        <button className="btn btn-sm btn-secondary" onClick={onLogout}>Выйти</button>
      </div>
    </nav>
  )
}

function ProtectedLayout({ user, setUser, children, adminOnly }) {
  const navigate = useNavigate()
  if (!user) return <Navigate to="/login" />
  if (user.must_change_password) return <Navigate to="/change-password" />
  if (adminOnly && user.role !== 'admin') {
    return <Navigate to={user.role === 'developer' ? '/library' : '/documents'} />
  }

  const handleLogout = () => {
    sessionStorage.removeItem('token')
    setUser(null)
    navigate('/login')
  }

  return (
    <>
      <NavBar user={user} onLogout={handleLogout} />
      <main className="main-content">{children}</main>
    </>
  )
}

export default function App() {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const token = sessionStorage.getItem('token')
    if (!token) { setLoading(false); return }
    getMe()
      .then(setUser)
      .catch(() => { sessionStorage.removeItem('token'); setUser(null) })
      .finally(() => setLoading(false))
  }, [])

  if (loading) return <div className="loading-screen">Loading...</div>

  return (
    <Routes>
      <Route path="/login" element={<LoginPage onLogin={setUser} />} />
      <Route path="/change-password" element={
        user
          ? <ChangePasswordPage user={user} setUser={setUser} />
          : <Navigate to="/login" />
      } />
      <Route path="/documents" element={
        <ProtectedLayout user={user} setUser={setUser}>
          <DocumentsListPage user={user} />
        </ProtectedLayout>
      } />
      <Route path="/documents/create" element={
        <ProtectedLayout user={user} setUser={setUser}>
          <CreateDocumentPage user={user} />
        </ProtectedLayout>
      } />
      <Route path="/documents/:id" element={
        <ProtectedLayout user={user} setUser={setUser}>
          <DocumentPage user={user} />
        </ProtectedLayout>
      } />
      <Route path="/library" element={
        <ProtectedLayout user={user} setUser={setUser}>
          <LibraryPage />
        </ProtectedLayout>
      } />
      <Route path="/library/:id" element={
        <ProtectedLayout user={user} setUser={setUser}>
          <LibraryDocumentPage user={user} />
        </ProtectedLayout>
      } />
      <Route path="/admin/users" element={
        <ProtectedLayout user={user} setUser={setUser} adminOnly>
          <AdminUsersPage />
        </ProtectedLayout>
      } />
      <Route path="/admin/users/new" element={
        <ProtectedLayout user={user} setUser={setUser} adminOnly>
          <CreateUserPage />
        </ProtectedLayout>
      } />
      <Route path="*" element={<Navigate to={user ? (user.role === 'developer' ? '/library' : '/documents') : '/login'} />} />
    </Routes>
  )
}
