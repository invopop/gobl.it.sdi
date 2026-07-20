package fatturapa

import (
	"testing"

	"github.com/invopop/gobl/cbc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOtherDataToAttribute(t *testing.T) {
	t.Run("maps a text reference", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "LOTTO", TextReference: "ABC-123"})
		require.NotNil(t, a)
		assert.Equal(t, cbc.Code("LOTTO"), a.Type)
		assert.Equal(t, "ABC-123", a.Text)
	})

	t.Run("maps a numeric reference to an amount", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "PESO", NumReference: "12.50"})
		require.NotNil(t, a)
		require.NotNil(t, a.Amount)
		assert.Equal(t, "12.50", a.Amount.String())
		assert.Empty(t, a.Text)
	})

	t.Run("preserves the raw value when the numeric reference is invalid", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "PESO", NumReference: "not-a-number"})
		require.NotNil(t, a)
		assert.Nil(t, a.Amount)
		assert.Equal(t, "not-a-number", a.Text)
	})

	t.Run("maps a date reference to a date", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "SCADENZA", DateReference: "2024-03-15"})
		require.NotNil(t, a)
		require.NotNil(t, a.Date)
		assert.Equal(t, "2024-03-15", a.Date.String())
		assert.Empty(t, a.Text)
	})

	t.Run("preserves the raw value when the date reference is invalid", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "SCADENZA", DateReference: "31/12/2024"})
		require.NotNil(t, a)
		assert.Nil(t, a.Date)
		assert.Equal(t, "31/12/2024", a.Text)
	})

	t.Run("trims whitespace around the data type", func(t *testing.T) {
		a := otherDataToAttribute(&OtherData{DataType: "  LOTTO  ", TextReference: "ABC-123"})
		require.NotNil(t, a)
		assert.Equal(t, cbc.Code("LOTTO"), a.Type)
	})

	t.Run("returns nil when there is no reference value", func(t *testing.T) {
		assert.Nil(t, otherDataToAttribute(&OtherData{DataType: "LOTTO"}))
	})

	t.Run("returns nil for a blank or nil block", func(t *testing.T) {
		assert.Nil(t, otherDataToAttribute(&OtherData{DataType: "   ", TextReference: "x"}))
		assert.Nil(t, otherDataToAttribute(nil))
	})
}

func TestValidTipoDato(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain type", "LOTTO", "LOTTO"},
		{"empty", "", ""},
		{"too long", "TOOLONGTIPO", ""},
		{"exactly ten", "TIPODATO10", "TIPODATO10"},
		{"reserved INVCONT", tipoDatoINVCONT, ""},
		{"non basic latin", "PESÓ", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, validTipoDato(tt.in))
		})
	}
}
