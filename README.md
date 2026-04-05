 ask 🤖

   ask  is a lightweight, powerful CLI-based AI assistant powered by Google
  Gemini. It brings the intelligence of Large Language Models directly to your
  terminal with beautiful Markdown rendering, Google Search integration, and
  support for piping files directly into your prompts.

  Image: Go Version → https://img.shields.io/badge/go-1.25+-blue.svg Image:
  License → https://img.shields.io/badge/license-MIT-green.svg

  ## ✨ Features

  • 🔍 Google Search Integration: The models can use Google Search to provide
  up-to-date information and ground their answers in real-world facts.
  • 🎨 Beautiful Markdown: Uses  glamour  to render code blocks, headers, and
  lists beautifully in your terminal.
  • 📥 Stdin Support: Seamlessly pipe logs, code files, or text outputs into
  the assistant.
  • 🧠 Thinking Mode: Optimized to utilize Gemini's reasoning and "thinking"
  capabilities.
  • 🚀 Model Presets: Quickly switch between different model tiers (Free,
  Cheap, Experimental) using simple aliases.

  ## 🛠️ Installation

  ### Prerequisites

  • Go https://go.dev/doc/install (version 1.25 or higher)
  • A Google Gemini API Key https://aistudio.google.com/

  ### Setup

  1. Clone the repository:
    git clone https://github.com/yourusername/ask.git
    cd ask

  2. Build the binary:
    go build -o ask

  3. Configure your API Key:
  Add your Gemini API key to your environment variables (e.g., in your  .
  bashrc  or  .zshrc ):
    export GEMINI_API_KEY='your_api_key_here'

  4. Move to your PATH (Optional):
    sudo mv ask /usr/local/bin/


  ## 🚀 Usage

  ### Basic Prompts

  Ask a question directly from the command line:

    ask "What is a goroutine?"
    ask "Explain the difference between an interface and a struct in Go"

  ### Using Model Presets

  You can switch models using the  --model  flag. Available presets:

  •  free  (Default):  gemma-4-26b-a4b-it
  •  cheap :  gemini-3.1-flash-lite-preview
  •  exp :  gemini-3-flash-preview

    # Use the experimental high-power model
    ask --model exp "Analyze this complex architecture..."

    # Use the lightweight model for quick questions
    ask --model cheap "What is 2+2?"

  ### Piping Input (The Power User Way)

  You can pipe the contents of a file or the output of another command into
  ask . This is perfect for code reviews or log analysis.

    # Explain a specific file
    cat main.go | ask "Explain what this code does"

    # Analyze error logs
    tail -n 50 error.log | ask "Summarize the main errors found in these logs"

    # Combine a prompt with a file
    echo "Refactor this code for better performance:" | ask main.go

  ### Help Menu

  To see all available options:

    ask --help

  ## ⚙️ Technical Details

  • Backend:  google.golang.org/genai
  • Rendering:  github.com/charmbracelet/glamour
  • Capabilities: The tool is configured with  GoogleSearch  tools enabled,
  allowing the model to perform real-time web searches when required.

  ## 🤝 Contributing

  Contributions are welcome! Please feel free to submit a Pull Request.

  1. Fork the project
  2. Create your Feature Branch ( git checkout -b feature/AmazingFeature )
  3. Commit your changes ( git commit -m 'Add some AmazingFeature' )
  4. Push to the Branch ( git push origin feature/AmazingFeature )
  5. Open a Pull Request

  ## 📄 License

  Distributed under the MIT License. See  LICENSE  for more information
