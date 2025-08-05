#!/bin/bash
# ProxyHawk bash completion script
# Source this file or add it to your bash completion directory

_proxyhawk_completion() {
    local cur prev opts
    COMPREPLY=()
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    # Main options
    opts="--help -h --version --quickstart -l --list -config -c --concurrency -t --timeout -v --verbose -d --debug -r --rdns"
    opts="$opts -o --output -j --json -wp -wpa --no-ui"
    opts="$opts --progress --progress-width --progress-no-color"
    opts="$opts --rate-limit --rate-delay --rate-per-host --rate-per-proxy"
    opts="$opts --hot-reload --metrics --metrics-addr --metrics-path"

    # File completion for specific options
    case "${prev}" in
        -l|--list|-config|-o|--output|-j|--json|-wp|-wpa)
            COMPREPLY=( $(compgen -f -- ${cur}) )
            return 0
            ;;
        --progress)
            COMPREPLY=( $(compgen -W "none basic bar spinner dots percent" -- ${cur}) )
            return 0
            ;;
        --rate-delay)
            COMPREPLY=( $(compgen -W "100ms 500ms 1s 2s 5s" -- ${cur}) )
            return 0
            ;;
        -c|--concurrency)
            COMPREPLY=( $(compgen -W "1 5 10 20 50 100" -- ${cur}) )
            return 0
            ;;
        -t|--timeout)
            COMPREPLY=( $(compgen -W "5 10 15 30 60" -- ${cur}) )
            return 0
            ;;
        --progress-width)
            COMPREPLY=( $(compgen -W "20 30 40 50 60 80" -- ${cur}) )
            return 0
            ;;
        --metrics-addr)
            COMPREPLY=( $(compgen -W ":9090 :8080 localhost:9090" -- ${cur}) )
            return 0
            ;;
        --metrics-path)
            COMPREPLY=( $(compgen -W "/metrics /prometheus /stats" -- ${cur}) )
            return 0
            ;;
    esac

    # Default completion
    COMPREPLY=( $(compgen -W "${opts}" -- ${cur}) )
    return 0
}

# Register the completion function
complete -F _proxyhawk_completion proxyhawk