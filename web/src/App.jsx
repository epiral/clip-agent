import { useState, useEffect, useCallback } from 'react'
import { pinixInvoke, pinixInvokeStream } from '../bridge.js'
import { TopicList } from './components/TopicList'
import { ChatView } from './components/ChatView'

export default function App() {
  const [topics, setTopics] = useState([])
  const [currentTopicId, setCurrentTopicId] = useState(null)
  const [runs, setRuns] = useState([])
  const [isSending, setIsSending] = useState(false)

  // 加载话题列表
  const loadTopics = useCallback(async () => {
    try {
      const result = await pinixInvoke('list-topics')
      const stdout = result.stdout ?? result
      const raw = typeof stdout === 'string' ? stdout : JSON.stringify(stdout)
      const data = JSON.parse(raw)
      setTopics(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadTopics:', e)
      setTopics([])
    }
  }, [])

  // 加载对话记录
  const loadRuns = useCallback(async (topicId) => {
    const id = topicId || currentTopicId
    if (!id) return
    try {
      const result = await pinixInvoke('get-runs', JSON.stringify({ topic_id: id }))
      const stdout = result.stdout ?? result
      const raw = typeof stdout === 'string' ? stdout : JSON.stringify(stdout)
      const data = JSON.parse(raw)
      setRuns(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadRuns:', e)
      setRuns([])
    }
  }, [currentTopicId])

  // 初始化加载
  useEffect(() => {
    loadTopics()
  }, [loadTopics])

  // 选择话题
  const handleSelectTopic = useCallback(async (topicId) => {
    setCurrentTopicId(topicId)
    setRuns([])
    try {
      const result = await pinixInvoke('get-runs', JSON.stringify({ topic_id: topicId }))
      const stdout = result.stdout ?? result
      const raw = typeof stdout === 'string' ? stdout : JSON.stringify(stdout)
      const data = JSON.parse(raw)
      setRuns(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('loadRuns:', e)
      setRuns([])
    }
  }, [])

  // 新建话题
  const handleNewTopic = useCallback(async (name) => {
    if (!name) return
    try {
      const result = await pinixInvoke('create-topic', JSON.stringify({ name }))
      const stdout = result.stdout || result
      const data = JSON.parse(typeof stdout === 'string' ? stdout : JSON.stringify(stdout))
      const newTopicId = data.topic_id
      setCurrentTopicId(newTopicId)
      await loadTopics()
      // 新话题无历史记录
      setRuns([])
    } catch (e) {
      console.error('createTopic:', e)
    }
  }, [loadTopics])

  // 发送消息（流式）
  const handleSend = useCallback((message) => {
    if (!currentTopicId) return
    setIsSending(true)

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

    pinixInvokeStream(
      'send-message',
      JSON.stringify({ topic_id: currentTopicId, message }),
      (chunk) => {
        streamBuffer += chunk
        // 更新流式文本
        setRuns((prev) =>
          prev.map((r) =>
            r.id === '__streaming__' ? { ...r, _streamText: streamBuffer } : r
          )
        )
      },
      () => {
        setIsSending(false)
        // 重新加载完整 runs
        loadRuns(currentTopicId)
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
        <TopicList
          topics={topics}
          currentTopicId={currentTopicId}
          onSelectTopic={handleSelectTopic}
          onNewTopic={handleNewTopic}
        />
        {currentTopicId ? (
          <ChatView
            topic={currentTopic}
            runs={runs}
            isSending={isSending}
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
