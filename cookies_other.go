//go:build !darwin

package ghimg

import "errors"

func browserCandidates() ([]string, error) {
	return nil, errors.New("browser cookie reading is macOS only")
}
