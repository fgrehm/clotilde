#!/usr/bin/env bash
# shellcheck disable=SC2154  # jq variable substitution
#
# Claude Code Context Monitor
# Real-time context usage monitoring with visual indicators and session analytics
# Inspired by: https://github.com/davila7/claude-code-templates/blob/8b25e9499862aba33a6e5878e016c12339ce5ba7/cli-tool/components/settings/statusline/context-monitor.py
# Bash version with jq for JSON parsing
#
# NOTE: Message counting (👤/🤖) parses the entire transcript file on every statusline
# update. This could become slow for very long sessions with hundreds of messages.
# Consider caching or disabling if performance becomes an issue.
#

set -eo pipefail

# Color codes
readonly RED='\033[31m'
readonly RED_BOLD='\033[31;1m'
readonly LIGHT_RED='\033[91m'
readonly YELLOW='\033[33m'
readonly GREEN='\033[32m'
readonly BLUE='\033[94m'
readonly BRIGHT_YELLOW='\033[93m'
readonly DARK_GRAY='\033[90m'
readonly RESET='\033[0m'

# Unicode characters
readonly PROGRESS_FILLED='█'
readonly PROGRESS_EMPTY='▁'

# Check if the last assistant message had thinking enabled
check_thinking_mode() {
    local transcript_path="$1"

    if [[ -z "$transcript_path" ]] || [[ ! -f "$transcript_path" ]]; then
        return 1
    fi

    # Get the last assistant message and check if it has thinking blocks
    tac "$transcript_path" 2>/dev/null | \
        jq -e 'select(.type == "assistant") | .message.content[] | select(.type == "thinking")' 2>/dev/null >/dev/null && return 0

    return 1
}

# Get git branch from transcript
get_git_branch() {
    local transcript_path="$1"

    if [[ -z "$transcript_path" ]] || [[ ! -f "$transcript_path" ]]; then
        return 1
    fi

    # Get the most recent entry's git branch
    tail -1 "$transcript_path" 2>/dev/null | jq -r '.gitBranch // ""' 2>/dev/null
}

# Get message counts from transcript
get_message_counts() {
    local transcript_path="$1"

    if [[ -z "$transcript_path" ]] || [[ ! -f "$transcript_path" ]]; then
        echo "user=0|assistant=0"
        return
    fi

    # Count user and assistant messages
    local user_count assistant_count
    user_count=$(jq -r 'select(.type == "user") | .type' "$transcript_path" 2>/dev/null | wc -l)
    assistant_count=$(jq -r 'select(.type == "assistant") | .type' "$transcript_path" 2>/dev/null | wc -l)

    echo "user=$user_count|assistant=$assistant_count"
}

# Parse context usage from transcript file
parse_context_from_transcript() {
    local transcript_path="$1"

    if [[ -z "$transcript_path" ]] || [[ ! -f "$transcript_path" ]]; then
        return 1
    fi

    # Check last 15 lines for context information
    local recent_lines
    recent_lines=$(tail -n 15 "$transcript_path" 2>/dev/null || echo "")

    while IFS= read -r line; do
        [[ -z "$line" ]] && continue

        # Try to parse as JSON
        local type
        type=$(echo "$line" | jq -r '.type // empty' 2>/dev/null) || continue

        # Method 1: Parse usage tokens from assistant messages
        if [[ "$type" == "assistant" ]]; then
            local input_tokens cache_read cache_creation total_tokens percent_used

            input_tokens=$(echo "$line" | jq '.message.usage.input_tokens // 0' 2>/dev/null) || continue
            cache_read=$(echo "$line" | jq '.message.usage.cache_read_input_tokens // 0' 2>/dev/null) || continue
            cache_creation=$(echo "$line" | jq '.message.usage.cache_creation_input_tokens // 0' 2>/dev/null) || continue

            total_tokens=$((input_tokens + cache_read + cache_creation))

            if [[ $total_tokens -gt 0 ]]; then
                # Assume 200k context for Claude models
                percent_used=$(awk "BEGIN {printf \"%.0f\", ($total_tokens / 200000) * 100}")
                [[ $percent_used -gt 100 ]] && percent_used=100

                echo "percent=$percent_used|tokens=$total_tokens|method=usage"
                return 0
            fi
        fi

        # Method 2: Parse system context warnings
        if [[ "$type" == "system_message" ]]; then
            local content percent_left
            content=$(echo "$line" | jq -r '.content // ""' 2>/dev/null)

            # "Context left until auto-compact: X%"
            if [[ $content =~ Context\ left\ until\ auto-compact:\ ([0-9]+)% ]]; then
                percent_left="${BASH_REMATCH[1]}"
                percent_used=$((100 - percent_left))
                echo "percent=$percent_used|warning=auto-compact|method=system"
                return 0
            fi

            # "Context low (X% remaining)"
            if [[ $content =~ Context\ low\ \(([0-9]+)%\ remaining\) ]]; then
                percent_left="${BASH_REMATCH[1]}"
                percent_used=$((100 - percent_left))
                echo "percent=$percent_used|warning=low|method=system"
                return 0
            fi
        fi
    done <<< "$recent_lines"

    return 1
}

