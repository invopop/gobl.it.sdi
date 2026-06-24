package fatturapa

import (
	"errors"

	sdi "github.com/invopop/gobl.fatturapa/addon"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
)

const (
	statoLiquidazioneDefault    = "LN"
	nonITCitizenTaxCodeDefault  = "0000000"
	nonEUBusinessTaxCodeDefault = "OO99999999999"
)

// Supplier describes the seller/provider of the invoice.
type Supplier struct {
	Identity               *Identity               `xml:"DatiAnagrafici"`
	Address                *Address                `xml:"Sede"`
	PermanentEstablishment *PermanentEstablishment `xml:"StabileOrganizzazione,omitempty"`
	Registration           *Registration           `xml:"IscrizioneREA,omitempty"`
	Contact                *Contact                `xml:"Contatti,omitempty"`
}

// Customer contains the details about who the invoice is addressed to.
type Customer struct {
	Identity *Identity `xml:"DatiAnagrafici"`
	Address  *Address  `xml:"Sede"`
}

// Identity (DatiAnagrafici) contains information related to an individual or company
type Identity struct {
	TaxID      *TaxID   `xml:"IdFiscaleIVA,omitempty"` // nolint:revive
	FiscalCode string   `xml:"CodiceFiscale,omitempty"`
	Profile    *Profile `xml:"Anagrafica"`
	// FiscaleRegime identifies the tax system to be applied
	// Has the form RFXX where XX is numeric; required only for the supplier
	FiscalRegime string `xml:"RegimeFiscale,omitempty"`
}

// TaxID is the VAT identification number consisting of a country code and the
// actual VAT number.
type TaxID struct {
	Country string `xml:"IdPaese"` // ISO 3166-1 alpha-2 country code
	Code    string `xml:"IdCodice"`
}

// PermanentEstablishment (StabileOrganizzazione) to be filled in if the seller/provider
// is not resident, but has a permanent establishment in Italy
type PermanentEstablishment struct {
	Street   string `xml:"Indirizzo"`
	Number   string `xml:"NumeroCivico,omitempty"`
	PostCode string `xml:"CAP"`
	Locality string `xml:"Comune"`
	Region   string `xml:"Provincia,omitempty"` // Province initials (2 characters) for IT country
	Country  string `xml:"Nazione"`             // Country code ISO alpha-2
}

// Profile contains identity data of the seller/provider
type Profile struct {
	// Name of the organization
	Name string `xml:"Denominazione,omitempty"`
	// Natural person's first or given name if no "Denominazione" is provided
	Given string `xml:"Nome,omitempty"`
	// Surname of the person
	Surname string `xml:"Cognome,omitempty"`
	// Title of the person
	Title string `xml:"Titolo,omitempty"`
	// EORI (Economic Operator Registration and Identification) code
	EORI string `xml:"CodEORI,omitempty"`
}

// Registration contains information related to the company registration details (REA)
type Registration struct {
	// Initials of the province where the company's Registry Office is located
	Office string `xml:"Ufficio,omitempty"`
	// Company's REA registration number
	Entry string `xml:"NumeroREA,omitempty"`
	// Company's share capital
	Capital string `xml:"CapitaleSociale,omitempty"`
	// Indication of whether the company has a sole shareholder.
	// Possible values: SU (sole shareholder), SM (multiple shareholders).
	SoleShareholder string `xml:"SocioUnico,omitempty"`
	// Indication of whether the Company is in liquidation or not.
	// Possible values: LS (in liquidation), LN (not in liquidation)
	LiquidationState string `xml:"StatoLiquidazione,omitempty"`
}

// Contact describes how the party can be contacted
type Contact struct {
	Telephone string `xml:"Telefono,omitempty"`
	Email     string `xml:"Email,omitempty"`
}

