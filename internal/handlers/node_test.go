package handlers

import (
	"bytes"
	"context"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"nodal/internal/auth"
	"nodal/internal/middleware"
	"nodal/internal/platform/database"
)

func TestCreateNodeHandler(t *testing.T) {
	t.Run("Happy Path - Success", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open sqlmock: %v", err)
		}
		defer db.Close()

		title := "My Awesome Node"
		desc := "A node description"
		category := "gaming"
		expectedSlug := database.GenerateSlug(title)
		expectedID := "uuid-1234"

		// Mock standard CreateNode DB behavior
		expectedQuery := regexp.QuoteMeta("INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id")
		mock.ExpectQuery(expectedQuery).
			WithArgs(expectedSlug, title, desc, category, nil, nil).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedID))

		// Create multipart request body
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("title", title)
		_ = writer.WriteField("description", desc)
		_ = writer.WriteField("category", category)
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/nodes", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()

		// Call handler
		handler := CreateNodeHandler(db, nil) // passing nil for NATS
		handler.ServeHTTP(rr, req)

		// Assert response
		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d", rr.Code)
		}

		expectedBodySubstring := `Nodo "<strong>My Awesome Node</strong>" creado con éxito`
		if !strings.Contains(rr.Body.String(), expectedBodySubstring) {
			t.Errorf("expected body to contain %q, got %q", expectedBodySubstring, rr.Body.String())
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled DB expectations: %s", err)
		}
	})

	t.Run("Validation Error - Empty Title", func(t *testing.T) {
		db, _, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open sqlmock: %v", err)
		}
		defer db.Close()

		// Title is empty, description is present
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("title", "")
		_ = writer.WriteField("description", "A description")
		_ = writer.WriteField("category", "gaming")
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/nodes", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()

		handler := CreateNodeHandler(db, nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request, got %d", rr.Code)
		}

		expectedBodySubstring := `<div class="error">El título no puede estar vacío</div>`
		if !strings.Contains(rr.Body.String(), expectedBodySubstring) {
			t.Errorf("expected body to contain %q, got %q", expectedBodySubstring, rr.Body.String())
		}
	})

	t.Run("DB Connection Error or Timeout", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open sqlmock: %v", err)
		}
		defer db.Close()

		title := "Database Error Node"
		desc := "A node description"
		category := "gaming"
		expectedSlug := database.GenerateSlug(title)
		dbErr := errors.New("connection timeout")

		// Mock DB error
		expectedQuery := regexp.QuoteMeta("INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id")
		mock.ExpectQuery(expectedQuery).
			WithArgs(expectedSlug, title, desc, category, nil, nil).
			WillReturnError(dbErr)

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("title", title)
		_ = writer.WriteField("description", desc)
		_ = writer.WriteField("category", category)
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/nodes", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		rr := httptest.NewRecorder()

		handler := CreateNodeHandler(db, nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500 Internal Server Error, got %d", rr.Code)
		}

		expectedBodySubstring := `<div class="error">Error creando nodo: failed to create node: connection timeout</div>`
		if !strings.Contains(rr.Body.String(), expectedBodySubstring) {
			t.Errorf("expected body to contain %q, got %q", expectedBodySubstring, rr.Body.String())
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled DB expectations: %s", err)
		}
	})

	t.Run("With Authenticated User Context", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("failed to open sqlmock: %v", err)
		}
		defer db.Close()

		title := "Auth Node"
		desc := "My description"
		category := "gaming"
		expectedSlug := database.GenerateSlug(title)
		expectedID := "uuid-5678"
		userID := "user-123"

		// Expect query with owner_id
		expectedQuery := regexp.QuoteMeta("INSERT INTO nodes (slug, title, description, category, image_url, owner_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id")
		mock.ExpectQuery(expectedQuery).
			WithArgs(expectedSlug, title, desc, category, nil, &userID).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(expectedID))

		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		_ = writer.WriteField("title", title)
		_ = writer.WriteField("description", desc)
		_ = writer.WriteField("category", category)
		_ = writer.Close()

		req := httptest.NewRequest("POST", "/nodes", &body)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Inject Claims Context
		claims := &auth.Claims{
			UserID: userID,
			Role:   "user",
		}
		ctx := context.WithValue(req.Context(), middleware.ClaimsContextKey, claims)
		req = req.WithContext(ctx)

		rr := httptest.NewRecorder()

		handler := CreateNodeHandler(db, nil)
		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusCreated {
			t.Errorf("expected status 201 Created, got %d", rr.Code)
		}

		if err := mock.ExpectationsWereMet(); err != nil {
			t.Errorf("unfulfilled DB expectations: %s", err)
		}
	})
}
