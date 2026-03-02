import { useState, useEffect, useCallback } from 'react'
import { pinixInvoke, pinixInvokeStream } from '../bridge.js'
import { TopicList } from './components/TopicList'
import { ChatView } from './components/ChatView'

export default function App() {
  const [topics, setTopics] = useState([])
  const [currentTopicId, setCurrentTopicId] = useState(null)
  const [runs, setRuns] = useState([])
  const [isSending, setIsSending] = useState(false)
  const [isLoading, setIsLoading] = useState(true)
  const [error, setError] = useState(null)

  // 解析 Bridge 返回的 stdout
  const parseResult = useCallback((result) => {
    const stdout = result?.stdout ?? result
    if (!stdout && stdout !== 0) return null
    const raw = typeof stdout === 'string' ? stdout.trim() : JSON.stringify(stdout)
    if (!raw) return null
    return JSON.parse(raw)
  }, [])

  // 加载话题列表
  const loadTopics = useCallback(async () => {
    try {
      setIsLoading(true)
      const result = await pinixInvoke('list-topics')
      if (result?.exitCode && result.exitCode !== 0) {
        console.error('loadTopics failed:', result.stderr)
        setTopics([])
        return
      }
      const data = parseResult(result)
      setTopics(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadTopics:', e)
      setTopics([])
    } finally {
      setIsLoading(false)
    }
  }, [parseResult])

  // 加载对话记录
  const loadRuns = useCallback(async (topicId) => {
    const id = topicId || currentTopicId
    if (!id) return
    try {
      const result = await pinixInvoke('get-runs', JSON.stringify({ topic_id: id }))
      if (result?.exitCode && result.exitCode !== 0) {
        console.error('loadRuns failed:', result.stderr)
        setRuns([])
        return
      }
      const data = parseResult(result)
      setRuns(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadRuns:', e)
      setRuns([])
    }
  }, [currentTopicId, parseResult])

  // 初始化加载
  useEffect(() => {
    loadTopics()
  }, [loadTopics])

  // 选择话题
  const handleSelectTopic = useCallback(async (topicId) => {
    setCurrentTopicId(topicId)
    setRuns([])
    setError(null)
    try {
      const result = await pinixInvoke('get-runs', JSON.stringify({ topic_id: topicId }))
      if (result?.exitCode && result.exitCode !== 0) {
        console.error('loadRuns failed:', result.stderr)
        setRuns([])
        return
      }
      const data = parseResult(result)
      setRuns(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadRuns:', e)
      setRuns([])
    }
  }, [parseResult])

  // 新建话题
  const handleNewTopic = useCallback(async (name) => {
    if (!name) return
    try {
      const result = await pinixInvoke('create-topic', JSON.stringify({ name }))
      if (result?.exitCode && result.exitCode !== 0) {
        console.error('createTopic failed:', result.stderr)
        return
      }
      const data = parseResult(result)
      const newTopicId = data?.topic_id
      if (newTopicId) setCurrentTopicId(newTopicId)
      await loadTopics()
      setRuns([])
    } catch (e) {
      console.error('createTopic:', e)
    }
  }, [loadTopics, parseResult])

  // 发送消息（流式）
  const handleSend = useCallback((message) => {
    if (!currentTopicId) return
    setIsSending(true)
    setError(null)

    // 添加用户消息 + 流式占位
    setRuns((prev) => [
      ...prev,
      {
        id: '__streaming__',
        user_message: message,
        status: 'in_progress',
        _streamText: '',
      },
    ])

    let streamBuffer = ''
    const topicId = currentTopicId

    pinixInvokeStream(
      'send-message',
      JSON.stringify({ topic_id: topicId, message }),
      (chunk) => {
        streamBuffer += chunk
        setRuns((prev) =>
          prev.map((r) =>
            r.id === '__streaming__' ? { ...r, _streamText: streamBuffer } : r
          )
        )
      },
      (exitCode) => {
        setIsSending(false)
        if (exitCode && exitCode !== 0) {
          setError('消息发送失败，请重试')
          // Remove streaming placeholder on error
          setRuns((prev) => prev.filter((r) => r.id !== '__streaming__'))
        } else {
          loadRuns(topicId)
        }
      }
    )
  }, [currentTopicId, loadRuns])

  const currentTopic = topics.find((t) => t.id === currentTopicId)

  return (
    <>
      <header className="border-b border-border px-6 py-3 font-display">
        <h1 className="text-2xl font-bold tracking-tight">agents, assembled.</h1>
      </header>
      <div className="flex flex-1 min-h-0">
        {isLoading ? (
          <aside className="w-56 shrink-0 border-r border-border flex items-center justify-center bg-sidebar">
            <div className="flex flex-col items-center gap-2 text-muted-foreground">
              <div className="w-5 h-5 border-2 border-muted-foreground border-t-transparent rounded-full animate-spin" />
              <span className="text-xs">加载中</span>
            </div>
          </aside>
        ) : (
          <TopicList
            topics={topics}
            currentTopicId={currentTopicId}
            onSelectTopic={handleSelectTopic}
            onNewTopic={handleNewTopic}
          />
        )}
        {currentTopicId ? (
          <ChatView
            topic={currentTopic}
            runs={runs}
            isSending={isSending}
            error={error}
            onSend={handleSend}
          />
        ) : (
          <main className="flex-1 flex items-center justify-center text-muted-foreground text-sm">
            选择一个话题或创建新话题
          </main>
        )}
      </div>
    </>
  )
}
