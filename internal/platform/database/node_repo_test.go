package database

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateNode(t *testing.T) {
	t.Run("Happy Path - Success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer db.Close()

		expectedID := "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"
		title := "Test Node Title"
		description := "Test Node Description"
		category := "gaming"
		var imageURL *string
		ownerID := "user-uuid"

		expectedSlug := GenerateSlug(title)

		// The query uses QueryRow, which expects QueryRow.Scan()
		rows := sqlmock.NewRows([]string{"id"}).AddRow(expectedID)
		
		// Escape query for regex matching
		expectedQuery := regexp.QuoteMeta("INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id")
		mock.ExpectQuery(expectedQuery).
			WithArgs(expectedSlug, title, description, category, imageURL, &ownerID).
			WillReturnRows(rows)

		id, err := CreateNode(db, title, description, category, imageURL, &ownerID)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}

		if id != expectedID {
			t.Errorf("expected ID %s, got %s", expectedID, id)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})

	t.Run("DB Connection Error or Timeout", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
		}
		defer db.Close()

		title := "Test Node Title"
		description := "Test Node Description"
		category := "gaming"
		var imageURL *string
		ownerID := "user-uuid"

		expectedSlug := GenerateSlug(title)
		dbErr := errors.New("connection timeout")

		expectedQuery := regexp.QuoteMeta("INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id")
		mock.ExpectQuery(expectedQuery).
			WithArgs(expectedSlug, title, description, category, imageURL, &ownerID).
			WillReturnError(dbErr)

		_, err = CreateNode(db, title, description, category, imageURL, &ownerID)
		if err == nil {
			t.Errorf("expected error, got nil")
		}

		if err.Error() != "failed to create node: connection timeout" {
			t.Errorf("expected failed to create node error, got %v", err)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("there were unfulfilled expectations: %s", err)
		}
	})
}
