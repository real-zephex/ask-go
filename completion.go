package main

import (
	"fmt"
	"strings"
)

func handleCompletionCommand(args []string) int {
	if len(args) == 0 || args[0] != "completion" {
		return -1
	}

	if len(args) != 2 {
		fmt.Println("Usage: ask completion [bash|zsh|fish]")
		return 1
	}

	script, err := completionScript(args[1])
	if err != nil {
		fmt.Println(err.Error())
		return 1
	}

	fmt.Print(script)
	return 0
}

func completionScript(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", fmt.Errorf("unsupported shell %q. expected one of: bash, zsh, fish", shell)
	}
}

func bashCompletionScript() string {
	return `# bash completion for ask
_ask_completion() {
  local cur prev words cword
  _init_completion || return

  case "$prev" in
    --model)
      COMPREPLY=( $(compgen -W "free cheap exp" -- "$cur") )
      return
      ;;
    --reason)
      COMPREPLY=( $(compgen -W "HIGH MED LOW MIN" -- "$cur") )
      return
      ;;
    --stream|--agent|--yolo)
      COMPREPLY=( $(compgen -W "true false" -- "$cur") )
      return
      ;;
    --system)
      COMPREPLY=( $(compgen -f -- "$cur") )
      return
      ;;
  esac

  if [[ ${words[1]} == "completion" ]]; then
    COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
    return
  fi

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=( $(compgen -W "chat completion --help --version --model --reason --clear --chat --stream --agent --yolo --system" -- "$cur") )
    return
  fi

  COMPREPLY=( $(compgen -W "--help --version --model --reason --clear --chat --stream --agent --yolo --system" -- "$cur") )
}

complete -F _ask_completion ask
`
}

func zshCompletionScript() string {
	return `#compdef ask

_ask() {
  local -a flags
  flags=(
    '--help[Show help menu]'
    '--version[Show current version]'
    '--model[Model alias or full model name]:model:(free cheap exp)'
    '--reason[Reasoning effort]:reason:(HIGH MED LOW MIN)'
    '--clear[Clear local conversation history database]'
    '--chat[Start interactive chat mode]'
    '--stream[Stream incremental rendered markdown updates]:bool:(true false)'
    '--agent[Enable agent mode in chat]:bool:(true false)'
    '--yolo[Auto-approve shell commands in agent mode]:bool:(true false)'
    '--system[Path to system prompt file]:file:_files'
  )

  _arguments -C \
    $flags \
    '1:command:(chat completion)' \
    '2:arg:->arg' \
    '*::args:->args'

  case $state in
    arg)
      if [[ $words[2] == completion ]]; then
        _values 'shell' bash zsh fish
      fi
      ;;
  esac
}

_ask "$@"
`
}

func fishCompletionScript() string {
	return `# fish completion for ask
complete -c ask -f

complete -c ask -n '__fish_use_subcommand' -a 'chat' -d 'Start interactive chat mode'
complete -c ask -n '__fish_use_subcommand' -a 'completion' -d 'Generate shell completion script'

complete -c ask -n '__fish_seen_subcommand_from completion' -a 'bash zsh fish'

complete -c ask -l help -d 'Show help menu'
complete -c ask -l version -d 'Show current version'
complete -c ask -l clear -d 'Clear local conversation history database'
complete -c ask -l chat -d 'Start interactive chat mode'
complete -c ask -l model -r -a 'free cheap exp' -d 'Model alias or full model name'
complete -c ask -l reason -r -a 'HIGH MED LOW MIN' -d 'Reasoning effort'
complete -c ask -l stream -r -a 'true false' -d 'Enable or disable streaming'
complete -c ask -l agent -r -a 'true false' -d 'Enable or disable agent mode'
complete -c ask -l yolo -r -a 'true false' -d 'Enable or disable auto approval'
complete -c ask -l system -r -d 'Path to system prompt file'
`
}
