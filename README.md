# ask 🤖

<p align="center"><b>Cloud intelligence. Local control.</b></p>
<p align="center">Local-first, agentic CLI assistant with tool use + long-term memory.</p>

<p align="center">
  <a href="https://go.dev/"><img alt="Go" src="https://img.shields.io/badge/go-1.25%2B-blue.svg"></a>
  <a href="./LICENSE"><img alt="License: MIT" src="https://img.shields.io/badge/license-MIT-green.svg"></a>
  <a href="#-chat-mode-repl"><img alt="Interactive REPL" src="https://img.shields.io/badge/chat-REPL-7c3aed.svg"></a>
  <a href="#-agent-mode-shell-tool-use"><img alt="Agent mode" src="https://img.shields.io/badge/agent-shell%20tools-f97316.svg"></a>
  <a href="#-architecture-current"><img alt="Local-first memory" src="https://img.shields.io/badge/memory-local--first-9333ea.svg"></a>
</p>

<p align="center">
  <a href="./assets/demo-smooth.gif">
    <img src="./assets/demo-small.gif" alt="ask demo" width="900" />
  </a>
  <br/>
  <sub>Click the demo to open a higher-quality version.</sub>
</p>

`ask` is a local-first, agentic CLI assistant built in Go.

Most AI tools stop at text. `ask` can **chat**, **run shell tools with approval**, and **remember what matters** across sessions — directly in your terminal.

Privacy-first by architecture:
- **[SQLite](https://www.sqlite.org/) (local):** short-term conversation history
- **[chromem-go](https://github.com/philippgille/chromem-go) (local):** long-term vector memory
- **[Google Gemini](https://ai.google.dev/) (remote):** generation, memory extraction, and embeddings

> Except model inference/embedding calls, your assistant state stays on your machine.

**Quick links:** [Features](#-features) · [Architecture](#-architecture-current) · [Usage](#-usage) · [Chat Mode](#-chat-mode-repl) · [Agent Mode](#-agent-mode-shell-tool-use) · [Shell Completions](#-shell-completions)

Quick try:

```bash
ask --chat --agent --model cheap
```

---

## ✨ What this project is

`ask` is designed to be an **interactive personal CLI assistant**:
- fast enough for everyday terminal workflows
- controllable via flags and slash commands
- agentic when needed (with explicit safety prompts)
- capable of remembering preferences and recurring context across sessions

---

## ✨ Features

- 🔎 **Google Search tool enabled** in generation config
- 🎨 **Markdown rendering** via [`glamour`](https://github.com/charmbracelet/glamour)
- ⚡ **Streaming rendered markdown** (not raw chunk spam)
- 💬 **Interactive chat (REPL)** with slash commands
- 🛠️ **Agent mode** with shell tool calls (`run_shell_command`)
- ✅ **Approval flow** for tool calls (`--yolo` to auto-approve)
- 🧾 **System prompt from file** (`--system path/to/file.txt`)
- 🧠 **Long-term memory** with:
  - extraction pass per completed turn
  - vector storage in chromem-go
  - top relevant memories injected into future prompts
- ⌨️ **Shell completion generation** (bash/zsh/fish)

---

## 🧱 Architecture (current)

### 1) Short-term history
- Stored in SQLite: `~/.ask-go.db`
- Last ~20 messages are sent as conversational context

### 2) Long-term memory
- Stored in chromem-go persistent DB under: `~/db`
- Flow per turn:
  1. user + assistant turn completed
  2. async memory extractor decides what to keep (JSON array of strings)
  3. kept memories are hashed + embedded
  4. saved in vector DB

- On each new query:
  - retrieve top **5** relevant memories
  - inject them as memory context before the user query

### 3) Async memory processing
- Memory save runs in a goroutine (non-blocking response path)
- On interrupt/exit, app waits for pending memory tasks to finish
- Prints memory status such as:
  - `🧠 memory: saved N item(s)`
  - `🧠 memory: no new items saved`

---

## 📦 Requirements

- Go **1.25+**
- `GEMINI_API_KEY` set in environment

```bash
export GEMINI_API_KEY="your_key_here"
```

---

## 🚀 Installation

```bash
git clone https://github.com/zephex/go-ask.git
cd go-ask
go build -o ask
```

Optional:
```bash
sudo mv ask /usr/local/bin/
```

---

## 🧪 Usage

### Basic
```bash
ask "What is a goroutine?"
ask "Explain interfaces in Go"
```

### Model presets
- `free`  → `gemma-4-26b-a4b-it` (default)
- `cheap` → `gemini-3.1-flash-lite-preview`
- `exp`   → `gemini-3-flash-preview`

```bash
ask --model exp "Analyze this architecture"
```

### Reasoning levels
Accepted values map to Gemini thinking levels:
- `HIGH`
- `MED` / `MID` / `MEDIUM`
- `LOW`
- `MIN` / `MINIMAL`

```bash
ask --reason HIGH "Design a scalable queue worker"
```

### Stream toggle
Streaming is on by default.

```bash
ask --stream=false "Explain Go interfaces"
```

### System prompt from file
```bash
ask --system ./system.txt "Review this module"
ask --chat --system ./prompts/mentor.txt
```

### Pipe input
```bash
cat main.go | ask "Explain this code"
tail -n 50 app.log | ask --model cheap "Summarize errors"
```

---

## 💬 Chat mode (REPL)

Start:
```bash
ask chat
# or
ask --chat
```

### Slash commands
- `/help` – show commands
- `/status` – current model/reasoning/stream/agent/yolo/cwd
- `/model <alias|name>` – switch model
- `/reason <HIGH|MED|LOW|MIN>` – switch reasoning
- `/stream on|off`
- `/agent on|off`
- `/yolo on|off`
- `/pwd`
- `/cd <path>`
- `/history [n]`
- `/clear`
- `/exit` or `/quit`

---

## 🤖 Agent mode (shell tool use)

Enable:
```bash
ask --chat --agent
```

Auto-approve tool calls (dangerous):
```bash
ask --chat --agent --yolo
```

Behavior:
- model can request `run_shell_command`
- command output (stdout/stderr/exit code) is returned to the model
- default path asks user approval per command

---

## ⌨️ Shell completions

Generate scripts:
```bash
ask completion bash
ask completion zsh
ask completion fish
```

Install examples:
```bash
# bash
ask completion bash > ~/.local/share/bash-completion/completions/ask

# zsh
ask completion zsh > ~/.zfunc/_ask

# fish
ask completion fish > ~/.config/fish/completions/ask.fish
```

---

## 🗂️ Data locations

- Chat history (SQLite): `~/.ask-go.db`
- Vector memory DB (chromem): `~/db`

---

## ⚠️ Safety notes

- `--chat --agent --yolo` executes model-requested shell commands without manual approval.
- Use YOLO mode only in trusted environments.
- Never paste secrets into prompts if you don’t want them in logs/history.

---

## 🛠️ Current limitations

- Memory retrieval is simple top-k (no reranker yet)
- Memory extraction prompt is static
- No memory dashboard/management commands yet

---

## 🤝 Contributing

PRs are welcome. If you open one, include:
- what changed
- why it changed
- how you tested it

---

## 📄 License

MIT — see `LICENSE`.
