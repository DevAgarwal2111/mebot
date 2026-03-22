import { useState, useEffect, useRef } from 'react';
import { Send, User, Bot, Wrench, AlertCircle, CheckCircle2, Plus, Trash2, BookOpen, X, ChevronRight } from 'lucide-react';
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

interface SkillData {
  name: string;
  description: string;
  triggers: string[];
  cron: string;
  content: string;
  is_builtin: boolean;
}

const API_BASE = `${window.location.protocol}//${window.location.hostname}:8085`;

const SKILL_TEMPLATE = `---
name: my_skill_name
description: What this skill does
triggers:
  - "keyword1"
  - "keyword2"
---

# Skill Instructions

Write the instructions for the bot here. This content gets injected into the
conversation when the trigger keywords match the user's message.
`;

export default function App() {
  const [messages, setMessages] = useState<Message[]>([]);
  const [toolCalls, setToolCalls] = useState<{ [id: string]: ToolCall }>({});
  const [inputTitle, setInputTitle] = useState('');
  const [isConnected, setIsConnected] = useState(false);
  const [statusText, setStatusText] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);

  // Skill Manager State
  const [showSkillPanel, setShowSkillPanel] = useState(false);
  const [skills, setSkills] = useState<SkillData[]>([]);
  const [showCreateForm, setShowCreateForm] = useState(false);
  const [newSkillName, setNewSkillName] = useState('');
  const [newSkillContent, setNewSkillContent] = useState(SKILL_TEMPLATE);
  const [skillError, setSkillError] = useState('');

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

  // Load skills when panel opens
  useEffect(() => {
    if (showSkillPanel) {
      fetchSkills();
    }
  }, [showSkillPanel]);

  const fetchSkills = async () => {
    try {
      const resp = await fetch(`${API_BASE}/api/skills`);
      if (resp.ok) {
        const data = await resp.json();
        setSkills(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch skills:', err);
    }
  };

  const createSkill = async () => {
    setSkillError('');
    if (!newSkillName.trim() || !newSkillContent.trim()) {
      setSkillError('Name and content are required');
      return;
    }

    try {
      const resp = await fetch(`${API_BASE}/api/skills`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newSkillName.trim(), content: newSkillContent }),
      });

      if (resp.ok) {
        setShowCreateForm(false);
        setNewSkillName('');
        setNewSkillContent(SKILL_TEMPLATE);
        fetchSkills();
      } else {
        const text = await resp.text();
        setSkillError(text || 'Failed to create skill');
      }
    } catch (err) {
      setSkillError('Network error');
    }
  };

  const deleteSkill = async (name: string) => {
    try {
      const resp = await fetch(`${API_BASE}/api/skills/${encodeURIComponent(name)}`, {
        method: 'DELETE',
      });
      if (resp.ok) {
        fetchSkills();
      }
    } catch (err) {
      console.error('Failed to delete skill:', err);
    }
  };

  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
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

    setMessages(prev => [...prev, { id: Date.now().toString(), role: 'user', content: trimmed }]);
    setToolCalls({});

    if (wsRef.current) {
      setIsProcessing(true);
      wsRef.current.send(JSON.stringify({
        type: 'user_message',
        content: trimmed
      }));
    }

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
    e.target.style.height = 'auto';
    e.target.style.height = `${Math.min(e.target.scrollHeight, 200)}px`;
  };

  return (
    <div className="app-container">
      <div className={`chat-panel ${showSkillPanel ? 'with-sidebar' : ''}`}>
        {/* Header Bar */}
        <div className="header-bar">
          <div className={`connection-badge ${isConnected ? 'connected' : 'disconnected'}`}>
            <div className="badge-dot"></div>
            {isConnected ? 'Connected' : 'Reconnecting...'}
          </div>
          <button
            className={`skills-toggle ${showSkillPanel ? 'active' : ''}`}
            onClick={() => setShowSkillPanel(!showSkillPanel)}
            title="Skill Manager"
          >
            <BookOpen size={18} />
            Skills
          </button>
        </div>

        {/* Message List */}
        <div className="messages-container">
          {messages.length === 0 && (
            <div style={{ margin: 'auto', textAlign: 'center', color: 'var(--text-muted)' }}>
              <h2>🤖 MeBot</h2>
              <p>Your personal AI assistant with skills.</p>
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

          {/* Active Tool Calls */}
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
                    let textParts = tc.result.split('\n\n');
                    let base64Img = '';
                    let cleanText = tc.result;

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

      {/* Skill Manager Sidebar */}
      {showSkillPanel && (
        <div className="skill-panel">
          <div className="skill-panel-header">
            <h3><BookOpen size={18} /> Skills</h3>
            <div className="skill-panel-actions">
              <button className="skill-add-btn" onClick={() => { setShowCreateForm(true); setSkillError(''); }}>
                <Plus size={16} /> Add
              </button>
              <button className="skill-close-btn" onClick={() => setShowSkillPanel(false)}>
                <X size={16} />
              </button>
            </div>
          </div>

          {/* Create Form */}
          {showCreateForm && (
            <div className="skill-create-form">
              <input
                type="text"
                placeholder="Skill name (e.g. morning_routine)"
                value={newSkillName}
                onChange={(e) => setNewSkillName(e.target.value)}
                className="skill-name-input"
              />
              <textarea
                value={newSkillContent}
                onChange={(e) => setNewSkillContent(e.target.value)}
                className="skill-content-textarea"
                rows={12}
              />
              {skillError && <div className="skill-error">{skillError}</div>}
              <div className="skill-form-actions">
                <button className="skill-save-btn" onClick={createSkill}>Save Skill</button>
                <button className="skill-cancel-btn" onClick={() => setShowCreateForm(false)}>Cancel</button>
              </div>
            </div>
          )}

          {/* Skill List */}
          <div className="skill-list">
            {skills.length === 0 && !showCreateForm && (
              <div className="skill-empty">
                <p>No skills loaded yet.</p>
                <p style={{ fontSize: '12px' }}>Skills are auto-created by the bot or added manually.</p>
              </div>
            )}
            {skills.map((skill) => (
              <div key={skill.name} className="skill-card">
                <div className="skill-card-header">
                  <div className="skill-card-title">
                    <ChevronRight size={14} />
                    <span>{skill.name}</span>
                    {skill.is_builtin && <span className="skill-badge builtin">built-in</span>}
                    {!skill.is_builtin && <span className="skill-badge user">user</span>}
                  </div>
                  {!skill.is_builtin && (
                    <button className="skill-delete-btn" onClick={() => deleteSkill(skill.name)} title="Delete skill">
                      <Trash2 size={14} />
                    </button>
                  )}
                </div>
                {skill.description && <p className="skill-desc">{skill.description}</p>}
                {skill.triggers && skill.triggers.length > 0 && (
                  <div className="skill-triggers">
                    {skill.triggers.map((t, i) => (
                      <span key={i} className="skill-trigger-tag">{t}</span>
                    ))}
                  </div>
                )}
                {skill.cron && <div className="skill-cron">⏰ {skill.cron}</div>}
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