# Generate context display with visual indicators
get_context_display() {
    local context_info="$1"

    if [[ -z "$context_info" ]]; then
        echo "${BLUE}???${RESET}"
        return
    fi

    # Parse context info string
    local percent warning
    percent=$(echo "$context_info" | grep -oP 'percent=\K[0-9]+')
    warning=$(echo "$context_info" | grep -oP 'warning=\K[^|]*')

    # Determine color, icon, and alert based on usage level
    local icon color alert

    if [[ $percent -ge 95 ]]; then
        icon="🚨"
        color="$RED_BOLD"
        alert="CRIT"
    elif [[ $percent -ge 90 ]]; then
        color="$RED"
        alert="HIGH"
    elif [[ $percent -ge 75 ]]; then
        color="$LIGHT_RED"
        alert=""
    elif [[ $percent -ge 50 ]]; then
        color="$YELLOW"
        alert=""
    else
        color="$GREEN"
        alert=""
    fi

    # Create progress bar (8 segments)
    local segments=8
    local filled=$((percent * segments / 100))
    local empty=$((segments - filled))
    local bar
    bar=$(printf '%0.s'"$PROGRESS_FILLED" $(seq 1 "$filled"))
    bar+=$(printf '%0.s'"$PROGRESS_EMPTY" $(seq 1 "$empty"))

    # Special warnings override alert
    if [[ "$warning" == "auto-compact" ]]; then
        alert="AUTO-COMPACT!"
    elif [[ "$warning" == "low" ]]; then
        alert="LOW!"
    fi

    local alert_str=""
    if [[ -n "$alert" ]]; then
        alert_str=" $alert"
    fi

    if [[ -n "$icon" ]]; then
        printf "%s%s%s%s %d%%%s" "$icon" "$color" "$bar" "$RESET" "$percent" "$alert_str"
    else
        printf "%s%s%s %d%%%s" "$color" "$bar" "$RESET" "$percent" "$alert_str"
    fi
}

# Get directory display name
get_directory_display() {
    local workspace_data="$1"
    local current_dir

    current_dir=$(echo "$workspace_data" | jq -r '.current_dir // ""' 2>/dev/null)

    if [[ -n "$current_dir" ]]; then
        basename "$current_dir"
    else
        echo "unknown"
    fi
}

