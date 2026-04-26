const BASE = '/api/v1'

function getToken() {
  return sessionStorage.getItem('token')
}

async function request(method, path, body) {
  const headers = { 'Content-Type': 'application/json' }
  const token = getToken()
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(`${BASE}${path}`, {
    method,
    headers,
    body: body ? JSON.stringify(body) : undefined,
  })

  if (res.status === 204) return null

  const data = await res.json()
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`)
  return data
}

// Аутентификация
export const login = (email, password) =>
  request('POST', '/auth/login', { email, password })

export const getMe = () => request('GET', '/auth/me')

export const changePassword = (oldPassword, newPassword) =>
  request('POST', '/auth/change-password', { old_password: oldPassword, new_password: newPassword })

// Управление пользователями (только admin)
export const adminListUsers = () => request('GET', '/admin/users')

export const adminCreateUser = (payload) =>
  request('POST', '/admin/users', payload)

export const adminUpdateUser = (id, payload) =>
  request('PUT', `/admin/users/${id}`, payload)

export const adminSetActive = (id, isActive) =>
  request('PUT', `/admin/users/${id}/active`, { is_active: isActive })

export const adminResetPassword = (id, newPassword) =>
  request('POST', `/admin/users/${id}/reset-password`, { new_password: newPassword })

// Документы
export const listDocuments = () => request('GET', '/documents')

export const getDocument = (id) => request('GET', `/documents/${id}`)

export const createDocument = (payload) =>
  request('POST', '/documents', payload)

export const updateDocument = (id, payload) =>
  request('PUT', `/documents/${id}`, payload)

// Версии документа
export const listVersions = (docId) =>
  request('GET', `/documents/${docId}/versions`)

export const uploadVersion = (docId, payload) =>
  request('POST', `/documents/${docId}/versions`, payload)

// Переходы по статусам ревью
export const submitForReview = (docId) =>
  request('POST', `/documents/${docId}/submit`)

export const approveDocument = (docId) =>
  request('POST', `/documents/${docId}/approve`)

export const requestRevision = (docId, note) =>
  request('POST', `/documents/${docId}/revision`, { note })

export const publishDocument = (docId) =>
  request('POST', `/documents/${docId}/publish`)

export const unpublishDocument = (docId) =>
  request('POST', `/documents/${docId}/unpublish`)

// Агрегирующий эндпоинт страницы обсуждения
export const getDiscussionView = (docId) =>
  request('GET', `/documents/${docId}/discussion-view`)

// Треды и сообщения
export const createThread = (docId, versionId, payload) =>
  request('POST', `/documents/${docId}/versions/${versionId}/threads`, payload)

export const getMessages = (threadId) =>
  request('GET', `/threads/${threadId}/messages`)

export const sendMessage = (threadId, message) =>
  request('POST', `/threads/${threadId}/messages`, { message })

export const resolveThread = (threadId) =>
  request('POST', `/threads/${threadId}/resolve`)

// Селекторы пользователей по роли
export const listUsers = (role) => request('GET', `/users?role=${role}`)

// Diff между двумя версиями
export const getVersionDiff = (docId, v1, v2) =>
  request('GET', `/documents/${docId}/diff?v1=${v1}&v2=${v2}`)

// Библиотека опубликованных
export const listLibrary = () => request('GET', '/library')

export const getLibraryDocument = (id) => request('GET', `/library/${id}`)

// Публичные обсуждения опубликованного документа — открыты любому залогиненному
export const listLibraryThreads = (docId) =>
  request('GET', `/library/${docId}/threads`)

export const createLibraryThread = (docId, payload) =>
  request('POST', `/library/${docId}/threads`, payload)

// Compliance: админский CRUD над наборами правил
export const adminListRuleSets = () => request('GET', '/admin/rule-sets')
export const adminGetRuleSet = (id) => request('GET', `/admin/rule-sets/${id}`)
export const adminCreateRuleSet = (payload) => request('POST', '/admin/rule-sets', payload)
export const adminUpdateRuleSet = (id, payload) => request('PUT', `/admin/rule-sets/${id}`, payload)
export const adminDeleteRuleSet = (id) => request('DELETE', `/admin/rule-sets/${id}`)
export const adminCreateRule = (setId, payload) =>
  request('POST', `/admin/rule-sets/${setId}/rules`, payload)
export const adminUpdateRule = (setId, ruleId, payload) =>
  request('PUT', `/admin/rule-sets/${setId}/rules/${ruleId}`, payload)
export const adminDeleteRule = (setId, ruleId) =>
  request('DELETE', `/admin/rule-sets/${setId}/rules/${ruleId}`)

// Compliance: запуск проверки и история — для writer/reviewer/admin
export const listActiveRuleSets = () => request('GET', '/rule-sets/active')
export const runCompliance = (docId, versionId, ruleSetId) =>
  request('POST', `/documents/${docId}/versions/${versionId}/compliance`, { rule_set_id: ruleSetId })
export const listComplianceChecks = (docId, versionId) =>
  request('GET', `/documents/${docId}/versions/${versionId}/compliance`)
