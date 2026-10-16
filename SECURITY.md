# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| v0.1.x  | :white_check_mark: |

---

## Reporting a Vulnerability

Prompt injection, adversarial payload hijacking, and financial settlement integrity are of paramount importance to the Agent Advertising Protocol.

If you identify a security flaw:
1. **Do not create a public issue.**
2. Report vulnerabilities privately to **`security@noooblien.com`**.
3. Include:
   - Detailed attack vector (e.g. prompt injection bypassing `pkg/guard`).
   - Sample sponsored payload / auction bid payload.
   - Impact assessment (e.g. instruction hijacking, double-spend in escrow).

---

## Adversarial Threat Model & Mitigations

- **Prompt Injection Defense (`pkg/guard`)**: All sponsored text undergoes regex and heuristic sanitization to disarm delimiters, markdown image exfiltration exploits, and system-prompt overrides.
- **Micro-Settlement Integrity (`pkg/escrow`, `pkg/attest`)**: Proof-of-Impression receipts are cryptographically hashed and signed (EIP-712 / Ed25519) before funds clear.
