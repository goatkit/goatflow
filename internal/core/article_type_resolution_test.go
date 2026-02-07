package core

import (
	"testing"

	"github.com/goatkit/goatflow/internal/constants"
)

func TestDetermineArticleTypeBasicMappings(t *testing.T) {
	tests := []struct {
		name      string
		intent    ArticleIntent
		wantType  int
		wantVis   bool
		wantError bool
	}{
		{"Email agent", ArticleIntent{Interaction: constants.InteractionEmail, SenderTypeID: constants.ArticleSenderAgent}, constants.ArticleTypeEmailExternal, true, false},
		{"Email customer", ArticleIntent{Interaction: constants.InteractionEmail, SenderTypeID: constants.ArticleSenderCustomer}, constants.ArticleTypeEmailExternal, true, false},
		{"Phone agent", ArticleIntent{Interaction: constants.InteractionPhone, SenderTypeID: constants.ArticleSenderAgent}, constants.ArticleTypePhone, true, false},
		{"Internal note agent", ArticleIntent{Interaction: constants.InteractionInternalNote, SenderTypeID: constants.ArticleSenderAgent}, constants.ArticleTypeNoteInternal, false, false},
		{"Fallback customer", ArticleIntent{SenderTypeID: constants.ArticleSenderCustomer}, constants.ArticleTypeEmailExternal, true, false},
		{"Fallback agent", ArticleIntent{SenderTypeID: constants.ArticleSenderAgent}, constants.ArticleTypeNoteInternal, false, false},
		{"Explicit override", ArticleIntent{SenderTypeID: constants.ArticleSenderAgent, ExplicitArticleTypeID: constants.ArticleTypeNoteExternal}, constants.ArticleTypeNoteExternal, true, false},
		{"Invalid sender", ArticleIntent{Interaction: constants.InteractionEmail, SenderTypeID: 99}, 0, false, true},
		{"Invalid interaction", ArticleIntent{Interaction: "weird", SenderTypeID: constants.ArticleSenderAgent}, 0, false, true},
	}

	for _, tc := range tests {
		res, err := DetermineArticleType(tc.intent)
		if tc.wantError && err == nil {
			t.Fatalf("%s: expected error got nil", tc.name)
		}
		if !tc.wantError && err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if tc.wantError {
			continue
		}
		if res.ArticleTypeID != tc.wantType {
			t.Errorf("%s: got type %d want %d", tc.name, res.ArticleTypeID, tc.wantType)
		}
		if res.CustomerVisible != tc.wantVis {
			t.Errorf("%s: visibility mismatch got %v want %v", tc.name, res.CustomerVisible, tc.wantVis)
		}
	}
}

func TestValidateArticleCombination(t *testing.T) {
	// Customer cannot create internal note
	if err := ValidateArticleCombination(constants.ArticleTypeNoteInternal, constants.ArticleSenderCustomer, false); err == nil {
		t.Errorf("expected error for customer internal note")
	}
	// Internal note forced visible rejected
	if err := ValidateArticleCombination(constants.ArticleTypeNoteInternal, constants.ArticleSenderAgent, true); err == nil {
		t.Errorf("expected error for internal note visible")
	}
	// System phone disallowed (conservative rule)
	if err := ValidateArticleCombination(constants.ArticleTypePhone, constants.ArticleSenderSystem, true); err == nil {
		t.Errorf("expected error for system phone")
	}
	// Valid agent external note
	if err := ValidateArticleCombination(constants.ArticleTypeNoteExternal, constants.ArticleSenderAgent, true); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
