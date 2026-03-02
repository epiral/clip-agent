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

function MessageBubble({ label, children, align }) {
  const isUser = align === 'right'
  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'} mb-3`}>
      <div className={`max-w-[80%] ${isUser ? 'bg-primary text-primary-foreground' : 'bg-secondary text-secondary-foreground border border-border'} px-4 py-2.5`}>
        <div className="text-xs opacity-50 font-mono mb-1">{label}</div>
        <div className="text-sm whitespace-pre-wrap">{children}</div>
      </div>
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
    <div className="flex-1 overflow-y-auto px-6 py-4 space-y-2">
      {runs.map((run, i) => (
        <div key={run.id || i}>
          {/* 用户消息 */}
          {run.user_message && (
            <MessageBubble label="you" align="right">{run.user_message}</MessageBubble>
          )}

          {/* agent 回复 */}
          {run.id !== '__streaming__' && getAgentResponse(run) && (
            <MessageBubble label="agent" align="left">{getAgentResponse(run)}</MessageBubble>
          )}

          {/* 流式回复 */}
          {run.id === '__streaming__' && (
            <div className="flex justify-start mb-3">
              <div className="max-w-[80%] bg-secondary text-secondary-foreground border border-border px-4 py-2.5">
                <div className="text-xs opacity-50 font-mono mb-1">agent</div>
                <div className="text-sm whitespace-pre-wrap">
                  {run._streamText || ''}
                  <span className="inline-block w-1.5 h-4 bg-foreground animate-pulse ml-0.5 align-text-bottom" />
                </div>
              </div>
            </div>
          )}
        </div>
      ))}
      <div ref={bottomRef} />
    </div>
  )
}
