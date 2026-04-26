import { useState, useRef, useEffect } from 'react'
import { sendMessage } from '../../api/client'

export default function MessageComposer({ thread, onSent }) {
  const [text, setText] = useState('')
  const [sending, setSending] = useState(false)
  const [error, setError] = useState('')
  const taRef = useRef(null)

  const isResolved = thread.status === 'resolved'

  // Auto-grow: высота textarea подстраивается под содержимое.
  // Сначала сбрасываем height в 'auto', чтобы scrollHeight рассчитался корректно
  // при удалении строк, потом ставим в scrollHeight. CSS-ограничение max-height
  // включит внутренний скролл, если текст совсем большой.
  const autoResize = () => {
    const el = taRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${el.scrollHeight}px`
  }

  // Срабатывает при первом рендере и при сбросе text (после отправки).
  useEffect(autoResize, [text])

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!text.trim() || isResolved) return

    setSending(true)
    setError('')
    try {
      await sendMessage(thread.id, text.trim())
      setText('')
      onSent()
    } catch (err) {
      setError(err.message)
    } finally {
      setSending(false)
    }
  }

  if (isResolved) return null

  return (
    <form className="message-composer" onSubmit={handleSubmit}>
      {error && <div className="error-banner">{error}</div>}
      <div className="composer-row">
        <textarea
          ref={taRef}
          value={text}
          onChange={(e) => setText(e.target.value)}
          placeholder="Написать сообщение... (Enter — отправить, Shift+Enter — новая строка)"
          rows={2}
          disabled={sending}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault()
              handleSubmit(e)
            }
          }}
        />
        <button type="submit" className="btn btn-primary" disabled={sending || !text.trim()}>
          {sending ? '...' : 'Отправить'}
        </button>
      </div>
    </form>
  )
}
