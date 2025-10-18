#!/bin/bash

# Script to re-sign all commits in the repository with your current GPG key
# This will rewrite commit history, so use with caution

set -e

echo "=== Git Commit Re-signing Script ==="
echo "This will rewrite ALL commit history to be authored by you with GPG signatures."
echo "Current GPG signing key: $(git config user.signingkey)"
echo "Current user: $(git config user.name) <$(git config user.email)>"
echo ""

# Check if GPG signing is enabled
if [[ "$(git config commit.gpgsign)" != "true" ]]; then
    echo "❌ GPG signing is not enabled. Please run: git config commit.gpgsign true"
    exit 1
fi

# Check if we have a signing key configured
if [[ -z "$(git config user.signingkey)" ]]; then
    echo "❌ No GPG signing key configured. Please run: git config user.signingkey YOUR_KEY_ID"
    exit 1
fi

echo "Creating backup branch..."
BACKUP_BRANCH="backup-before-resign-$(date +%Y%m%d-%H%M%S)"
git checkout -b "$BACKUP_BRANCH"
git checkout master

echo "Starting commit history rewrite..."
echo "This may take a while depending on the number of commits..."

# Set up GPG environment for non-interactive signing
export GPG_TTY=$(tty)
export FILTER_BRANCH_SQUELCH_WARNING=1

# Test GPG signing first
echo "Testing GPG signing..."
if ! echo "test" | gpg --batch --yes --default-key "$(git config user.signingkey)" --detach-sign > /dev/null 2>&1; then
    echo "❌ GPG signing test failed. You may need to enter your passphrase."
    echo "Trying interactive GPG test..."
    if ! echo "test" | gpg --default-key "$(git config user.signingkey)" --detach-sign > /dev/null; then
        echo "❌ GPG signing failed. Please check your GPG setup."
        exit 1
    fi
fi
echo "✅ GPG signing test passed"

# Use git filter-branch to rewrite ALL commits as you with signatures
git filter-branch -f --env-filter '
    export GIT_AUTHOR_NAME="'"$(git config user.name)"'"
    export GIT_AUTHOR_EMAIL="'"$(git config user.email)"'"
    export GIT_COMMITTER_NAME="'"$(git config user.name)"'"
    export GIT_COMMITTER_EMAIL="'"$(git config user.email)"'"
    export GPG_TTY=$(tty)
' --commit-filter '
    git commit-tree -S "$@"
' HEAD

echo ""
echo "✅ Commit history rewrite complete!"
echo "🔒 All commits are now authored by you and signed with your current GPG key"
echo "📦 Backup created in branch: $BACKUP_BRANCH"
echo ""
echo "Next steps:"
echo "1. Verify the signatures: git log --show-signature"
echo "2. If everything looks good, you can delete the backup: git branch -D $BACKUP_BRANCH"
echo "3. Force push to update remote: git push --force-with-lease origin master"
echo ""
echo "⚠️  WARNING: This has rewritten commit history and changed all commit authorship."
echo "   All commits now appear to be authored by you with your GPG signature."