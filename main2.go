package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"io/ioutil"
	"net/smtp"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Book модель
type Book struct {
	ID        uint   `json:"id"`
	Title     string `json:"title"`
	Author    string `json:"author"`
	Published string `json:"published"`
}
type UserProfile struct {
	ID       uint   `json:"id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

var db *gorm.DB
var err error

// Логгер
var logger = logrus.New()

// Ограничитель скорости
var limiter = rate.NewLimiter(1, 5) // 1 запрос в секунду, максимум 5 подряд

func main() {
	// Настройка логгера
	logger.SetFormatter(&logrus.JSONFormatter{})
	logger.SetLevel(logrus.InfoLevel)

	r := mux.NewRouter()
	filePath := "/Users/zhasminyergalikyzy/Desktop/bookworm/bouquiniste.html"
	r.HandleFunc("/bouquiniste", func(w http.ResponseWriter, r *http.Request) {
    	http.ServeFile(w, r, filePath)
 	}).Methods("GET")
	// Открытие базы данных SQLite с использованием нового GORM
	db, err = gorm.Open(sqlite.Open("./books.db"), &gorm.Config{})
	if err != nil {
		// Логирование с дополнительными полями
		logger.WithFields(logrus.Fields{
			"action": "db_connection",
			"status": "failure",
		}).WithError(err).Fatal("Failed to connect to the database")
	}

	// Автоматическая миграция структуры Book
	db.AutoMigrate(&Book{})

	// Роуты для CRUD операций
	mux := http.NewServeMux()
	mux.HandleFunc("/books", rateLimitMiddleware(getBooks))       // Получение всех книг
	mux.HandleFunc("/books/add", rateLimitMiddleware(addBook))    // Добавление книги
	mux.HandleFunc("/books/update", rateLimitMiddleware(updateBook)) // Обновление книги
	mux.HandleFunc("/books/delete", rateLimitMiddleware(deleteBook)) // Удаление книги
	mux.HandleFunc("/books/search", rateLimitMiddleware(getBookByID)) // Поиск книги по ID
	mux.HandleFunc("/profile/update", rateLimitMiddleware(updateUserProfile)) // Обновление профиля пользователя
	mux.HandleFunc("/support/message", rateLimitMiddleware(sendSupportMessage)) // Отправка сообщения в поддержку

	// Запуск сервера в отдельной горутине
	server := &http.Server{
		Addr:    ":8080",
		Handler: enableCORS(mux),
	}

	// Логирование старта сервера
	logger.WithFields(logrus.Fields{
		"action": "start",
		"status": "success",
	}).Info("Server started at :8080")

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// Логирование ошибок сервера
			logger.WithFields(logrus.Fields{
				"action": "server_error",
				"status": "failure",
			}).WithError(err).Fatal("Server failed")
		}
	}()

	// Перехват сигнала для graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	logger.WithFields(logrus.Fields{
		"action": "shutdown",
		"status": "initiated",
	}).Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		// Логирование ошибки при выключении
		logger.WithFields(logrus.Fields{
			"action": "shutdown_error",
			"status": "failure",
		}).WithError(err).Fatal("Server forced to shutdown")
	}

	logger.WithFields(logrus.Fields{
		"action": "shutdown",
		"status": "success",
	}).Info("Server exiting")
}
	

// Ограничение скорости запросов
func rateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			// Логирование превышения лимита
			logger.WithFields(logrus.Fields{
				"action": "rate_limit_exceeded",
				"status": "failure",
				"ip":     r.RemoteAddr,
			}).Warn("Rate limit exceeded")

			// Возвращение кода ошибки 429
			http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// Получение всех книг с фильтрацией, сортировкой и пагинацией
func getBooks(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action":  "getBooks",
		"status":  "start",
		"request": r.URL.String(),
	}).Info("Fetching books")

	var books []Book

	// Параметры фильтрации
	title := r.URL.Query().Get("title")
	author := r.URL.Query().Get("author")
	published := r.URL.Query().Get("published")

	query := db.Model(&Book{})

	if title != "" {
		query = query.Where("title LIKE ?", "%"+title+"%")
	}
	if author != "" {
		query = query.Where("author LIKE ?", "%"+author+"%")
	}
	if published != "" {
		query = query.Where("published LIKE ?", "%"+published+"%")
	}

	// Параметры сортировки
	sortBy := r.URL.Query().Get("sortBy")
	sortOrder := r.URL.Query().Get("sortOrder")
	if sortBy != "" {
		if sortOrder == "desc" {
			query = query.Order(sortBy + " DESC")
		} else {
			query = query.Order(sortBy + " ASC")
		}
	}

	// Параметры пагинации
	page := r.URL.Query().Get("page")
	limit := 10 // Количество записей на странице
	offset := 0 // Смещение
	if page != "" {
		if p, err := strconv.Atoi(page); err == nil && p > 0 {
			offset = (p - 1) * limit
		}
	}
	query = query.Offset(offset).Limit(limit)

	// Выполнение запроса к базе данных
	if err := query.Find(&books).Error; err != nil {
		logger.WithError(err).Error("Failed to fetch books")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(books)

	logger.WithFields(logrus.Fields{
		"action": "getBooks",
		"status": "success",
	}).Info("Books fetched successfully")
}

// Добавление книги
func addBook(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "addBook",
		"status": "start",
	}).Info("Adding a new book")

	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		logger.WithError(err).Warn("Failed to decode request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := db.Create(&book).Error; err != nil {
		logger.WithError(err).Error("Failed to add book")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)

	logger.WithFields(logrus.Fields{
		"action": "addBook",
		"status": "success",
		"bookID": book.ID,
	}).Info("Book added successfully")
}

// Обновление книги
func updateBook(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "updateBook",
		"status": "start",
	}).Info("Updating a book")

	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		logger.WithError(err).Warn("Failed to decode request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := db.Save(&book).Error; err != nil {
		logger.WithError(err).Error("Failed to update book")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(book)

	logger.WithFields(logrus.Fields{
		"action": "updateBook",
		"status": "success",
		"bookID": book.ID,
	}).Info("Book updated successfully")
}

// Удаление книги
func deleteBook(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "deleteBook",
		"status": "start",
	}).Info("Deleting a book")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	if err := db.Delete(&Book{}, id).Error; err != nil {
		logger.WithError(err).Error("Failed to delete book")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)

	logger.WithFields(logrus.Fields{
		"action": "deleteBook",
		"status": "success",
		"bookID": id,
	}).Info("Book deleted successfully")
}

// Поиск книги по ID
func getBookByID(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "getBookByID",
		"status": "start",
	}).Info("Fetching book by ID")

	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	var book Book
	if err := db.First(&book, id).Error; err != nil {
		logger.WithError(err).Warn("Book not found")
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(book)

	logger.WithFields(logrus.Fields{
		"action": "getBookByID",
		"status": "success",
		"bookID": id,
	}).Info("Book fetched successfully")
}

// Обновление профиля пользователя
func updateUserProfile(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "updateUserProfile",
		"status": "start",
	}).Info("Updating user profile")

	var userProfile UserProfile
	if err := json.NewDecoder(r.Body).Decode(&userProfile); err != nil {
		logger.WithError(err).Warn("Failed to decode request body")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := db.Save(&userProfile).Error; err != nil {
		logger.WithError(err).Error("Failed to update user profile")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(userProfile)

	logger.WithFields(logrus.Fields{
		"action": "updateUserProfile",
		"status": "success",
		"userID": userProfile.ID,
	}).Info("User profile updated successfully")
}

// Отправка сообщения в поддержку
func sendSupportMessage(w http.ResponseWriter, r *http.Request) {
	logger.WithFields(logrus.Fields{
		"action": "sendSupportMessage",
		"status": "start",
	}).Info("Sending support message")

	r.ParseMultipartForm(10 << 20) // 10 MB

	sender := r.FormValue("email")
	message := r.FormValue("message")
	file, handler, err := r.FormFile("attachment")

	var attachment []byte
	if err == nil {
		defer file.Close()
		attachment, err = ioutil.ReadAll(file)
		if err != nil {
			logger.WithError(err).Warn("Failed to read attachment")
			http.Error(w, "Failed to read attachment", http.StatusInternalServerError)
			return
		}
	}

	smtpHost := "smtp.example.com"
	smtpPort := "587"
	smtpUser := "your-email@example.com"
	smtpPass := "your-password"

	subject := "Support Request"
	body := "From: " + sender + "\n" + message

	msg := []byte("To: support@example.com\r\n" + "Subject: " + subject + "\r\n\r\n" + body)

	auth := smtp.PlainAuth("", smtpUser, smtpPass, smtpHost)
	if err := smtp.SendMail(smtpHost+":"+smtpPort, auth, smtpUser, []string{"support@example.com"}, msg); err != nil {
		logger.WithError(err).Error("Failed to send support email")
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	if attachment != nil {
		logger.WithFields(logrus.Fields{
			"action":    "sendSupportMessage",
			"attachment": handler.Filename,
		}).Info("Attachment included in support message")
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Support message sent successfully"))

	logger.WithFields(logrus.Fields{
		"action": "sendSupportMessage",
		"status": "success",
	}).Info("Support message sent successfully")
}


// enableCORS добавляет заголовки для поддержки CORS
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
