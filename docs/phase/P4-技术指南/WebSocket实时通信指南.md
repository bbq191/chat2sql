# 📡 WebSocket实时通信指南

## 🎯 技术概述

WebSocket技术为Chat2SQL提供了实时双向通信能力，实现查询状态的实时推送、结果的流式传输和多用户协作功能。本指南详细介绍P4阶段WebSocket的实现策略。

### ✨ 核心价值

| 功能特性 | 技术实现 | 业务价值 | 性能提升 |
|---------|---------|---------|---------|
| **实时查询状态** | WebSocket状态推送 | 用户体验提升 | 响应时间减少60% |
| **流式结果传输** | 分块数据推送 | 大数据集优化 | 首屏时间减少70% |
| **多用户协作** | 广播消息机制 | 团队协作增强 | 同步延迟<100ms |
| **连接保活** | 心跳检测机制 | 连接稳定性 | 可用性>99.5% |

### 🎁 业务场景

- **实时查询反馈**：SQL执行进度、错误提示、完成通知
- **流式数据展示**：大结果集的分批推送和渐进显示
- **协作查询会话**：多用户共享查询空间和实时协作
- **系统状态通知**：服务状态、维护通知、性能警告

---

## 🏗️ WebSocket客户端架构

### 📦 连接管理核心

```tsx
// WebSocket连接管理器
class WebSocketManager {
  private ws: WebSocket | null = null;
  private url: string;
  private protocols: string[];
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 1000;
  private heartbeatInterval: NodeJS.Timeout | null = null;
  private messageQueue: any[] = [];
  private listeners = new Map<string, Set<Function>>();
  
  constructor(url: string, protocols: string[] = []) {
    this.url = url;
    this.protocols = protocols;
  }
  
  // 建立WebSocket连接
  connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url, this.protocols);
        
        this.ws.onopen = (event) => {
          console.log('WebSocket连接已建立');
          this.reconnectAttempts = 0;
          this.startHeartbeat();
          this.processMessageQueue();
          this.emit('connected', event);
          resolve();
        };
        
        this.ws.onmessage = (event) => {
          this.handleMessage(event);
        };
        
        this.ws.onclose = (event) => {
          console.log('WebSocket连接已关闭', event.code, event.reason);
          this.cleanup();
          this.emit('disconnected', event);
          
          if (!event.wasClean && this.shouldReconnect()) {
            this.reconnect();
          }
        };
        
        this.ws.onerror = (error) => {
          console.error('WebSocket错误:', error);
          this.emit('error', error);
          reject(error);
        };
        
      } catch (error) {
        reject(error);
      }
    });
  }
  
  // 发送消息
  send(message: any): boolean {
    if (this.isConnected()) {
      try {
        this.ws!.send(JSON.stringify(message));
        return true;
      } catch (error) {
        console.error('发送消息失败:', error);
        this.messageQueue.push(message);
        return false;
      }
    } else {
      // 连接断开时将消息加入队列
      this.messageQueue.push(message);
      return false;
    }
  }
  
  // 检查连接状态
  isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }
  
  // 断开连接
  disconnect(): void {
    if (this.ws) {
      this.ws.close(1000, '主动断开连接');
    }
    this.cleanup();
  }
  
  // 事件监听
  on(event: string, callback: Function): void {
    if (!this.listeners.has(event)) {
      this.listeners.set(event, new Set());
    }
    this.listeners.get(event)!.add(callback);
  }
  
  // 移除事件监听
  off(event: string, callback: Function): void {
    this.listeners.get(event)?.delete(callback);
  }
  
  // 触发事件
  private emit(event: string, data: any): void {
    this.listeners.get(event)?.forEach(callback => {
      try {
        callback(data);
      } catch (error) {
        console.error(`事件处理器错误 (${event}):`, error);
      }
    });
  }
  
  // 处理接收消息
  private handleMessage(event: MessageEvent): void {
    try {
      const message = JSON.parse(event.data);
      
      // 心跳响应
      if (message.type === 'pong') {
        return;
      }
      
      // 业务消息处理
      this.emit('message', message);
      this.emit(message.type, message.payload);
      
    } catch (error) {
      console.error('消息解析失败:', error);
      this.emit('error', error);
    }
  }
  
  // 自动重连
  private reconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.error('重连次数已达上限，停止重连');
      this.emit('reconnect_failed');
      return;
    }
    
    const delay = this.reconnectDelay * Math.pow(2, this.reconnectAttempts);
    this.reconnectAttempts++;
    
    console.log(`${delay}ms后进行第${this.reconnectAttempts}次重连`);
    this.emit('reconnecting', { attempt: this.reconnectAttempts, delay });
    
    setTimeout(() => {
      this.connect().catch(error => {
        console.error('重连失败:', error);
        this.reconnect();
      });
    }, delay);
  }
  
  // 判断是否应该重连
  private shouldReconnect(): boolean {
    return this.reconnectAttempts < this.maxReconnectAttempts;
  }
  
  // 心跳检测
  private startHeartbeat(): void {
    this.heartbeatInterval = setInterval(() => {
      if (this.isConnected()) {
        this.send({ type: 'ping', timestamp: Date.now() });
      }
    }, 30000); // 30秒心跳
  }
  
  // 处理消息队列
  private processMessageQueue(): void {
    while (this.messageQueue.length > 0 && this.isConnected()) {
      const message = this.messageQueue.shift();
      this.send(message);
    }
  }
  
  // 清理资源
  private cleanup(): void {
    if (this.heartbeatInterval) {
      clearInterval(this.heartbeatInterval);
      this.heartbeatInterval = null;
    }
  }
}
```

