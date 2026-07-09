package fatturapa

import "github.com/invopop/gobl/l10n"

var euCountries = []l10n.Code{
	l10n.AT, // Austria
	l10n.BE, // Belgium
	l10n.BG, // Bulgaria
	l10n.HR, // Croatia
	l10n.CY, // Cyprus
	l10n.CZ, // Czech Republic
	l10n.DK, // Denmark
	l10n.EE, // Estonia
	l10n.FI, // Finland
	l10n.FR, // France
	l10n.DE, // Germany
	l10n.EL, // Greece
	l10n.HU, // Hungary
	l10n.IE, // Ireland
	l10n.IT, // Italy
	l10n.LV, // Latvia
	l10n.LT, // Lithuania
	l10n.LU, // Luxembourg
	l10n.MT, // Malta
	l10n.NL, // Netherlands
	l10n.PL, // Poland
	l10n.PT, // Portugal
	l10n.RO, // Romania
	l10n.SK, // Slovakia
	l10n.SI, // Slovenia
	l10n.ES, // Spain
	l10n.SE, // Sweden
}

func isEUCountry(c l10n.Code) bool {
	for _, cc := range euCountries {
		if c == cc {
			return true
		}
	}
	return false
}
