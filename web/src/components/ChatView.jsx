import { useState } from 'react'
import { Send } from 'lucide-react'
import { MessageList } from './MessageList'
import { toTitleCase } from '../lib/utils'

export function ChatView({ topic, runs, isSending, error, onSend }) {
  const [input, setInput] = useState('')

  const handleSubmit = (e) => {
    e.preventDefault()
    const message = input.trim()
    if (!message || isSending) return
    setInput('')
    onSend(message)
  }

  return (
    <main className="flex-1 flex flex-col min-w-0">
      {/* 话题标题 */}
      <div className="px-6 py-3 border-b border-border">
        <h2 className="text-lg font-display font-bold">{toTitleCase(topic?.name) || ''}</h2>
      </div>

      {/* 消息列表 */}
      <MessageList runs={runs} />

      {/* 错误提示 */}
      {error && (
        <div className="px-6 py-2 text-sm text-accent border-t border-border bg-secondary">
          {error}
        </div>
      )}

      {/* 输入区 */}
      <div className="border-t border-border px-6 py-3">
        <form onSubmit={handleSubmit} className="flex gap-2">
          <input
            type="text"
            placeholder="输入消息..."
            value={input}
            onChange={(e) => setInput(e.target.value)}
            disabled={isSending}
            autoComplete="off"
            className="flex-1 px-3 py-2 border border-input bg-background text-foreground text-sm outline-none focus:border-foreground transition-colors duration-150"
          />
          <button
            type="submit"
            disabled={isSending}
            className="flex items-center gap-1.5 px-4 py-2 text-sm font-medium bg-primary text-primary-foreground hover:opacity-90 transition-opacity duration-150 disabled:opacity-50"
          >
            {isSending ? (
              <span className="inline-block w-4 h-4 border-2 border-primary-foreground border-t-transparent rounded-full animate-spin" />
            ) : (
              <>
                <Send className="w-4 h-4" />
                发送
              </>
            )}
          </button>
        </form>
      </div>
    </main>
  )
}
