package privacy

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	emailRegex    = regexp.MustCompile(`(?i)\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`)
	phoneRegex    = regexp.MustCompile(`\b(?:\+?1[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}\b`)
	ssnRegex      = regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`)
	ipv4Regex     = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`)
	awsKeyRegex   = regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`)
	openAIKeyReg  = regexp.MustCompile(`\bsk-[a-zA-Z0-9_-]{24,}\b`)
	githubPatReg  = regexp.MustCompile(`\bghp_[a-zA-Z0-9]{36}\b`)
	jwtRegex      = regexp.MustCompile(`\bey[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`)
	cardCandRegex = regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`)

	medicalKeywords = []string{
		"diabetes", "insulin", "chemotherapy", "oncology", "hiv", "aids", "depression",
		"schizophrenia", "bipolar", "biopsy", "prescription", "cardiology", "pacemaker",
	}

	streetRegex = regexp.MustCompile(`(?i)\b\d{1,5}\s+(?:[A-Za-z0-9.]+\s+){1,3}(?:Street|St|Avenue|Ave|Boulevard|Blvd|Road|Rd|Drive|Dr|Lane|Ln|Court|Ct)\b`)
)

// Redactor performs automated scanning and masking of PII and confidential secrets.
type Redactor struct{}

// NewRedactor creates a new PII redactor.
func NewRedactor() *Redactor {
	return &Redactor{}
}

// Redact extracts all PII instances, replaces them with typed tokens, and returns entities.
func (r *Redactor) Redact(text string) (string, []RedactionEntity) {
	var entities []RedactionEntity
	sanitized := text

	// 1. AWS Keys
	for _, m := range awsKeyRegex.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIAWSCredential,
			Original:    m,
			Mask:        "[REDACTED:AWS_KEY]",
			Severity:    "CRITICAL",
			Description: "Amazon Web Services Access Key ID",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:AWS_KEY]")
	}

	// 2. OpenAI / Provider Keys
	for _, m := range openAIKeyReg.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIAPIToken,
			Original:    m,
			Mask:        "[REDACTED:API_KEY]",
			Severity:    "CRITICAL",
			Description: "Provider API Secret Key",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:API_KEY]")
	}

	// 3. GitHub PATs
	for _, m := range githubPatReg.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIAPIToken,
			Original:    m,
			Mask:        "[REDACTED:GITHUB_PAT]",
			Severity:    "CRITICAL",
			Description: "GitHub Personal Access Token",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:GITHUB_PAT]")
	}

	// 4. JWT Tokens
	for _, m := range jwtRegex.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIJWT,
			Original:    m,
			Mask:        "[REDACTED:JWT_TOKEN]",
			Severity:    "HIGH",
			Description: "JSON Web Token (Bearer Auth)",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:JWT_TOKEN]")
	}

	// 5. Credit Cards (with Luhn Algorithm verification)
	for _, cand := range cardCandRegex.FindAllString(sanitized, -1) {
		cleaned := strings.ReplaceAll(strings.ReplaceAll(cand, "-", ""), " ", "")
		if len(cleaned) >= 13 && len(cleaned) <= 19 && isLuhnValid(cleaned) {
			entities = append(entities, RedactionEntity{
				Category:    PIICreditCard,
				Original:    cand,
				Mask:        "[REDACTED:CREDIT_CARD]",
				Severity:    "CRITICAL",
				Description: fmt.Sprintf("Payment Card (%s)", detectCardBrand(cleaned)),
			})
			sanitized = strings.ReplaceAll(sanitized, cand, "[REDACTED:CREDIT_CARD]")
		}
	}

	// 6. SSN
	for _, m := range ssnRegex.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIISSN,
			Original:    m,
			Mask:        "[REDACTED:SSN]",
			Severity:    "CRITICAL",
			Description: "US Social Security Number",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:SSN]")
	}

	// 7. Emails
	for _, m := range emailRegex.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIEmail,
			Original:    m,
			Mask:        "[REDACTED:EMAIL]",
			Severity:    "HIGH",
			Description: "Personal / Corporate Email Address",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:EMAIL]")
	}

	// 8. Phone Numbers
	for _, m := range phoneRegex.FindAllString(sanitized, -1) {
		clean := strings.TrimSpace(m)
		// filter out simple numbers that aren't real phones
		digitsOnly := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(clean, "-", ""), ".", ""), " ", "")
		if len(digitsOnly) >= 10 && len(digitsOnly) <= 11 {
			entities = append(entities, RedactionEntity{
				Category:    PIIPhone,
				Original:    m,
				Mask:        "[REDACTED:PHONE]",
				Severity:    "HIGH",
				Description: "Direct Telephone Number",
			})
			sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:PHONE]")
		}
	}

	// 9. IPv4 Addresses
	for _, m := range ipv4Regex.FindAllString(sanitized, -1) {
		if m != "127.0.0.1" && m != "0.0.0.0" {
			entities = append(entities, RedactionEntity{
				Category:    PIIIPAddress,
				Original:    m,
				Mask:        "[REDACTED:IP_ADDR]",
				Severity:    "MEDIUM",
				Description: "Network IP Address",
			})
			sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:IP_ADDR]")
		}
	}

	// 10. Street Addresses
	for _, m := range streetRegex.FindAllString(sanitized, -1) {
		entities = append(entities, RedactionEntity{
			Category:    PIIGeoAddress,
			Original:    m,
			Mask:        "[REDACTED:STREET_ADDR]",
			Severity:    "HIGH",
			Description: "Physical Street Address",
		})
		sanitized = strings.ReplaceAll(sanitized, m, "[REDACTED:STREET_ADDR]")
	}

	// 11. Medical / HIPAA Keywords
	lower := strings.ToLower(sanitized)
	for _, kw := range medicalKeywords {
		if strings.Contains(lower, kw) {
			re := regexp.MustCompile(fmt.Sprintf(`(?i)\b%s\b`, kw))
			for _, m := range re.FindAllString(sanitized, -1) {
				entities = append(entities, RedactionEntity{
					Category:    PIIMedicalCondition,
					Original:    m,
					Mask:        "[REDACTED:HEALTH_INFO]",
					Severity:    "CRITICAL",
					Description: "Protected Health Information (HIPAA)",
				})
				sanitized = re.ReplaceAllString(sanitized, "[REDACTED:HEALTH_INFO]")
			}
		}
	}

	return sanitized, entities
}

// isLuhnValid checks standard Luhn mod-10 formula for credit card validation.
func isLuhnValid(number string) bool {
	sum := 0
	alternate := false
	for i := len(number) - 1; i >= 0; i-- {
		n, err := strconv.Atoi(string(number[i]))
		if err != nil {
			return false
		}
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}
	return sum%10 == 0
}

func detectCardBrand(num string) string {
	switch {
	case strings.HasPrefix(num, "4"):
		return "Visa"
	case strings.HasPrefix(num, "51") || strings.HasPrefix(num, "52") || strings.HasPrefix(num, "53") || strings.HasPrefix(num, "54") || strings.HasPrefix(num, "55"):
		return "MasterCard"
	case strings.HasPrefix(num, "34") || strings.HasPrefix(num, "37"):
		return "American Express"
	case strings.HasPrefix(num, "6011") || strings.HasPrefix(num, "65"):
		return "Discover"
	default:
		return "Credit/Debit Card"
	}
}