# Get session metrics display
get_session_metrics() {
    local cost_data="$1"

    if [[ -z "$cost_data" ]]; then
        return
    fi

    local metrics=()

    # Cost
    local cost_usd cost_color cost_str
    cost_usd=$(echo "$cost_data" | jq '.total_cost_usd // 0' 2>/dev/null)

    if (( $(echo "$cost_usd > 0" | bc -l) )); then
        if (( $(echo "$cost_usd >= 0.10" | bc -l) )); then
            cost_color="$RED"
        elif (( $(echo "$cost_usd >= 0.05" | bc -l) )); then
            cost_color="$YELLOW"
        else
            cost_color="$GREEN"
        fi

        if (( $(echo "$cost_usd < 0.01" | bc -l) )); then
            cost_str=$(printf "%.0f¢" "$(echo "$cost_usd * 100" | bc)")
        else
            cost_str=$(printf "\$%.3f" "$cost_usd")
        fi

        metrics+=("${cost_color}💰 ${cost_str}${RESET}")
    fi

    # Duration
    local duration_ms minutes duration_color duration_str
    duration_ms=$(echo "$cost_data" | jq '.total_duration_ms // 0' 2>/dev/null)

    if [[ $duration_ms -gt 0 ]]; then
        minutes=$(awk "BEGIN {printf \"%.0f\", $duration_ms / 60000}")

        if [[ $minutes -ge 30 ]]; then
            duration_color="$YELLOW"
        else
            duration_color="$GREEN"
        fi

        if [[ $duration_ms -lt 60000 ]]; then
            duration_str=$((duration_ms / 1000))s
        else
            duration_str=${minutes}m
        fi

        metrics+=("${duration_color}⌛ ${duration_str}${RESET}")
    fi

    # Lines changed
    local lines_added lines_removed net_lines lines_color
    lines_added=$(echo "$cost_data" | jq '.total_lines_added // 0' 2>/dev/null)
    lines_removed=$(echo "$cost_data" | jq '.total_lines_removed // 0' 2>/dev/null)

    if [[ $lines_added -gt 0 || $lines_removed -gt 0 ]]; then
        net_lines=$((lines_added - lines_removed))

        if [[ $net_lines -gt 0 ]]; then
            lines_color="$GREEN"
        elif [[ $net_lines -lt 0 ]]; then
            lines_color="$RED"
        else
            lines_color="$YELLOW"
        fi

        local sign=""
        [[ $net_lines -ge 0 ]] && sign="+"

        metrics+=("${lines_color}📝 ${sign}${net_lines}${RESET}")
    fi

    if [[ ${#metrics[@]} -gt 0 ]]; then
        printf " %b" "${DARK_GRAY}|${RESET}"
        printf " %s" "${metrics[@]}"
    fi
}

main() {
    local json_data model_name context_display session_metrics model_display status_line thinking_indicator git_branch clotilde_session

    # Read JSON input from Claude Code
    json_data=$(cat)

    if [[ -z "$json_data" ]]; then
        echo "${BLUE}[Claude]${RESET} ${RED}[Error: No input]${RESET}"
        return 1
    fi

    # Get clotilde session name if available
    clotilde_session="${CLOTILDE_SESSION:-}"

    # Extract information
    model_name=$(echo "$json_data" | jq -r '.model.display_name // "Claude"' 2>/dev/null)
    local transcript_path cost_data context_info
    transcript_path=$(echo "$json_data" | jq -r '.transcript_path // ""' 2>/dev/null)
    cost_data=$(echo "$json_data" | jq '.cost // {}' 2>/dev/null)

    # Get git branch from transcript
    git_branch=$(get_git_branch "$transcript_path" 2>/dev/null) || git_branch=""

    # Get message counts from transcript
    local message_counts user_count assistant_count
    message_counts=$(get_message_counts "$transcript_path" 2>/dev/null)
    user_count=$(echo "$message_counts" | grep -oP 'user=\K[0-9]+')
    assistant_count=$(echo "$message_counts" | grep -oP 'assistant=\K[0-9]+')

    # Parse context usage
    context_info=$(parse_context_from_transcript "$transcript_path" 2>/dev/null) || context_info=""

    # Check for thinking mode
    if check_thinking_mode "$transcript_path" >/dev/null 2>&1; then
        thinking_indicator="🧠"
    else
        thinking_indicator=""
    fi

    # Build status components
    context_display=$(get_context_display "$context_info")
    session_metrics=$(get_session_metrics "$cost_data")

    # Model display with context-aware coloring
    if [[ -n "$context_info" ]]; then
        local percent
        percent=$(echo "$context_info" | grep -oP 'percent=\K[0-9]+')

        if [[ $percent -ge 90 ]]; then
            model_display="${RED}[${model_name}]${RESET}"
        elif [[ $percent -ge 75 ]]; then
            model_display="${YELLOW}[${model_name}]${RESET}"
        else
            model_display="${GREEN}[${model_name}]${RESET}"
        fi
    else
        model_display="${BLUE}[${model_name}]${RESET}"
    fi

    # Combine all components
    status_line="${model_display}"

    # Add clotilde session name if available
    if [[ -n "$clotilde_session" ]]; then
        status_line="${status_line} ${BRIGHT_YELLOW}[${clotilde_session}]${RESET}"
    fi

    status_line="${status_line} ${context_display}${session_metrics}"

    # Add message counts if available
    if [[ -n "$user_count" && -n "$assistant_count" ]]; then
        status_line="${status_line} ${DARK_GRAY}|${RESET} 👤 ${user_count} 🤖 ${assistant_count}"
    fi

    # Add git branch at the end if available
    if [[ -n "$git_branch" ]]; then
        status_line="${status_line} ${DARK_GRAY}|${RESET} ${git_branch}"
    fi

    # Add thinking indicator at the very end if present
    if [[ -n "$thinking_indicator" ]]; then
        status_line="${status_line} ${DARK_GRAY}|${RESET} ${thinking_indicator}"
    fi

    printf "%b\n" "$status_line"
}

main "$@"