func newSupplier(s *org.Party) (*Supplier, error) {
	ns := &Supplier{
		Identity: &Identity{
			Profile: newProfile(s),
		},
		Registration: newRegistration(s),
		Contact:      newContact(s),
	}

	if s.TaxID != nil {
		ns.Identity.TaxID = partyTaxID(s.TaxID)
	}
	// Unlike the customer's, the supplier's IdFiscaleIVA is mandatory in the
	// FatturaPA schema, so fail early instead of generating XML that SDI
	// would reject.
	if ns.Identity.TaxID == nil {
		return nil, errors.New("supplier tax ID is required")
	}
	if id := org.IdentityForKey(s.Identities, it.IdentityKeyFiscalCode); id != nil {
		ns.Identity.FiscalCode = id.Code.String()
	}

	if v := s.Ext.Get(sdi.ExtKeyFiscalRegime); v != "" {
		ns.Identity.FiscalRegime = v.String()
	} else {
		ns.Identity.FiscalRegime = "RF01"
	}

	if len(s.Addresses) > 0 {
		ns.Address = newAddress(s.Addresses[0])
	}

	return ns, nil
}

func newCustomer(c *org.Party) *Customer {
	if c == nil {
		return nil
	}

	nc := new(Customer)
	if len(c.Addresses) > 0 {
		nc.Address = newAddress(c.Addresses[0])
	}

	da := &Identity{
		Profile: newProfile(c),
	}

	if c.TaxID != nil {
		da.TaxID = partyTaxID(c.TaxID)
	}
	if id := org.IdentityForKey(c.Identities, it.IdentityKeyFiscalCode); id != nil {
		da.FiscalCode = id.Code.String()
	}

	nc.Identity = da

	return nc
}

func newProfile(party *org.Party) *Profile {
	// A company may not have a tax ID if they have a codice fiscale
	// This means that we need to assume that it's a company if it has a name
	if party.Name != "" {
		return &Profile{
			Name: party.Name,
		}
	}
	// not a company
	if len(party.People) > 0 {
		name := party.People[0].Name
		return &Profile{
			Given:   name.Given,
			Surname: name.Surname,
			Title:   name.Prefix,
		}
	}

	return nil

}

func newContact(party *org.Party) *Contact {
	c := new(Contact)
	if len(party.Emails) > 0 {
		c.Email = party.Emails[0].Address
	}
	if len(party.Telephones) > 0 {
		c.Telephone = party.Telephones[0].Number
	}
	return c
}

// partyTaxID builds the FatturaPA tax ID for a party (supplier or customer),
// applying the SDI defaults for foreign parties: a non-IT party with no VAT
// number is treated as a private individual (0000000), and a non-EU party with
// a VAT number uses the generic non-EU placeholder (OO99999999999). An IT party
// with no code returns nil: an IT customer may identify itself with the
// CodiceFiscale instead, while an IT supplier must always provide a VAT number.
func partyTaxID(id *tax.Identity) *TaxID {
	code := id.Code.String()

	if code == "" {
		if id.Country.Code() == l10n.IT {
			return nil
		}
		// Assume private individual
		code = nonITCitizenTaxCodeDefault
	} else {
		// Must be a company with a local tax ID
		if !isEUCountry(id.Country.Code()) {
			code = nonEUBusinessTaxCodeDefault
		}
	}

	return &TaxID{
		Country: id.Country.String(),
		Code:    code,
	}
}

func newRegistration(supplier *org.Party) *Registration {
	if supplier.Registration == nil {
		return nil
	}

	capital := supplier.Registration.Capital
	var capitalFormatted string

	if capital == nil {
		capitalFormatted = ""
	} else {
		capitalFormatted = capital.Rescale(2).String()
	}

	liquidationState := supplier.Registration.Ext.Get(sdi.ExtKeyLiquidationState).String()
	if liquidationState == "" {
		liquidationState = statoLiquidazioneDefault
	}

	return &Registration{
		Office:           supplier.Registration.Office,
		Entry:            supplier.Registration.Entry,
		Capital:          capitalFormatted,
		SoleShareholder:  supplier.Registration.Ext.Get(sdi.ExtKeyShareholderState).String(),
		LiquidationState: liquidationState,
	}
}
