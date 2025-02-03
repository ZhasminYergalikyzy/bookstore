package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() {
	db, _ = gorm.Open(sqlite.Open("test_books.db"), &gorm.Config{})
	db.AutoMigrate(&Book{})
}

func teardownTestDB() {
	db.Exec("DROP TABLE books")
}

func TestCreateBook(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	book := Book{
		Title:       "Test Book",
		Author:      "Author A",
		Published:   "2025",
		Description: "A test book for unit testing.",
		Price:       9.99,
		ImageURL:    "https://example.com/image.png",
	}

	// Симулируем JSON-запрос
	body, _ := json.Marshal(book)
	req, _ := http.NewRequest("POST", "/books/add", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Тестируем обработчик
	handler := http.HandlerFunc(addBook)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created")
}

func TestGetBooks(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	// тестовые данные
	db.Create(&Book{
		Title:       "Existing Book",
		Author:      "Author B",
		Published:   "2020",
		Description: "A pre-existing book in the database.",
		Price:       19.99,
		ImageURL:    "https://example.com/image.png",
	})

	req, _ := http.NewRequest("GET", "/books", nil)
	rr := httptest.NewRecorder()

	// Тестируем обработчик
	handler := http.HandlerFunc(getBooks)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK")
	assert.Contains(t, rr.Body.String(), "Existing Book", "Expected book to be present in response")
}

func TestUpdateBook(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	// тестовые данные
	book := Book{
		Title:       "Old Title",
		Author:      "Author C",
		Published:   "2015",
		Description: "An old book title.",
		Price:       15.99,
		ImageURL:    "https://example.com/image.png",
	}
	db.Create(&book)

	// Обновляем данные книги
	book.Title = "New Title"
	body, _ := json.Marshal(book)
	req, _ := http.NewRequest("PUT", "/books/update", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Тестируем обработчик
	handler := http.HandlerFunc(updateBook)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK")
	assert.Contains(t, rr.Body.String(), "New Title", "Expected title to be updated")
}

func TestDeleteBook(t *testing.T) {
	setupTestDB()
	defer teardownTestDB()

	// тестовые данные
	book := Book{
		Title:       "Book to Delete",
		Author:      "Author D",
		Published:   "2018",
		Description: "This book will be deleted.",
		Price:       10.99,
		ImageURL:    "https://example.com/image.png",
	}
	db.Create(&book)

	// Удаляем книгу
	req, _ := http.NewRequest("DELETE", "/books/delete?id=1", nil)
	rr := httptest.NewRecorder()

	// Тестируем обработчик
	handler := http.HandlerFunc(deleteBook)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code, "Expected 204 No Content")
}
