# Archived Documentation

This folder contains historical documentation from ProxyHawk's development process.

## Purpose

These documents represent:
- **Proposals** that have since been implemented
- **Implementation notes** from completed features
- **Analysis documents** from before features were finalized

They are kept for historical reference and to understand the evolution of the project.

---

## Archived Documents

### Feature Proposals (Now Implemented)

**CHECK_MODES_PROPOSAL.md**
- **Status:** ✅ Fully Implemented
- **Date:** Pre-2026-02-09
- **Description:** Proposal for 3-tier check mode system (basic/intense/vulns)
- **Current Status:** All three modes implemented in [cmd/proxyhawk/main.go:293-327](../../cmd/proxyhawk/main.go#L293-L327)
- **See Also:** [IMPLEMENTATION_STATUS.md](../IMPLEMENTATION_STATUS.md)

### TUI Implementation Documents (Completed)

**TUI_COMPLETE.md**
- **Status:** ✅ Implementation Complete
- **Description:** Terminal UI implementation completion notes

**TUI_REFACTORING_COMPLETE.md**
- **Status:** ✅ Refactoring Complete
- **Description:** Component-based architecture refactoring completion

**TUI_REDESIGN_SUMMARY.md**
- **Status:** ✅ Redesign Complete
- **Description:** Summary of TUI redesign decisions and implementation

**TUI_IMPROVEMENTS_APPLIED.md**
- **Status:** ✅ Improvements Complete
- **Description:** Applied improvements to TUI system

**TUI_REVIEW_AND_RECOMMENDATIONS.md**
- **Status:** ✅ Reviews Addressed
- **Description:** TUI review feedback and recommendations (applied)

**RUN_NEW_TUI.md**
- **Status:** ✅ Completed
- **Description:** Instructions for running new TUI (now the default)

### Security Implementation (Completed)

**SECURITY_IMPROVEMENTS_COMPLETE.md**
- **Status:** ✅ Implementation Complete
- **Description:** Security features implementation completion notes
- **Features:** SSRF testing (60+ targets), Header injection (62+ vectors), Protocol smuggling, DNS rebinding, Cache poisoning, Enhanced anonymity detection

---

## Current Documentation

For up-to-date documentation, see:

### Main Documentation
- [README.md](../../README.md) - Main project documentation
- [CLAUDE.md](../../CLAUDE.md) - Development guidance for Claude Code
- [IMPLEMENTATION_STATUS.md](../IMPLEMENTATION_STATUS.md) - Current feature implementation status

### Active Documentation (`/docs/`)
- [PROJECT_STRUCTURE.md](../PROJECT_STRUCTURE.md) - Codebase organization
- [PROXY_CHECKING_FLOW.md](../PROXY_CHECKING_FLOW.md) - How proxy checking works (⚠️ needs update)
- [PROXY_SECURITY_ANALYSIS.md](../PROXY_SECURITY_ANALYSIS.md) - Security features analysis
- [REGIONAL_TESTING.md](../REGIONAL_TESTING.md) - Regional proxy testing guide
- [INTEGRATION_GUIDE.md](../INTEGRATION_GUIDE.md) - Integration instructions

### Guide Documentation (`/docs/guides/`)
- [CONFIGURATION.md](../guides/CONFIGURATION.md) - Configuration file guide
- [CLI_EXAMPLES.md](../guides/CLI_EXAMPLES.md) - Command-line usage examples

---

## Note on Historical Context

When reviewing these archived documents:

1. **Proposals are now reality** - Features described as "proposed" are now implemented
2. **Code references may be outdated** - File paths and line numbers may have changed
3. **Best practices evolved** - Current implementation may differ from original proposals
4. **Architecture improved** - Implementation details refined during development

For current implementation details, always refer to:
- The actual source code
- [IMPLEMENTATION_STATUS.md](../IMPLEMENTATION_STATUS.md)
- Active documentation in `/docs/` (not `/docs/archive/`)

---

Last Updated: 2026-02-09