### 🎯 React Hook封装

```tsx
// WebSocket Hook
function useWebSocket(url: string, protocols?: string[]) {
  const wsManagerRef = useRef<WebSocketManager | null>(null);
  const [connectionState, setConnectionState] = useState({
    connected: false,
    connecting: false,
    error: null as Error | null,
    reconnecting: false,
    reconnectAttempt: 0
  });
  
  const [messages, setMessages] = useState<any[]>([]);
  const [lastMessage, setLastMessage] = useState<any>(null);
  
  // 初始化WebSocket管理器
  useEffect(() => {
    wsManagerRef.current = new WebSocketManager(url, protocols);
    
    const wsManager = wsManagerRef.current;
    
    // 连接状态监听
    wsManager.on('connected', () => {
      setConnectionState(prev => ({
        ...prev,
        connected: true,
        connecting: false,
        error: null,
        reconnecting: false
      }));
    });
    
    wsManager.on('disconnected', () => {
      setConnectionState(prev => ({
        ...prev,
        connected: false,
        connecting: false
      }));
    });
    
    wsManager.on('error', (error: Error) => {
      setConnectionState(prev => ({
        ...prev,
        error,
        connecting: false
      }));
    });
    
    wsManager.on('reconnecting', ({ attempt }: { attempt: number }) => {
      setConnectionState(prev => ({
        ...prev,
        reconnecting: true,
        reconnectAttempt: attempt
      }));
    });
    
    // 消息监听
    wsManager.on('message', (message: any) => {
      setLastMessage(message);
      setMessages(prev => [...prev, message]);
    });
    
    // 自动连接
    setConnectionState(prev => ({ ...prev, connecting: true }));
    wsManager.connect().catch(error => {
      console.error('初始连接失败:', error);
    });
    
    // 清理函数
    return () => {
      wsManager.disconnect();
    };
  }, [url, protocols]);
  
  // 发送消息
  const sendMessage = useCallback((message: any) => {
    return wsManagerRef.current?.send(message) || false;
  }, []);
  
  // 手动连接
  const connect = useCallback(() => {
    if (wsManagerRef.current) {
      setConnectionState(prev => ({ ...prev, connecting: true }));
      return wsManagerRef.current.connect();
    }
    return Promise.reject(new Error('WebSocket管理器未初始化'));
  }, []);
  
  // 手动断开
  const disconnect = useCallback(() => {
    wsManagerRef.current?.disconnect();
  }, []);
  
  return {
    connectionState,
    messages,
    lastMessage,
    sendMessage,
    connect,
    disconnect
  };
}
```

---

## 🔄 实时查询状态同步

### 📊 查询执行状态管理

