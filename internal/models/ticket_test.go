package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTicketHelpers(t *testing.T) {
	t.Run("IsLocked returns correct status", func(t *testing.T) {
		ticket := &Ticket{TicketLockID: TicketUnlocked}
		assert.False(t, ticket.IsLocked())

		ticket.TicketLockID = TicketLocked
		assert.True(t, ticket.IsLocked())

		ticket.TicketLockID = TicketTmpLocked
		assert.True(t, ticket.IsLocked())
	})

	t.Run("IsArchived returns correct status", func(t *testing.T) {
		ticket := &Ticket{ArchiveFlag: 0}
		assert.False(t, ticket.IsArchived())

		ticket.ArchiveFlag = 1
		assert.True(t, ticket.IsArchived())
	})

	t.Run("CanBeEditedBy checks permissions correctly", func(t *testing.T) {
		userID := 5
		otherUserID := 10
		customerID := "CUST001"

		ticket := &Ticket{
			UserID:       &userID,
			CustomerID:   &customerID,
			TicketLockID: TicketUnlocked,
		}

		// Admin can always edit
		assert.True(t, ticket.CanBeEditedBy(otherUserID, "Admin"))
		assert.True(t, ticket.CanBeEditedBy(userID, "Admin"))

		// Agent can edit owned tickets
		assert.True(t, ticket.CanBeEditedBy(userID, "Agent"))
		assert.False(t, ticket.CanBeEditedBy(otherUserID, "Agent"))

		// Agent can edit unassigned tickets
		ticket.UserID = nil
		assert.True(t, ticket.CanBeEditedBy(otherUserID, "Agent"))

		// Agent can edit if responsible
		ticket.UserID = &userID
		ticket.ResponsibleUserID = &otherUserID
		assert.True(t, ticket.CanBeEditedBy(otherUserID, "Agent"))

		// Customer can edit unlocked tickets (simplified implementation)
		ticket.CustomerID = &customerID
		ticket.TicketLockID = TicketUnlocked
		assert.True(t, ticket.CanBeEditedBy(userID, "Customer"))
		// Note: Current implementation allows any customer to edit unlocked tickets
		assert.True(t, ticket.CanBeEditedBy(otherUserID, "Customer"))

		// Customer cannot edit locked tickets
		ticket.TicketLockID = TicketLocked
		assert.False(t, ticket.CanBeEditedBy(userID, "Customer"))
	})
}

func TestNullableHelpers(t *testing.T) {
	t.Run("NullableUint handles zero values", func(t *testing.T) {
		assert.Nil(t, NullableUint(0))

		val := NullableUint(5)
		assert.NotNil(t, val)
		assert.Equal(t, uint(5), *val)
	})

	t.Run("NullableString handles empty strings", func(t *testing.T) {
		assert.Nil(t, NullableString(""))

		val := NullableString("test")
		assert.NotNil(t, val)
		assert.Equal(t, "test", *val)
	})

	t.Run("DerefUint handles nil pointers", func(t *testing.T) {
		assert.Equal(t, uint(0), DerefUint(nil))

		val := uint(10)
		assert.Equal(t, uint(10), DerefUint(&val))
	})

	t.Run("DerefString handles nil pointers", func(t *testing.T) {
		assert.Equal(t, "", DerefString(nil))

		val := "test"
		assert.Equal(t, "test", DerefString(&val))
	})
}

func TestValidationFunctions(t *testing.T) {
	t.Run("ValidateTicketLock", func(t *testing.T) {
		assert.True(t, ValidateTicketLock(TicketUnlocked))
		assert.True(t, ValidateTicketLock(TicketLocked))
		assert.True(t, ValidateTicketLock(TicketTmpLocked))
		assert.False(t, ValidateTicketLock(0))
		assert.False(t, ValidateTicketLock(4))
	})

	t.Run("ValidateSenderType", func(t *testing.T) {
		assert.True(t, ValidateSenderType(SenderTypeAgent))
		assert.True(t, ValidateSenderType(SenderTypeSystem))
		assert.True(t, ValidateSenderType(SenderTypeCustomer))
		assert.False(t, ValidateSenderType(0))
		assert.False(t, ValidateSenderType(4))
	})
}

