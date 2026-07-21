#!/usr/bin/env bash

set -Eeuo pipefail

# Collect the initial HomeDNS Analytics performance baseline.
# This script is intended to run on a Raspberry Pi using Raspberry Pi OS.

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
readonly PROJECT_ROOT="$(cd -- "$SCRIPT_DIR/../.." && pwd)"
readonly TIMESTAMP="$(date '+%Y-%m-%d_%H-%M-%S')"

readonly BASELINE_ROOT="$PROJECT_ROOT/benchmarks/baseline"
readonly RUNS_DIR="$BASELINE_ROOT/runs"
readonly OUTPUT_DIR="$RUNS_DIR/$TIMESTAMP"

command_exists() {
    command -v "$1" >/dev/null 2>&1
}

print_not_available() {
    local item="${1:-Measurement}"
    printf '%s: not available\n' "$item"
}

on_error() {
    local exit_code=$?
    local line_number="${1:-unknown}"

    printf 'Baseline collection failed at line %s with exit code %s.\n' \
        "$line_number" "$exit_code" >&2

    if [[ -d "$OUTPUT_DIR" ]]; then
        printf 'Partial results may exist in: %s\n' "$OUTPUT_DIR" >&2
    fi

    exit "$exit_code"
}

trap 'on_error "$LINENO"' ERR

validate_environment() {
    local os_type
    os_type="$(uname -s)"

    if [[ "$os_type" != "Linux" ]]; then
        printf 'Error: this baseline collector only supports Linux.\n' >&2
        printf 'Detected operating system: %s\n' "$os_type" >&2
        printf 'Run this script directly on the Raspberry Pi.\n' >&2
        exit 1
    fi

    if [[ ! -r /proc/device-tree/model ]]; then
        printf 'Error: no readable Raspberry Pi device-tree model was found.\n' >&2
        printf 'This script is intended to run on a Raspberry Pi.\n' >&2
        exit 1
    fi
}

validate_required_commands() {
    local required_commands=(
        awk
        cat
        date
        df
        free
        hostname
        lscpu
        mkdir
        ps
        uname
        uptime
    )

    local command_name
    local missing_command=false

    for command_name in "${required_commands[@]}"; do
        if ! command_exists "$command_name"; then
            printf 'Error: required command is missing: %s\n' \
                "$command_name" >&2
            missing_command=true
        fi
    done

    if [[ "$missing_command" == true ]]; then
        exit 1
    fi
}

collect_metadata() {
    {
        echo "HomeDNS Analytics - Baseline Metadata"
        echo "====================================="
        echo

        printf 'Benchmark name: %s\n' "Initial Raspberry Pi Baseline"
        printf 'Result identifier: %s\n' "$TIMESTAMP"
        printf 'Local timestamp: %s\n' "$(date '+%Y-%m-%d %H:%M:%S %Z')"
        printf 'UTC timestamp: %s\n' "$(date -u '+%Y-%m-%d %H:%M:%S UTC')"
        printf 'Hostname: %s\n' "$(hostname)"
        printf 'Current user: %s\n' "$(whoami)"
        printf 'Project root: %s\n' "$PROJECT_ROOT"

        echo
        echo "Raspberry Pi model"
        echo "------------------"
        tr -d '\0' < /proc/device-tree/model
        echo

        echo
        echo "Operating system"
        echo "----------------"

        if [[ -r /etc/os-release ]]; then
            cat /etc/os-release
        else
            print_not_available "/etc/os-release"
        fi

        echo
        echo "Kernel and architecture"
        echo "-----------------------"
        printf 'Kernel release: %s\n' "$(uname -r)"
        printf 'Architecture: %s\n' "$(uname -m)"
        printf 'Full uname output: %s\n' "$(uname -a)"

        echo
        echo "Uptime"
        echo "------"
        uptime

        echo
        echo "Timezone and clock"
        echo "------------------"

        if command_exists timedatectl; then
            timedatectl || print_not_available "timedatectl output"
        elif [[ -r /etc/timezone ]]; then
            cat /etc/timezone
        else
            print_not_available "Timezone"
        fi

        echo
        echo "Git repository"
        echo "--------------"

        if command_exists git &&
            git -C "$PROJECT_ROOT" rev-parse \
                --is-inside-work-tree >/dev/null 2>&1; then

            printf 'Commit: %s\n' \
                "$(git -C "$PROJECT_ROOT" rev-parse HEAD)"

            printf 'Branch: %s\n' \
                "$(git -C "$PROJECT_ROOT" branch --show-current || true)"

            if [[ -z "$(git -C "$PROJECT_ROOT" status --porcelain)" ]]; then
                echo "Working tree: clean"
            else
                echo "Working tree: modified"
                echo
                git -C "$PROJECT_ROOT" status --short
            fi
        else
            print_not_available "Git repository information"
        fi
    } > "$OUTPUT_DIR/metadata.txt"
}

