import { useState, useEffect, useCallback, useMemo } from 'react'
import { useParams } from 'react-router-dom'
import {
  getLibraryDocument,
  listLibraryThreads, createLibraryThread,
  getMessages,
  listUsers,
} from '../api/client'
import ThreadList from '../components/discussions/ThreadList'
import ThreadMessages from '../components/discussions/ThreadMessages'
import MessageComposer from '../components/discussions/MessageComposer'
import CreateThreadModal from '../components/discussions/CreateThreadModal'
import FullscreenViewer from '../components/FullscreenViewer'

export default function LibraryDocumentPage({ user }) {
  const { id } = useParams()
  const [data, setData] = useState(null)
  const [threads, setThreads] = useState([])
  const [selectedThreadId, setSelectedThreadId] = useState(null)
  const [messages, setMessages] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showFullscreen, setShowFullscreen] = useState(false)
  const [showCreate, setShowCreate] = useState(false)
  const [discussionOpen, setDiscussionOpen] = useState(true)
  const [userNames, setUserNames] = useState({})
  // seenAt[threadId] — последний просмотренный last_message_at; см. DocumentPage.
  const [seenAt, setSeenAt] = useState({})

  const reload = useCallback(async () => {
    try {
      setError('')
      const [doc, ts, revs, writers, admins, devs] = await Promise.all([
        getLibraryDocument(id),
        listLibraryThreads(id).catch(() => []),
        listUsers('reviewer').catch(() => []),
        listUsers('writer').catch(() => []),
        listUsers('admin').catch(() => []),
        listUsers('developer').catch(() => []),
      ])
      setData(doc)
      const list = ts || []
      setThreads(list)
      const map = {}
      for (const u of [...(revs || []), ...(writers || []), ...(admins || []), ...(devs || [])]) {
        map[u.id] = u.name
      }
      setUserNames(map)
      // Если выбранный тред ещё в списке — обновляем его сообщения.
      if (selectedThreadId) {
        const t = list.find((x) => x.id === selectedThreadId)
        setMessages(t ? t.messages || [] : [])
        if (!t) setSelectedThreadId(null)
        else setSeenAt((prev) => ({ ...prev, [selectedThreadId]: t.last_message_at || new Date().toISOString() }))
      }
    } catch (e) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [id, selectedThreadId])

  useEffect(() => { reload() }, [id])

  // Polling — каждые 5 секунд тянем список тредов, чтобы новые сообщения от
  // других пользователей появлялись без ручного обновления. Селектор тредов
  // получает unreadIds, неоткрытый тред с новым сообщением подсветится.
  useEffect(() => {
    const tick = async () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      try {
        const ts = await listLibraryThreads(id)
        const list = ts || []
        setThreads(list)
        if (selectedThreadId) {
          const t = list.find((x) => x.id === selectedThreadId)
          if (t) {
            setMessages(t.messages || [])
            setSeenAt((prev) => ({ ...prev, [selectedThreadId]: t.last_message_at || prev[selectedThreadId] }))
          }
        }
      } catch { /* network blip */ }
    }
    const intervalId = setInterval(tick, 5000)
    return () => clearInterval(intervalId)
  }, [id, selectedThreadId])

  const selectThread = async (threadId) => {
    setSelectedThreadId(threadId)
    try {
      const msgs = await getMessages(threadId)
      setMessages(msgs || [])
      const t = threads.find((x) => x.id === threadId)
      setSeenAt((prev) => ({ ...prev, [threadId]: t?.last_message_at || new Date().toISOString() }))
    } catch (e) { setError(e.message) }
  }

  // Какие треды содержат непрочитанные сообщения для текущего пользователя.
  const unreadThreadIds = useMemo(() => {
    const s = new Set()
    for (const t of threads) {
      if (!t.last_message_at) continue
      const seen = seenAt[t.id]
      if (!seen || t.last_message_at > seen) s.add(t.id)
    }
    return s
  }, [threads, seenAt])

  if (loading) return <div className="loading-screen">Загрузка...</div>
  if (error && !data) return <div className="error-screen">Ошибка: {error}</div>

  const doc = data.document
  const ver = data.published_version
  const fileURL = ver
    ? `/api/v1/documents/${doc.id}/versions/${ver.id}/file?token=${sessionStorage.getItem('token')}`
    : null
  const selectedThread = threads.find((t) => t.id === selectedThreadId)
  // Публичные треды библиотеки закрывает только admin. ThreadMessages
  // прячет кнопку «Закрыть» по этому флагу.
  const canResolve = user.role === 'admin'

  return (
    <div className="page">
      <div className="doc-header">
        <div className="doc-header-info">
          <h1>{doc.title}</h1>
          {doc.description && <p className="doc-desc">{doc.description}</p>}
          <div className="doc-meta">
            <span className="badge status-published">Опубликован</span>
            {ver && <span className="meta-item">Версия {ver.version_number} &middot; {ver.file_name}</span>}
          </div>
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="version-card" style={{ marginTop: '1rem' }}>
        {ver ? (() => {
          const previewURL = ver.file_type === 'docx' ? `${fileURL}&format=pdf` : fileURL
          return (
            <>
              <div className="preview-toolbar">
                <span className="preview-meta">{ver.file_name} ({ver.file_type})</span>
                <button className="btn btn-sm btn-secondary" onClick={() => setShowFullscreen(true)}>
                  Открыть в большом окне
                </button>
              </div>
              <iframe className="pdf-viewer" src={previewURL} title="published preview" />
            </>
          )
        })() : (
          <div className="pdf-placeholder">У документа нет опубликованной версии</div>
        )}
      </div>

      {/* Public discussion — collapsible */}
      <div className="collapsible-section" style={{ marginTop: '1rem' }}>
        <button
          className="collapsible-header"
          onClick={() => setDiscussionOpen(!discussionOpen)}
          aria-expanded={discussionOpen}
        >
          <span className="collapsible-arrow">{discussionOpen ? '▼' : '▶'}</span>
          <span>Обсуждение ({threads.length})</span>
          <span className="collapsible-hint">Видно всем пользователям</span>
        </button>

        {discussionOpen && (
          <div className="collapsible-body">
            <div className="discussions-header">
              <h3 style={{ margin: 0 }}>Темы</h3>
              <button className="btn btn-sm btn-primary" onClick={() => setShowCreate(true)}>
                Новая тема
              </button>
            </div>

            <div className="two-col" style={{ marginTop: '0.5rem' }}>
              <div className="col-left">
                {threads.length === 0
                  ? <div className="empty-state">Тем пока нет. Откройте первую — она будет видна всем пользователям.</div>
                  : <ThreadList threads={threads} selectedId={selectedThreadId} onSelect={selectThread} unreadIds={unreadThreadIds} />}
              </div>
              <div className="col-right">
                {selectedThread ? (
                  <>
                    <ThreadMessages thread={selectedThread} messages={messages}
                      currentUserId={user.id} canResolve={canResolve} onResolved={reload}
                      userNames={userNames} />
                    <MessageComposer thread={selectedThread} onSent={reload} />
                  </>
                ) : (
                  threads.length > 0 && <div className="empty-state">Выберите тему слева</div>
                )}
              </div>
            </div>
          </div>
        )}
      </div>

      {showFullscreen && fileURL && (
        <FullscreenViewer
          src={ver?.file_type === 'docx' ? `${fileURL}&format=pdf` : fileURL}
          title={ver ? `${ver.file_name} (v${ver.version_number})` : 'Просмотр'}
          onClose={() => setShowFullscreen(false)} />
      )}

      {showCreate && (
        <CreateThreadModal
          docId={doc.id}
          createFn={(payload) => createLibraryThread(doc.id, payload)}
          onClose={() => setShowCreate(false)}
          onCreated={async (t) => {
            setShowCreate(false)
            await reload()
            setSelectedThreadId(t.id)
            setMessages([])
          }}
        />
      )}
    </div>
  )
}
