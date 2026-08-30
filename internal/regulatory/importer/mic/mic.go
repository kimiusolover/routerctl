// Package mic imports a constrained, reviewable subset of MIC test reports.
package mic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/example/routerctl/internal/regulatory/importer"
	"github.com/example/routerctl/internal/regulatory/model"
)

var (
	certificationID = regexp.MustCompile(`(?im)(?:認証番号|Certification ID)\s*[:：]\s*([0-9]{3}-[A-Za-z0-9-]+)`)
	frequency       = regexp.MustCompile(`(?im)(\d{4})\s*MHz\s*(?:-|–|to)\s*(\d{4})\s*MHz`)
	bandwidth       = regexp.MustCompile(`(?im)Bandwidth\s*[:：]\s*([0-9 ,/]+)\s*MHz`)
	power           = regexp.MustCompile(`(?im)(Certified Maximum|Maximum) Conducted Output Power\s*[:：]\s*([0-9]+(?:\.[0-9]+)?)\s*dBm`)
	antenna         = regexp.MustCompile(`(?im)Antenna Gain\s*[:：]\s*([0-9]+(?:\.[0-9]+)?)\s*dBi`)
	page            = regexp.MustCompile(`(?im)(?:^|\n)\s*(?:Page|ページ)\s*([0-9]+)\b`)
	vendor          = regexp.MustCompile(`(?im)^[ \t]*Vendor\s*[:：]\s*(.+)$`)
	applicant       = regexp.MustCompile(`(?im)^[ \t]*Applicant\s*[:：]\s*(.+)$`)
	manufacturer    = regexp.MustCompile(`(?im)^[ \t]*Manufacturer\s*[:：]\s*(.+)$`)
	deviceModel     = regexp.MustCompile(`(?im)^[ \t]*Model\s*[:：]\s*(.+)$`)
	seriesModel     = regexp.MustCompile(`(?im)^[ \t]*Series Model\s*[:：]\s*(.+)$`)
	revision        = regexp.MustCompile(`(?im)^[ \t]*Revision\s*[:：]\s*(.+)$`)
)

type Importer struct{}

func (Importer) Detect(doc importer.Document) bool {
	return certificationID.MatchString(doc.Text) && (strings.Contains(doc.Text, "MIC") || strings.Contains(doc.Text, "技術基準適合"))
}

func (Importer) Extract(doc importer.Document) (*model.CertificationRecord, error) {
	id := certificationID.FindStringSubmatch(doc.Text)
	if len(id) < 2 {
		return nil, fmt.Errorf("mic: certification ID not found")
	}
	record := &model.CertificationRecord{
		APIVersion: model.APIVersion, Kind: "CertificationRecord", Jurisdiction: "JP",
		Certification: model.Certification{Authority: "MIC", Number: id[1], Source: model.Source{Document: doc.Name, SHA256: doc.SHA256}},
	}
	record.Device = observedDevice(doc.Text)
	radio := model.Radio{Band: "unknown", Evidence: model.Evidence{Document: doc.Name, Page: sourcePage(doc.Text)}, Confidence: "extracted"}
	if match := frequency.FindStringSubmatch(doc.Text); len(match) == 3 {
		radio.Frequency.MinMHz, _ = strconv.Atoi(match[1])
		radio.Frequency.MaxMHz, _ = strconv.Atoi(match[2])
		radio.Band = bandFor(radio.Frequency.MinMHz)
	}
	if match := bandwidth.FindStringSubmatch(doc.Text); len(match) == 2 {
		for _, token := range regexp.MustCompile(`\d+`).FindAllString(match[1], -1) {
			v, _ := strconv.Atoi(token)
			radio.BandwidthMHz = append(radio.BandwidthMHz, v)
		}
	}
	if match := power.FindStringSubmatch(doc.Text); len(match) == 3 {
		v, _ := strconv.ParseFloat(match[2], 64)
		// "Certified Maximum Conducted Output Power" remains a conducted
		// observation; it is not itself a firmware setting or a limit.
		radio.TXPower = &model.Value{Value: v, Unit: "dBm", Meaning: model.MeaningConductedPower}
		radio.Evidence.Table = "Conducted Output Power"
	}
	if match := antenna.FindStringSubmatch(doc.Text); len(match) == 2 {
		v, _ := strconv.ParseFloat(match[1], 64)
		radio.AntennaGain = &model.Value{Value: v, Unit: "dBi", Meaning: "measured"}
	}
	if radio.Frequency.MinMHz == 0 && radio.TXPower == nil {
		return nil, fmt.Errorf("mic: no RF observations found")
	}
	record.Radios = []model.Radio{radio}
	return record, nil
}

