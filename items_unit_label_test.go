package fatturapa_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/invopop/gobl.it.sdi/test"
	"github.com/invopop/gobl/bill"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/org"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// UnitaMisura is free text, so a unit GOBL has no name for is kept as the
// supplier wrote it, under the same item meta key gobl.pl.ksef uses.
func TestUnitLabel(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(test.GetDataPath(test.PathFatturaPAGOBL), "invoice-windows1252.xml"))
	require.NoError(t, err)
	data = bytes.Replace(data, []byte("<UnitaMisura>PZ</UnitaMisura>"), []byte("<UnitaMisura> CONF </UnitaMisura>"), 1)

	env, err := test.ConvertToGOBL(data)
	require.NoError(t, err)
	inv, ok := env.Extract().(*bill.Invoice)
	require.True(t, ok)

	t.Run("keeps a unit GOBL has no name for as the unit label", func(t *testing.T) {
		item := inv.Lines[0].Item
		assert.Empty(t, item.Unit)
		assert.Equal(t, "CONF", item.Meta["unit-label"])
		assert.Empty(t, item.Description)
	})

	t.Run("maps the Italian units around it", func(t *testing.T) {
		item := inv.Lines[1].Item
		assert.Equal(t, cbc.Key("piece"), item.Unit)
		assert.Empty(t, item.Meta)
	})

	t.Run("writes the unit label back as UnitaMisura", func(t *testing.T) {
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		assert.Equal(t, "CONF", doc.Body[0].GoodsServices.LineDetails[0].Unit)
		assert.Equal(t, "piece", doc.Body[0].GoodsServices.LineDetails[1].Unit)
	})
}

func TestUnitExport(t *testing.T) {
	t.Run("writes a unit GOBL keeps as a UN/ECE code", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Unit = "PZ"
		})
		// GOBL moves a unit written as a code into the untdid-unit extension.
		require.NoError(t, env.Calculate())
		inv, ok := env.Extract().(*bill.Invoice)
		require.True(t, ok)
		require.Empty(t, inv.Lines[0].Item.Unit)

		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		assert.Equal(t, "PZ", doc.Body[0].GoodsServices.LineDetails[0].Unit)
	})

	t.Run("prefers the supplier's label over the unit", func(t *testing.T) {
		env := test.LoadTestFile("invoice-simple.json", test.PathGOBLFatturaPA)
		test.ModifyInvoice(env, func(inv *bill.Invoice) {
			inv.Lines[0].Item.Unit = "piece"
			inv.Lines[0].Item.Meta = cbc.Meta{"unit-label": "CONF"}
		})
		doc, err := test.ConvertFromGOBL(env)
		require.NoError(t, err)
		assert.Equal(t, "CONF", doc.Body[0].GoodsServices.LineDetails[0].Unit)
	})
}

// Every GOBL unit is written as UnitaMisura, so each key must fit its
// String10Type: 1 to 10 Basic Latin characters.
func TestUnitKeysFitUnitaMisura(t *testing.T) {
	string10 := regexp.MustCompile(`^[[:ascii:]]{1,10}$`)
	for _, def := range org.UnitDefinitions {
		assert.Regexp(t, string10, def.Key.String())
	}
}