```tsx
// 查询执行状态Hook
function useQueryExecution() {
  const { connectionState, lastMessage, sendMessage } = useWebSocket(
    'ws://localhost:8080/ws/query'
  );
  
  const [queryState, setQueryState] = useState({
    executing: false,
    progress: 0,
    currentStep: '',
    result: null,
    error: null,
    executionId: null,
    startTime: null,
    endTime: null
  });
  
  // 处理WebSocket消息
  useEffect(() => {
    if (!lastMessage) return;
    
    const { type, payload } = lastMessage;
    
    switch (type) {
      case 'query_started':
        setQueryState(prev => ({
          ...prev,
          executing: true,
          progress: 0,
          currentStep: '开始执行查询',
          executionId: payload.executionId,
          startTime: payload.timestamp,
          error: null,
          result: null
        }));
        break;
        
      case 'query_progress':
        setQueryState(prev => ({
          ...prev,
          progress: payload.progress,
          currentStep: payload.step
        }));
        break;
        
      case 'query_completed':
        setQueryState(prev => ({
          ...prev,
          executing: false,
          progress: 100,
          currentStep: '查询完成',
          result: payload.result,
          endTime: payload.timestamp
        }));
        break;
        
      case 'query_error':
        setQueryState(prev => ({
          ...prev,
          executing: false,
          error: payload.error,
          currentStep: '查询失败',
          endTime: payload.timestamp
        }));
        break;
        
      case 'query_cancelled':
        setQueryState(prev => ({
          ...prev,
          executing: false,
          currentStep: '查询已取消',
          endTime: payload.timestamp
        }));
        break;
    }
  }, [lastMessage]);
  
  // 执行查询
  const executeQuery = useCallback((query: string, options?: any) => {
    if (!connectionState.connected) {
      throw new Error('WebSocket连接未建立');
    }
    
    const message = {
      type: 'execute_query',
      payload: {
        query,
        options,
        timestamp: Date.now()
      }
    };
    
    return sendMessage(message);
  }, [connectionState.connected, sendMessage]);
  
  // 取消查询
  const cancelQuery = useCallback(() => {
    if (queryState.executionId) {
      const message = {
        type: 'cancel_query',
        payload: {
          executionId: queryState.executionId,
          timestamp: Date.now()
        }
      };
      
      return sendMessage(message);
    }
    return false;
  }, [queryState.executionId, sendMessage]);
  
  return {
    queryState,
    connectionState,
    executeQuery,
    cancelQuery
  };
}
```

### 🎨 查询状态UI组件

```tsx
// 查询执行状态组件
function QueryExecutionStatus() {
  const { queryState, connectionState, cancelQuery } = useQueryExecution();
  
  if (!connectionState.connected) {
    return (
      <div className="query-status offline">
        <Icon name="wifi-off" />
        <span>连接已断开</span>
        {connectionState.reconnecting && (
          <span>（正在重连...）</span>
        )}
      </div>
    );
  }
  
  if (!queryState.executing) {
    return null;
  }
  
  return (
    <div className="query-execution-status">
      <div className="status-header">
        <div className="status-title">
          <Icon name="play" className="executing" />
          正在执行查询
        </div>
        
        <Button 
          size="small" 
          danger 
          onClick={cancelQuery}
          icon={<Icon name="stop" />}
        >
          取消
        </Button>
      </div>
      
      <div className="status-content">
        <div className="progress-info">
          <span className="current-step">{queryState.currentStep}</span>
          <span className="progress-percent">{queryState.progress}%</span>
        </div>
        
        <Progress 
          percent={queryState.progress}
          strokeColor={{
            '0%': '#108ee9',
            '100%': '#87d068',
          }}
          showInfo={false}
        />
        
        {queryState.startTime && (
          <div className="execution-time">
            执行时间: {formatDuration(Date.now() - queryState.startTime)}
          </div>
        )}
      </div>
    </div>
  );
}
```

---

## 📨 流式数据传输

### 🔄 分块数据处理

