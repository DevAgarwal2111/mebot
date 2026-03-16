# MeBot — Full System Specification
> Version 1.0 | Status: Active Development  
> Stack: Go (Backend/Engine) · React (Frontend) · Playwright (Browser Automation) · WebSockets

---

## Table of Contents
1. [What We Are Building](#1-what-we-are-building)
2. [Current State](#2-current-state)
3. [High-Level Architecture](#3-high-level-architecture)
4. [The VM Setup](#4-the-vm-setup)
5. [Engine Loop — How It Works](#5-engine-loop--how-it-works)
6. [Complete Message & Protocol Language](#6-complete-message--protocol-language)
7. [Tool Registry — Full List](#7-tool-registry--full-list)
8. [Browser Automation Layer](#8-browser-automation-layer)
9. [Session Management](#9-session-management)
10. [Screen Streaming](#10-screen-streaming)
11. [Human-in-the-Loop Flows](#11-human-in-the-loop-flows)
12. [WebSocket Events (Engine ↔ Frontend)](#12-websocket-events-engine--frontend)
13. [Go Project Structure](#13-go-project-structure)
14. [Installation & Deployment](#14-installation--deployment)
15. [What To Build Next (Prioritized)](#15-what-to-build-next-prioritized)

---

## 1. What We Are Building

MeBot is a **personal AI assistant that lives on a VPS** and controls a real browser on your behalf. Think of it as your own private computer in the cloud that you talk to through a phone app.

### Core Capabilities (Target State)
- Send WhatsApp messages, emails, fill forms — by controlling a real browser
- Execute tasks that require logging into websites on your behalf
- Remember your sessions so you only log in once per service
- Show you what it's doing in real time via a live screen stream to your phone
- Pause and ask you when it hits a QR code, OTP, or CAPTCHA
- Run entirely on a ~$6/month VPS, controlled from your phone over WebSockets

### What Makes This Different From a Normal Chatbot
A normal chatbot returns text. MeBot **does things**. It opens Chrome, navigates websites, clicks buttons, types text, and reports back. The LLM is the brain; Playwright running on a virtual display is the hands.

---

## 2. Current State

The following is **already built and working:**
- Basic Go WebSocket server
- React frontend with chat UI
- LLM API integration (message in → message out)
- Basic conversation history management

The following is **not yet built** and is what this spec defines:
- Playwright browser automation layer
- Xvfb virtual display setup
- Session/cookie manager
- Tool registry and dispatcher
- Screen streaming to frontend
- Human-in-the-loop pause/resume flows
- Agentic loop (LLM → tool call → result → LLM → ...)

---

## 3. High-Level Architecture

```
┌─────────────────────────────────────────────────────┐
│                      VPS                            │
│                                                     │
│   ┌─────────────┐      ┌───────────────────────┐   │
│   │  Go Engine  │─────▶│  Playwright Controller │   │
│   │  (Brain)    │      │  (Browser Automation)  │   │
│   └──────┬──────┘      └──────────┬────────────┘   │
│          │                        │                 │
│          │             ┌──────────▼────────────┐   │
│          │             │  Chrome on Xvfb        │   │
│          │             │  (Virtual Display :99) │   │
│          │             └──────────┬────────────┘   │
│          │                        │                 │
│          │             ┌──────────▼────────────┐   │
│          │             │  Session Manager       │   │
│          │             │  (cookies per service) │   │
│          │             └───────────────────────┘   │
│          │                                          │
│   ┌──────▼──────┐      ┌───────────────────────┐   │
│   │  LLM Client │      │  Screen Streamer       │   │
│   │  (Gemini)   │      │  (screenshots/WebRTC)  │   │
│   └─────────────┘      └──────────┬────────────┘   │
│                                   │                 │
└───────────────────────────────────┼─────────────────┘
                                    │ WebSocket (wss://)
                              ┌─────▼──────┐
                              │ Phone App  │
                              │ Chat UI    │
                              │ Live View  │
                              └────────────┘
```

### Key Principle
The VPS is **entirely self-contained**. There is no local agent on the user's machine. The browser, the tools, the sessions — everything runs on the VPS. The phone app is just a thin client that sends messages and renders the stream.

---

## 4. The VM Setup

### Required Packages on the VPS

```bash
# Virtual display (so Chrome has somewhere to render)
apt install xvfb x11vnc

# Chrome (must be real Chrome, not Chromium — WhatsApp detects Chromium)
wget https://dl.google.com/linux/direct/google-chrome-stable_current_amd64.deb
apt install ./google-chrome-stable_current_amd64.deb

# Playwright Go bindings
go get github.com/playwright-community/playwright-go

# noVNC for web-based screen streaming (optional but recommended)
apt install novnc
```

### Starting the Virtual Display

```bash
# Start Xvfb on display :99, 1280x720 resolution
Xvfb :99 -screen 0 1280x720x24 &
export DISPLAY=:99

# Start VNC server on top of Xvfb (for streaming)
x11vnc -display :99 -forever -nopw -listen localhost -xkb &
```

This should be managed by **systemd** so it auto-starts on boot. Your Go engine sets `DISPLAY=:99` in its environment before launching Playwright.

### Minimum VPS Specs
- 2 vCPU, 2GB RAM (Chrome is hungry)
- 20GB SSD
- Ubuntu 22.04 LTS
- Estimated cost: ~$6–12/month (Hetzner CX21 or DigitalOcean Basic)

---

## 5. Engine Loop — How It Works

This is the core of the system. A single user message can trigger many tool calls before a final answer is produced.

```
User sends message
       │
       ▼
Engine adds to conversation history
       │
       ▼
┌──────────────────────────────────┐
│           INNER LOOP             │
│                                  │
│  Send conversation to LLM        │
│          │                       │
│          ▼                       │
│  Parse LLM response              │
│          │                       │
│    ┌─────┴──────┐                │
│    │            │                │
│  tool_call   final_answer        │
│    │            │                │
│    ▼            └──────────────┐ │
│  Route tool:                   │ │
│  - remote → run on VPS         │ │
│  - browser → Playwright        │ │
│    │                           │ │
│    ▼                           │ │
│  Execute tool                  │ │
│    │                           │ │
│    ▼                           │ │
│  Capture screenshot (if browser)│ │
│    │                           │ │
│    ▼                           │ │
│  Add result to conversation    │ │
│  (text + screenshot image)     │ │
│    │                           │ │
│    └── loop back to LLM ───────┘ │
└──────────────────────────────────┘
       │
       ▼ (final_answer)
Stream answer to frontend
       │
       ▼
Wait for next user message
```

### Stop Conditions for the Inner Loop
- LLM returns `stop_reason: end_turn` with no tool calls → final answer, exit loop
- LLM returns `stop_reason: tool_use` → execute tools, loop again
- Engine detects `await_*` tool call → pause loop, send event to frontend, wait for user response channel
- Loop iteration count exceeds max (e.g. 20) → safety exit, report to user
- Timeout exceeded (e.g. 120 seconds total) → safety exit

---

## 6. Complete Message & Protocol Language

This section defines every message type in the system. There are three layers of communication:

### Layer 1: Engine → LLM (Gemini API format)

These are the roles in the `messages` array sent to the API.

| Role | When Used | Content |
|---|---|---|
| `system` | Once, at start of every API call | Persona, tool definitions, behavioral rules |
| `user` | Human turn | Plain text, or text + image(s) for multimodal |
| `assistant` | Previous assistant turns | Text and/or tool_use blocks |
| `user` (tool_result) | After tool execution | Tool output: text + optional screenshot image |

**Example multimodal tool result (what the LLM sees after a browser action):**
```json
{
  "role": "user",
  "content": [
    {
      "type": "tool_result",
      "tool_use_id": "toolu_abc123",
      "content": [
        { "type": "text", "text": "Clicked element. Page loaded." },
        { "type": "image", "source": { "type": "base64", "media_type": "image/png", "data": "..." } }
      ]
    }
  ]
}
```

### Layer 2: LLM → Engine (Parsed Response Types)

| Type | Description | Fields |
|---|---|---|
| `text` | Plain language, final answer | `content: string` |
| `tool_use` | LLM wants to call a tool | `id`, `name`, `input: map` |
| `tool_use_batch` | Multiple parallel tool calls | Array of `tool_use` |
| `stop_reason: end_turn` | Fully done, deliver answer | — |
| `stop_reason: tool_use` | Paused, needs tool result | — |
| `stop_reason: max_tokens` | Cut off, handle gracefully | — |

### Layer 3: Engine ↔ Frontend (WebSocket JSON events)

See [Section 12](#12-websocket-events-engine--frontend) for the full list.

### Tool Call Structure (sent to LLM in system prompt as tool definitions)

```json
{
  "name": "browser_click",
  "description": "Click an element on the current page",
  "input_schema": {
    "type": "object",
    "properties": {
      "selector": { "type": "string", "description": "CSS selector or xpath" },
      "fallback_coords": {
        "type": "object",
        "properties": { "x": { "type": "number" }, "y": { "type": "number" } }
      }
    },
    "required": ["selector"]
  }
}
```

### Tool Result Structure (returned from engine after execution)

```json
{
  "tool_use_id": "toolu_abc123",
  "status": "success",
  "output": "Clicked element span[title='Priyanshu']",
  "screenshot_base64": "iVBORw0KGgo...",
  "page_url": "https://web.whatsapp.com",
  "page_title": "WhatsApp"
}
```

**Tool result status codes:**

| Status | Meaning |
|---|---|
| `success` | Ran and returned output |
| `empty` | Ran but returned nothing |
| `error` | Exception or non-zero exit |
| `timeout` | Took too long, was killed |
| `denied` | User rejected confirmation |
| `not_found` | Tool name not in registry |
| `permission_denied` | OS-level access blocked |
| `element_not_found` | CSS selector matched nothing |
| `page_unexpected` | Landed on wrong/error page |
| `session_expired` | Saved login no longer valid |
| `awaiting_human` | Paused for user input |

### Engine Internal State Machine

The engine transitions through these states. Track this explicitly in your Go struct.

```
IDLE              → waiting for user input
SENDING           → dispatching to LLM API
AWAITING_LLM      → waiting for LLM response
PARSING           → parsing LLM output
EXECUTING_TOOL    → running a tool
AWAITING_CONFIRM  → waiting for user to approve dangerous action
AWAITING_QR       → waiting for user to scan QR code
AWAITING_OTP      → waiting for user to provide OTP
AWAITING_CAPTCHA  → waiting for user to solve CAPTCHA
STREAMING         → streaming final answer tokens to frontend
ERROR             → recoverable error
FATAL             → session must reset
```

---

## 7. Tool Registry — Full List

Tools are grouped by location. All tools in this system run **on the VPS**.

### Browser Navigation Tools
| Tool | Args | Returns |
|---|---|---|
| `browser_navigate` | `url: string` | page title, screenshot |
| `browser_back` | — | screenshot |
| `browser_forward` | — | screenshot |
| `browser_refresh` | — | screenshot |
| `browser_new_tab` | `url?: string` | tab_id, screenshot |
| `browser_switch_tab` | `tab_id: string` | screenshot |
| `browser_close_tab` | `tab_id: string` | — |

### Browser Interaction Tools
| Tool | Args | Returns |
|---|---|---|
| `browser_click` | `selector: string, fallback_coords?: {x,y}` | screenshot |
| `browser_type` | `selector: string, text: string, clear_first?: bool` | screenshot |
| `browser_press_key` | `key: string` (e.g. "Enter", "Tab") | screenshot |
| `browser_scroll` | `direction: up/down, amount: int` | screenshot |
| `browser_hover` | `selector: string` | screenshot |
| `browser_select` | `selector: string, value: string` | screenshot |
| `browser_upload_file` | `selector: string, file_path: string` | screenshot |
| `browser_execute_js` | `script: string` | return value, screenshot |

### Browser Vision Tools
| Tool | Args | Returns |
|---|---|---|
| `browser_screenshot` | — | base64 PNG screenshot |
| `browser_extract_text` | — | all visible text on page |
| `browser_extract_dom` | — | simplified accessibility tree |
| `browser_wait_for` | `selector: string, timeout_ms?: int` | found/timeout, screenshot |
| `browser_find_text` | `text: string` | selector of matching element |

### Session Tools
| Tool | Args | Returns |
|---|---|---|
| `session_check` | `service: string` | `{logged_in: bool, expires_at?: string}` |
| `session_save` | `service: string` | success/error |
| `session_load` | `service: string` | success/not_found |
| `session_clear` | `service: string` | success |
| `session_list` | — | list of saved services |

### Screen Tools
| Tool | Args | Returns |
|---|---|---|
| `screen_screenshot` | — | full VM screenshot |
| `screen_read_qr` | — | decoded QR string or not_found |
| `stream_start` | — | stream URL |
| `stream_stop` | — | — |

### Human-in-the-Loop Tools
These cause the engine to PAUSE and wait for user input before continuing.
| Tool | Args | Returns |
|---|---|---|
| `await_qr_scan` | `service: string` | resumes when login detected |
| `await_otp` | `prompt: string` | `{otp: string}` from user |
| `await_captcha` | — | resumes when user solves it |
| `await_confirmation` | `message: string, action: string` | `{confirmed: bool}` |
| `await_manual_login` | `service: string, url: string` | resumes when login detected |
| `await_user_input` | `prompt: string` | `{input: string}` from user |

### Memory Tools
| Tool | Args | Returns |
|---|---|---|
| `memory_store` | `key: string, value: string` | success |
| `memory_fetch` | `key: string` | `{value: string}` |
| `memory_delete` | `key: string` | success |
| `memory_list` | `prefix?: string` | list of keys |

### Utility Tools
| Tool | Args | Returns |
|---|---|---|
| `web_request` | `url, method, headers?, body?` | status, response body |
| `get_current_time` | `timezone?: string` | ISO timestamp |
| `get_page_url` | — | current page URL |

---

## 8. Browser Automation Layer

We use **Playwright for Go** (`github.com/playwright-community/playwright-go`).

### Initialization

```go
// engine/browser/controller.go

type BrowserController struct {
    pw      *playwright.Playwright
    browser playwright.Browser
    context playwright.BrowserContext
    page    playwright.Page
}

func NewBrowserController(sessionPath string) (*BrowserController, error) {
    pw, err := playwright.Run()

    // Launch real Chrome (not Chromium) on the virtual display
    browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
        Headless:       playwright.Bool(false),  // NOT headless — runs on Xvfb
        ExecutablePath: playwright.String("/usr/bin/google-chrome"),
        Args: []string{
            "--display=:99",
            "--no-sandbox",
            "--disable-dev-shm-usage",
            // Stealth flags to avoid bot detection
            "--disable-blink-features=AutomationControlled",
        },
    })

    // Load saved session if it exists
    opts := playwright.BrowserNewContextOptions{}
    if _, err := os.Stat(sessionPath); err == nil {
        opts.StorageStatePath = playwright.String(sessionPath)
    }

    context, _ := browser.NewContext(opts)
    page, _ := context.NewPage()

    return &BrowserController{pw, browser, context, page}, nil
}
```

### Screenshot with Every Action

Every browser tool should take a screenshot AFTER the action and return it. This is how the LLM verifies the action worked:

```go
func (b *BrowserController) Click(selector string) (ToolResult, error) {
    err := b.page.Click(selector)

    screenshot, _ := b.page.Screenshot()
    url := b.page.URL()

    return ToolResult{
        Status:           "success",
        Output:           fmt.Sprintf("Clicked: %s", selector),
        ScreenshotBase64: base64.StdEncoding.EncodeToString(screenshot),
        PageURL:          url,
    }, err
}
```

### Anti-Detection for WhatsApp and Similar Services

WhatsApp Web actively detects headless browsers and automation. Add these measures:

```go
// Inject stealth JS before any page load
b.page.AddInitScript(playwright.PageAddInitScriptOptions{
    Script: playwright.String(`
        Object.defineProperty(navigator, 'webdriver', { get: () => undefined });
        Object.defineProperty(navigator, 'plugins', { get: () => [1,2,3] });
    `),
})

// Add realistic delays between actions (humans don't click instantly)
time.Sleep(time.Duration(rand.Intn(500)+200) * time.Millisecond)
```

---

## 9. Session Management

Each service (WhatsApp, Gmail, etc.) gets its own saved session stored on disk.

### Directory Structure

```
~/.mebot/
└── sessions/
    ├── whatsapp/
    │   └── state.json       ← Playwright storage state (cookies + localStorage)
    ├── gmail/
    │   └── state.json
    └── github/
        └── state.json
```

### Session Lifecycle

```
First use of a service:
  1. session_check("whatsapp") → { logged_in: false }
  2. LLM calls browser_navigate to the service URL
  3. LLM calls await_qr_scan or await_manual_login
  4. Engine streams page to user, user logs in
  5. Engine polls until login is confirmed (checks for logged-in DOM elements)
  6. LLM calls session_save("whatsapp")
  7. State saved to ~/.mebot/sessions/whatsapp/state.json

Every subsequent use:
  1. session_check("whatsapp") → { logged_in: true }
  2. session_load("whatsapp") → loads cookies
  3. browser_navigate to service → already logged in ✅
```

### Detecting Login

For each supported service, define a "login detector" — a CSS selector that only exists when logged in:

```go
var loginDetectors = map[string]string{
    "whatsapp": "div[data-testid='chat-list']",
    "gmail":    "div[data-tooltip='Main Menu']",
    "github":   "img.avatar",
    "twitter":  "a[data-testid='AppTabBar_Home_Link']",
}

func (b *BrowserController) IsLoggedIn(service string) bool {
    selector := loginDetectors[service]
    element, err := b.page.QuerySelector(selector)
    return err == nil && element != nil
}
```

---

## 10. Screen Streaming

The frontend needs to see what the browser is doing in real time.

### Phase 1 — Screenshot Polling (Build This First)

Simple and good enough for most tasks. The engine sends a screenshot every 500ms while a task is running.

```go
func (e *Engine) StartStreaming(ctx context.Context, send func(WSEvent)) {
    ticker := time.NewTicker(500 * time.Millisecond)
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            screenshot, err := e.browser.Page.Screenshot()
            if err != nil { continue }

            send(WSEvent{
                Type:    "stream_frame",
                Content: base64.StdEncoding.EncodeToString(screenshot),
            })
        }
    }
}
```

On the React side, render the stream frame as an `<img>` tag that updates in place.

### Phase 2 — VNC / noVNC (Interactive, Build Later)

For full interactive control from the phone:

```bash
# VNC server reads from Xvfb :99
x11vnc -display :99 -forever -nopw -listen localhost -rfbport 5900 &

# noVNC bridges VNC to WebSocket so the browser can connect
websockify --web /usr/share/novnc 6080 localhost:5900 &
```

The frontend embeds `https://your-vps/novnc/vnc.html` in an iframe. The user can tap to interact directly.

---

## 11. Human-in-the-Loop Flows

These are moments the AI cannot proceed without you. The engine must **pause the agentic loop** and wait.

### Implementation Pattern

Use a `chan UserResponse` to block the engine goroutine:

```go
type UserResponse struct {
    Type  string // "otp", "confirmed", "input", "scanned"
    Value string
}

type Engine struct {
    // ...
    awaitCh chan UserResponse  // nil when not awaiting
}

// Called when LLM requests await_otp
func (e *Engine) AwaitOTP(prompt string, send func(WSEvent)) (string, error) {
    e.awaitCh = make(chan UserResponse, 1)

    // Tell frontend we're waiting
    send(WSEvent{Type: "otp_required", Content: prompt})

    // Block until user responds (or timeout)
    select {
    case resp := <-e.awaitCh:
        return resp.Value, nil
    case <-time.After(5 * time.Minute):
        return "", fmt.Errorf("OTP timeout")
    }
}

// Called when frontend sends { type: "user_response", value: "123456" }
func (e *Engine) ResolveAwait(value string) {
    if e.awaitCh != nil {
        e.awaitCh <- UserResponse{Value: value}
        e.awaitCh = nil
    }
}
```

### QR Code Flow (WhatsApp Example)

```
Engine: browser_navigate → web.whatsapp.com
Engine: browser_screenshot → sees QR code
Engine: calls screen_read_qr → gets QR data  
Engine: sends { type: "qr_detected", qr_data: "...", screenshot: "..." } to frontend
Engine: calls await_qr_scan → BLOCKS

Frontend: shows QR to user
User: scans with their WhatsApp app
WhatsApp Web: logs in automatically

Engine: polls IsLoggedIn("whatsapp") every 2s
Engine: detects login → sends session_save → UNBLOCKS await_qr_scan
Engine: continues task ("now find Priyanshu's chat...")
```

---

## 12. WebSocket Events (Engine ↔ Frontend)

### Frontend → Engine

```json
{ "type": "user_message", "content": "Send Priyanshu a message saying I'll be late" }
{ "type": "user_response", "response_type": "otp", "value": "483921" }
{ "type": "user_response", "response_type": "confirmed", "value": "true" }
{ "type": "user_response", "response_type": "input", "value": "some text" }
{ "type": "stream_toggle", "enabled": true }
{ "type": "ping" }
```

### Engine → Frontend

```json
{ "type": "pong" }
{ "type": "thinking",       "content": "Working on it..." }
{ "type": "tool_call",      "tool": "browser_navigate", "args": {"url": "..."} }
{ "type": "tool_result",    "tool": "browser_navigate", "status": "success", "content": "Page loaded" }
{ "type": "stream_frame",   "content": "" }
{ "type": "qr_detected",    "service": "whatsapp", "screenshot": "" }
{ "type": "otp_required",   "content": "Please enter the OTP sent to your phone" }
{ "type": "captcha_required","screenshot": "" }
{ "type": "await_confirm",  "content": "Delete this file?", "action": "rm ~/important.txt" }
{ "type": "login_detected", "service": "whatsapp" }
{ "type": "login_lost",     "service": "whatsapp" }
{ "type": "assistant",      "content": "Message sent to Priyanshu ✓" }
{ "type": "llm_token",      "content": "Mes" }
{ "type": "task_progress",  "step": 2, "total": 5, "content": "Finding Priyanshu's chat..." }
{ "type": "task_complete",  "content": "Done" }
{ "type": "task_failed",    "content": "Could not find contact", "recoverable": true }
{ "type": "error",          "content": "Something went wrong", "code": "TIMEOUT" }
{ "type": "session_start",  "session_id": "abc123" }
{ "type": "memory_updated", "key": "user_preference_timezone", "value": "Asia/Kolkata" }
```

---

## 13. Go Project Structure

```
mebot/
├── main.go                        ← entry point, starts WS server
│
├── websocket/
│   ├── handler.go                 ← WS upgrade, per-connection goroutine
│   └── types.go                   ← IncomingMessage, WSEvent structs
│
├── engine/
│   ├── engine.go                  ← core agentic loop (THE most important file)
│   ├── conversation.go            ← message history, token counting, truncation
│   ├── parser.go                  ← parse LLM response → tool call or text
│   └── state.go                   ← EngineState enum, transitions
│
├── llm/
│   └── client.go                  ← Gemini API wrapper, streaming support
│
├── browser/
│   ├── controller.go              ← Playwright init, Chrome launch on Xvfb
│   ├── actions.go                 ← click, type, scroll, navigate, screenshot
│   ├── vision.go                  ← extract_text, extract_dom, find_element, read_qr
│   ├── sessions.go                ← save/load/check browser storage state
│   └── stealth.go                 ← anti-detection JS injections
│
├── tools/
│   ├── registry.go                ← tool name → handler function map
│   ├── browser_tools.go           ← wraps browser/actions.go as tool handlers
│   ├── session_tools.go           ← wraps browser/sessions.go as tool handlers
│   ├── memory_tools.go            ← key-value persistent storage
│   ├── await_tools.go             ← human-in-the-loop pause/resume
│   └── utility_tools.go           ← time, web_request, etc.
│
├── stream/
│   └── streamer.go                ← screenshot polling loop, VNC bridge
│
├── config/
│   └── config.go                  ← load from ~/.config/mebot/config.yaml
│
└── types/
    └── types.go                   ← shared structs: ToolCall, ToolResult, Message, etc.
```

### Core Structs (types/types.go)

```go
type Message struct {
    Role    string      `json:"role"`
    Content interface{} `json:"content"`  // string or []ContentBlock
}

type ContentBlock struct {
    Type      string `json:"type"`           // text, image, tool_use, tool_result
    Text      string `json:"text,omitempty"`
    ID        string `json:"id,omitempty"`   // for tool_use
    Name      string `json:"name,omitempty"` // for tool_use
    Input     any    `json:"input,omitempty"`
    Source    *ImageSource `json:"source,omitempty"` // for image blocks
    ToolUseID string `json:"tool_use_id,omitempty"`  // for tool_result
    Content   []ContentBlock `json:"content,omitempty"` // nested in tool_result
}

type ImageSource struct {
    Type      string `json:"type"`       // "base64"
    MediaType string `json:"media_type"` // "image/png"
    Data      string `json:"data"`       // base64 string
}

type ToolResult struct {
    Status           string `json:"status"`
    Output           string `json:"output"`
    ScreenshotBase64 string `json:"screenshot_base64,omitempty"`
    PageURL          string `json:"page_url,omitempty"`
    PageTitle        string `json:"page_title,omitempty"`
}

type WSEvent struct {
    Type    string `json:"type"`
    Content string `json:"content,omitempty"`
    Tool    string `json:"tool,omitempty"`
    Args    any    `json:"args,omitempty"`
    Status  string `json:"status,omitempty"`
    Step    int    `json:"step,omitempty"`
    Total   int    `json:"total,omitempty"`
}

type EngineState string
const (
    StateIdle           EngineState = "idle"
    StateSending        EngineState = "sending"
    StateAwaitingLLM    EngineState = "awaiting_llm"
    StateExecutingTool  EngineState = "executing_tool"
    StateAwaitingHuman  EngineState = "awaiting_human"
    StateStreaming       EngineState = "streaming"
    StateError          EngineState = "error"
)
```

---

## 14. Installation & Deployment

### One-Command Install (get.mebot.dev)

```bash
curl -sSL https://get.mebot.dev | sh
```

The installer script must:
1. Detect OS and CPU architecture
2. Download the correct Go binary from GitHub Releases
3. Install system dependencies (Xvfb, Chrome, x11vnc)
4. Create `~/.config/mebot/config.yaml` with a generated pairing token
5. Install and start a systemd service
6. Print a QR code to the terminal for pairing with the phone app

### Systemd Service

```ini
[Unit]
Description=MeBot AI Assistant
After=network.target

[Service]
ExecStartPre=/usr/bin/Xvfb :99 -screen 0 1280x720x24
ExecStart=/usr/local/bin/mebot serve
Restart=always
User=%i
Environment=DISPLAY=:99
Environment=HOME=/home/%i

[Install]
WantedBy=multi-user.target
```

### Config File (~/.config/mebot/config.yaml)

```yaml
pairing_token: 
llm_api_key: 
llm_model: gemini-2.5-flash
port: 8080
max_tool_iterations: 20
tool_timeout_seconds: 30
session_dir: ~/.mebot/sessions
stream_fps: 2
```

---

## 15. What To Build Next (Prioritized)

Build in this order. Each step is independently testable.

### Step 1 — Agentic Loop (No Browser Yet)
Extend the existing chatbot so the LLM can call tools and receive results before producing a final answer. Use simple tools like `get_current_time` and `web_request` to test the loop. This gives you the core engine pattern without browser complexity.

**Done when:** User asks "what time is it?" and the LLM calls `get_current_time`, gets the result, and answers naturally.

### Step 2 — Browser Controller Basics
Set up Xvfb, install Chrome, initialize Playwright. Implement `browser_navigate`, `browser_screenshot`, `browser_click`, `browser_type`. Take a screenshot after every action.

**Done when:** The engine can navigate to google.com and take a screenshot.

### Step 3 — Screenshot Streaming
Start streaming screenshots to the frontend every 500ms during task execution. Render them in the React UI as a live view panel.

**Done when:** User sends a message, can see the browser moving in real time on their screen.

### Step 4 — Session Management
Implement session save/load/check. Build the `await_qr_scan` and `await_manual_login` pause flows.

**Done when:** User can log into WhatsApp Web once and stay logged in across sessions.

### Step 5 — First Real Task (WhatsApp)
Wire it all together for the WhatsApp send message flow. This is the end-to-end integration test for the whole system.

**Done when:** User says "send Priyanshu a message saying I'll be late" and it works.

### Step 6 — Polish
Memory tools, task progress events, better error recovery, LLM token streaming, conversation history truncation for long sessions.

---

*This document is the source of truth for MeBot's architecture. Update it as the design evolves.*
