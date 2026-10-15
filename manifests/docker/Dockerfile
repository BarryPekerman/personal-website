FROM alpine:latest

# Install necessary packages
RUN apk add --no-cache \
    bash \
    less \
    nano \
    curl \
    wget \
    git \
    man-pages \
    mandoc \
    && rm -rf /var/cache/apk/*

# Create guest user
RUN adduser -D -s /bin/bash -u 1000 guest

# Set up home directory
USER guest
WORKDIR /home/guest

# Create bash configuration
RUN echo '# Portfolio Terminal Configuration' > ~/.bashrc && \
    echo 'export PS1="\u@portfolio:~$ "' >> ~/.bashrc && \
    echo 'alias ls="ls --color=auto"' >> ~/.bashrc && \
    echo 'alias ll="ls -alF"' >> ~/.bashrc && \
    echo 'alias la="ls -A"' >> ~/.bashrc && \
    echo 'alias l="ls -CF"' >> ~/.bashrc && \
    echo 'alias cv="cat cv.txt"' >> ~/.bashrc && \
    echo 'alias resume="cat cv.txt"' >> ~/.bashrc && \
    echo 'alias contact="cat contact_info.md"' >> ~/.bashrc && \
    echo 'alias projects="ls projects/"' >> ~/.bashrc && \
    echo 'alias help="echo \"Available commands:\" && echo \"  cv/resume    - View CV\" && echo \"  contact      - View contact information\" && echo \"  projects     - List projects\" && echo \"  cat          - View file contents\" && echo \"  less         - View files with paging\" && echo \"  ls/ll        - List directory contents\" && echo \"  help         - Show this help\""' >> ~/.bashrc && \
    echo '' >> ~/.bashrc

# Create welcome message
RUN echo "Welcome to the Portfolio Terminal!" > ~/.welcome && \
    echo "Type 'help' for available commands." >> ~/.welcome && \
    echo "" >> ~/.welcome

# Set bash as the default shell
ENV SHELL=/bin/bash

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD echo "OK" || exit 1

# Default command
CMD ["/bin/bash", "--rcfile", "/home/guest/.bashrc"]
