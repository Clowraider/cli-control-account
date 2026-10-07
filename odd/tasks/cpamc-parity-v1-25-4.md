# Feature: CPAMC Upstream v1.25.4 Quota Parity Synchronization

## Objective
Sincronizar la lógica de cuotas y metadatos del plugin `cli-control-account` con las especificaciones de la fuente de verdad upstream CPAMC v1.25.4 (commit 6abace9).

## Problem & Why
La auditoría de la sesión anterior identificó divergencias clave entre el plugin cliente (`index.html`) y upstream CPAMC:
1. Claude: `parseClaudePlan` evaluaba flags individuales antes de `organization_type == 'claude_team'`, causando falsas clasificaciones para usuarios de equipos. Además faltaba el header `User-Agent: claude-cli/2.1.280 (external, cli)`.
2. Kimi: Hardcodeado a `api.kimi.com` en lugar de resolver dinámicamente entre `.com` y `.ai`. No procesaba contadores relativos (`reset_in`, `resetIn`, `ttl`).
3. Codex: No consultaba `CODEX_SUBSCRIPTION_URL` en vivo para `active_until` ni extraía el balance de créditos (`payload.credits`).
4. xAI: No enriquecía el plan/tier con `/v1/user` y `/v1/settings`.
5. Test harness: `antigravity_subscription_test.js` requería `AbortController` en el sandbox VM.

## Scope & Constraints
- Archivos afectados:
  - `internal/web/assets/index.html` (lógica SPA y renderizado)
  - `internal/web/testdata/antigravity_subscription_test.js` (fix del entorno VM)
  - `internal/web/testdata/quota_sync_test.js` (tests unitarios JS)
  - `internal/web/embed_test.go` (tests de integración Go)
- Mantener compatibilidad total y arquitectura client-side SPA.
- Respetar TDD y verificar cada cambio con pruebas en Node y Go.

## Tasks Checklist
- [x] TASK-1: Corregir sandbox de test en `antigravity_subscription_test.js` (`AbortController`)
- [x] TASK-2: Sincronizar paridad Claude: precedencia `claude_team` en `parseClaudePlan` y `User-Agent` en `fetchClaudeQuota`
- [x] TASK-3: Sincronizar paridad Kimi: resolución dinámica de dominio (`.com` vs `.ai`) y soporte de contadores relativos (`reset_in`, `ttl`)
- [x] TASK-4: Sincronizar paridad Codex: consulta en vivo de `CODEX_SUBSCRIPTION_URL` para `active_until` y soporte de `payload.credits` (balance / unlimited)
- [x] TASK-5: Sincronizar paridad xAI: enriquecimiento de suscripción con `XAI_USER_URL` y `XAI_SETTINGS_URL` (`resolveXaiSubscriptionPlan`)
- [x] TASK-6: Ejecutar y validar suites completas de pruebas (`go test ./...` y `node --test`)

## Verification Evidence
- Node tests: `node --test internal/web/testdata/quota_sync_test.js internal/web/testdata/antigravity_subscription_test.js`
- Go tests: `go test -v ./internal/web/...` y `go test ./...`
