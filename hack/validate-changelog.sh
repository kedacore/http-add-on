#!/usr/bin/env bash
set -euo pipefail

SCRIPT_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
CHANGELOG="${1:-$SCRIPT_ROOT/CHANGELOG.md}"

if [[ ! -f "$CHANGELOG" ]]; then
    echo "Error: $CHANGELOG not found" >&2
    exit 1
fi

# Format: - **Component**: description ([#NUM](URL)|...) where NUM matches URL path
LINK='\[#([0-9]+|TODO)\]\(https://github\.com/kedacore/[^/]+/(pull|issues|discussions)/([0-9]+|TODO)\)'
LINE_PATTERN="^- \\*\\*[^*]+\\*\\*: .+\\($LINK(\\|$LINK)*\\)\$"

SECTIONS=("Breaking Changes" "New" "Improvements" "Fixes" "Deprecations" "Other")

errors=0

# Get content between two markdown headers
get_section() {
    local version="$1"
    local section="$2"
    # Match until next ## header (v* or Unreleased) or EOF
    sed -n "/^## $version\$/,/^## [vU]/p" "$CHANGELOG" | sed -n "/^### $section\$/,/^### /p" | grep '^- \*\*' || true
}

# Validate [#NUM] matches URL path number (skips TODO links)
validate_link_numbers() {
    local line="$1" link link_num url_num valid=0
    for link in $(echo "$line" | grep -oE '\[#[0-9]+\]\([^)]+\)'); do
        link_num=$(echo "$link" | grep -oE '\[#[0-9]+\]' | tr -d '[]#')
        url_num=$(echo "$link" | grep -oE '(pull|issues|discussions)/[0-9]+' | grep -oE '[0-9]+$' || true)
        if [[ -z "$url_num" ]]; then
            echo "could not extract URL number from: $link" >&2
            valid=1
        elif [[ "$link_num" != "$url_num" ]]; then
            echo "link [#$link_num] does not match URL number $url_num" >&2
            valid=1
        fi
    done
    return $valid
}

# Sort: General lines first, then rest alphabetically
sort_section() {
    local input general_lines other_lines
    input=$(cat)
    general_lines=$(echo "$input" | grep '^- \*\*General\*\*:' | LC_ALL=C sort -f || true)
    other_lines=$(echo "$input" | grep -v '^- \*\*General\*\*:' | LC_ALL=C sort -f || true)
    # Output non-empty parts, avoiding extra newlines
    [[ -n "$general_lines" ]] && echo "$general_lines"
    [[ -n "$other_lines" ]] && echo "$other_lines"
    true
}

# Get versions from History section
versions=$(sed -n '/^## History/,/^## /p' "$CHANGELOG" | grep -o '\[[^]]*\]' | tr -d '[]' || true)

if [[ -z "$versions" ]]; then
    echo "Error: No versions found in ## History section" >&2
    exit 1
fi

for version in $versions; do
    echo "Checking: $version"

    for section in "${SECTIONS[@]}"; do
        content=$(get_section "$version" "$section")
        [[ -z "$content" ]] && continue

        # Check format and link numbers
        while IFS= read -r line; do
            if ! echo "$line" | grep -qE "$LINE_PATTERN"; then
                echo "  Error: [$section] Invalid format: $line" >&2
                errors=1
            elif ! validate_link_numbers "$line"; then
                echo "  Error: [$section] $line" >&2
                errors=1
            fi
        done <<< "$content"

        # Check sorting
        sorted=$(echo "$content" | sort_section)
        if [[ "$content" != "$sorted" ]]; then
            echo "  Error: [$section] Not sorted. Expected:" >&2
            echo "$sorted" | sed 's/^/    /' >&2
            errors=1
        fi
    done
done

if [[ $errors -eq 0 ]]; then
    echo "OK"
else
    echo "Validation failed" >&2
fi

exit $errors
