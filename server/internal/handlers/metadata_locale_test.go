package handlers

import "testing"

func TestMetadataLocale(t *testing.T) {
	for _, c := range []struct{ locale, lang, region string }{
		{"en", "en", "US"},
		{"fr", "fr", "FR"},
		{"en-GB", "en", "GB"},
		{"pt-BR", "pt", "BR"},
		{"pt", "pt", "BR"},
		{"ja", "ja", "JP"},
		{"de", "de", "DE"},
		{"es-419", "es", "ES"},
		{"zh-Hant", "zh", "CN"},
		{"fil", "en", "US"}, // no ISO 639-1 code
	} {
		lang, region := metadataLocale(c.locale)
		if lang != c.lang || region != c.region {
			t.Errorf("metadataLocale(%q) = %s/%s, want %s/%s", c.locale, lang, region, c.lang, c.region)
		}
	}
}
