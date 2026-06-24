package fatturapa

import (
	sdi "github.com/invopop/gobl.fatturapa/addon"
	"github.com/invopop/gobl/cbc"
	"github.com/invopop/gobl/l10n"
	"github.com/invopop/gobl/org"
	"github.com/invopop/gobl/regimes/it"
	"github.com/invopop/gobl/tax"
)

func goblOrgPartyFromSupplier(supplier *Supplier) *org.Party {
	if supplier == nil {
		return nil
	}

	party := new(org.Party)

	goblOrgPartyAddIdentity(party, supplier.Identity)
	goblOrgPartyAddRegistration(party, supplier.Registration)
	goblOrgPartyAddContact(party, supplier.Contact)

	if party.Addresses == nil {
		party.Addresses = []*org.Address{}
	}
	party.Addresses = append(party.Addresses, goblOrgAddressFromAddress(supplier.Address))

	return party
}

func goblOrgPartyFromCustomer(customer *Customer) *org.Party {
	if customer == nil {
		return nil
	}

	party := new(org.Party)

	goblOrgPartyAddIdentity(party, customer.Identity)

	if party.Addresses == nil {
		party.Addresses = []*org.Address{}
	}
	party.Addresses = append(party.Addresses, goblOrgAddressFromAddress(customer.Address))

	return party
}

func goblOrgPartyAddIdentity(party *org.Party, identity *Identity) {
	if party == nil || identity == nil {
		return
	}

	if party.TaxID == nil {
		party.TaxID = &tax.Identity{
			Country: l10n.TaxCountryCode(l10n.IT),
		}
	}

	if identity.TaxID != nil {
		party.TaxID.Country = l10n.TaxCountryCode(identity.TaxID.Country)
		if identity.TaxID.Code != "" && identity.TaxID.Code != "0000000" {
			party.TaxID.Code = cbc.Code(identity.TaxID.Code)
		}
	}

	if identity.FiscalCode != "" {
		if party.Identities == nil {
			party.Identities = []*org.Identity{}
		}
		party.Identities = append(party.Identities, &org.Identity{
			Key:  it.IdentityKeyFiscalCode,
			Code: cbc.Code(identity.FiscalCode),
		})
	}

	if identity.FiscalRegime != "" {
		party.Ext = party.Ext.Set(sdi.ExtKeyFiscalRegime, cbc.Code(identity.FiscalRegime))
	}

	if identity.Profile == nil {
		return
	}

	party.Name = identity.Profile.Name

	if identity.Profile.Given != "" {
		party.Name = identity.Profile.Given + " " + identity.Profile.Surname

		if party.People == nil {
			party.People = []*org.Person{}
		}

		party.People = append(party.People, &org.Person{
			Name: &org.Name{
				Given:   identity.Profile.Given,
				Surname: identity.Profile.Surname,
				Prefix:  identity.Profile.Title,
			},
		})
	}
}

func goblOrgPartyAddRegistration(party *org.Party, registration *Registration) {
	if party == nil || registration == nil {
		return
	}

	party.Registration = &org.Registration{
		Office: registration.Office,
		Entry:  registration.Entry,
	}

	if registration.Capital != "" {
		capital, err := parseAmount(registration.Capital)
		if err == nil {
			party.Registration.Capital = &capital
		}
	}
}

func goblOrgPartyAddContact(party *org.Party, contact *Contact) {
	if party == nil || contact == nil {
		return
	}

	if contact.Email != "" {
		if party.Emails == nil {
			party.Emails = []*org.Email{}
		}

		party.Emails = append(party.Emails, &org.Email{
			Address: contact.Email,
		})
	}

	if contact.Telephone != "" {
		if party.Telephones == nil {
			party.Telephones = []*org.Telephone{}
		}

		party.Telephones = append(party.Telephones, &org.Telephone{
			Number: contact.Telephone,
		})
	}
}
