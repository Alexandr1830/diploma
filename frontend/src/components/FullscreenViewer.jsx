import { useEffect } from 'react'

// FullscreenViewer — модалка почти на весь экран с iframe внутри. Удобно
// для inline-просмотра PDF / docx / txt без скачивания и ухода со страницы.
export default function FullscreenViewer({ src, title, onClose }) {
  useEffect(() => {
    const onKey = (e) => { if (e.key === 'Escape') onClose() }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])

  return (
    <div className="fullscreen-modal" onClick={onClose}>
      <div className="fullscreen-frame" onClick={(e) => e.stopPropagation()}>
        <div className="fullscreen-header">
          <span className="fullscreen-title">{title || 'Просмотр файла'}</span>
          <button className="fullscreen-close" onClick={onClose} aria-label="Закрыть">×</button>
        </div>
        <iframe className="fullscreen-iframe" src={src} title={title || 'preview'} />
      </div>
    </div>
  )
}
