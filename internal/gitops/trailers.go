package gitops

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	AIAssistedByTrailer    = "AI-Assisted-By"
	GeneratedByTrailer     = "Generated-By"
	ReviewedByTrailer      = "Reviewed-By"
	AutomationActorTrailer = "Automation-Actor"
	routerctlSync          = "routerctl sync"
	routerOSBot            = "router-os-bot[bot]"
	maxAIAssistedByLen     = 128
)

// TrailerOptions describes declared provenance for a commit created by routerctl.
// Empty fields are omitted. AIAssistedBy names the AI system(s) that materially
// assisted; ReviewedBy must name a human when supplied.
type TrailerOptions struct {
	// AIAssistedBy lists AI product names that materially assisted development.
	// Each entry becomes one AI-Assisted-By trailer. Values are free-form product
	// names (for example "Grok", "OpenAI ChatGPT", "Claude"); there is no vendor
	// whitelist.
	AIAssistedBy    []string
	ReviewedBy      string
	AutomationActor string
}

// CommitVerification is the policy decision for one commit.
type CommitVerification struct {
	ReviewRequired bool     `json:"review_required"`
	Errors         []string `json:"errors,omitempty"`
	Warnings       []string `json:"warnings,omitempty"`
}

func (v CommitVerification) OK() bool { return len(v.Errors) == 0 }

// AddSyncTrailers appends only policy-defined trailers to a generated sync
// commit. It deliberately never claims AI assistance or human review unless
// the caller supplies that fact.
func AddSyncTrailers(message string, options TrailerOptions) (string, error) {
	trailers := parseTrailers(message)
	if err := validateTrailerValues(trailers); err != nil {
		return "", err
	}
	if values := trailers[GeneratedByTrailer]; len(values) > 0 && values[0] != routerctlSync {
		return "", fmt.Errorf("%s must be %q", GeneratedByTrailer, routerctlSync)
	}
	appendTrailer := func(key, value string) {
		message = strings.TrimRight(message, "\n") + "\n\n" + key + ": " + value
		trailers[key] = append(trailers[key], value)
	}
	if _, exists := trailers[GeneratedByTrailer]; !exists {
		appendTrailer(GeneratedByTrailer, routerctlSync)
	}
	for _, name := range options.AIAssistedBy {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if err := validateAIAssistedByValue(name); err != nil {
			return "", err
		}
		appendTrailer(AIAssistedByTrailer, name)
	}
	if reviewed := strings.TrimSpace(options.ReviewedBy); reviewed != "" {
		appendTrailer(ReviewedByTrailer, reviewed)
	}
	if actor := strings.TrimSpace(options.AutomationActor); actor != "" {
		if actor != routerOSBot {
			return "", fmt.Errorf("%s must be %q", AutomationActorTrailer, routerOSBot)
		}
		appendTrailer(AutomationActorTrailer, actor)
	}
	if err := validateTrailerValues(parseTrailers(message)); err != nil {
		return "", err
	}
	return message, nil
}

// VerifyCommitMessage enforces the project trailer policy. Sensitive changes
// require a real human reviewer; a routerctl-generated sync without a bot
// actor remains valid but is reported for audit follow-up.
func VerifyCommitMessage(paths []string, message string) CommitVerification {
	trailers := parseTrailers(message)
	result := CommitVerification{ReviewRequired: reviewRequired(paths)}
	if err := validateTrailerValues(trailers); err != nil {
		result.Errors = append(result.Errors, err.Error())
	}
	if result.ReviewRequired && strings.TrimSpace(first(trailers[ReviewedByTrailer])) == "" {
		result.Errors = append(result.Errors, "Reviewed-By is required for verified promotion, regulatory value, device application, or public release changes")
	}
	if first(trailers[GeneratedByTrailer]) == routerctlSync && first(trailers[AutomationActorTrailer]) == "" {
		result.Warnings = append(result.Warnings, "Generated-By: routerctl sync has no Automation-Actor; record router-os-bot[bot] when automation performed the GitHub operation")
	}
	return result
}

func reviewRequired(paths []string) bool {
	for _, path := range paths {
		path = strings.TrimSpace(strings.ReplaceAll(path, "\\", "/"))
		switch {
		case strings.HasPrefix(path, "devices/"),
			strings.HasPrefix(path, "profiles/"),
			strings.HasPrefix(path, "examples/") && strings.Contains(path, "/regulatory/"),
			strings.HasPrefix(path, "schemas/certification-"),
			strings.HasPrefix(path, "internal/regulatory/") && (strings.Contains(path, "derive") || strings.Contains(path, "profile")),
			strings.HasPrefix(path, ".github/workflows/") && (strings.Contains(path, "release") || strings.Contains(path, "publish")):
			return true
		}
	}
	return false
}

func validateTrailerValues(trailers map[string][]string) error {
	// Generated-By, Reviewed-By, and Automation-Actor appear at most once.
	// AI-Assisted-By may appear multiple times (one trailer per AI system).
	for _, key := range []string{GeneratedByTrailer, ReviewedByTrailer, AutomationActorTrailer} {
		if len(trailers[key]) > 1 {
			return fmt.Errorf("%s must appear at most once", key)
		}
	}
	if value := first(trailers[GeneratedByTrailer]); value != "" && value != routerctlSync {
		return fmt.Errorf("%s must be %q", GeneratedByTrailer, routerctlSync)
	}
	if value := first(trailers[AutomationActorTrailer]); value != "" && value != routerOSBot {
		return fmt.Errorf("%s must be %q", AutomationActorTrailer, routerOSBot)
	}
	if value := first(trailers[ReviewedByTrailer]); value != "" {
		if value == routerOSBot {
			return fmt.Errorf("%s must name a human, not %q", ReviewedByTrailer, value)
		}
		for _, ai := range trailers[AIAssistedByTrailer] {
			if value == ai {
				return fmt.Errorf("%s must name a human, not an AI system (%q)", ReviewedByTrailer, value)
			}
		}
	}
	for _, value := range trailers[AIAssistedByTrailer] {
		if err := validateAIAssistedByValue(value); err != nil {
			return err
		}
	}
	return nil
}

// validateAIAssistedByValue accepts any non-empty AI product name. There is no
// vendor whitelist: the value identifies the system actually used.
func validateAIAssistedByValue(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s value must not be empty", AIAssistedByTrailer)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s value must not contain newlines", AIAssistedByTrailer)
	}
	if len(value) > maxAIAssistedByLen {
		return fmt.Errorf("%s value must be at most %d characters", AIAssistedByTrailer, maxAIAssistedByLen)
	}
	if value == routerOSBot {
		return fmt.Errorf("%s must name an AI system, not %q", AIAssistedByTrailer, value)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%s value must not contain control characters", AIAssistedByTrailer)
		}
	}
	return nil
}

func parseTrailers(message string) map[string][]string {
	known := map[string]bool{AIAssistedByTrailer: true, GeneratedByTrailer: true, ReviewedByTrailer: true, AutomationActorTrailer: true}
	trailers := make(map[string][]string)
	for _, line := range strings.Split(message, "\n") {
		key, value, ok := strings.Cut(line, ":")
		if ok && known[key] {
			trailers[key] = append(trailers[key], strings.TrimSpace(value))
		}
	}
	return trailers
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
