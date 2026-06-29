package i18n

import (
	"testing"
)

func TestQueueDescriptionTranslations(t *testing.T) {
	i18n := GetInstance()

	// Test queue description translations
	tests := []struct {
		name     string
		lang     string
		key      string
		expected string
	}{
		{
			name:     "English default queue description",
			lang:     "en",
			key:      "queue_descriptions.All new tickets are placed in this queue by default",
			expected: "All new tickets are placed in this queue by default",
		},
		{
			name:     "German default queue description",
			lang:     "de",
			key:      "queue_descriptions.All new tickets are placed in this queue by default",
			expected: "Alle neuen Tickets werden standardmäßig in diese Queue eingeordnet",
		},
		{
			name:     "English spam queue description",
			lang:     "en",
			key:      "queue_descriptions.Spam and junk emails",
			expected: "Spam and junk emails",
		},
		{
			name:     "German spam queue description",
			lang:     "de",
			key:      "queue_descriptions.Spam and junk emails",
			expected: "Spam- und Junk-E-Mails",
		},
		{
			name:     "English misc queue description",
			lang:     "en",
			key:      "queue_descriptions.Miscellaneous tickets",
			expected: "Miscellaneous tickets",
		},
		{
			name:     "German misc queue description",
			lang:     "de",
			key:      "queue_descriptions.Miscellaneous tickets",
			expected: "Verschiedene Tickets",
		},
		{
			name:     "English support queue description",
			lang:     "en",
			key:      "queue_descriptions.General support requests",
			expected: "General support requests",
		},
		{
			name:     "German support queue description",
			lang:     "de",
			key:      "queue_descriptions.General support requests",
			expected: "Allgemeine Support-Anfragen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := i18n.T(tt.lang, tt.key)
			if result != tt.expected {
				t.Errorf("T(%q, %q) = %q; want %q", tt.lang, tt.key, result, tt.expected)
			}
		})
	}
}
