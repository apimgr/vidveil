# Project Audit

Started: 2026-11-24

## Pass 1: Security
- [x] MCP: endpoint accepted requests without requiring a valid API Bearer token — fixed in `src/server/mcp/handler.go` (pending primary verification and commit).
- [x] MCP: browser Origin rejection used an inconsistent response envelope — fixed in `src/server/mcp/handler.go` (pending primary verification and commit).

## Pass 2: Code Quality
- [x] MCP: tool definitions omitted HTTP method safety annotations — fixed in `src/server/mcp/handler.go` and `src/server/mcp/handler_test.go` (pending primary verification and commit).

## Pass 3: Logic
- [x] MCP: required protocol metadata/header matching had missing-header gaps and legacy revision handling — fixed with validation ordering and regression cases in `src/server/mcp/handler.go` and `src/server/mcp/handler_test.go` (pending primary verification and commit).
- [x] MCP: tool listing omitted operator-only API operations and could expose cookie-bound preferences or MCP management operations — fixed filters and operator visibility checks in `src/server/mcp/handler.go` and `src/server/mcp/handler_test.go` (pending primary verification and commit).
- [x] Restore: public backup restore route/page contradicted the CLI-only restore requirement — removed public route, handler, templates, tests, restart wiring, and references while preserving maintenance CLI restore (pending primary verification and commit; generated OpenAPI artifact remains with swagger owner).

## Pass 4: Documentation
- [x] MCP help and integration docs described anonymous access despite required Bearer authentication — corrected help template, admin/API/integration docs, and localized help copy (pending primary verification and commit).
- [x] MCP integration guidance recommended sharing the primary owner token — changed it to recommend a separate linked owner token per client and reserve the operator token for operator-only tools (pending primary verification and commit).

## Pass 5: Spec Compliance
- [x] MCP: endpoint contract did not enforce authenticated, stateless protocol requests and route exclusions — fixed implementation and coverage as above (pending primary verification and commit).
- [x] Restore: server UI/API exposed destructive restore despite AI.md requiring host CLI-only restore — fixed implementation and project docs as above (pending primary verification and commit).

## Pass 6: Code Flow Trace
- [x] MCP: tool call path now retains Bearer identity while dispatching through the API router and checks operator visibility before registering tools — fixed in `src/server/mcp/handler.go` (pending primary verification and commit).

## Completed
- MCP contract and restore-route alignment changes are implemented in the shared working tree; the primary agent must run the project verification pass, then remove this tracker after the findings are committed.
