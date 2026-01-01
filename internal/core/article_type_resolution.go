// Package core provides core business logic for article type resolution.
package core

import (
	"errors"
	"strings"

	"github.com/gotrs-io/gotrs-ce/internal/constants"
)

// ArticleIntent captures high-level request/user intent for creating an article.
type ArticleIntent struct {
	// Interaction is a coarse UI semantic (email, phone, internal_note, external_note)
	Interaction constants.InteractionType
	// ExplicitArticleTypeID if >0 overrides Interaction mapping
	ExplicitArticleTypeID int
	// SenderTypeID required (agent=1, system=2, customer=3)
	SenderTypeID int
	// ForceVisible optional override for customer visibility (nil => derive default)
	ForceVisible *bool
}

// ResolvedArticle holds derived fields ready for persistence.
type ResolvedArticle struct {
	ArticleTypeID       int
	ArticleSenderTypeID int
	CustomerVisible     bool
}

var (
	errInvalidSender         = errors.New("invalid sender type id")
	errInvalidArticleType    = errors.New("invalid article type id")
	errInvalidInteraction    = errors.New("invalid interaction type")
	errDisallowedCombination = errors.New("disallowed article type + sender combination")
)

// 3. Fallback (internal note for agent/system, email-external for customer).
func DetermineArticleType(intent ArticleIntent) (ResolvedArticle, error) {
	res := ResolvedArticle{ArticleSenderTypeID: intent.SenderTypeID}

	if intent.SenderTypeID != constants.ArticleSenderAgent &&
		intent.SenderTypeID != constants.ArticleSenderSystem &&
		intent.SenderTypeID != constants.ArticleSenderCustomer {
		return ResolvedArticle{}, errInvalidSender
	}

	// Helper applying metadata defaults & overrides
	applyDefault := func(typeID int) (ResolvedArticle, error) {
		meta, ok := constants.ArticleTypesMetadata[typeID]
		if !ok {
			return ResolvedArticle{}, errInvalidArticleType
		}
		res.ArticleTypeID = typeID
		// Visibility resolution
		if intent.ForceVisible != nil {
			res.CustomerVisible = *intent.ForceVisible && !meta.InternalOnly
		} else {
			res.CustomerVisible = meta.CustomerVisible
		}
		return res, nil
	}

	// 1. Explicit override
	if intent.ExplicitArticleTypeID > 0 {
		resolved, err := applyDefault(intent.ExplicitArticleTypeID)
		if err != nil {
			return ResolvedArticle{}, err
		}
		if err = validateCombination(resolved.ArticleTypeID, intent.SenderTypeID, resolved.CustomerVisible); err != nil {
			return ResolvedArticle{}, err
		}
		return resolved, nil
	}

	// 2. Interaction mapping
	switch intent.Interaction {
	case constants.InteractionEmail:
		resolved, err := applyDefault(constants.ArticleTypeEmailExternal)
		if err != nil {
			return ResolvedArticle{}, err
		}
		// Customer-created email stays visible; agent/system also visible via metadata
		return resolved, nil
	case constants.InteractionPhone:
		resolved, err := applyDefault(constants.ArticleTypePhone)
		if err != nil {
			return ResolvedArticle{}, err
		}
		return resolved, nil
	case constants.InteractionInternalNote:
		resolved, err := applyDefault(constants.ArticleTypeNoteInternal)
		if err != nil {
			return ResolvedArticle{}, err
		}
		return resolved, nil
	case constants.InteractionExternalNote:
		// Might be disabled in UI but logic supports it
		resolved, err := applyDefault(constants.ArticleTypeNoteExternal)
		if err != nil {
			return ResolvedArticle{}, err
		}
		return resolved, nil
	case "":
		// 3. Fallback heuristic (no interaction provided)
		if intent.SenderTypeID == constants.ArticleSenderCustomer {
			resolved, err := applyDefault(constants.ArticleTypeEmailExternal)
			if err != nil {
				return ResolvedArticle{}, err
			}
			return resolved, nil
		}
		resolved, err := applyDefault(constants.ArticleTypeNoteInternal)
		if err != nil {
			return ResolvedArticle{}, err
		}
		return resolved, nil
	default:
		return ResolvedArticle{}, errInvalidInteraction
	}
}

// ValidateArticleCombination ensures semantic correctness for persistence time.
// e.g., customer cannot create internal note; system cannot create phone call initiated by itself (enforced conservatively here).
func ValidateArticleCombination(articleTypeID, senderTypeID int, visible bool) error {
	return validateCombination(articleTypeID, senderTypeID, visible)
}

func validateCombination(articleTypeID, senderTypeID int, visible bool) error {
	meta, ok := constants.ArticleTypesMetadata[articleTypeID]
	if !ok {
		return errInvalidArticleType
	}

	// Disallow customer creating internal-only types
	if senderTypeID == constants.ArticleSenderCustomer && meta.InternalOnly {
		return errDisallowedCombination
	}
	// System shouldn’t originate phone or chat external (conservative rule – adjust later)
	if senderTypeID == constants.ArticleSenderSystem {
		if articleTypeID == constants.ArticleTypePhone || strings.HasPrefix(meta.Name, "chat-") {
			return errDisallowedCombination
		}
	}
	// Internal-only types must never be visible
	if meta.InternalOnly && visible {
		return errDisallowedCombination
	}
	return nil
}
