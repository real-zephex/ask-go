Good evening to whosoever is reading this.
This is my first time writing a DEVLOG. I am not sure how it works, but I will try to cover as much as I can.

### Motivation
Every time I wanted to query an LLM for something, I had to:
1. Open browser
2. Browse to the website
3. Write the prompt
4. Copy-paste the content from local files (optional)
This is slow and takes up too much time. I wanted a better solution, and since I mainly live in the terminal, I wanted something that also lives in the terminal so the switching overhead is not that huge. I wanted to have a tool that would let me call the best models right from my CLI. However, I didn't want an agent like Opencode, Claude Code, Codex, etc. They are way overkill for the use case I had in mind.

I was targeting a use case that looked like this:
Imagine you are running a Minecraft server and, for some reason, your server keeps crashing. You have no idea what's going on, and you are very persistent and want to fix whatever the fuck is going wrong with it. The Minecraft log files are readable, but hey, nobody wants to read them.

The tool should be able to take in this entire file from the CLI and use it as context.

#### Version 1 | Typescript + Bun
Let's be real here for a moment: Bun has revolutionized how we write TypeScript. Node comes nowhere close to Bun.

Since I am making this tool for CLI, it needs to be fast, like Lightning McQueen fast. So, I opted for Groq. The models there are really fast (well, fast does not mean smart, we will get to that later). Bun supports args, SQLite, and a ton of other features by default, so it was a breeze setting it up.

You can even compile your code into one final executable and target specific platforms. However, the final binary (without any optimization flag) was over 100 MB. For a simple CLI tool, that's unacceptable.

#### Version 2 | Go
I watch Primeagen a lot. He got me interested in Go. I had also heard a lot of good things about Go and wanted to try a new language. So, I spent my entire Sunday learning the basics of Go. The next day, when I found myself comfortable with the basics, I started rewriting ask in Go. It was easy for the most part, but this time I opted to use Gemini because they recently dropped the Gemma 4 models, which are very good, and I wanted to try them.

However, the Gemini API is a mess. I like the models, but not the way we are supposed to interact with them programmatically. Literally every other lab has an OpenAI-compatible endpoint, but Google still sticks to its own fuckery. I am aware that Google does have an endpoint, but it slashes the speeds by a lot.

The Gemini SDK code for Go is very verbose. I am not a big fan of the SDK yet, but if it works then I am happy. I also baked in database support using SQLite, and the last 20 messages are passed with every message as context.

I was exploring the possibility of adding support for streaming, but I was not able to find any viable way to do so without causing issues like duplication, broken markdown rendering, etc. I am sticking with no streaming for now.

### Update | April 25, 2026
The project has evolved into a stable CLI tool for quick LLM interactions. It is now fully functional with its core features:
- **CLI-based prompt engineering**: Seamlessly integrates with the local terminal environment.
- **Context Handling**: Successfully implemented persistent chat history via SQLite, allowing for coherent multi-turn conversations.
- **Interactive UI**: Leverages charmbracelet libraries (lipgloss, glamour) to provide a clean, readable output directly in the terminal, bypassing the need for browsers.
- **Project Structure**: Well-organized codebase separating API logic (gemini.go), database interactions (db.go), and UI rendering (renderer.go, ui.go).
- **Current State**: The tool is now actively used for debugging and rapid development tasks. While streaming remains unimplemented to ensure output stability, the request-response cycle is fast enough for most CLI workflows.