collect_memory() {
    {
        echo "HomeDNS Analytics - Memory Baseline"
        echo "==================================="
        echo

        echo "Human-readable memory information"
        echo "---------------------------------"
        free -h

        echo
        echo "Memory information in bytes"
        echo "---------------------------"
        free -b

        echo
        echo "/proc/meminfo"
        echo "-------------"

        if [[ -r /proc/meminfo ]]; then
            cat /proc/meminfo
        else
            print_not_available "/proc/meminfo"
        fi
    } > "$OUTPUT_DIR/memory.txt"
}

collect_system() {
    {
        echo "HomeDNS Analytics - CPU and System Baseline"
        echo "==========================================="
        echo

        echo "CPU information"
        echo "---------------"
        lscpu

        echo
        echo "Uptime and load averages"
        echo "------------------------"
        uptime

        if [[ -r /proc/loadavg ]]; then
            printf '/proc/loadavg: '
            cat /proc/loadavg
        else
            print_not_available "/proc/loadavg"
        fi

        echo
        echo "CPU frequency information"
        echo "-------------------------"

        local frequency_data_found=false
        local cpu_directory
        local cpu_name
        local metric

        shopt -s nullglob

        for cpu_directory in /sys/devices/system/cpu/cpu[0-9]*/cpufreq; do
            [[ -d "$cpu_directory" ]] || continue

            frequency_data_found=true
            cpu_name="$(basename "$(dirname "$cpu_directory")")"

            printf '[%s]\n' "$cpu_name"

            for metric in \
                scaling_cur_freq \
                scaling_min_freq \
                scaling_max_freq \
                cpuinfo_min_freq \
                cpuinfo_max_freq \
                scaling_governor; do

                if [[ -r "$cpu_directory/$metric" ]]; then
                    printf '%s: ' "$metric"
                    cat "$cpu_directory/$metric"
                fi
            done

            echo
        done

        shopt -u nullglob

        if [[ "$frequency_data_found" == false ]]; then
            print_not_available "CPU frequency information"
        else
            echo "Frequency values are normally expressed in kHz."
        fi
    } > "$OUTPUT_DIR/system.txt"
}

collect_storage() {
    {
        echo "HomeDNS Analytics - Storage Baseline"
        echo "===================================="
        echo

        echo "Human-readable filesystem usage"
        echo "-------------------------------"
        df -h

        echo
        echo "Filesystem usage in bytes"
        echo "-------------------------"
        df -B1

        echo
        echo "Block devices"
        echo "-------------"

        if command_exists lsblk; then
            lsblk -o NAME,MODEL,SIZE,TYPE,FSTYPE,MOUNTPOINTS
        else
            print_not_available "lsblk"
        fi

        echo
        echo "Mounted filesystems"
        echo "-------------------"

        if command_exists findmnt; then
            findmnt
        else
            print_not_available "findmnt"
        fi

        echo
        echo "Project filesystem"
        echo "------------------"
        df -h "$PROJECT_ROOT"
        df -B1 "$PROJECT_ROOT"
    } > "$OUTPUT_DIR/storage.txt"
}

