// Package adapter provides mail account adapters for inbound email processing.
package adapter

import (
	"strings"
	"time"

	"github.com/goatkit/goatflow/internal/email/inbound/connector"
	"github.com/goatkit/goatflow/internal/models"
)

// AccountFromModel converts a persistence EmailAccount to the connector payload.
func AccountFromModel(model *models.EmailAccount) connector.Account {
	if model == nil {
		return connector.Account{}
	}

	accountType := strings.ToLower(strings.TrimSpace(model.AccountType))
	if accountType == "" {
		accountType = "pop3"
	}

	folder := ""
	if model.IMAPFolder != nil {
		folder = *model.IMAPFolder
	}
	pollInterval := time.Duration(model.PollIntervalSeconds) * time.Second

	return connector.Account{
		ID:                  model.ID,
		QueueID:             model.QueueID,
		Type:                accountType,
		Host:                model.Host,
		Username:            model.Login,
		Password:            []byte(model.PasswordEncrypted),
		Trusted:             model.Trusted,
		IMAPFolder:          folder,
		DispatchingMode:     model.DispatchingMode,
		AllowTrustedHeaders: model.AllowTrustedHeaders,
		PollInterval:        pollInterval,
	}
}
