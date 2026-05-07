#!/usr/bin/env bash
set -euo pipefail

# ============================================================================
# Rinha de Backend 2023 Q3 - Stress Test Runner
# ============================================================================
# Downloads Gatling (if needed), fetches test resources, and runs the official
# stress test simulation against the local stack on port 9999.
# ============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

GATLING_VERSION="${GATLING_VERSION:-3.9.5}"
GATLING_HOME="${GATLING_HOME:-$HOME/gatling/$GATLING_VERSION}"
GATLING_BIN="$GATLING_HOME/bin/gatling.sh"
GATLING_URL="https://repo1.maven.org/maven2/io/gatling/highcharts/gatling-charts-highcharts-bundle/$GATLING_VERSION/gatling-charts-highcharts-bundle-$GATLING_VERSION-bundle.zip"

RESOURCES_DIR="$SCRIPT_DIR/user-files/resources"
RESULTS_DIR="$SCRIPT_DIR/user-files/results"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
ok()    { echo -e "${GREEN}[ OK ]${NC} $*"; }
err()   { echo -e "${RED}[FAIL]${NC} $*"; }

# ============================================================================
# Pre-flight checks
# ============================================================================

check_java() {
    if ! command -v java &>/dev/null; then
        err "Java not found. Gatling requires Java 11+."
        echo "  Install with: brew install openjdk@17"
        exit 1
    fi
    JAVA_VER=$(java -version 2>&1 | head -1 | awk -F '"' '{print $2}' | cut -d. -f1)
    if [[ "$JAVA_VER" -lt 11 ]]; then
        err "Java 11+ required (found: $JAVA_VER)"
        exit 1
    fi
    ok "Java $JAVA_VER found"
}

check_stack() {
    if ! curl -sf http://localhost:9999/contagem-pessoas >/dev/null 2>&1; then
        err "Stack not reachable at http://localhost:9999"
        echo "  Start it with: make up"
        exit 1
    fi
    ok "Stack is running on port 9999"
}

# ============================================================================
# Setup Gatling
# ============================================================================

install_gatling() {
    if [[ -x "$GATLING_BIN" ]]; then
        ok "Gatling $GATLING_VERSION already installed at $GATLING_HOME"
        return
    fi

    info "Downloading Gatling $GATLING_VERSION..."
    local tmp_zip="/tmp/gatling-$GATLING_VERSION.zip"
    curl -L --progress-bar "$GATLING_URL" -o "$tmp_zip"

    info "Extracting to $GATLING_HOME..."
    mkdir -p "$(dirname "$GATLING_HOME")"
    unzip -qo "$tmp_zip" -d /tmp/gatling-extract
    mv "/tmp/gatling-extract/gatling-charts-highcharts-bundle-$GATLING_VERSION" "$GATLING_HOME"
    rm -rf /tmp/gatling-extract "$tmp_zip"

    chmod +x "$GATLING_BIN"
    ok "Gatling installed at $GATLING_HOME"
}

# ============================================================================
# Download test resources (TSV files from official repo)
# ============================================================================

download_resources() {
    local base_url="https://raw.githubusercontent.com/zanfranceschi/rinha-de-backend-2023-q3/main/stress-test/user-files/resources"

    if [[ -f "$RESOURCES_DIR/pessoas-payloads.tsv" && -f "$RESOURCES_DIR/termos-busca.tsv" ]]; then
        ok "Test resources already present"
        return
    fi

    info "Downloading test resources (~23MB)..."
    mkdir -p "$RESOURCES_DIR"

    curl -L --progress-bar "$base_url/pessoas-payloads.tsv" -o "$RESOURCES_DIR/pessoas-payloads.tsv"
    curl -L --progress-bar "$base_url/termos-busca.tsv" -o "$RESOURCES_DIR/termos-busca.tsv"

    ok "Resources downloaded to $RESOURCES_DIR"
}

# ============================================================================
# Run stress test
# ============================================================================

run_test() {
    info "Starting Gatling stress test (~3 minutes)..."
    echo ""

    mkdir -p "$RESULTS_DIR"

    sh "$GATLING_BIN" -rm local -s RinhaBackendSimulation \
        -rd "rinha-go-$(date +%Y%m%d-%H%M%S)" \
        -rf "$RESULTS_DIR" \
        -sf "$SCRIPT_DIR/user-files/simulations" \
        -rsf "$RESOURCES_DIR"

    echo ""
    info "Waiting for async writes to flush..."
    sleep 5

    echo ""
    echo "============================================"
    PERSON_COUNT=$(curl -sf http://localhost:9999/contagem-pessoas)
    echo -e "  ${GREEN}Contagem de Pessoas: ${PERSON_COUNT}${NC}"
    echo "============================================"
    echo ""

    # Find and display the latest report path
    LATEST_REPORT=$(find "$RESULTS_DIR" -name "index.html" -type f | sort | tail -1)
    if [[ -n "$LATEST_REPORT" ]]; then
        ok "Report: $LATEST_REPORT"
        echo ""
        echo "  Open with: open $LATEST_REPORT"
    fi
}

# ============================================================================
# Main
# ============================================================================

main() {
    echo ""
    echo "╔══════════════════════════════════════════╗"
    echo "║   Rinha de Backend 1 - Stress Test   ║"
    echo "╚══════════════════════════════════════════╝"
    echo ""

    check_java
    check_stack
    install_gatling
    download_resources

    echo ""
    run_test
}

main "$@"