```tsx
// 流式数据接收Hook
function useStreamingData() {
  const { lastMessage } = useWebSocket('ws://localhost:8080/ws/data');
  const [streamingData, setStreamingData] = useState({
    chunks: [] as any[],
    totalRows: 0,
    receivedRows: 0,
    completed: false,
    error: null as string | null
  });
  
  useEffect(() => {
    if (!lastMessage) return;
    
    const { type, payload } = lastMessage;
    
    switch (type) {
      case 'data_stream_start':
        setStreamingData({
          chunks: [],
          totalRows: payload.totalRows,
          receivedRows: 0,
          completed: false,
          error: null
        });
        break;
        
      case 'data_chunk':
        setStreamingData(prev => ({
          ...prev,
          chunks: [...prev.chunks, payload.data],
          receivedRows: prev.receivedRows + payload.data.length
        }));
        break;
        
      case 'data_stream_complete':
        setStreamingData(prev => ({
          ...prev,
          completed: true
        }));
        break;
        
      case 'data_stream_error':
        setStreamingData(prev => ({
          ...prev,
          error: payload.error,
          completed: true
        }));
        break;
    }
  }, [lastMessage]);
  
  // 获取所有数据
  const getAllData = useCallback(() => {
    return streamingData.chunks.flat();
  }, [streamingData.chunks]);
  
  // 获取进度
  const getProgress = useCallback(() => {
    if (streamingData.totalRows === 0) return 0;
    return (streamingData.receivedRows / streamingData.totalRows) * 100;
  }, [streamingData.totalRows, streamingData.receivedRows]);
  
  return {
    streamingData,
    getAllData,
    getProgress
  };
}
```

### 📊 流式表格组件

```tsx
// 流式数据表格
function StreamingDataTable() {
  const { streamingData, getAllData, getProgress } = useStreamingData();
  const [visibleData, setVisibleData] = useState([]);
  const [displayCount, setDisplayCount] = useState(50);
  
  // 渐进式显示数据
  useEffect(() => {
    const allData = getAllData();
    const newVisibleData = allData.slice(0, displayCount);
    setVisibleData(newVisibleData);
  }, [getAllData, displayCount]);
  
  // 自动加载更多数据
  useEffect(() => {
    if (streamingData.completed && visibleData.length < getAllData().length) {
      const timer = setTimeout(() => {
        setDisplayCount(prev => Math.min(prev + 50, getAllData().length));
      }, 100);
      
      return () => clearTimeout(timer);
    }
  }, [streamingData.completed, visibleData.length, getAllData]);
  
  const handleLoadMore = useCallback(() => {
    const allData = getAllData();
    setDisplayCount(prev => Math.min(prev + 100, allData.length));
  }, [getAllData]);
  
  if (streamingData.error) {
    return (
      <div className="streaming-error">
        <Alert
          message="数据传输错误"
          description={streamingData.error}
          type="error"
          showIcon
        />
      </div>
    );
  }
  
  return (
    <div className="streaming-data-table">
      {/* 进度显示 */}
      {!streamingData.completed && (
        <div className="streaming-progress">
          <div className="progress-info">
            <span>正在接收数据...</span>
            <span>{streamingData.receivedRows} / {streamingData.totalRows}</span>
          </div>
          <Progress percent={Math.round(getProgress())} size="small" />
        </div>
      )}
      
      {/* 数据表格 */}
      <Table
        dataSource={visibleData}
        columns={generateColumns(visibleData[0])}
        pagination={false}
        loading={!streamingData.completed && visibleData.length === 0}
        size="small"
        scroll={{ y: 400 }}
      />
      
      {/* 加载更多 */}
      {visibleData.length < getAllData().length && (
        <div className="load-more-section">
          <Button onClick={handleLoadMore} loading={!streamingData.completed}>
            加载更多数据 ({getAllData().length - visibleData.length} 行待显示)
          </Button>
        </div>
      )}
      
      {/* 完成状态 */}
      {streamingData.completed && (
        <div className="streaming-complete">
          <Icon name="check-circle" style={{ color: '#52c41a' }} />
          <span>数据接收完成，共 {getAllData().length} 行</span>
        </div>
      )}
    </div>
  );
}
```

---

## 🤝 多用户协作功能

### 👥 协作会话管理