// ExtractWithNumber binds a human-reviewed authority reference from an
// evidence bundle to a report which does not repeat that number.
func (i Importer) ExtractWithNumber(doc importer.Document, number string, source model.NumberSource) (*model.CertificationRecord, error) {
	record, err := i.extractWithoutNumber(doc)
	if err != nil {
		return nil, err
	}
	record.Certification.Number, record.Certification.NumberSource = number, source
	return record, nil
}

func MatchesDevice(record *model.CertificationRecord, expected model.Device) error {
	if record == nil || !matchesAny(record.Device.Vendor, record.Device.VendorCandidates, expected.Vendor, sameVendor) || !matchesAny(record.Device.Model, record.Device.ModelCandidates, expected.Model, same) || (record.Device.Revision != "" && !same(record.Device.Revision, expected.Revision)) {
		return fmt.Errorf("mic: document device does not match evidence bundle")
	}
	return nil
}

// observedDevice keeps report labels as observations. Series Model is preferred
// over Test Model because the latter identifies the tested representative, not
// necessarily the marketed product. A blank revision remains explicitly unknown.
func observedDevice(text string) model.Device {
	vendors := captures(text, applicant, manufacturer, vendor)
	models := captures(text, seriesModel)
	if len(models) == 0 {
		models = captures(text, deviceModel)
	}
	d := model.Device{VendorCandidates: vendors, ModelCandidates: models, Revision: capture(revision, text)}
	if len(vendors) > 0 {
		d.Vendor = vendors[0]
	}
	if len(models) > 0 {
		d.Model = models[0]
	}
	return d
}

func captures(text string, patterns ...*regexp.Regexp) []string {
	var values []string
	for _, pattern := range patterns {
		if value := capture(pattern, text); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func matchesAny(primary string, candidates []string, expected string, compare func(string, string) bool) bool {
	if compare(primary, expected) {
		return true
	}
	for _, candidate := range candidates {
		if compare(candidate, expected) {
			return true
		}
	}
	return false
}

func same(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }
func sameVendor(a, b string) bool {
	return normalizedVendor(a) == normalizedVendor(b)
}

// normalizedVendor only removes explicitly allowed legal-entity suffixes.
// It deliberately does not use a substring match: a shared brand word is not
// enough to establish that two observed vendor names are the same entity.
func normalizedVendor(value string) string {
	words := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	legalSuffix := map[string]bool{
		"company": true, "co": true, "corporation": true, "corp": true,
		"inc": true, "incorporated": true, "limited": true, "ltd": true,
		"llc": true, "plc": true,
	}
	for len(words) > 0 && legalSuffix[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
}
func (Importer) extractWithoutNumber(doc importer.Document) (*model.CertificationRecord, error) {
	if !strings.Contains(doc.Text, "MIC") && !strings.Contains(doc.Text, "技術基準") {
		return nil, fmt.Errorf("mic: document does not look like an MIC report")
	}
	// Reuse normal extraction with a synthetic local marker; only the caller can
	// supply the number, and it must have separately verified bundle evidence.
	copy := doc
	copy.Text = "Certification ID: 000-bundle\n" + doc.Text
	return (Importer{}).Extract(copy)
}

func sourcePage(text string) int {
	m := page.FindAllStringSubmatch(text, -1)
	if len(m) == 0 {
		return 0
	}
	v, _ := strconv.Atoi(m[len(m)-1][1])
	return v
}
func bandFor(mhz int) string {
	if mhz >= 2400 && mhz < 2500 {
		return "2.4GHz"
	}
	if mhz >= 4900 && mhz < 6000 {
		return "5GHz"
	}
	if mhz >= 5925 && mhz < 7125 {
		return "6GHz"
	}
	return "unknown"
}

func capture(pattern *regexp.Regexp, text string) string {
	match := pattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return ""
	}
	return strings.TrimSpace(match[1])
}
