# Changelog

All notable changes to **AgentAd (AAP)** will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [Unreleased]

### Added
- **Semantic Vector Matcher & Zero-PII Intent Matching Engine (`pkg/matcher`)**: Sub-millisecond vector indexing and retrieval engine performing real-time cosine similarity search across advertiser target inventories.
- **Zero-PII Cryptographic Intent Commitments**: Generates SHA-256 salted intent commitment hashes protecting raw user prompts from advertiser exfiltration while enabling verifiable audit trails.
- **Deterministic Token-Shingle Vector Projector**: Sub-millisecond zero-dependency embedding generation projecting freeform queries and advertiser inventory into 384-dimensional unit vectors.
- **CLI Subcommand & Visualizations (`agentad match`)**: Terminal ANSI scorecard formatting (`RenderMatchScorecard`) and Mermaid pipeline flowcharts (`RenderMatchMermaid`).
- EIP-712 structured data signing for EVM settlement receipts and Proof-of-Impression claims.
- Lightning L402 / HTTP 402 payment hashes with automated multi-party escrow clearing (70/20/10 split).
- Anti-jailbreak prompt injection guardrail scanner with delimiter stripping.
- Real-Time Bidding (RTB) Vickrey second-price auction engine.

---

## [v0.1.0] - 2026-10-16

### Added
- Initial specification for Agent Advertising Protocol (AAP).
- `/.well-known/agent-ads.json` crawler and parser.
- Real-Time Bidding (RTB) semantic auction engine.
- Model Context Protocol (MCP) tool injection proxy.
- Cryptographic attestation and proof-of-impression generator.
