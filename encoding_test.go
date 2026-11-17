package fatturapa

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToUTF8(t *testing.T) {
	t.Run("UTF-8 document passes through unchanged", func(t *testing.T) {
		input := []byte(`<?xml version="1.0" encoding="UTF-8"?><root>Test</root>`)
		output, err := convertToUTF8(input)
		require.NoError(t, err)
		assert.Equal(t, input, output)
	})

	t.Run("No encoding declaration defaults to UTF-8", func(t *testing.T) {
		input := []byte(`<?xml version="1.0"?><root>Test</root>`)
		output, err := convertToUTF8(input)
		require.NoError(t, err)
		assert.Equal(t, input, output)
	})

	t.Run("Windows-1252 document is converted to UTF-8", func(t *testing.T) {
		// Create a windows-1252 encoded document with special character (à = 0xE0 in windows-1252)
		// In UTF-8, à is encoded as 0xC3 0xA0
		input := []byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?><root>Qt\xE0</root>")
		output, err := convertToUTF8(input)
		require.NoError(t, err)

		// Check that encoding declaration was updated
		assert.Contains(t, string(output), "encoding=\"UTF-8\"")

		// Check that the special character was properly converted to UTF-8
		// à in UTF-8 is 0xC3 0xA0
		assert.Contains(t, string(output), "Qt\xC3\xA0")
	})

	t.Run("Unsupported encoding returns error", func(t *testing.T) {
		input := []byte(`<?xml version="1.0" encoding="iso-8859-5"?><root>Test</root>`)
		_, err := convertToUTF8(input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unsupported encoding")
	})
}

func TestConvertLatinToUTF8(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		body     string
		expected string
	}{
		{
			name:     "ISO-8859-1",
			label:    "ISO-8859-1",
			body:     "Qt\xE0",
			expected: "Qtà",
		},
		{
			name:     "lowercase iso-8859-1",
			label:    "iso-8859-1",
			body:     "Qt\xE0",
			expected: "Qtà",
		},
		{
			name:     "ISO-8859-1 reads 0x80 as the windows-1252 euro sign",
			label:    "ISO-8859-1",
			body:     "10 \x80",
			expected: "10 €",
		},
		{
			name:     "ISO-8859-15 reads 0xA4 as the euro sign",
			label:    "ISO-8859-15",
			body:     "10 \xA4",
			expected: "10 €",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := []byte(`<?xml version="1.0" encoding="` + tt.label + `"?><root>` + tt.body + `</root>`)
			output, err := convertToUTF8(input)
			require.NoError(t, err)
			assert.Equal(t, `<?xml version="1.0" encoding="UTF-8"?><root>`+tt.expected+`</root>`, string(output))
		})
	}
}

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "UTF-8 encoding",
			input:    `<?xml version="1.0" encoding="UTF-8"?>`,
			expected: "UTF-8",
		},
		{
			name:     "windows-1252 encoding",
			input:    `<?xml version="1.0" encoding="windows-1252"?>`,
			expected: "windows-1252",
		},
		{
			name:     "Single quotes",
			input:    `<?xml version='1.0' encoding='windows-1252'?>`,
			expected: "windows-1252",
		},
		{
			name:     "No encoding attribute",
			input:    `<?xml version="1.0"?>`,
			expected: "",
		},
		{
			name:     "Case insensitive",
			input:    `<?xml version="1.0" encoding="Windows-1252"?>`,
			expected: "Windows-1252",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectEncoding([]byte(tt.input))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReplaceEncodingDeclaration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		newEnc   string
		expected string
	}{
		{
			name:     "Replace windows-1252 with UTF-8",
			input:    `<?xml version="1.0" encoding="windows-1252"?>`,
			newEnc:   "UTF-8",
			expected: `<?xml version="1.0" encoding="UTF-8"?>`,
		},
		{
			name:     "Replace with single quotes",
			input:    `<?xml version='1.0' encoding='windows-1252'?>`,
			newEnc:   "UTF-8",
			expected: `<?xml version='1.0' encoding='UTF-8'?>`,
		},
		{
			name:     "Full XML declaration",
			input:    `<?xml version="1.0" encoding="windows-1252" standalone="yes"?>`,
			newEnc:   "UTF-8",
			expected: `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceEncodingDeclaration([]byte(tt.input), tt.newEnc)
			assert.Equal(t, tt.expected, string(result))
		})
	}
}
