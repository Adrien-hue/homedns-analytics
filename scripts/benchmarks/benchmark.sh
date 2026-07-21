#!/usr/bin/env bash

set -Eeuo pipefail

# Central entry point for the HomeDNS Analytics benchmark suite.
# Individual benchmark scripts are selected using the first argument.

readonly SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"

print_usage() {
    cat <<'EOF'
Usage:
  benchmark.sh baseline
  benchmark.sh full
  benchmark.sh help

Commands:
  baseline    Collect the initial Raspberry Pi system baseline.
  full        Run the complete benchmark suite currently available.
  help        Display this help message.
EOF
}

run_baseline() {
    echo "Running HomeDNS baseline benchmark..."
    "$SCRIPT_DIR/collect-baseline.sh"
}

run_full_suite() {
    echo "Running the complete HomeDNS benchmark suite..."

    # The baseline collector is currently the only available benchmark.
    # Additional benchmark scripts will be added here progressively.
    run_baseline

    echo "Complete benchmark suite finished."
}

main() {
    local command="${1:-full}"

    case "$command" in
        baseline)
            run_baseline
            ;;

        full)
            run_full_suite
            ;;

        help | --help | -h)
            print_usage
            ;;

        *)
            printf 'Error: unknown benchmark command: %s\n\n' "$command" >&2
            print_usage >&2
            exit 2
            ;;
    esac
}

main "$@"