func TestTicketStructure(t *testing.T) {
	t.Run("Ticket fields are properly structured", func(t *testing.T) {
		now := time.Now()
		userID := 1
		customerID := "CUST002"
		customerUserID := "customer@example.com"

		typeID := 1
		ticket := Ticket{
			ID:               1,
			TicketNumber:     "20240101-000001",
			Title:            "Test Ticket",
			QueueID:          1,
			TicketLockID:     TicketUnlocked,
			TypeID:           &typeID,
			UserID:           &userID,
			CustomerID:       &customerID,
			CustomerUserID:   &customerUserID,
			TicketStateID:    1,
			TicketPriorityID: 3,
			CreateTime:       now,
			CreateBy:         1,
			ChangeTime:       now,
			ChangeBy:         1,
		}

		assert.Equal(t, 1, ticket.ID)
		assert.Equal(t, "20240101-000001", ticket.TicketNumber)
		assert.Equal(t, "Test Ticket", ticket.Title)
		assert.Equal(t, 1, ticket.QueueID)
		assert.False(t, ticket.IsLocked())
		assert.False(t, ticket.IsArchived())
		assert.Equal(t, userID, *ticket.UserID)
		assert.Equal(t, customerID, *ticket.CustomerID)
		assert.Equal(t, customerUserID, *ticket.CustomerUserID)
	})

	t.Run("Article fields are properly structured", func(t *testing.T) {
		now := time.Now()
		subject := "Re: Test Ticket"

		article := Article{
			ID:                   1,
			TicketID:             1,
			SenderTypeID:         SenderTypeCustomer,
			IsVisibleForCustomer: 1,
			Subject:              subject,
			Body:                 "This is the article body",
			BodyType:             "text/plain",
			CreateTime:           now,
			CreateBy:             1,
			ChangeTime:           now,
			ChangeBy:             1,
		}

		assert.Equal(t, 1, article.ID)
		assert.Equal(t, 1, article.TicketID)
		assert.Equal(t, SenderTypeCustomer, article.SenderTypeID)
		assert.Equal(t, 1, article.IsVisibleForCustomer)
		assert.Equal(t, subject, article.Subject)
		assert.Equal(t, "This is the article body", article.Body)
	})

	t.Run("Attachment fields are properly structured", func(t *testing.T) {
		now := time.Now()
		contentID := "attachment1"

		attachment := Attachment{
			ID:          1,
			ArticleID:   1,
			Filename:    "document.pdf",
			ContentType: "application/pdf",
			ContentSize: 1024,
			ContentID:   &contentID,
			Disposition: "attachment",
			Content:     "base64encodedcontent",
			CreateTime:  now,
			CreateBy:    1,
			ChangeTime:  now,
			ChangeBy:    1,
		}

		assert.Equal(t, uint(1), attachment.ID)
		assert.Equal(t, uint(1), attachment.ArticleID)
		assert.Equal(t, "document.pdf", attachment.Filename)
		assert.Equal(t, "application/pdf", attachment.ContentType)
		assert.Equal(t, 1024, attachment.ContentSize)
		assert.Equal(t, contentID, *attachment.ContentID)
	})
}

func TestTicketRequests(t *testing.T) {
	t.Run("TicketCreateRequest structure", func(t *testing.T) {
		customerID := uint(1)
		customerUserID := "customer@example.com"

		req := TicketCreateRequest{
			Title:          "New Ticket",
			QueueID:        1,
			PriorityID:     3,
			StateID:        1,
			CustomerID:     &customerID,
			CustomerUserID: &customerUserID,
			Body:           "Ticket description",
			BodyType:       "text/plain",
			Subject:        "Initial message",
		}

		assert.Equal(t, "New Ticket", req.Title)
		assert.Equal(t, uint(1), req.QueueID)
		assert.Equal(t, uint(3), req.PriorityID)
		assert.Equal(t, customerID, *req.CustomerID)
	})

	t.Run("TicketUpdateRequest structure", func(t *testing.T) {
		newTitle := "Updated Title"
		newQueueID := uint(2)
		newOwnerID := uint(5)

		req := TicketUpdateRequest{
			Title:   &newTitle,
			QueueID: &newQueueID,
			UserID:  &newOwnerID,
		}

		assert.Equal(t, newTitle, *req.Title)
		assert.Equal(t, newQueueID, *req.QueueID)
		assert.Equal(t, newOwnerID, *req.UserID)
	})

	t.Run("TicketListRequest structure", func(t *testing.T) {
		queueID := uint(1)
		stateID := uint(2)
		archiveFlag := 0
		req := TicketListRequest{
			Page:                1,
			PerPage:             25,
			QueueID:             &queueID,
			StateID:             &stateID,
			Search:              "urgent",
			SortBy:              "created",
			SortOrder:           "desc",
			ArchiveFlag:         &archiveFlag,
			ExcludeClosedStates: true,
		}

		assert.Equal(t, 1, req.Page)
		assert.Equal(t, 25, req.PerPage)
		assert.Equal(t, queueID, *req.QueueID)
		assert.Equal(t, stateID, *req.StateID)
		assert.Equal(t, "urgent", req.Search)
		assert.True(t, req.ExcludeClosedStates)
	})
}
