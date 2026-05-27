# Context Backup Route Guidance

This file adds local rules for `frontend/src/app/context`.

This repo has no external users yet, so clean architecture and current best practices beat compatibility shims or speculative legacy paths.

Backup and restore payloads must stay gateway-shaped. Use `ContextBackupPayload`, `formatContextBackupPayload`, and `sanitizeContextBackupPayload` instead of ad hoc JSON transforms.

Never restore unsanitized text directly. Preserve validation, sensitive-key redaction, and clear success or error toasts for copy, download, import, and restore flows.
