import { useState, useEffect, useCallback, useMemo, useRef, useId, Fragment } from 'react'
import { Label } from '../components/ui/Label'
import { SelectNative } from '../components/ui/SelectNative'
import { useParams } from 'react-router-dom'
import {
  getDiscussionView, getMessages, listVersions, uploadVersion,
  updateDocument, submitForReview, approveDocument, requestRevision,
  publishDocument, unpublishDocument,
  listUsers,
} from '../api/client'
import ThreadList from '../components/discussions/ThreadList'
import ThreadMessages from '../components/discussions/ThreadMessages'
import MessageComposer from '../components/discussions/MessageComposer'
import CreateThreadModal from '../components/discussions/CreateThreadModal'
import FullscreenViewer from '../components/FullscreenViewer'
import DiffModal from '../components/DiffModal'

const STATUS_LABELS = {
  draft: 'Черновик', in_review: 'На ревью', needs_revision: 'Доработка',
  approved: 'Одобрен', published: 'Опубликован', archived: 'Архив',
}

// formatDateTime — компактный человекочитаемый формат даты+времени для
// мест вроде «загружена: 25.04.2026 14:30». Используется в шапке документа.
function formatDateTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleString('ru-RU', {
    day: '2-digit', month: '2-digit', year: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export default function DocumentPage({ user }) {
  const { id } = useParams()
  const [data, setData] = useState(null)
  const [versions, setVersions] = useState([])
  const [reviewers, setReviewers] = useState([])
  const [userNames, setUserNames] = useState({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [selectedThreadId, setSelectedThreadId] = useState(null)
  const [messages, setMessages] = useState([])
  const [showCreateThread, setShowCreateThread] = useState(false)
  const [showUpload, setShowUpload] = useState(false)
  const [showSetReviewer, setShowSetReviewer] = useState(false)
  const [showEditMeta, setShowEditMeta] = useState(false)
  const [actionBusy, setActionBusy] = useState(false)
  const [showFullscreen, setShowFullscreen] = useState(false)
  const [diffPair, setDiffPair] = useState(null) // { v1, v2, v1Label, v2Label } | null
  // seenAt[threadId] = last_message_at ISO timestamp, который пользователь
  // уже видел. Используется чтобы пометить треды с новыми сообщениями бейджем
  // «новое» в ThreadList. Обновляется при выборе треда, отправке и polling'е
  // выбранного треда.
  const [seenAt, setSeenAt] = useState({})

  const load = useCallback(async () => {
    try {
      setError('')
      const [resp, vers, revs, writers, admins, devs] = await Promise.all([
        getDiscussionView(id),
        listVersions(id).catch(() => []),
        listUsers('reviewer').catch(() => []),
        listUsers('writer').catch(() => []),
        listUsers('admin').catch(() => []),
        listUsers('developer').catch(() => []),
      ])
      setData(resp)
      setVersions(vers || [])
      setReviewers(revs || [])
      // Собираем id→name по всем ролям (включая developer'ов — они пишут в
      // публичные треды библиотеки). Подгружаем разом, чтобы вместо имени не
      // вываливался fallback «Пользователь #N».
      const map = {}
      for (const u of [...(revs||[]), ...(writers||[]), ...(admins||[]), ...(devs||[])]) {
        map[u.id] = u.name
      }
      setUserNames(map)
      if (resp.threads.length > 0 && !selectedThreadId) {
        setSelectedThreadId(resp.threads[0].id)
        setMessages(resp.threads[0].messages || [])
      } else if (selectedThreadId) {
        const t = resp.threads.find((t) => t.id === selectedThreadId)
        setMessages(t ? t.messages || [] : [])
      }
    } catch (err) {
      setError(err.message)
    } finally {
      setLoading(false)
    }
  }, [id, selectedThreadId])

  useEffect(() => { load() }, [id])

  // Polling — каждые 5 секунд тянем discussion-view, чтобы автор сразу видел
  // новые треды и сообщения от ревьюера без ручного обновления страницы.
  // Если selected тред есть — обновляем и его сообщения, и помечаем seenAt
  // (открытый тред считается «прочитанным»). Polling приостанавливается, если
  // вкладка скрыта (visibilityState).
  useEffect(() => {
    const tick = async () => {
      if (typeof document !== 'undefined' && document.visibilityState !== 'visible') return
      try {
        const resp = await getDiscussionView(id)
        setData(resp)
        if (selectedThreadId) {
          const t = resp.threads.find((x) => x.id === selectedThreadId)
          if (t) {
            setMessages(t.messages || [])
            setSeenAt((prev) => ({ ...prev, [selectedThreadId]: t.last_message_at || prev[selectedThreadId] }))
          }
        }
      } catch { /* network blip — повторим через 5 сек */ }
    }
    const intervalId = setInterval(tick, 5000)
    return () => clearInterval(intervalId)
  }, [id, selectedThreadId])

  const selectThread = async (threadId) => {
    setSelectedThreadId(threadId)
    try {
      const msgs = await getMessages(threadId)
      setMessages(msgs || [])
      // Помечаем тред прочитанным — берём last_message_at из data, т.к. msgs
      // не содержит этого поля сами по себе.
      const t = data?.threads.find((x) => x.id === threadId)
      setSeenAt((prev) => ({ ...prev, [threadId]: t?.last_message_at || new Date().toISOString() }))
    } catch (err) { setError(err.message) }
  }

  const refreshAfterAction = async () => {
    const resp = await getDiscussionView(id)
    setData(resp)
    if (selectedThreadId) {
      const t = resp.threads.find((t) => t.id === selectedThreadId)
      setMessages(t ? t.messages || [] : [])
      if (t) setSeenAt((prev) => ({ ...prev, [selectedThreadId]: t.last_message_at || new Date().toISOString() }))
    }
  }

  const doAction = async (fn, ...args) => {
    setActionBusy(true)
    setError('')
    try {
      await fn(...args)
      await load()
    } catch (err) { setError(err.message) }
    finally { setActionBusy(false) }
  }

  const versionByID = useMemo(() => {
    const m = {}
    for (const v of versions) m[v.id] = v.version_number
    return m
  }, [versions])

  // Какие треды сейчас имеют непрочитанные сообщения (last_message_at >
  // seenAt[id]). Используется для бейджа «новое» в ThreadList. Текущий
  // выбранный тред автоматически помечается прочитанным polling'ом, поэтому
  // он сюда не попадёт.
  const unreadThreadIds = useMemo(() => {
    if (!data?.threads) return new Set()
    const s = new Set()
    for (const t of data.threads) {
      if (!t.last_message_at) continue
      const seen = seenAt[t.id]
      if (!seen || t.last_message_at > seen) s.add(t.id)
    }
    return s
  }, [data, seenAt])

  if (loading) return <div className="loading-screen">Загрузка документа...</div>
  if (error && !data) return <div className="error-screen">Ошибка: {error}</div>

  const doc = data.document
  const ver = data.current_version
  const status = doc.status
  const role = user.role
  const isOwner = doc.created_by === user.id
  const reviewerId = doc.reviewer_id?.Valid ? doc.reviewer_id.Int64 : null
  const isAssignedReviewer = reviewerId === user.id
  const canResolve = role === 'reviewer' || role === 'admin'
  const selectedThread = data?.threads.find((t) => t.id === selectedThreadId)
  const fileURL = ver
    ? `/api/v1/documents/${doc.id}/versions/${ver.id}/file?token=${sessionStorage.getItem('token')}`
    : null

  return (
    <div className="page">
      {/* Header */}
      <div className="doc-header">
        <div className="doc-header-info">
          <div className="doc-header-title-row">
            <h1>{doc.title}</h1>
            {role !== 'developer' && (
              <button className="btn btn-sm btn-secondary" onClick={() => setShowEditMeta(true)}>
                Изменить
              </button>
            )}
          </div>
          {doc.description && <p className="doc-desc">{doc.description}</p>}
          <dl className="doc-meta-grid">
            <div className="doc-meta-row">
              <dt>Статус</dt>
              <dd><span className={`badge status-${status}`}>{STATUS_LABELS[status] || status}</span></dd>
            </div>
            <div className="doc-meta-row">
              <dt>Автор</dt>
              <dd>{userNames[doc.created_by] || `#${doc.created_by}`}</dd>
            </div>
            <div className="doc-meta-row">
              <dt>Ревьюер</dt>
              <dd>{reviewerId ? (userNames[reviewerId] || `#${reviewerId}`) : <span className="text-muted">не назначен</span>}</dd>
            </div>
            {ver ? (
              <>
                <div className="doc-meta-row">
                  <dt>Версия</dt>
                  <dd>v{ver.version_number} <span className="text-muted">({ver.file_name})</span></dd>
                </div>
                <div className="doc-meta-row">
                  <dt>Загружена</dt>
                  <dd>{formatDateTime(ver.created_at)}</dd>
                </div>
              </>
            ) : (
              <div className="doc-meta-row">
                <dt>Версия</dt>
                <dd className="text-muted">нет</dd>
              </div>
            )}
          </dl>
        </div>
        <div className="doc-header-actions">
          <ActionButtons
            status={status} role={role} isOwner={isOwner}
            isAssignedReviewer={isAssignedReviewer} reviewerId={reviewerId}
            busy={actionBusy} docId={doc.id}
            onAction={doAction}
            onSetReviewer={() => setShowSetReviewer(true)}
          />
        </div>
      </div>

      {error && <div className="error-banner">{error}</div>}

      <div className="two-col">
        {/* Left column */}
        <div className="col-left">
          {/* Versions */}
          <div className="version-card">
            <div className="version-card-header">
              <h3>Версии документа</h3>
              {(isOwner || role === 'admin') && (
                <button className="btn btn-sm btn-primary" onClick={() => setShowUpload(!showUpload)}>
                  {showUpload ? 'Отмена' : 'Загрузить версию'}
                </button>
              )}
            </div>

            {showUpload && <UploadForm docId={doc.id} onDone={() => { setShowUpload(false); load() }} />}

            {versions.length === 0 ? (
              <div className="empty-state">Версий пока нет</div>
            ) : (
              <table className="versions-table">
                <thead><tr><th>v</th><th>Файл</th><th>Тип</th><th>Текущая</th><th></th></tr></thead>
                <tbody>
                  {versions.map((v) => {
                    const summary = v.change_summary?.Valid ? v.change_summary.String : ''
                    return (
                      // Используем Fragment + key, чтобы на одну версию рендерить
                      // две строки: основную (метаданные) и подстроку с описанием
                      // изменений. Подстрока сцепляется визуально с верхней через
                      // отсутствие верхней границы и общий highlight class.
                      <Fragment key={v.id}>
                        <tr className={`${v.is_current ? 'current-version' : ''} version-main-row`}>
                          <td>{v.version_number}</td>
                          <td>{v.file_name}</td>
                          <td>{v.file_type}</td>
                          <td>{v.is_current ? 'Да' : ''}</td>
                          <td>
                            <a
                              className="btn btn-sm btn-secondary"
                              href={`/api/v1/documents/${doc.id}/versions/${v.id}/file?token=${sessionStorage.getItem('token')}`}
                              download={v.file_name}
                              target="_blank"
                              rel="noreferrer"
                            >Скачать</a>
                          </td>
                        </tr>
                        <tr className={`version-summary-row ${v.is_current ? 'current-version' : ''}`}>
                          <td></td>
                          <td colSpan={4} className="version-summary-cell">
                            {summary
                              ? <><span className="version-summary-label">Изменения:</span> {summary}</>
                              : <span className="text-muted">Описание изменений не указано</span>}
                          </td>
                        </tr>
                      </Fragment>
                    )
                  })}
                </tbody>
              </table>
            )}

            {ver ? (
              <FilePreview ver={ver} fileURL={fileURL} onFullscreen={() => setShowFullscreen(true)} />
            ) : (
              <div className="pdf-placeholder">Нет текущей версии</div>
            )}
          </div>

          {/* Set reviewer */}
          {showSetReviewer && (
            <SetReviewerForm docId={doc.id} docTitle={doc.title} currentReviewerId={reviewerId}
              onDone={() => { setShowSetReviewer(false); load() }} />
          )}

          {versions.length >= 2 && (
            <DiffPicker versions={versions} onCompare={(p) => setDiffPair(p)} />
          )}
        </div>

        {/* Right column: Discussions */}
        <div className="col-right">
          <div className="discussions-header">
            <h3>Обсуждения</h3>
            {ver && <button className="btn btn-sm btn-primary" onClick={() => setShowCreateThread(true)}>Новый тред</button>}
          </div>

          <ThreadList threads={data.threads} selectedId={selectedThreadId} onSelect={selectThread} versionByID={versionByID} unreadIds={unreadThreadIds} />

          {selectedThread ? (
            <>
              <ThreadMessages thread={selectedThread} messages={messages}
                currentUserId={user.id} canResolve={canResolve} onResolved={refreshAfterAction}
                userNames={userNames} />
              <MessageComposer thread={selectedThread} onSent={refreshAfterAction} />
            </>
          ) : (
            data.threads.length === 0 && <div className="empty-state">Обсуждений пока нет</div>
          )}
        </div>
      </div>

      {showCreateThread && ver && (
        <CreateThreadModal docId={doc.id} versionId={ver.id}
          onClose={() => setShowCreateThread(false)}
          onCreated={async (t) => { setShowCreateThread(false); await load(); setSelectedThreadId(t.id); setMessages([]) }} />
      )}

      {showFullscreen && fileURL && (
        <FullscreenViewer
          src={ver?.file_type === 'docx' ? `${fileURL}&format=pdf` : fileURL}
          title={ver ? `${ver.file_name} (v${ver.version_number})` : 'Просмотр'}
          onClose={() => setShowFullscreen(false)} />
      )}

      {diffPair && (
        <DiffModal docId={doc.id} v1={diffPair.v1} v2={diffPair.v2}
          v1Label={diffPair.v1Label} v2Label={diffPair.v2Label}
          onClose={() => setDiffPair(null)} />
      )}

      {showEditMeta && (
        <EditDocMetaModal
          doc={doc}
          onClose={() => setShowEditMeta(false)}
          onSaved={() => { setShowEditMeta(false); load() }}
        />
      )}
    </div>
  )
}

/* --- Sub-components --- */

// EditDocMetaModal — изменение названия и описания документа.
// Доступно всем, кроме developer'а; backend (canEditMeta) пропускает только
// writer-владельца, reviewer-назначенного и admin'а — UI это лишь зеркалит.
// Категория и ревьюер не меняются здесь (последний — отдельной кнопкой
// «Назначить reviewer»), но передаются в payload как есть, потому что
// UpdateDocumentRequest требует category_id и трактует null reviewer_id как
// «снять ревьюера» — нам это не нужно.
function EditDocMetaModal({ doc, onClose, onSaved }) {
  const [title, setTitle] = useState(doc.title)
  const [description, setDescription] = useState(doc.description || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!title.trim()) return
    setBusy(true); setError('')
    try {
      const reviewerId = doc.reviewer_id?.Valid ? doc.reviewer_id.Int64 : null
      await updateDocument(doc.id, {
        title: title.trim(),
        description: description,
        category_id: doc.category_id,
        reviewer_id: reviewerId,
      })
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
        <h2>Изменить документ</h2>
        <form onSubmit={handleSubmit}>
          {error && <div className="error-banner">{error}</div>}
          <label>
            Название
            <input type="text" value={title} required autoFocus
              onChange={(e) => setTitle(e.target.value)} />
          </label>
          <label>
            Описание
            <textarea value={description} rows={4}
              onChange={(e) => setDescription(e.target.value)} />
          </label>
          <div className="modal-actions">
            <button type="button" className="btn btn-secondary" onClick={onClose} disabled={busy}>
              Отмена
            </button>
            <button type="submit" className="btn btn-primary" disabled={busy || !title.trim()}>
              {busy ? 'Сохранение...' : 'Сохранить'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// FilePreview показывает inline-просмотр текущей версии в iframe.
// Для docx используется PDF-рендер, который backend генерирует через
// LibreOffice во время upload (?format=pdf). Это нужно, потому что
// браузер всегда скачивает .docx — а PDF рендерится нативно. Кнопка
// скачивания оригинала здесь не нужна: она есть в строке таблицы версий.
// Для txt/md/yaml iframe грузит файл как есть — браузер показывает их
// как plain text.
function FilePreview({ ver, fileURL, onFullscreen }) {
  const previewURL = ver.file_type === 'docx' ? `${fileURL}&format=pdf` : fileURL
  return (
    <>
      <div className="preview-toolbar">
        <span className="preview-meta">
          {ver.file_name} ({ver.file_type})
        </span>
        <button className="btn btn-sm btn-secondary" onClick={onFullscreen}>
          Открыть в большом окне
        </button>
      </div>
      <iframe className="pdf-viewer" src={previewURL} title="preview" />
    </>
  )
}

function ActionButtons({ status, role, isOwner, isAssignedReviewer, reviewerId, busy, docId, onAction, onSetReviewer }) {
  const btns = []

  if ((role === 'writer' && isOwner) || role === 'admin') {
    if (status === 'draft' || status === 'needs_revision') {
      if (!reviewerId) {
        btns.push(<button key="rev" className="btn btn-secondary" onClick={onSetReviewer} disabled={busy}>Назначить reviewer</button>)
      }
      btns.push(
        <button key="submit" className="btn btn-primary" onClick={() => onAction(submitForReview, docId)}
          disabled={busy || !reviewerId}>Отправить на ревью</button>
      )
    }
    if (status === 'approved') {
      btns.push(<button key="pub" className="btn btn-success" onClick={() => onAction(publishDocument, docId)} disabled={busy}>Опубликовать</button>)
    }
    if (status === 'published') {
      btns.push(<button key="unpub" className="btn btn-warning" onClick={() => onAction(unpublishDocument, docId)} disabled={busy}>Снять публикацию</button>)
    }
  }

  // Reviewer тоже может снять публикацию — например, заметил ошибку уже после approve.
  if (role === 'reviewer' && isAssignedReviewer && status === 'published') {
    btns.push(<button key="unpub" className="btn btn-warning" onClick={() => onAction(unpublishDocument, docId)} disabled={busy}>Снять публикацию</button>)
  }

  if ((role === 'reviewer' && isAssignedReviewer) || role === 'admin') {
    if (status === 'in_review') {
      btns.push(<button key="appr" className="btn btn-success" onClick={() => onAction(approveDocument, docId)} disabled={busy}>Одобрить</button>)
      btns.push(<button key="rev" className="btn btn-warning" onClick={() => {
        const note = prompt('Причина доработки (комментарий обязателен, попадёт в обсуждение):')
        if (note === null) return // отмена
        const trimmed = note.trim()
        if (!trimmed) {
          alert('Комментарий обязателен — без него нельзя отправить на доработку.')
          return
        }
        onAction(requestRevision, docId, trimmed)
      }} disabled={busy}>На доработку</button>)
    }
  }

  return <>{btns}</>
}

function UploadForm({ docId, onDone }) {
  const [file, setFile] = useState(null)
  const [summary, setSummary] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const summaryRef = useRef(null)

  // Auto-grow textarea «Описание изменений»: сбрасываем height, потом ставим
  // в scrollHeight, чтобы поле росло вместе с текстом. Потолок задан CSS
  // (max-height на .upload-summary), после него включается внутренний скролл.
  useEffect(() => {
    const el = summaryRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }, [summary])

  // .yml на бэке как enum нет — мапим в "yaml".
  const EXT_TO_TYPE = { docx: 'docx', txt: 'txt', md: 'md', yaml: 'yaml', yml: 'yaml' }

  const handleFileChange = (e) => {
    const f = e.target.files[0]
    if (!f) { setFile(null); return }
    const ext = f.name.split('.').pop().toLowerCase()
    if (!EXT_TO_TYPE[ext]) {
      setError('Допустимые форматы: PDF, DOCX, TXT, MD, YAML')
      setFile(null)
      e.target.value = ''
      return
    }
    setError('')
    setFile(f)
  }

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!file) return
    setBusy(true); setError('')
    const ext = file.name.split('.').pop().toLowerCase()
    try {
      await uploadVersion(docId, {
        file_name: file.name,
        file_path: `/files/${file.name}`,
        file_type: EXT_TO_TYPE[ext],
        change_summary: summary,
      })
      onDone()
    } catch (err) { setError(err.message) }
    finally { setBusy(false) }
  }

  return (
    <form className="upload-form" onSubmit={handleSubmit}>
      {error && <div className="error-banner">{error}</div>}
      <div className="upload-file-row">
        <label className="file-picker">
          <input type="file" accept=".docx,.txt,.md,.yaml,.yml" onChange={handleFileChange} />
          <span className="btn btn-sm btn-secondary">Выбрать файл</span>
          <span className="file-picker-name">{file ? file.name : 'Файл не выбран'}</span>
        </label>
      </div>
      <div className="upload-bottom-row">
        <textarea
          ref={summaryRef}
          className="upload-summary"
          placeholder="Описание изменений (что нового по сравнению с прошлой версией)"
          value={summary}
          onChange={(e) => setSummary(e.target.value)}
          rows={1}
        />
        <button type="submit" className="btn btn-sm btn-primary" disabled={busy || !file}>
          {busy ? 'Загрузка...' : 'Загрузить'}
        </button>
      </div>
    </form>
  )
}

function SetReviewerForm({ docId, docTitle, currentReviewerId, onDone }) {
  const [reviewers, setReviewers] = useState([])
  const [reviewerId, setReviewerId] = useState(currentReviewerId || '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  useEffect(() => {
    listUsers('reviewer').then((data) => setReviewers(data || [])).catch(() => {})
  }, [])

  const handleSubmit = async (e) => {
    e.preventDefault()
    setBusy(true); setError('')
    try {
      await updateDocument(docId, {
        title: docTitle,
        category_id: 1,
        reviewer_id: Number(reviewerId) || null,
      })
      onDone()
    } catch (err) { setError(err.message) }
    finally { setBusy(false) }
  }

  return (
    <form className="inline-form" onSubmit={handleSubmit} style={{ marginTop: '0.75rem' }}>
      {error && <div className="error-banner">{error}</div>}
      <SelectNative value={reviewerId} onChange={(e) => setReviewerId(e.target.value)} required style={{ minWidth: 220 }}>
        <option value="">Выберите reviewer</option>
        {reviewers.map((r) => (
          <option key={r.id} value={r.id}>{r.name} (ID {r.id})</option>
        ))}
      </SelectNative>
      <button type="submit" className="btn btn-sm btn-primary" disabled={busy || !reviewerId}>
        {busy ? '...' : 'Назначить'}
      </button>
    </form>
  )
}

// DiffPicker — селекторы «Старая» / «Новая» и кнопка «Сравнить». Списки
// взаимозависимы: «Старая» не показывает самую свежую версию, «Новая» — самую
// раннюю. После выбора «Старой» в «Новой» остаются только версии новее.
// Сам diff рендерится в отдельной модалке (DiffModal).
function DiffPicker({ versions, onCompare }) {
  const oldSelectId = useId()
  const newSelectId = useId()
  // С бэка версии приходят от новой к старой (created_at DESC). Для селекторов
  // удобнее обратный порядок: v1 → vN читается естественно, и фильтр
  // «новее старой» делается простым slice'ом.
  const sorted = useMemo(
    () => [...versions].sort((a, b) => Number(a.version_number) - Number(b.version_number)),
    [versions]
  )
  const oldOptions = sorted.slice(0, -1)            // самую новую исключаем
  const [oldId, setOldId] = useState(() => oldOptions[0] ? String(oldOptions[0].id) : '')
  const oldVer = sorted.find((v) => String(v.id) === oldId)
  const newOptions = oldVer
    ? sorted.filter((v) => Number(v.version_number) > Number(oldVer.version_number))
    : sorted.slice(1)                                // самую старую исключаем
  const [newId, setNewId] = useState('')

  // Когда меняется список доступных «Новых», подставляем ближайшую к
  // текущей «Старой» версию — обычно это и есть то, что хочет пользователь.
  useEffect(() => {
    if (!newOptions.length) { setNewId(''); return }
    if (!newOptions.find((v) => String(v.id) === newId)) {
      setNewId(String(newOptions[0].id))
    }
  }, [oldId, sorted])

  const handleClick = () => {
    if (!oldId || !newId || oldId === newId) return
    const ver1 = sorted.find((v) => String(v.id) === oldId)
    const ver2 = sorted.find((v) => String(v.id) === newId)
    onCompare({
      v1: Number(oldId), v2: Number(newId),
      v1Label: ver1 ? `v${ver1.version_number}` : oldId,
      v2Label: ver2 ? `v${ver2.version_number}` : newId,
    })
  }

  return (
    <div className="diff-section">
      <h3>Сравнение версий</h3>
      <div className="inline-form">
        <div className="diff-select-label">
          <Label htmlFor={oldSelectId}>Старая</Label>
          <SelectNative id={oldSelectId} value={oldId} onChange={(e) => setOldId(e.target.value)}>
            {oldOptions.map((v) => <option key={v.id} value={v.id}>v{v.version_number}</option>)}
          </SelectNative>
        </div>
        <div className="diff-select-label">
          <Label htmlFor={newSelectId}>Новая</Label>
          <SelectNative id={newSelectId} value={newId} onChange={(e) => setNewId(e.target.value)} disabled={!newOptions.length}>
            {newOptions.map((v) => <option key={v.id} value={v.id}>v{v.version_number}</option>)}
          </SelectNative>
        </div>
        <button className="btn btn-sm btn-primary" onClick={handleClick}
          disabled={!oldId || !newId || oldId === newId}>Сравнить</button>
      </div>
      <p className="hint">Откроется отдельное окно с распарсенным текстом и подсветкой изменений</p>
    </div>
  )
}
