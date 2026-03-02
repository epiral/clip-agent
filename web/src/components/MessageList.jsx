import { useEffect, useRef } from 'react'

// 从 run 中提取 agent 回复
function getAgentResponse(run) {
  if (run.messages) {
    try {
      const msgs = JSON.parse(run.messages)
      const assistant = msgs.find((m) => m.role === 'assistant')
      if (assistant) return assistant.content
    } catch {}
  }
  if (run.summary && run.status === 'completed') {
    return run.summary
  }
  return null
}

function MessageBubble({ label, children }) {
  return (
    <div className="border-b border-border pb-3">
      <div className="text-xs text-muted-foreground font-mono mb-1">{label}</div>
      <div className="text-sm whitespace-pre-wrap">{children}</div>
    </div>
  )
}

export function MessageList({ runs }) {
  const bottomRef = useRef(null)

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [runs])

  if (runs.length === 0) {
    return (
      <div className="flex-1 overflow-y-auto px-6 py-4">
        <p className="text-sm text-muted-foreground">暂无消息，发一条开始对话</p>
      </div>
    )
  }

  return (
    <div className="flex-1 overflow-y-auto px-6 py-4 space-y-4">
      {runs.map((run, i) => (
        <div key={run.id || i}>
          {/* 用户消息 */}
          {run.user_message && (
            <MessageBubble label="user">{run.user_message}</MessageBubble>
          )}

          {/* agent 回复 */}
          {run.id !== '__streaming__' && getAgentResponse(run) && (
            <MessageBubble label="agent">{getAgentResponse(run)}</MessageBubble>
          )}

          {/* 流式回复 */}
          {run.id === '__streaming__' && (
            <div className="border-b border-border pb-3">
              <div className="text-xs text-muted-foreground font-mono mb-1">agent</div>
              <div className="text-sm whitespace-pre-wrap">
                {run._streamText || ''}
                <span className="inline-block w-1.5 h-4 bg-foreground animate-pulse ml-0.5 align-text-bottom" />
              </div>
            </div>
          )}
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
