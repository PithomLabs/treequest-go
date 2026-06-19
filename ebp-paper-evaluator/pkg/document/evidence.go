package document

import "errors"

func VerifySpan(d DocumentBundle, s EvidenceSpan) error {
	if s.SourceHash != d.Hash {
		return errors.New("evidence source hash mismatch")
	}
	for _, section := range d.Sections {
		if section.ID == s.Section {
			if s.Start < 0 || s.End < s.Start || s.End > len(section.Text) {
				return errors.New("evidence offsets out of bounds")
			}
			if section.Text[s.Start:s.End] != s.Quote {
				return errors.New("evidence quote mismatch")
			}
			return nil
		}
	}
	return errors.New("evidence section not found")
}
