#!/bin/bash
set -e

# Setup test repo if it doesn't exist
REPO_PATH="/root/test-repo"
REMOTE_PATH="/root/test-remote.git"

if [ ! -d "$REPO_PATH/.git" ]; then
    echo "Initializing test environment..."
    
    # Create remote bare repo
    mkdir -p "$REMOTE_PATH"
    git init --bare "$REMOTE_PATH"

    # Create local repo
    mkdir -p "$REPO_PATH"
    cd "$REPO_PATH"
    git init
    git config --global user.email "tester@lazycommit.io"
    git config --global user.name "Lazy Tester"
    
    echo "# Test Repository" > README.md
    git add README.md
    git commit -m "initial commit"
    
    git remote add origin "$REMOTE_PATH"
    git push origin master
    
    echo "Test environment ready at $REPO_PATH"
fi

# Execute the main command
exec "$@"
