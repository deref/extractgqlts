package internal

import (
	"io"
	"regexp"
	"unicode/utf8"
)

func FromString(s string) ([]string, error) {
	return FromBytes([]byte(s))
}

var startRE = regexp.MustCompile("\\`\\#graphql")

func FromBytes(bs []byte) ([]string, error) {
	var res []string
scan:
	for len(bs) > 0 {
		found := startRE.FindIndex(bs)
		if found == nil {
			break
		}
		bs = bs[found[0]+1:]

		// Scan until the end of the string.
		// TODO: Handle nested string templates, etc.
		i := 0
		for i < len(bs) {
			r, size := utf8.DecodeRune(bs[i:])
			i += size
			if r == '`' {
				res = append(res, string(bs[:i-size]))
				bs = bs[i:]
				continue scan
			}
		}

		return nil, io.ErrUnexpectedEOF
	}
	return res, nil
}
