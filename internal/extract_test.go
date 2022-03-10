package internal

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtract(t *testing.T) {
	tests := []struct {
		Input    string
		Expected []string
	}{
		{
			Input:    "",
			Expected: nil,
		},
		{
			Input:    "`#graphql { hello }`",
			Expected: []string{"#graphql { hello }"},
		},
		{
			Input: "\n\n`#graphql { hello { goodbye } } `   \n  `#graphql fragment Foo {\n  bar\n}`",
			Expected: []string{
				"#graphql { hello { goodbye } } ",
				"#graphql fragment Foo {\n  bar\n}",
			},
		},
	}
	for _, test := range tests {
		actual, err := FromString(test.Input)
		if assert.NoError(t, err) {
			assert.Equal(t, test.Expected, actual, "input: %s", test.Input)
		}
	}

	{
		_, err := FromString("`#graphql")
		assert.ErrorIs(t, err, io.ErrUnexpectedEOF)
	}
}
