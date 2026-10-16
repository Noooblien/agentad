# AGENT.md: AgentAd (AAP) Autonomous Engineering Guide

Welcome, AI Agent. This specification governs how autonomous agents and developers build, maintain, and extend the **Agent Advertising Protocol (AAP)**.

AgentAd is the open-source monetization and sponsored discovery protocol for AI-to-AI commerce, autonomous real-time bidding (RTB), zero-PII intent matching, and prompt-injection-safe tool injection.

---

## 1. Core Operating Principles

1. **Adversarial Safety First (Zero-Jailbreak Mandate)**:
   - Sponsored tool recommendations and payload descriptions are strictly untrusted data.
   - All payloads must pass through `pkg/guard` (anti-jailbreak heuristic scanner, prompt-injection sanitizers, control-character stripping) before being exposed to LLM context or tool catalogs.
   - Advertisers can never override agent system instructions, exfiltrate user context, or execute unauthorized transactions.

2. **Zero-PII Intent Matching**:
   - Agent context is never shared in plaintext with bidding brokers.
   - Semantic auctions use zero-PII embedding vectors (`pkg/auction`, `pkg/matcher`) and anonymous cryptographic intent hashes.

3. **Cryptographic Micro-Settlement**:
   - Actions and impressions are attested with cryptographic proofs (HMAC-SHA256, Ed25519, and EIP-712 structured data signing in `pkg/attest` and `pkg/escrow`).
   - Supports Lightning L402 / HTTP 402 payment hashes and automated revenue splits (e.g. 70% publisher / 20% host / 10% protocol).

4. **Strict Branch Protection & Git Hygiene**:
   - **`main` is protected.** Never attempt direct pushes to `main`.
   - Always branch: `feat/<name>`, `fix/<name>`, or `docs/<name>`.
   - Create a Pull Request via GitHub CLI (`gh pr create`), verify test matrix, and squash-merge (`gh pr merge <num> --squash --delete-branch`).

5. **Test Integrity**:
   - Verify that all unit and integration tests pass cleanly with `go test -v ./...`.

---

## 2. Subsystem Architecture

```
agentad/
├── pkg/
│   ├── attest/        # Proof-of-Impression & EIP-712 settlement receipts
│   ├── auction/       # Vickrey second-price & generalized first-price RTB auctions
│   ├── escrow/        # Micro-settlement ledger, L402 payment hashes, revenue splitting
│   ├── guard/         # Anti-jailbreak scanner, prompt-injection & SSRF sanitizer
│   ├── matcher/       # Low-latency cosine similarity & semantic intent matcher
│   ├── mcp/           # MCP proxy injecting sponsored tools dynamically into agent catalogs
│   ├── protocol/      # Canonical AAP data structures & /.well-known/agent-ads.json parser
│   └── tracker/       # Conversion attribution, budget caps, and fraud detection
├── tests/             # End-to-end integration and unit tests
├── .github/           # CI matrix workflows and PR templates
├── AGENT.md           # This autonomous engineer specification
├── CONTRIBUTING.md    # Contribution guidelines
├── SECURITY.md        # Vulnerability disclosure policy
├── CHANGELOG.md       # Semantic versioning release log
└── README.md          # Project overview and documentation
```

---

## 3. Essential Commands

| Task | Command |
|---|---|
| **Run all tests** | `go test -v ./...` |
| **Run specific test** | `go test -v ./tests -run Test<Name>` |
| **Run benchmarks** | `go test -bench=. ./tests` |
| **Create PR** | `gh pr create --base main --head <branch> --title "..." --body "..."` |
| **Squash & Merge** | `gh pr merge <pr_number> --squash --delete-branch` |
