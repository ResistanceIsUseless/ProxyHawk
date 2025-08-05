#compdef proxyhawk

# ProxyHawk zsh completion script
# Place this file in your zsh completions directory or add to fpath

_proxyhawk() {
    local context state line
    typeset -A opt_args

    _arguments -C \
        '(--help -h)'{--help,-h}'[Show help message]' \
        '--version[Show version information]' \
        '--quickstart[Show quick start guide]' \
        '(-l --list)'{-l,--list}'[Proxy list file]:file:_files' \
        '-config[Configuration file]:file:_files -g "*.yaml" -g "*.yml"' \
        '(-c --concurrency)'{-c,--concurrency}'[Number of concurrent checks]:number:(1 5 10 20 50 100)' \
        '(-t --timeout)'{-t,--timeout}'[Timeout per proxy check]:seconds:(5 10 15 30 60)' \
        '(-v --verbose)'{-v,--verbose}'[Enable verbose output]' \
        '(-d --debug)'{-d,--debug}'[Enable debug mode]' \
        '(-r --rdns)'{-r,--rdns}'[Use reverse DNS for host headers]' \
        '(-o --output)'{-o,--output}'[Save results to text file]:file:_files' \
        '(-j --json)'{-j,--json}'[Save results as JSON]:file:_files' \
        '-wp[Save only working proxies]:file:_files' \
        '-wpa[Save only anonymous proxies]:file:_files' \
        '--no-ui[Disable terminal UI]' \
        '--progress[Progress indicator type]:type:(none basic bar spinner dots percent)' \
        '--progress-width[Progress bar width]:width:(20 30 40 50 60 80)' \
        '--progress-no-color[Disable colored progress output]' \
        '--rate-limit[Enable rate limiting]' \
        '--rate-delay[Delay between requests]:duration:(100ms 500ms 1s 2s 5s)' \
        '--rate-per-host[Apply rate limit per host]' \
        '--rate-per-proxy[Apply rate limit per proxy]' \
        '--hot-reload[Enable config hot-reloading]' \
        '--metrics[Enable Prometheus metrics]' \
        '--metrics-addr[Metrics server address]:address:(:9090 :8080 localhost:9090)' \
        '--metrics-path[Metrics endpoint path]:path:(/metrics /prometheus /stats)'
}

_proxyhawk "$@"