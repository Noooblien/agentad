package attest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// EIP712Domain defines the standard RFC/EIP-712 domain separator for smart contracts.
type EIP712Domain struct {
	Name              string `json:"name"`
	Version           string `json:"version"`
	ChainID           int64  `json:"chain_id"`
	VerifyingContract string `json:"verifying_contract"`
}

// ComputeEIP712DomainSeparator returns the 32-byte digest of the domain structure.
func ComputeEIP712DomainSeparator(domain EIP712Domain) string {
	raw := fmt.Sprintf("EIP712Domain(string name,string version,uint256 chainId,address verifyingContract):%s:%s:%d:%s",
		domain.Name, domain.Version, domain.ChainID, domain.VerifyingContract)
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// HashTypedReceipt produces an EVM-compatible digest for cryptographic settlement receipts.
func HashTypedReceipt(domainSeparator string, r *AttestationReceipt) string {
	payload := fmt.Sprintf("%s:%s:%s:%s:%.4f:%s",
		domainSeparator, r.ReceiptID, r.AuctionID, r.CampaignID, r.ClearingPriceCPM, r.Nonce)
	h := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(h[:])
}
