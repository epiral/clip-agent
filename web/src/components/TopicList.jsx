import { useState, useRef, useEffect } from 'react'
import { Plus } from 'lucide-react'
import { cn, toTitleCase } from '../lib/utils'

export function TopicList({ topics, currentTopicId, onSelectTopic, onNewTopic }) {
  const [creating, setCreating] = useState(false)
  const [name, setName] = useState('')
  const inputRef = useRef(null)
  const submittedRef = useRef(false)

  useEffect(() => {
    if (creating && inputRef.current) {
      inputRef.current.focus()
      submittedRef.current = false
    }
  }, [creating])

  const handleSubmit = async () => {
    if (submittedRef.current) return
    submittedRef.current = true
    const trimmed = name.trim()
    if (trimmed) {
      await onNewTopic(trimmed)
    }
    setCreating(false)
    setName('')
  }

  const handleKeyDown = (e) => {
    if (e.key === 'Enter') handleSubmit()
    if (e.key === 'Escape') { setCreating(false); setName('') }
  }

  return (
    <aside className="w-56 shrink-0 border-r border-border flex flex-col bg-sidebar text-sidebar-foreground">
      <div className="p-3 border-b border-sidebar-border">
        {creating ? (
          <input
            ref={inputRef}
            value={name}
            onChange={(e) => setName(e.target.value)}
            onKeyDown={handleKeyDown}
            onBlur={handleSubmit}
            placeholder="话题名称..."
            className="w-full px-2 py-1.5 text-sm border border-input bg-background text-foreground outline-none focus:border-foreground transition-colors"
          />
        ) : (
          <button
            onClick={() => setCreating(true)}
            className="w-full flex items-center justify-center gap-1.5 px-3 py-1.5 text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity duration-150"
          >
            <Plus className="w-4 h-4" />
            New Topic
          </button>
        )}
      </div>
      <div className="flex-1 overflow-y-auto">
        {topics.length === 0 ? (
          <p className="p-3 text-sm text-muted-foreground">暂无话题</p>
        ) : (
          topics.map((t) => (
            <button
              key={t.id}
              onClick={() => onSelectTopic(t.id)}
              className={cn(
                'group w-full flex items-center gap-2 text-left px-4 py-3 text-sm border-b border-sidebar-border border-l-[3px] border-l-transparent cursor-pointer transition-all duration-200 hover:bg-sidebar-accent',
                t.id === currentTopicId && 'bg-sidebar-accent font-medium !border-l-accent text-sidebar-accent-foreground'
              )}
            >
              <span className={cn(
                'w-1.5 h-1.5 rounded-full shrink-0',
                t.id === currentTopicId ? 'bg-accent' : 'bg-muted-foreground'
              )} />
              <span className="flex-1 truncate">{toTitleCase(t.name)}</span>
              <span className="opacity-30 group-hover:opacity-100 transition-opacity duration-200 text-muted-foreground text-xs">›</span>
            </button>
          ))
        )}
      </div>
    </aside>
  )
}
