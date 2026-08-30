package gitops

import (
	"fmt"
	"strings"
)

const (
	AIAssistedByTrailer    = "AI-Assisted-By"
	GeneratedByTrailer     = "Generated-By"
	ReviewedByTrailer      = "Reviewed-By"
	AutomationActorTrailer = "Automation-Actor"
	chatGPT                = "OpenAI ChatGPT"
	routerctlSync          = "routerctl sync"
	routerOSBot            = "router-os-bot[bot]"
)

// TrailerOptions describes declared provenance for a commit created by routerctl.
// Empty fields are omitted; ReviewedBy must name a human when supplied.
type TrailerOptions struct {
	AIAssisted      bool
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
		if _, exists := trailers[key]; !exists {
			message = strings.TrimRight(message, "\n") + "\n\n" + key + ": " + value
			trailers[key] = []string{value}
		}
	}
	appendTrailer(GeneratedByTrailer, routerctlSync)
	if options.AIAssisted {
		appendTrailer(AIAssistedByTrailer, chatGPT)
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
	for _, key := range []string{AIAssistedByTrailer, GeneratedByTrailer, ReviewedByTrailer, AutomationActorTrailer} {
		values := trailers[key]
		if len(values) > 1 {
			return fmt.Errorf("%s must appear at most once", key)
		}
	}
	if value := first(trailers[AIAssistedByTrailer]); value != "" && value != chatGPT {
		return fmt.Errorf("%s must be %q", AIAssistedByTrailer, chatGPT)
	}
	if value := first(trailers[GeneratedByTrailer]); value != "" && value != routerctlSync {
		return fmt.Errorf("%s must be %q", GeneratedByTrailer, routerctlSync)
	}
	if value := first(trailers[AutomationActorTrailer]); value != "" && value != routerOSBot {
		return fmt.Errorf("%s must be %q", AutomationActorTrailer, routerOSBot)
	}
	if value := first(trailers[ReviewedByTrailer]); value == chatGPT || value == routerOSBot {
		return fmt.Errorf("%s must name a human, not %q", ReviewedByTrailer, value)
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
