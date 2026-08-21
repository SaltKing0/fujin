#compdef fujin
# zsh completion for fujin — place in $fpath (e.g. ~/.zfunc/) and run
#   compinit && autoload -U compinit
# or source directly:  source /path/to/contrib/completions/fujin.zsh

_fujin() {
  local -a commands
  commands=(
    'push:push with automatic failover (flushes queue first)'
    'flush:replay queued pushes'
    'status:show health of all remotes'
    'queue:show queued pushes'
    'log:show push history'
    'ci:run all workflows locally via raijin'
    'init:interactive first-run setup'
    'install-hook:install a pre-push hook'
    'uninstall-hook:remove the pre-push hook'
    'hook:internal pre-push hook handler'
  )

  if (( CURRENT == 2 )); then
    _describe 'command' commands
    return
  fi

  case "${words[2]}" in
    push)
      _arguments '1:branch:__git_branches'
      ;;
    status)
      _arguments '--json[machine-readable output]'
      ;;
    hook)
      if (( CURRENT == 3 )); then
        compadd 'pre-push'
      fi
      ;;
  esac
}

__git_branches() {
  compadd -- $(git branch --format='%(refname:short)' 2>/dev/null)
}

_compdef _fujin fujin
