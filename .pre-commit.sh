#!/bin/bash
# Pre-commit hook to run lint and tests.

echo "🔍 Running pre-commit checks..."

# Run lint
echo "🧹 Running lint..."
make lint
if [ $? -ne 0 ]; then
    echo "❌ Lint failed. Commit aborted."
    exit 1
fi

# Run tests
echo "🧪 Running tests..."
make test
if [ $? -ne 0 ]; then
    echo "❌ Tests failed. Commit aborted."
    exit 1
fi

echo "✅ Pre-commit checks passed!"
exit 0
