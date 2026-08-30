package profile

import "fmt"

func Validate(p *Profile) error {
	if p == nil || p.APIVersion != APIVersion || p.Kind != "CertificationProfile" {
		return fmt.Errorf("certification profile: unsupported profile")
	}
	if p.Metadata.ID == "" || p.Subject.Jurisdiction == "" || p.Subject.Authority == "" || p.Subject.Certification.Scheme == "" || p.Subject.Certification.Number == "" {
		return fmt.Errorf("certification profile: identity and certification are required")
	}
	if p.Subject.Hardware.Manufacturer == "" || p.Subject.Hardware.Model == "" || p.Subject.Hardware.Revision == "" || !p.Subject.Hardware.CalibrationRequired {
		return fmt.Errorf("certification profile: complete hardware identity and required calibration are required")
	}
	if p.Constraints.Enforcement.Default != "deny" {
		return fmt.Errorf("certification profile: default enforcement must be deny")
	}
	if len(p.Constraints.Spectrum) == 0 {
		return fmt.Errorf("certification profile: no spectrum constraints")
	}
	for id, b := range p.Constraints.Spectrum {
		if len(b.LegalRangeMHz) != 2 || b.LegalRangeMHz[0] >= b.LegalRangeMHz[1] || len(b.PrimaryChannels) == 0 || len(b.WidthsMHz) == 0 {
			return fmt.Errorf("certification profile: band %s has incomplete spectrum constraints", id)
		}
		for _, width := range b.WidthsMHz {
			if b.Power.Regulatory[width] <= 0 || b.Power.CertifiedRated[width] <= 0 {
				return fmt.Errorf("certification profile: band %s width %d lacks regulatory or certified rated power", id, width)
			}
		}
		if b.DFSRequired && p.Constraints.DFS.BypassAllowed {
			return fmt.Errorf("certification profile: DFS bypass is forbidden")
		}
	}
	if p.Constraints.DFS.CACMinSeconds < 60 || p.Constraints.DFS.ChannelMoveMaxSeconds <= 0 || p.Constraints.DFS.NonOccupancyMinSeconds < 1800 {
		return fmt.Errorf("certification profile: incomplete DFS protection")
	}
	return nil
}
