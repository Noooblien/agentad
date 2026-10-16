# Contributing to AgentAd Protocol (AAP)

Thank you for contributing to **AgentAd Protocol (AAP)**, the zero-jailbreak, machine-to-machine sponsored discovery and monetization standard for autonomous AI agents.

---

## Architecture Principles

1. **Zero Unstructured Text in Payloads:** All ad content must follow `application/vnd.agentad.factsheet+json` scalar types (numbers, enum tags, verified benchmark metrics) to prevent prompt injection.
2. **Sub-50ms RTB SLA:** Auction clearing and quality scoring algorithms must execute deterministically under strict latency budgets.
3. **Cryptographic Attestation:** Every impression or downstream action must yield an Ed25519 or EIP-712 nonced receipt for auditable clearing.

---

## Local Development & Testing

```bash
git clone https://github.com/Noooblien/agentad.git
cd agentad
go build -o bin/agentad ./cmd/agentad
go test -v -race ./...
```

---

## Pull Request Guidelines

1. Branch from `main` using `feat/` or `fix/`.
2. Ensure `main` branch protection rules are respected (direct pushes are blocked).
3. Open a Pull Request referencing the related issue.
