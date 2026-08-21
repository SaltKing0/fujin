# bash completion for fujin — source this file or install via:
#   fujin completion bash > ~/.local/share/bash-completion/completions/fujin
# or add to your shell profile:
#   source /path/to/contrib/completions/fujin.bash

_fujin() {
    local cur prev commands
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    commands="push flush status log queue ci init install-hook uninstall-hook hook --version -version --help -help"

    if [[ ${COMP_CWORD} -eq 1 ]]; then
        COMPREPLY=( $(compgen -W "${commands}" -- "${cur}") )
        return 0
    fi

    case "${COMP_WORDS[1]}" in
        push)
            # complete remote branches for refspecs
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "$(git branch 2>/dev/null | sed 's/^[* ] //')" -- "${cur}") )
                return 0
            fi
            ;;
        hook)
            if [[ ${COMP_CWORD} -eq 2 ]]; then
                COMPREPLY=( $(compgen -W "pre-push" -- "${cur}") )
                return 0
            fi
            ;;
        status)
            COMPREPLY=( $(compgen -W "--json" -- "${cur}") )
            return 0
            ;;
        --config)
            COMPREPLY=( $(compgen -f -- "${cur}") )
            return 0
            ;;
    esac
    return 0
}
complete -F _fujin fujin
