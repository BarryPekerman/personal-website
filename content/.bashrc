# Portfolio Terminal Configuration
# Compatible with both bash and sh

# Detect shell type FIRST before using bash-specific features
if [ -n "$BASH_VERSION" ]; then
    # Bash-specific features
    export HISTSIZE=1000
    export HISTFILESIZE=2000
    export HISTCONTROL=ignoredups:erasedups
    shopt -s histappend 2>/dev/null || true
    
    # Enable tab completion for bash
    if [ -f /etc/bash_completion ] && ! shopt -oq posix; then
        . /etc/bash_completion 2>/dev/null || true
    fi
    
    if [ -f /usr/share/bash-completion/bash_completion ] && ! shopt -oq posix; then
        . /usr/share/bash-completion/bash_completion 2>/dev/null || true
    fi
    
    # Functions (bash only - sh doesn't support function syntax)
    ls() { command ls --color=auto "$@" 2>/dev/null || command ls "$@"; }
    ll() { command ls -alF "$@" 2>/dev/null || command ls -l "$@"; }
    la() { command ls -A "$@" 2>/dev/null || command ls -a "$@"; }
    l() { command ls -CF "$@" 2>/dev/null || command ls "$@"; }
    grep() { command grep --color=auto "$@" 2>/dev/null || command grep "$@"; }
    
    # Aliases for bash
    alias ls='ls --color=auto 2>/dev/null || ls' 2>/dev/null || true
    alias ll='ls -alF 2>/dev/null || ls -l' 2>/dev/null || true
    alias la='ls -A 2>/dev/null || ls -a' 2>/dev/null || true
    alias l='ls -CF 2>/dev/null || ls' 2>/dev/null || true
    alias grep='grep --color=auto 2>/dev/null || grep' 2>/dev/null || true
else
    # sh fallback - basic history only, no functions
    HISTFILE="$HOME/.sh_history"
    HISTSIZE=1000
    # Aliases work in sh
    alias ll='ls -l' 2>/dev/null || true
    alias la='ls -a' 2>/dev/null || true
fi

# Set prompt (works in both bash and sh)
export PS1='guest@portfolio:\w\$ '

# Portfolio-specific aliases (work in both)
alias cv='cat cv.txt' 2>/dev/null || true
alias resume='cat cv.txt' 2>/dev/null || true
alias contact='cat contact_info.md' 2>/dev/null || true
alias projects='ls projects/' 2>/dev/null || true
alias help='echo "Available commands:" && echo "  cv/resume    - View CV" && echo "  contact      - View contact information" && echo "  projects     - List projects" && echo "  cat          - View file contents" && echo "  less         - View files with paging" && echo "  ls/ll        - List directory contents" && echo "  help         - Show this help"' 2>/dev/null || true
