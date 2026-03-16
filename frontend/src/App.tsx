import { useState, useEffect, useRef } from 'react';
import { Send, User, Bot, Wrench, AlertCircle, CheckCircle2 } from 'lucide-react';
import './index.css';

// Types
interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
}

interface ToolCall {
  id: string;
  name: string;
  args: any;
  status?: 'running' | 'success' | 'error';
  result?: string;
}

export default function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [toolCalls, setToolCalls] = useState<{ [id: string]: ToolCall }>({});
  const [inputTitle, setInputTitle] = useState('');
  const [isConnected, setIsConnected] = useState(false);
  const [statusText, setStatusText] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);

  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, toolCalls, statusText]);

  // WebSocket Connection
  useEffect(() => {
    connectWebSocket();
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    // For local dev, we default to 8080 if not running from the Go server directly
    const wsUrl = `${protocol}//${window.location.hostname}:8085/ws`;

    console.log('Connecting to', wsUrl);
    const ws = new WebSocket(wsUrl);

    ws.onopen = () => {
      setIsConnected(true);
      setStatusText('');
    };

    ws.onclose = () => {
      setIsConnected(false);
      setIsProcessing(false);
      // Try to reconnect after 3 seconds
      setTimeout(connectWebSocket, 3000);
    };

    ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleWSEvent(data);
      } catch (err) {
        console.error('Failed to parse WS message:', err);
      }
    };

    wsRef.current = ws;
  };

  const handleWSEvent = (event: any) => {
    switch (event.type) {
      case 'session_start':
        setMessages(prev => [...prev, { id: Date.now().toString(), role: 'system', content: event.content }]);
        setToolCalls({});
        setIsProcessing(false);
        break;

      case 'thinking':
        setStatusText(event.content);
        break;

      case 'task_progress':
        setStatusText(event.content);
        break;

      case 'tool_call':
        setToolCalls(prev => ({
          ...prev,
          [event.tool]: {
            id: event.tool,
            name: event.tool,
            args: event.args,
            status: 'running'
          }
        }));
        setStatusText(`Running ${event.tool}...`);
        break;

      case 'tool_result':
        setToolCalls(prev => ({
          ...prev,
          [event.tool]: {
            ...prev[event.tool],
            status: event.status.includes('success') ? 'success' : 'error',
            result: event.content
          }
        }));
        setStatusText('Analyzing results...');
        break;

      case 'assistant':
        setMessages(prev => [...prev, { id: Date.now().toString(), role: 'assistant', content: event.content }]);
        setIsProcessing(false);
        setStatusText('');
        break;

      case 'task_complete':
        setIsProcessing(false);
        setStatusText('');
        break;

      case 'error':
        setMessages(prev => [...prev, { id: Date.now().toString(), role: 'system', content: `Error: ${event.content}` }]);
        setIsProcessing(false);
        setStatusText('');
        break;
    }
  };

  const sendMessage = () => {
    const trimmed = inputTitle.trim();
    if (!trimmed || !isConnected || isProcessing) return;

    // Add user message to UI
    setMessages(prev => [...prev, { id: Date.now().toString(), role: 'user', content: trimmed }]);

    // Clear tool calls from previous turn
    setToolCalls({});

    // Send over WS
    if (wsRef.current) {
      setIsProcessing(true);
      wsRef.current.send(JSON.stringify({
        type: 'user_message',
        content: trimmed
      }));
    }

    // Reset input
    setInputTitle('');
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputTitle(e.target.value);
    // Auto-resize textarea
    e.target.style.height = 'auto';
    e.target.style.height = `${Math.min(e.target.scrollHeight, 200)}px`;
  };

  return (
    <div className="app-container">
      <div className="chat-panel">
        {/* Connection Badge */}
        <div className={`connection-badge ${isConnected ? 'connected' : 'disconnected'}`}>
          <div className="badge-dot"></div>
          {isConnected ? 'Connected' : 'Reconnecting...'}
        </div>

        {/* Message List */}
        <div className="messages-container">
          {messages.length === 0 && (
            <div style={{ margin: 'auto', textAlign: 'center', color: 'var(--text-muted)' }}>
              <h2>🤖 MeBot</h2>
              <p>Your personal AI assistant.</p>
            </div>
          )}

          {messages.map((msg) => {
            if (msg.role === 'system') {
              if (msg.content.includes("Context history compressed")) {
                return (
                  <div key={msg.id} className="system-event" style={{ color: '#a371f7', border: '1px solid rgba(163, 113, 247, 0.2)', backgroundColor: 'rgba(163, 113, 247, 0.05)' }}>
                    <AlertCircle size={14} style={{ display: 'inline', marginRight: '6px', verticalAlign: 'middle' }} />
                    {msg.content}
                  </div>
                );
              }
              return <div key={msg.id} className="system-event">{msg.content}</div>;
            }

            return (
              <div key={msg.id} className={`message ${msg.role}`}>
                <div className="avatar">
                  {msg.role === 'user' ? <User size={20} color="#fff" /> : <Bot size={20} color="#c9d1d9" />}
                </div>
                <div className="bubble">
                  {msg.content}
                </div>
              </div>
            );
          })}

          {/* Render Active Tool Calls after all messages */}
          {Object.values(toolCalls).map((tc) => (
            <div key={tc.id} className="tool-card">
              <div className="tool-header">
                {tc.status === 'running' && <Wrench size={16} className="animate-spin" />}
                {tc.status === 'success' && <CheckCircle2 size={16} color="#3fb950" />}
                {tc.status === 'error' && <AlertCircle size={16} color="#f85149" />}
                <span>call: {tc.name}</span>
              </div>
              <div className="tool-args">
                {JSON.stringify(tc.args, null, 2)}
              </div>
              {tc.result && (
                <div className={`tool-result ${tc.status}`}>
                  {(() => {
                    // Check if the result contains a base64 image (specifically attached by browser tools)
                    // The backend appends: "\n\nScreenshot taken... (URL)\n<base64>" OR just plain text
                    // Our tools output "Screenshot... \n\niVBOR..." 
                    // Let's do a simple heuristic: if it contains a really long base64 string
                    // we'll split it and render the text + the image.
                    let textParts = tc.result.split('\n\n');
                    let base64Img = '';
                    let cleanText = tc.result;

                    // If the last part is a huge block of base64 (no spaces, very long)
                    if (textParts.length > 1) {
                      const possibleBase64 = textParts[textParts.length - 1].trim();
                      if (possibleBase64.length > 1000 && !possibleBase64.includes(' ')) {
                        base64Img = possibleBase64;
                        cleanText = textParts.slice(0, -1).join('\n\n');
                      }
                    }

                    return (
                      <>
                        <div style={{ whiteSpace: 'pre-wrap' }}>{cleanText}</div>
                        {base64Img && (
                          <div style={{ marginTop: '12px', border: '1px solid rgba(255,255,255,0.1)', borderRadius: '8px', overflow: 'hidden' }}>
                            <img
                              src={`data:image/jpeg;base64,${base64Img}`}
                              alt="Browser Screenshot"
                              style={{ display: 'block', width: '100%', height: 'auto' }}
                            />
                          </div>
                        )}
                      </>
                    );
                  })()}
                </div>
              )}
            </div>
          ))}

          <div ref={messagesEndRef} />
        </div>

        {/* Status Bar */}
        {statusText && (
          <div className="status-bar">
            <span className="thinking-dots">{statusText}</span>
          </div>
        )}

        {/* Input Area */}
        <div className="input-area">
          <div className="input-container">
            <textarea
              ref={textareaRef}
              value={inputTitle}
              onChange={handleInput}
              onKeyDown={handleKeyDown}
              placeholder="Message MeBot..."
              rows={1}
              disabled={!isConnected || isProcessing}
            />
            <button
              className="send-btn"
              onClick={sendMessage}
              disabled={!inputTitle.trim() || !isConnected || isProcessing}
            >
              <Send size={18} />
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
