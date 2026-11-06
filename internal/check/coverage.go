package check

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ErrNoStatements reports a coverage profile with nothing measurable.
var ErrNoStatements = errors.New("coverage profile has no statements")

// CoveragePercent returns the covered statement percentage of a Go cover profile.
func CoveragePercent(r io.Reader) (float64, error) {
	scanner := bufio.NewScanner(r)
	var total, covered int64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "mode:") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, fmt.Errorf("malformed coverage line %q", line)
		}
		statements, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("malformed statement count in %q: %w", line, err)
		}
		count, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("malformed execution count in %q: %w", line, err)
		}
		if statements < 0 || count < 0 {
			return 0, fmt.Errorf("negative value in coverage line %q", line)
		}

		total += statements
		if count > 0 {
			covered += statements
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, err
	}
	if total == 0 {
		return 0, ErrNoStatements
	}
	return float64(covered) / float64(total) * 100, nil
}
