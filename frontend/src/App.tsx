import { useState, useEffect, useRef } from 'react';
import { Send, User, Bot, Wrench, AlertCircle, CheckCircle2, MessageSquare, Cpu, Trash2, X, Plus, Paperclip } from 'lucide-react';
import MarkdownRenderer from './MarkdownRenderer';
import './index.css';

// --- Types ---
interface Message {
  id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  attachments?: ContentPart[];
}

interface ContentPart {
  type: string;
  mime_type: string;
  data: string;
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

interface AgentData {
  id: string;
  name: string;
  description: string;
  provider: string;
  model: string;
  prompt: string;
}

const API_BASE = `${window.location.protocol}//${window.location.hostname}:8085`;

const SKILL_TEMPLATE = `---
name: my_skill_name
description: What this skill does
triggers:
  - "keyword1"
---

# Skill Instructions
Write the instructions for the bot here.
`;

export default function App() {
  // Chat State
  const [messages, setMessages] = useState<Message[]>([]);
  const [toolCalls, setToolCalls] = useState<{ [id: string]: ToolCall }>({});
  const [inputText, setInputText] = useState('');
  const [attachments, setAttachments] = useState<ContentPart[]>([]);

  // Connection / Run State
  const [isConnected, setIsConnected] = useState(false);
  const [statusText, setStatusText] = useState('');
  const [isProcessing, setIsProcessing] = useState(false);

  // Sidebar Layout State
  const [activeTab, setActiveTab] = useState<'skills' | 'agents' | 'info'>('skills');

  // Skills State
  const [skills, setSkills] = useState<SkillData[]>([]);
  const [showCreateSkill, setShowCreateSkill] = useState(false);
  const [newSkillName, setNewSkillName] = useState('');
  const [newSkillContent, setNewSkillContent] = useState(SKILL_TEMPLATE);
  const [skillError, setSkillError] = useState('');

  // Agents State
  const [agents, setAgents] = useState<AgentData[]>([]);
  const [showCreateAgent, setShowCreateAgent] = useState(false);
  const [agentForm, setAgentForm] = useState({ name: '', description: '', provider: 'openrouter', model: '', prompt: '' });
  const [agentError, setAgentError] = useState('');

  // Refs
  const wsRef = useRef<WebSocket | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, toolCalls, statusText]);

  // Initial Load & WebSocket
  useEffect(() => {
    connectWebSocket();
    fetchSkills();
    fetchAgents();
    return () => {
      if (wsRef.current) wsRef.current.close();
    };
  }, []);

  // --- WebSockets ---
  const connectWebSocket = () => {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    const wsUrl = `${protocol}//${window.location.hostname}:8085/ws`;

    const ws = new WebSocket(wsUrl);

    ws.onopen = () => { setIsConnected(true); setStatusText(''); };
    ws.onclose = () => {
      setIsConnected(false); setIsProcessing(false);
      setTimeout(connectWebSocket, 3000);
    };

    ws.onmessage = (event) => {
      try { handleWSEvent(JSON.parse(event.data)); }
      catch (err) { console.error('WS Parse Error:', err); }
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
      case 'task_progress':
        setStatusText(event.content);
        break;
      case 'tool_call':
        setToolCalls(prev => ({
          ...prev,
          [event.tool]: { id: event.tool, name: event.tool, args: event.args, status: 'running' }
        }));
        setStatusText(`Running ${event.tool}...`);
        break;
      case 'tool_result':
        setToolCalls(prev => ({
          ...prev,
          [event.tool]: { ...prev[event.tool], status: event.status.includes('success') ? 'success' : 'error', result: event.content }
        }));
        setStatusText('Analyzing results...');
        break;
      case 'assistant':
        setMessages(prev => [...prev, { id: Date.now().toString(), role: 'assistant', content: event.content }]);
        setIsProcessing(false); setStatusText('');
        break;
      case 'task_complete':
        setIsProcessing(false); setStatusText('');
        break;
      case 'error':
        setMessages(prev => [...prev, { id: Date.now().toString(), role: 'system', content: `Error: ${event.content}` }]);
        setIsProcessing(false); setStatusText('');
        break;
    }
  };

  // --- Submissions ---
  const sendMessage = () => {
    const trimmed = inputText.trim();
    if ((!trimmed && attachments.length === 0) || !isConnected || isProcessing) return;

    setMessages(prev => [...prev, {
      id: Date.now().toString(),
      role: 'user',
      content: trimmed,
      attachments: attachments.map(a => ({ ...a }))
    }]);
    setToolCalls({});

    if (wsRef.current) {
      setIsProcessing(true);
      wsRef.current.send(JSON.stringify({ type: 'user_message', content: trimmed, attachments }));
    }

    setInputText('');
    setAttachments([]);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); sendMessage(); }
  };

  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setInputText(e.target.value);
    e.target.style.height = 'auto';
    e.target.style.height = `${Math.min(e.target.scrollHeight, 200)}px`;
  };

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;
    Array.from(files).forEach(file => {
      const reader = new FileReader();
      reader.onload = (ev) => {
        const result = ev.target?.result;
        if (result) {
          setAttachments(prev => [...prev, {
            type: file.type.startsWith('image') ? 'image_url' : 'file',
            mime_type: file.type,
            data: result as string
          }]);
        }
      };
      reader.readAsDataURL(file);
    });
    e.target.value = '';
  };

  const removeAttachment = (index: number) => {
    setAttachments(prev => prev.filter((_, i) => i !== index));
  };

  // --- API Calls: Skills ---
  const fetchSkills = async () => {
    try {
      const resp = await fetch(`${API_BASE}/api/skills`);
      if (resp.ok) setSkills(await resp.json() || []);
    } catch (err) { console.error('Failed to fetch skills', err); }
  };

  const createSkill = async () => {
    setSkillError('');
    if (!newSkillName.trim() || !newSkillContent.trim()) { setSkillError('Name & content required'); return; }
    try {
      const resp = await fetch(`${API_BASE}/api/skills`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: newSkillName.trim(), content: newSkillContent }),
      });
      if (resp.ok) { setShowCreateSkill(false); setNewSkillName(''); setNewSkillContent(SKILL_TEMPLATE); fetchSkills(); }
      else { setSkillError(await resp.text() || 'Failed'); }
    } catch { setSkillError('Network error'); }
  };

  const deleteSkill = async (name: string) => {
    try {
      const resp = await fetch(`${API_BASE}/api/skills/${encodeURIComponent(name)}`, { method: 'DELETE' });
      if (resp.ok) fetchSkills();
    } catch (err) { console.error(err); }
  };

  // --- API Calls: Agents ---
  const fetchAgents = async () => {
    try {
      const resp = await fetch(`${API_BASE}/api/agents`);
      if (resp.ok) setAgents(await resp.json() || []);
    } catch (err) { console.error('Failed to fetch agents', err); }
  };

  const createAgent = async () => {
    setAgentError('');
    if (!agentForm.name || !agentForm.provider || !agentForm.model) { setAgentError('Name, provider, and model required'); return; }
    try {
      const resp = await fetch(`${API_BASE}/api/agents`, {
        method: 'POST', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(agentForm),
      });
      if (resp.ok) { setShowCreateAgent(false); setAgentForm({ name: '', description: '', provider: 'openrouter', model: '', prompt: '' }); fetchAgents(); }
      else { setAgentError(await resp.text() || 'Failed'); }
    } catch (err) { setAgentError('Network error: ' + String(err)); }
  };

  const deleteAgent = async (id: string) => {
    try {
      const resp = await fetch(`${API_BASE}/api/agents?id=${encodeURIComponent(id)}`, { method: 'DELETE' });
      if (resp.ok) fetchAgents();
    } catch (err) { console.error(err); }
  };

  return (
    <div className="app-layout">
      {/* --- SIDEBAR --- */}
      <aside className="sidebar">
        <div className="sidebar-header">
          <Bot size={28} color="#3b82f6" />
          <h1>MeBot Engine</h1>
        </div>

        <nav className="nav-tabs">
          <button className={`nav-tab ${activeTab === 'skills' ? 'active' : ''}`} onClick={() => setActiveTab('skills')}><Wrench size={18} /> Skills</button>
          <button className={`nav-tab ${activeTab === 'agents' ? 'active' : ''}`} onClick={() => setActiveTab('agents')}><Cpu size={18} /> Agents</button>
          <button className={`nav-tab ${activeTab === 'info' ? 'active' : ''}`} onClick={() => setActiveTab('info')}><MessageSquare size={18} /> Info</button>
        </nav>

        <div className="panel-content">
          {/* Skills Tab */}
          {activeTab === 'skills' && (
            <>
              {!showCreateSkill ? (
                <button className="btn-primary" style={{ width: '100%' }} onClick={() => setShowCreateSkill(true)}><Plus size={16} /> New Memory / Skill</button>
              ) : (
                <div className="glass-card">
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}><h3>Create Skill</h3><X size={18} style={{ cursor: 'pointer' }} onClick={() => setShowCreateSkill(false)} /></div>
                  {skillError && <div style={{ color: 'var(--danger)', fontSize: '0.8rem' }}>{skillError}</div>}
                  <input className="glass-input" placeholder="Skill Name (e.g., wifi_password)" value={newSkillName} onChange={e => setNewSkillName(e.target.value)} />
                  <textarea className="glass-textarea" value={newSkillContent} onChange={e => setNewSkillContent(e.target.value)} />
                  <div className="form-actions"><button className="btn-primary" onClick={createSkill}>Save Skill</button></div>
                </div>
              )}
              {skills.map(skill => (
                <div className="glass-card" key={skill.name}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <strong style={{ color: 'var(--accent)' }}>{skill.name}</strong>
                    {!skill.is_builtin && <button className="btn-danger" onClick={() => deleteSkill(skill.name)}><Trash2 size={14} /></button>}
                  </div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{skill.description || 'No description'}</div>
                  {skill.cron && <div style={{ fontSize: '0.75rem', color: '#a78bfa', marginTop: '4px' }}>⏰ {skill.cron}</div>}
                  {skill.triggers?.length > 0 && <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '4px' }}>Triggers: {skill.triggers.join(', ')}</div>}
                </div>
              ))}
            </>
          )}

          {/* Agents Tab */}
          {activeTab === 'agents' && (
            <>
              {!showCreateAgent ? (
                <button className="btn-primary" style={{ width: '100%' }} onClick={() => setShowCreateAgent(true)}><Plus size={16} /> New Sub-Agent</button>
              ) : (
                <div className="glass-card">
                  <div style={{ display: 'flex', justifyContent: 'space-between' }}><h3>Create Sub-Agent</h3><X size={18} style={{ cursor: 'pointer' }} onClick={() => setShowCreateAgent(false)} /></div>
                  {agentError && <div style={{ color: 'var(--danger)', fontSize: '0.8rem' }}>{agentError}</div>}
                  <input className="glass-input" placeholder="Name (e.g. Code Reviewer)" value={agentForm.name} onChange={e => setAgentForm({ ...agentForm, name: e.target.value })} />
                  <input className="glass-input" placeholder="Description" value={agentForm.description} onChange={e => setAgentForm({ ...agentForm, description: e.target.value })} />
                  <select className="glass-select" value={agentForm.provider} onChange={e => setAgentForm({ ...agentForm, provider: e.target.value })}>
                    <option value="openrouter">OpenRouter</option>
                    <option value="nvidia">Nvidia NIM</option>
                    <option value="gemini">Gemini API</option>
                    <option value="ollama">Ollama (Local)</option>
                  </select>
                  <input className="glass-input" placeholder="Model ID (e.g. anthropic/claude-3.5-sonnet)" value={agentForm.model} onChange={e => setAgentForm({ ...agentForm, model: e.target.value })} />
                  <textarea className="glass-textarea" placeholder="Optional Prompt Prefix (e.g. You are an expert...)" value={agentForm.prompt} onChange={e => setAgentForm({ ...agentForm, prompt: e.target.value })} />
                  <div className="form-actions"><button className="btn-primary" onClick={createAgent}>Save Agent</button></div>
                </div>
              )}
              {agents.map(agent => (
                <div className="glass-card" key={agent.id}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <strong style={{ color: '#60a5fa' }}>{agent.name}</strong>
                    <button className="btn-danger" onClick={() => deleteAgent(agent.id)}><Trash2 size={14} /></button>
                  </div>
                  <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>{agent.provider} • {agent.model}</div>
                  <div style={{ fontSize: '0.85rem', marginTop: '4px' }}>{agent.description}</div>
                </div>
              ))}
            </>
          )}

          {/* Info Tab */}
          {activeTab === 'info' && (
            <div className="glass-card">
              <h3 style={{ color: 'var(--text-primary)' }}>System Overview</h3>
              <p style={{ fontSize: '0.85rem', color: 'var(--text-secondary)', marginTop: '8px', lineHeight: '1.5' }}>
                MeBot is a fully agentic orchestration engine.<br /><br />
                • <strong>Skills:</strong> Long-term logic and memory injected into the system prompt based on triggers.<br />
                • <strong>Agents:</strong> Symmetrical LLM orchestration. Use the @mention system or natural language to ask the primary bot to delegate tasks to OpenRouter, Nvidia, or Ollama models natively.
              </p>
            </div>
          )}
        </div>
      </aside>

      {/* --- MAIN CHAT --- */}
      <main className="main-chat">
        <header className="chat-header">
          <div className="connection-status">
            <div className={`status-dot ${isConnected ? 'connected' : 'disconnected'}`}></div>
            {isConnected ? 'Sync Active' : 'Connecting...'}
          </div>
          <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>End-to-End Encrypted Logic</div>
        </header>

        <div className="messages-area">
          {messages.length === 0 && (
            <div style={{ margin: 'auto', textAlign: 'center', color: 'var(--text-secondary)' }}>
              <Bot size={48} style={{ opacity: 0.2, marginBottom: '16px' }} />
              <h2>Awaiting Instructions</h2>
              <p style={{ fontSize: '0.9rem', marginTop: '8px' }}>Chat naturally or delegate tasks to your sub-agents.</p>
            </div>
          )}

          {messages.map((msg) => {
            if (msg.role === 'system') return <div key={msg.id} className="system-msg">{msg.content}</div>;

            return (
              <div key={msg.id} className={`message-wrapper ${msg.role}`}>
                <div className="avatar-box">
                  {msg.role === 'user' ? <User size={20} color="#fff" /> : <Bot size={20} color="#c9d1d9" />}
                </div>
                <div className={`bubble-content ${msg.role === 'assistant' ? 'markdown-bubble' : ''}`}>
                  {msg.content && (
                    msg.role === 'assistant'
                      ? <MarkdownRenderer content={msg.content} />
                      : <div>{msg.content}</div>
                  )}
                  {msg.attachments && msg.attachments.length > 0 && (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginTop: '12px' }}>
                      {msg.attachments.map((att, i) => (
                        att.type === 'image_url' ?
                          <img key={i} src={att.data} alt="attachment" className="bubble-image" />
                          : <div key={i} className="glass-card" style={{ padding: '8px', display: 'inline-block', width: 'fit-content' }}>📄 {att.mime_type}</div>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            );
          })}

          {/* Active Tool Calls */}
          {Object.values(toolCalls).map((tc) => (
            <div key={tc.id} className="tool-card">
              <div className="tool-card-header">
                {tc.status === 'running' && <Wrench size={14} className="animate-spin" />}
                {tc.status === 'success' && <CheckCircle2 size={14} color="var(--success)" />}
                {tc.status === 'error' && <AlertCircle size={14} color="var(--danger)" />}
                <span>{tc.name}</span>
              </div>
              <div className="tool-card-body">
                {JSON.stringify(tc.args, null, 2)}
              </div>
              {tc.result && (
                <div className={`tool-card-result ${tc.status}`}>
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
                          <div style={{ marginTop: '12px', borderRadius: '8px', overflow: 'hidden' }}>
                            <img src={`data:image/jpeg;base64,\${base64Img}`} alt="tool result" style={{ maxWidth: '100%', display: 'block' }} />
                          </div>
                        )}
                      </>
                    );
                  })()}
                </div>
              )}
            </div>
          ))}

          {statusText && (
            <div className="message-wrapper assistant" style={{ opacity: 0.7 }}>
              <div className="avatar-box"><Wrench size={18} className="animate-spin" /></div>
              <div className="bubble-content" style={{ fontStyle: 'italic', fontSize: '0.85rem' }}>{statusText}</div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {/* --- INPUT ZONE --- */}
        <div className="input-zone">
          {attachments.length > 0 && (
            <div className="attachment-previews">
              {attachments.map((att, i) => (
                <div key={i} className="attachment-thumb">
                  {att.type === 'image_url' ? <img src={att.data} alt="preview" className="attachment-img" /> : <div className="attachment-file">📄</div>}
                  <button className="attachment-remove" onClick={() => removeAttachment(i)}><X size={12} /></button>
                </div>
              ))}
            </div>
          )}

          <div className="input-container">
            <textarea
              ref={textareaRef}
              className="textarea-element"
              placeholder={isConnected ? "Send a message or delegate a task..." : "Connecting to engine..."}
              value={inputText}
              onChange={handleInput}
              onKeyDown={handleKeyDown}
              disabled={!isConnected || isProcessing}
              rows={1}
            />
            <div className="input-actions">
              <div style={{ display: 'flex', gap: '8px' }}>
                <input type="file" id="file-upload" multiple style={{ display: 'none' }} onChange={handleFileChange} />
                <label htmlFor="file-upload" className="action-btn" title="Attach file or image" style={{ cursor: 'pointer' }}>
                  <Paperclip size={18} />
                </label>
              </div>
              <div style={{ display: 'flex', gap: '8px' }}>
                {isProcessing && (
                  <button
                    className="send-btn"
                    style={{ backgroundColor: 'var(--danger)' }}
                    onClick={() => {
                      if (wsRef.current) wsRef.current.send(JSON.stringify({ type: 'cancel_run' }));
                      setIsProcessing(false);
                      setStatusText('');
                    }}
                  >
                    <X size={16} /> Stop
                  </button>
                )}
                <button
                  className="send-btn"
                  onClick={sendMessage}
                  disabled={(!inputText.trim() && attachments.length === 0) || !isConnected || isProcessing}
                >
                  <Send size={16} /> Send
                </button>
              </div>
            </div>
          </div>
        </div>

      </main>
    </div>
  );
}