```tsx
// 协作会话Hook
function useCollaborativeSession(sessionId: string) {
  const { connectionState, lastMessage, sendMessage } = useWebSocket(
    `ws://localhost:8080/ws/collaboration/${sessionId}`
  );
  
  const [sessionState, setSessionState] = useState({
    participants: [] as any[],
    currentQuery: '',
    queryAuthor: null as string | null,
    sharedCursor: null as any,
    chatMessages: [] as any[]
  });
  
  // 处理协作消息
  useEffect(() => {
    if (!lastMessage) return;
    
    const { type, payload } = lastMessage;
    
    switch (type) {
      case 'participant_joined':
        setSessionState(prev => ({
          ...prev,
          participants: [...prev.participants, payload.participant]
        }));
        break;
        
      case 'participant_left':
        setSessionState(prev => ({
          ...prev,
          participants: prev.participants.filter(
            p => p.id !== payload.participantId
          )
        }));
        break;
        
      case 'query_updated':
        setSessionState(prev => ({
          ...prev,
          currentQuery: payload.query,
          queryAuthor: payload.authorId
        }));
        break;
        
      case 'cursor_moved':
        setSessionState(prev => ({
          ...prev,
          sharedCursor: payload.cursor
        }));
        break;
        
      case 'chat_message':
        setSessionState(prev => ({
          ...prev,
          chatMessages: [...prev.chatMessages, payload.message]
        }));
        break;
    }
  }, [lastMessage]);
  
  // 更新查询
  const updateQuery = useCallback((query: string) => {
    sendMessage({
      type: 'update_query',
      payload: { query, timestamp: Date.now() }
    });
  }, [sendMessage]);
  
  // 移动光标
  const updateCursor = useCallback((position: any) => {
    sendMessage({
      type: 'move_cursor',
      payload: { cursor: position, timestamp: Date.now() }
    });
  }, [sendMessage]);
  
  // 发送聊天消息
  const sendChatMessage = useCallback((message: string) => {
    sendMessage({
      type: 'chat_message',
      payload: {
        message: {
          id: `${Date.now()}-${Math.random()}`,
          content: message,
          timestamp: Date.now(),
          authorId: 'current-user' // 应该从用户上下文获取
        }
      }
    });
  }, [sendMessage]);
  
  return {
    sessionState,
    connectionState,
    updateQuery,
    updateCursor,
    sendChatMessage
  };
}
```

### 💬 实时聊天组件

```tsx
// 协作聊天组件
function CollaborativeChat({ sessionId }: { sessionId: string }) {
  const { sessionState, sendChatMessage } = useCollaborativeSession(sessionId);
  const [newMessage, setNewMessage] = useState('');
  const messagesEndRef = useRef<HTMLDivElement>(null);
  
  // 自动滚动到最新消息
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [sessionState.chatMessages]);
  
  const handleSendMessage = useCallback(() => {
    if (newMessage.trim()) {
      sendChatMessage(newMessage.trim());
      setNewMessage('');
    }
  }, [newMessage, sendChatMessage]);
  
  const handleKeyPress = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  }, [handleSendMessage]);
  
  return (
    <div className="collaborative-chat">
      <div className="chat-header">
        <h4>协作聊天</h4>
        <div className="participants-count">
          {sessionState.participants.length} 人在线
        </div>
      </div>
      
      <div className="chat-messages">
        {sessionState.chatMessages.map(message => (
          <div key={message.id} className="chat-message">
            <div className="message-header">
              <span className="author">{message.authorId}</span>
              <span className="timestamp">
                {formatTime(message.timestamp)}
              </span>
            </div>
            <div className="message-content">{message.content}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>
      
      <div className="chat-input">
        <Input.TextArea
          value={newMessage}
          onChange={(e) => setNewMessage(e.target.value)}
          onKeyPress={handleKeyPress}
          placeholder="输入消息... (Enter发送)"
          autoSize={{ minRows: 1, maxRows: 3 }}
        />
        <Button
          type="primary"
          onClick={handleSendMessage}
          disabled={!newMessage.trim()}
        >
          发送
        </Button>
      </div>
    </div>
  );
}
```

---

## 🛡️ 连接稳定性与错误恢复

### 🔄 智能重连策略

```tsx
// 高级重连策略
class AdvancedReconnectionStrategy {
  private baseDelay = 1000;
  private maxDelay = 30000;
  private maxAttempts = 10;
  private jitterRange = 0.1;
  
  calculateDelay(attempt: number): number {
    // 指数退避 + 随机抖动
    const exponentialDelay = Math.min(
      this.baseDelay * Math.pow(2, attempt),
      this.maxDelay
    );
    
    const jitter = exponentialDelay * this.jitterRange * Math.random();
    return exponentialDelay + jitter;
  }
  
  shouldReconnect(attempt: number, error?: any): boolean {
    // 超过最大重试次数
    if (attempt >= this.maxAttempts) {
      return false;
    }
    
    // 根据错误类型决定是否重连
    if (error) {
      // 4xx错误通常不应该重连
      if (error.code >= 4000 && error.code < 5000) {
        return false;
      }
      
      // 身份验证错误
      if (error.code === 4001) {
        return false;
      }
    }
    
    return true;
  }
  
  getReconnectReason(attempt: number): string {
    if (attempt === 1) return '连接中断，尝试重连...';
    if (attempt <= 3) return '网络不稳定，继续重连...';
    if (attempt <= 6) return '连接困难，正在努力重连...';
    return '连接持续异常，最后几次重试...';
  }
}
```

### 📊 连接质量监控

```tsx
// 连接质量监控Hook
function useConnectionQuality() {
  const [quality, setQuality] = useState({
    latency: 0,
    packetsLost: 0,
    reconnectCount: 0,
    uptime: 0,
    stability: 'excellent' as 'excellent' | 'good' | 'poor' | 'critical'
  });
  
  const { connectionState, lastMessage, sendMessage } = useWebSocket(
    'ws://localhost:8080/ws/monitoring'
  );
  
  // 定期ping测试
  useEffect(() => {
    if (!connectionState.connected) return;
    
    const pingInterval = setInterval(() => {
      const pingStart = Date.now();
      
      sendMessage({
        type: 'ping',
        timestamp: pingStart
      });
      
      // 监听pong响应
      const handlePong = (message: any) => {
        if (message.type === 'pong') {
          const latency = Date.now() - pingStart;
          
          setQuality(prev => ({
            ...prev,
            latency: (prev.latency * 0.8) + (latency * 0.2) // 平滑处理
          }));
        }
      };
      
      // 临时监听器
      const removeListener = () => {
        // 实现监听器移除逻辑
      };
      
      setTimeout(removeListener, 5000);
      
    }, 10000); // 每10秒ping一次
    
    return () => clearInterval(pingInterval);
  }, [connectionState.connected, sendMessage]);
  
  // 计算连接稳定性
  useEffect(() => {
    let stability: typeof quality.stability;
    
    if (quality.latency < 100 && quality.packetsLost < 0.01) {
      stability = 'excellent';
    } else if (quality.latency < 300 && quality.packetsLost < 0.05) {
      stability = 'good';
    } else if (quality.latency < 1000 && quality.packetsLost < 0.1) {
      stability = 'poor';
    } else {
      stability = 'critical';
    }
    
    setQuality(prev => ({ ...prev, stability }));
  }, [quality.latency, quality.packetsLost]);
  
  return quality;
}
```

---

## 🧪 测试与监控

### 🔍 WebSocket测试套件

```tsx
// WebSocket连接测试
import { act, renderHook } from '@testing-library/react';

describe('WebSocket功能测试', () => {
  let mockWebSocket: any;
  
  beforeEach(() => {
    mockWebSocket = {
      send: jest.fn(),
      close: jest.fn(),
      readyState: WebSocket.OPEN,
      addEventListener: jest.fn(),
      removeEventListener: jest.fn()
    };
    
    (global as any).WebSocket = jest.fn(() => mockWebSocket);
  });
  
  test('应该正确建立连接', async () => {
    const { result } = renderHook(() => 
      useWebSocket('ws://localhost:8080/test')
    );
    
    // 模拟连接打开
    act(() => {
      mockWebSocket.onopen?.();
    });
    
    expect(result.current.connectionState.connected).toBe(true);
  });
  
  test('应该正确处理消息', async () => {
    const { result } = renderHook(() => 
      useWebSocket('ws://localhost:8080/test')
    );
    
    const testMessage = { type: 'test', payload: 'hello' };
    
    act(() => {
      mockWebSocket.onmessage?.({
        data: JSON.stringify(testMessage)
      });
    });
    
    expect(result.current.lastMessage).toEqual(testMessage);
  });
  
  test('应该正确处理重连', async () => {
    const { result } = renderHook(() => 
      useWebSocket('ws://localhost:8080/test')
    );
    
    // 模拟连接关闭
    act(() => {
      mockWebSocket.onclose?.({ wasClean: false });
    });
    
    expect(result.current.connectionState.connected).toBe(false);
    
    // 应该尝试重连
    await act(async () => {
      await new Promise(resolve => setTimeout(resolve, 1100));
    });
    
    expect(global.WebSocket).toHaveBeenCalledTimes(2);
  });
});
```

### 📊 性能监控

```tsx
// WebSocket性能监控
class WebSocketPerformanceMonitor {
  private metrics = {
    connectionTime: 0,
    messagesSent: 0,
    messagesReceived: 0,
    bytesTransferred: 0,
    reconnections: 0,
    errors: 0
  };
  
  private connectionStartTime = 0;
  
  onConnectionStart() {
    this.connectionStartTime = Date.now();
  }
  
  onConnectionEstablished() {
    this.metrics.connectionTime = Date.now() - this.connectionStartTime;
    this.reportMetric('websocket_connection_time', this.metrics.connectionTime);
  }
  
  onMessageSent(size: number) {
    this.metrics.messagesSent++;
    this.metrics.bytesTransferred += size;
    
    this.reportMetric('websocket_messages_sent', this.metrics.messagesSent);
    this.reportMetric('websocket_bytes_sent', size);
  }
  
  onMessageReceived(size: number) {
    this.metrics.messagesReceived++;
    this.metrics.bytesTransferred += size;
    
    this.reportMetric('websocket_messages_received', this.metrics.messagesReceived);
    this.reportMetric('websocket_bytes_received', size);
  }
  
  onReconnection() {
    this.metrics.reconnections++;
    this.reportMetric('websocket_reconnections', this.metrics.reconnections);
  }
  
  onError(error: any) {
    this.metrics.errors++;
    this.reportMetric('websocket_errors', this.metrics.errors);
    
    // 错误详情上报
    this.reportError('websocket_error', error);
  }
  
  private reportMetric(name: string, value: number) {
    // 发送到监控系统
    if (window.analytics) {
      window.analytics.track('WebSocket Metric', {
        metric: name,
        value,
        timestamp: Date.now()
      });
    }
  }
  
  private reportError(name: string, error: any) {
    // 错误上报
    if (window.errorReporting) {
      window.errorReporting.captureException(error, {
        tags: { component: 'websocket' },
        extra: { metrics: this.metrics }
      });
    }
  }
  
  getMetrics() {
    return { ...this.metrics };
  }
}
```

---

## 📚 最佳实践总结

### ✅ 推荐做法

1. **连接管理**
   - 实现指数退避重连策略
   - 使用心跳检测保持连接活跃
   - 正确处理各种连接状态

2. **消息处理**
   - 实现消息队列防止消息丢失
   - 使用JSON Schema验证消息格式
   - 处理消息的幂等性

3. **性能优化**
   - 使用消息批处理减少网络开销
   - 实现客户端消息缓存
   - 监控连接质量和性能指标

4. **错误处理**
   - 区分可恢复和不可恢复错误
   - 提供友好的错误提示
   - 实现优雅的降级机制

### ❌ 避免的陷阱

1. **过度重连**
   - 避免无限重连导致服务器压力
   - 正确识别永久性连接失败

2. **消息丢失**
   - 不要忽略连接断开时的消息处理
   - 实现可靠的消息传递机制

3. **内存泄漏**
   - 及时清理事件监听器
   - 正确管理定时器和间隔器

---

## 🔗 相关资源

- **WebSocket API文档**：https://developer.mozilla.org/en-US/docs/Web/API/WebSocket
- **WebSocket协议规范**：https://tools.ietf.org/html/rfc6455
- **性能优化指南**：https://websockets.readthedocs.io/en/stable/
- **安全最佳实践**：https://devcenter.heroku.com/articles/websocket-security

---

💡 **实施建议**：按照第3周的开发计划，先实现基础的WebSocket连接，然后逐步添加高级功能如重连机制、性能监控和协作功能。