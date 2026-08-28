package label

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type WordToken struct {
	Left       int
	Text       string
	Confidence float64
}

func ExtractCertificationNumbers(ctx context.Context, imgPath string, pattern *regexp.Regexp, minConf float64) ([]string, error) {
	return ExtractCertificationNumbersWithLang(ctx, imgPath, "eng", pattern, minConf)
}

func ExtractCertificationNumbersWithLang(ctx context.Context, imgPath string, lang string, pattern *regexp.Regexp, minConf float64) ([]string, error) {
	if lang == "" {
		lang = "eng"
	}
	cmd := exec.CommandContext(ctx, "tesseract", imgPath, "stdout", "--oem", "1", "-l", lang, "tsv")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("tesseract execution failed: %w: %s", err, stderr.String())
	}

	return ParseTSVAndExtractCandidates(&stdout, pattern, minConf)
}

func ParseTSVAndExtractCandidates(r io.Reader, pattern *regexp.Regexp, minConf float64) ([]string, error) {
	csvReader := csv.NewReader(r)
	csvReader.Comma = '\t'
	csvReader.LazyQuotes = true

	if _, err := csvReader.Read(); err != nil {
		return nil, fmt.Errorf("failed to read TSV header: %w", err)
	}

	lines := make(map[string][]WordToken)
	var lineOrder []string

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 12 {
			continue
		}

		// Full line key: page - block - par - line
		lineID := strings.Join([]string{
			record[1], // page_num
			record[2], // block_num
			record[3], // par_num
			record[4], // line_num
		}, "-")

		conf, err := strconv.ParseFloat(record[10], 64)
		if err != nil || conf < minConf {
			continue
		}

		text := strings.TrimSpace(record[11])
		if text == "" {
			continue
		}

		left, _ := strconv.Atoi(record[6])

		if _, exists := lines[lineID]; !exists {
			lineOrder = append(lineOrder, lineID)
		}

		lines[lineID] = append(lines[lineID], WordToken{
			Left:       left,
			Text:       text,
			Confidence: conf,
		})
	}

	var candidates []string
	seen := make(map[string]bool)

	for _, lineID := range lineOrder {
		words := lines[lineID]
		// Horizontal sorting by Left coordinate and concatenation
		sort.Slice(words, func(i, j int) bool {
			return words[i].Left < words[j].Left
		})

		var lineTextBuilder strings.Builder
		for _, w := range words {
			lineTextBuilder.WriteString(w.Text)
		}
		joinedLine := lineTextBuilder.String()

		matches := pattern.FindAllString(joinedLine, -1)
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				candidates = append(candidates, m)
			}
		}
	}

	return candidates, nil
}
