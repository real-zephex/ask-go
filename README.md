# ask

<p align="center"><b>Cloud intelligence. Local control.</b></p>
<p align="center">A local-first, agentic CLI assistant built in Go.</p>

<p align="center">
  <a href="./assets/demo-smooth.gif">
    <img src="./assets/demo-small.gif" alt="ask demo" width="900" />
  </a>
  <br/>
  <sub>Click the demo to open a higher-quality version.</sub>
</p>

`ask` runs in your terminal, keeps local state on disk, and can optionally use agent tools (shell, file edits, HTTP, clipboard, lists, and memory CRUD) with approval gates.

## Features

- Interactive REPL chat mode with slash commands
- One-shot prompt mode (`ask "..."`) with optional stdin piping
- Streaming rendered markdown output
- Agent mode with function/tool calling
- Per-action approval flow with optional `--yolo` auto-approve
- Local chat history in SQLite
- Local vector memory store (view/add/update/delete)
- Named lists/todos stored locally (usable via agent tool)
- Shell completion generation for bash/zsh/fish
- System prompt override from file (`--system`)

## Requirements

- Go 1.25+
- `GEMINI_API_KEY` in environment

```bash
export GEMINI_API_KEY="your_key_here"
```

## Install

```bash
git clone https://github.com/zephex/go-ask.git
cd go-ask
go build -o ask
```

Optional:

```bash
sudo mv ask /usr/local/bin/
```

## Usage

```bash
ask "What is a goroutine?"
ask --model exp "Analyze this architecture"
cat main.go | ask "Explain this code"
```

### Model aliases

- `free` -> `gemma-4-26b-a4b-it` (default)
- `cheap` -> `gemini-3.1-flash-lite-preview`
- `exp` -> `gemini-3-flash-preview`

### Reasoning levels

- `HIGH`
- `MED` / `MID` / `MEDIUM`
- `LOW`
- `MIN` / `MINIMAL`

### Useful flags

- `--chat` start REPL
- `--agent` enable tool calling in REPL
- `--yolo` auto-approve tool actions that normally prompt
- `--stream` stream rendered markdown (default `true`)
- `--system <file>` load custom system prompt
- `--clear` clear local conversation history

## Chat Mode

Start:

```bash
ask chat
# or
ask --chat
```

Slash commands:

- `/help`
- `/status`
- `/model <alias|name>`
- `/reason <HIGH|MED|LOW|MIN>`
- `/stream on|off`
- `/agent on|off`
- `/yolo on|off`
- `/pwd`
- `/cd <path>`
- `/history [n]`
- `/clear`
- `/memories` (opens memory manager)
- `/exit` or `/quit`

## Memory

Long-term memory is stored locally in a chromem persistent DB and is accessible in two ways:

- CLI: `ask memories` (list), `ask memories manage` (interactive manager)
- Agent tools: `memory_view`, `memory_add`, `memory_update`, `memory_delete`

Memory manager commands:

- `l` / `list`
- `d <n>` / `del <n>`
- `da` / `delall`
- `q` / `quit`

Important current behavior:

- Memory is **not automatically injected** into prompts in agent mode.
- Automatic per-turn memory extraction/saving is currently **disabled** in runtime flow.
- Memory changes currently happen through explicit memory management commands/tools.

### How Memory Works

1. Storage layer
- Memories are stored in a local chromem persistent DB under `~/db`.
- Each memory is a document with a stable `id` and `content`.
- IDs are generated from content hashing for deterministic identity.

2. Listing and management
- `ask memories` prints stored memory entries.
- `ask memories manage` opens an interactive manager for list/delete/delete-all.
- In agent mode, the model can call `memory_view`, `memory_add`, `memory_update`, and `memory_delete`.

3. Memory retrieval behavior
- There is retrieval/query code in the project (`recallMemories` and `injectMemoryContext`), but it is currently not wired into agent turn prompts.
- That means memory is not auto-attached to every model request right now.

4. Automatic memory extraction status
- The project includes async memory extraction/saving code (`scheduleRememberTurn` + extractor prompt flow).
- At runtime, those calls are currently commented out in the main chat/request flow.
- Result: memory writes are currently explicit/manual (CLI manager or memory CRUD tools), not automatic per turn.

## Agent Tools

When agent mode is on (`ask --chat --agent`), these tools are available:

1. `run_shell_command`
- Runs `bash -lc` command in selected directory
- Returns stdout/stderr/exit code/duration
- Requires approval unless `--yolo`

2. `read_file`
- Reads file contents
- Optional `start_line`/`end_line`
- Read-only, no approval

3. `write_file`
- Exact string replacement (`old_str` -> `new_str`)
- Shows a diff preview
- Requires approval unless `--yolo`

4. `clipboard`
- `read` clipboard (no approval)
- `write` clipboard (approval required unless `--yolo`)

5. `lists`
- Actions: `create_list`, `delete_list`, `get_lists`, `add_item`, `update_item`, `delete_item`, `get_items`
- `delete_list` requires approval unless `--yolo`

6. `http_request`
- Supports `GET`, `POST`, `PUT`, `PATCH`, `DELETE`
- GET does not require approval
- `POST`/`PUT`/`PATCH`/`DELETE` require approval unless `--yolo`

7. `memory_view`
- Lists stored memories with stable IDs

8. `memory_add`
- Adds a memory item

9. `memory_update`
- Updates memory content by ID

10. `memory_delete`
- Deletes memory by ID

## Completions

```bash
ask completion bash
ask completion zsh
ask completion fish
```

## Data Locations

- Conversation + lists (SQLite): `~/.ask-go.db`
- Vector memory DB (chromem): `~/db`

## Safety Notes

- `--chat --agent --yolo` auto-approves sensitive tool actions.
- Use `--yolo` only in trusted environments.
- Prompt content and chat history are stored locally.

## License

MIT (see `LICENSE`).