collect_temperature() {
    {
        echo "HomeDNS Analytics - Temperature Baseline"
        echo "========================================"
        echo

        echo "Raspberry Pi firmware measurements"
        echo "----------------------------------"

        if command_exists vcgencmd; then
            vcgencmd measure_temp ||
                print_not_available "vcgencmd temperature"

            vcgencmd get_throttled ||
                print_not_available "vcgencmd throttling status"
        else
            print_not_available "vcgencmd"
        fi

        echo
        echo "Linux thermal-zone measurements"
        echo "-------------------------------"

        local thermal_data_found=false
        local thermal_zone
        local zone_name
        local raw_temperature

        shopt -s nullglob

        for thermal_zone in /sys/class/thermal/thermal_zone*; do
            [[ -d "$thermal_zone" ]] || continue

            thermal_data_found=true
            zone_name="$(basename "$thermal_zone")"

            printf '[%s]\n' "$zone_name"

            if [[ -r "$thermal_zone/type" ]]; then
                printf 'Type: '
                cat "$thermal_zone/type"
            fi

            if [[ -r "$thermal_zone/temp" ]]; then
                raw_temperature="$(cat "$thermal_zone/temp")"

                printf 'Raw temperature: %s millidegrees Celsius\n' \
                    "$raw_temperature"

                if [[ "$raw_temperature" =~ ^-?[0-9]+$ ]]; then
                    awk -v value="$raw_temperature" \
                        'BEGIN {
                            printf "Temperature: %.3f °C\n", value / 1000
                        }'
                fi
            fi

            echo
        done

        shopt -u nullglob

        if [[ "$thermal_data_found" == false ]]; then
            print_not_available "Linux thermal-zone data"
        fi

        echo
        echo "Throttling note"
        echo "---------------"
        echo "A non-zero get_throttled value may indicate current or previous"
        echo "undervoltage, frequency capping, throttling, or overheating."
    } > "$OUTPUT_DIR/temperature.txt"
}

collect_boot() {
    {
        echo "HomeDNS Analytics - Boot Baseline"
        echo "================================="
        echo

        if command_exists systemd-analyze; then
            echo "Overall boot duration"
            echo "---------------------"
            systemd-analyze ||
                print_not_available "Overall boot duration"

            echo
            echo "Services ordered by startup duration"
            echo "------------------------------------"
            systemd-analyze blame ||
                print_not_available "systemd-analyze blame"

            echo
            echo "Critical boot chain"
            echo "-------------------"
            systemd-analyze critical-chain ||
                print_not_available "systemd-analyze critical-chain"
        else
            print_not_available "systemd-analyze"
        fi
    } > "$OUTPUT_DIR/boot.txt"
}

collect_processes() {
    {
        echo "HomeDNS Analytics - Process Baseline"
        echo "===================================="
        echo

        echo "Top 20 processes by CPU usage"
        echo "-----------------------------"

        ps -eo pid,user,%cpu,%mem,rss,vsz,etimes,comm,args \
            --sort=-%cpu |
            awk 'NR <= 21'

        echo
        echo "Top 20 processes by resident memory"
        echo "-----------------------------------"

        ps -eo pid,user,%cpu,%mem,rss,vsz,etimes,comm,args \
            --sort=-rss |
            awk 'NR <= 21'

        echo
        echo "RSS and VSZ values are expressed in KiB."
    } > "$OUTPUT_DIR/processes.txt"
}

validate_results() {
    local required_files=(
        "metadata.txt"
        "memory.txt"
        "system.txt"
        "storage.txt"
        "temperature.txt"
        "boot.txt"
        "processes.txt"
    )

    local file

    for file in "${required_files[@]}"; do
        if [[ ! -s "$OUTPUT_DIR/$file" ]]; then
            printf 'Error: output file is missing or empty: %s\n' \
                "$OUTPUT_DIR/$file" >&2
            return 1
        fi
    done
}

update_reference_links() {
    ln -sfn "runs/$TIMESTAMP" "$BASELINE_ROOT/latest"

    if [[ ! -e "$BASELINE_ROOT/reference" &&
        ! -L "$BASELINE_ROOT/reference" ]]; then

        ln -s "runs/$TIMESTAMP" "$BASELINE_ROOT/reference"
        echo "This run was selected as the initial reference baseline."
    else
        echo "The existing reference baseline was preserved."
    fi
}

main() {
    validate_environment
    validate_required_commands

    echo "Collecting HomeDNS baseline..."

    mkdir -p "$OUTPUT_DIR"

    echo "Output directory: $OUTPUT_DIR"

    collect_metadata
    collect_memory
    collect_system
    collect_storage
    collect_temperature
    collect_boot
    collect_processes

    validate_results
    update_reference_links

    echo "Baseline collection completed."
    echo "Results: $OUTPUT_DIR"
    echo "Reference: $BASELINE_ROOT/reference"
    echo "Latest: $BASELINE_ROOT/latest"
}

main "$@"