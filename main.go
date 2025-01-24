package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	// "io"
	"os"
	"os/signal"
	"syscall"
	"time"

	// "strconv"
	"text/template"

	// "html/template"
	// "log"
	"mime/multipart"
	"net/http"
	"net/smtp"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/time/rate"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Book модель
type Book struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Author      string  `json:"author"`
	Published   string  `json:"published"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
}
type Fantasy struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	ImageURL    string  `json:"image_url"`
}
type LogEntry struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Timestamp time.Time `json:"timestamp"` // Используем time.Time для корректной работы с TIMESTAMP
    Level     string    `json:"level"`
    Message   string    `json:"message"`
}
type User struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    Email     string    `gorm:"unique" json:"email"`
    Password  string    `json:"-"` // Храним хэш пароля
    Verified  bool      `json:"verified"` // Флаг подтверждения
    CreatedAt time.Time `json:"created_at"`
}

type VerificationToken struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    UserID    uint      `json:"user_id"`
    Token     string    `json:"token"`
    CreatedAt time.Time `json:"created_at"`
    ExpiresAt time.Time `json:"expires_at"`
}


var db *gorm.DB
var err error
var logger = logrus.New()

var limiter = rate.NewLimiter(1, 5) // 1 запрос в секунду с накоплением до 5 запросов

func initDB() {
	db, err = gorm.Open(sqlite.Open("./books.db"), &gorm.Config{})
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to the database")
	}

	err = db.AutoMigrate(&Book{})
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to the database")
	}

	err = db.AutoMigrate(&Fantasy{})
	if err != nil {
		logger.WithError(err).Fatal("failed to connect to the database")
	}
	// Миграция для таблицы User
    err = db.AutoMigrate(&User{})
    if err != nil {
        logger.WithError(err).Fatal("failed to migrate User table")
    }

    // Миграция для таблицы VerificationToken
    err = db.AutoMigrate(&VerificationToken{})
    if err != nil {
        logger.WithError(err).Fatal("failed to migrate VerificationToken table")
    }

}

type DBHook struct {
	DB *gorm.DB
}


// Fire записывает логи в базу данных
func (hook *DBHook) Fire(entry *logrus.Entry) error {
	if hook.DB == nil {
		return fmt.Errorf("database connection is not initialized")
	}

	log := LogEntry{
        Timestamp: time.Now(), // Используем текущую дату
        Level:     entry.Level.String(),
        Message:   entry.Message,
    }


	// Сохранение логов в базу данных
	if err := hook.DB.Create(&log).Error; err != nil {
		return fmt.Errorf("failed to save log to database: %w", err)
	}

	return nil
}
// Levels implements logrus.Hook.
func (hook *DBHook) Levels() []logrus.Level {
    return logrus.AllLevels // Возвращаем все уровни логов
}

func rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if !limiter.Allow() {
            http.Error(w, "429 Too Many Requests: Rate limit exceeded", http.StatusTooManyRequests)
            logger.WithFields(logrus.Fields{
                "path":   r.URL.Path,
                "method": r.Method,
                "client": r.RemoteAddr,
            }).Warn("Rate limit exceeded")
            return
        }
        next.ServeHTTP(w, r)
    })
}


func main() {
	initDB()

	logger := logrus.New()

	// Настройка логгера
	logger.SetFormatter(&logrus.JSONFormatter{})
	
	logger.AddHook(&DBHook{DB: db}) // Хук добавляется после инициализации базы данных

	// Пример использования логгера
	logger.WithFields(logrus.Fields{
		"test": "logging",
		"status": "ok",
	}).Info("Test log entry")	

	// Роуты для CRUD операций
	mux := http.NewServeMux()

	// Обслуживание HTML-страниц
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
			logger.WithFields(logrus.Fields{
				"path":   r.URL.Path,
				"method": r.Method,
			}).Info("HTML page served")
		} else {
			http.FileServer(http.Dir(".")).ServeHTTP(w, r)
		}
	}))
	mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "profile.html")
	})
	// mux.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
	// 	if r.Method == http.MethodPost {
	// 		handleSendMessage(w, r) // Обработка отправки сообщения
	// 	} else {
	// 		http.ServeFile(w, r, "profile.html") // Отправка HTML-файла
	// 	}
	// })
	mux.HandleFunc("/fantasy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.ServeFile(w, r, "fantasy.html")
		} else if r.Method == http.MethodPost {
			getFantasyBooks(w, r) // Возвращаем карточки при POST-запросе
		}
	})
	mux.HandleFunc("/bouquiniste", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "bouquiniste.html")
	})

	mux.HandleFunc("/books", getBooks)           // Получение всех книг
	mux.HandleFunc("/books/add", addBook)        // Добавление книги
	mux.HandleFunc("/books/update", updateBook)  // Обновление книги
	mux.HandleFunc("/books/delete", deleteBook)  // Удаление книги
	mux.HandleFunc("/books/search", getBookByID) //search
	mux.HandleFunc("/send-message", handleSendMessage) //send message

	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/verify", verifyEmailHandler)



	rateLimitedMux := rateLimitMiddleware(mux)

	// http.HandleFunc("/send-message", handleSendMessage)
	// Запуск сервера с поддержкой CORS
	// HTTP сервер
	srv := &http.Server{
		Addr:    ":8080",
		Handler: enableCORS(rateLimitedMux),
	}

	// Канал для получения сигналов завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Запуск сервера в отдельной горутине
	go func() {
		logger.Info("Server is starting...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Ожидание сигнала завершения
	<-quit
	logger.Info("Server is shutting down...")

	// Контекст с тайм-аутом для завершения активных соединений
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Завершение работы сервера
	if err := srv.Shutdown(ctx); err != nil {
		logger.Fatalf("Server forced to shutdown: %v", err)
	}

	logger.Info("Server exited gracefully.")
}

// Обработчик для загрузки fantasy.html с карточками
func getFantasyBooks(w http.ResponseWriter, r *http.Request) {
	var books []Fantasy

	// Извлечение данных из таблицы fantasy
	if err := db.Table("fantasy").Limit(30).Find(&books).Error; err != nil {
		logger.WithError(err).Error("Failed to fetch fantasy books")
		http.Error(w, "Ошибка при получении данных", http.StatusInternalServerError)
		return
	}

	cardTemplate := `
		{{range .}}
		<div class="book-card">
			<img src="{{.ImageURL}}" alt="{{.Title}}" class="book-image">
			<h3 class="book-title">{{.Title}}</h3>
			<p class="book-description">{{.Description}}</p>
			<p class="book-price">Price: ${{.Price}}</p>
		</div>
		{{end}}
	`

	tmpl, err := template.New("cards").Parse(cardTemplate)
	if err != nil {
		logger.WithError(err).Error("Template error")
		http.Error(w, "Ошибка при создании шаблона карточек", http.StatusInternalServerError)
		return
	}

	logger.WithFields(logrus.Fields{
		"action": "fetch_fantasy_books",
		"count":  len(books),
	}).Info("Fetched fantasy books")

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, books); err != nil {
		logger.WithError(err).Error("Template execution error")
		http.Error(w, "Ошибка при выполнении шаблона карточек", http.StatusInternalServerError)
	}
}


// Получение всех книг
func getBooks(w http.ResponseWriter, r *http.Request) {
	var books []Book
	if err := db.Find(&books).Error; err != nil {
		logger.WithError(err).Error("Failed to fetch books")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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
	// Выполнение запроса к базе данных
	if err := query.Find(&books).Error; err != nil {
		logger.WithError(err).Error("Failed to fetch books")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(books)

}

// Добавление книги
func addBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	// Декодируем JSON из тела запроса
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		logger.WithError(err).Error("Failed to decode request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
		return
	}

	// Проверяем, заполнены ли обязательные поля
	if book.Title == "" || book.Author == "" || book.Published == "" {
		err := fmt.Errorf("missing required fields: Title, Author, and Published are mandatory")
		logger.WithError(err).Error("Failed to add book due to missing fields")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Missing required fields"})
		return
	}

	// Сохраняем книгу в базе данных
	if err := db.Create(&book).Error; err != nil {
		logger.WithError(err).Error("Failed to add book")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to add book"})
		return
	}

	// Логируем успешное добавление книги
	logger.WithFields(logrus.Fields{
		"action":  "add_book",
		"book_id": book.ID,
		"title":   book.Title,
	}).Info("Book added successfully")

	// Возвращаем успешный ответ
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(book)
}

// func addBook(w http.ResponseWriter, r *http.Request) {
// 	var book Book
// 	// if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
// 	// 	logger.WithError(err).Error("Failed to decode book")
// 	// 	http.Error(w, err.Error(), http.StatusBadRequest)
// 	// 	return
// 	// }
// 	// if err := db.Create(&book).Error; err != nil {
// 	// 	logger.WithError(err).Error("Failed to add book")
// 	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
// 	// 	return
// 	// }
// 	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
// 		logger.WithError(err).Error("Failed to decode request body")
// 		http.Error(w, "Invalid request", http.StatusBadRequest)
// 		return
// 	}
// 	if err := db.Create(&book).Error; err != nil {
// 		logger.WithError(err).Error("Failed to add book")
// 		http.Error(w, "Failed to add book", http.StatusInternalServerError)
// 		return
// 	}
// 	w.WriteHeader(http.StatusCreated)
// 	if err := json.NewEncoder(w).Encode(book); err != nil {
// 		logger.WithError(err).Error("Failed to encode response")
// 	}
// }

// Обновление книги
func updateBook(w http.ResponseWriter, r *http.Request) {
	var book Book
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		logger.WithError(err).Error("Failed to decode book")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := db.Save(&book).Error; err != nil {
		logger.WithError(err).Error("Failed to update book")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(book); err != nil {
		logger.WithError(err).Error("Failed to encode response")
	}
}

// Удаление книги
func deleteBook(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		logger.Error("ID is required")
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}
	if err := db.Delete(&Book{}, id).Error; err != nil {
		logger.WithError(err).Error("Failed to delete book")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// search
func getBookByID(w http.ResponseWriter, r *http.Request) {
	// Получение ID из параметров запроса
	id := r.URL.Query().Get("id")
	if id == "" {
		logger.Error("ID is required")
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	// Поиск книги в базе данных
	var book Book
	if err := db.First(&book, id).Error; err != nil {
		logger.WithError(err).Error("Book not found")
		http.Error(w, "Book not found", http.StatusNotFound)
		return
	}

	// Отправка результата в формате JSON
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(book); err != nil {
		logger.WithError(err).Error("Failed to encode book")
	}
}

// Функция для отправки email с вложением
func sendEmailWithAttachment(toEmail, subject, message string, file multipart.File, fileHeader *multipart.FileHeader) error {
	from := "bouquiniste19@gmail.com" // Ваш email
	password := "fjnzynihbertfxye"   // Пароль от email
	smtpHost := "smtp.gmail.com"     // SMTP-сервер
	smtpPort := "587"                // Порт SMTP

	// Создаем MIME-сообщение
	mimeBoundary := "BOUNDARY_STRING"
	mimeMessage := fmt.Sprintf(
		"To: %s\nSubject: %s\nMIME-Version: 1.0\nContent-Type: multipart/mixed; boundary=%s\n\n",
		toEmail, subject, mimeBoundary,
	)

	// Текст сообщения
	mimeMessage += fmt.Sprintf("--%s\nContent-Type: text/plain; charset=utf-8\n\n%s\n", mimeBoundary, message)

	// Добавление файла
	if file != nil && fileHeader != nil {
		fileBytes, err := io.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}

		mimeMessage += fmt.Sprintf(
			"--%s\nContent-Type: application/octet-stream\nContent-Disposition: attachment; filename=\"%s\"\n\n%s\n",
			mimeBoundary, fileHeader.Filename, string(fileBytes),
		)
	}

	// Завершаем MIME-сообщение
	mimeMessage += fmt.Sprintf("--%s--", mimeBoundary)

	// Авторизация и отправка
	auth := smtp.PlainAuth("", from, password, smtpHost)
	return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, []byte(mimeMessage))
}

// Обработчик для отправки сообщения
func handleSendMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Чтение полей формы
	to := r.FormValue("to")
	subject := r.FormValue("subject")
	message := r.FormValue("message")

	// Проверка обязательных полей
	if to == "" || subject == "" || message == "" {
		http.Error(w, "Recipient, subject, and message are required", http.StatusBadRequest)
		return
	}

	// Обработка вложения
	var attachment multipart.File
	var fileHeader *multipart.FileHeader
	var err error

	attachment, fileHeader, err = r.FormFile("attachment")
	if err == http.ErrMissingFile {
		attachment = nil
		fileHeader = nil
	} else if err != nil {
		http.Error(w, "Failed to process attachment", http.StatusInternalServerError)
		return
	}
	if attachment != nil {
		defer attachment.Close()
	}

	// Отправка email
	err = sendEmailWithAttachment(to, subject, message, attachment, fileHeader)
	if err != nil {
		http.Error(w, "Failed to send email", http.StatusInternalServerError)
		return
	}

	// Успешный ответ
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "Message sent successfully"})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
    var user User
    if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    // Проверка, если email уже зарегистрирован
    var existingUser User
    if err := db.Where("email = ?", user.Email).First(&existingUser).Error; err == nil {
        http.Error(w, "Email already registered", http.StatusBadRequest)
        return
    }

    // Хэширование пароля
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
    if err != nil {
        http.Error(w, "Failed to hash password", http.StatusInternalServerError)
        return
    }
    user.Password = string(hashedPassword)
    user.Verified = false
    user.CreatedAt = time.Now()

    // Сохранение пользователя
    if err := db.Create(&user).Error; err != nil {
        http.Error(w, "Failed to register user", http.StatusInternalServerError)
        return
    }

    // Генерация токена подтверждения
    token := fmt.Sprintf("%x", time.Now().UnixNano())
    verificationToken := VerificationToken{
        UserID:    user.ID,
        Token:     token,
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(24 * time.Hour),
    }

    if err := db.Create(&verificationToken).Error; err != nil {
        http.Error(w, "Failed to create verification token", http.StatusInternalServerError)
        return
    }

    // Отправка email
    go sendVerificationEmail(user.Email, token)

    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{"message": "Registration successful. Check your email for verification."})
}

func sendVerificationEmail(toEmail, token string) {
    subject := "Email Verification"
    message := fmt.Sprintf("Please verify your email by clicking the link: http://localhost:8080/verify?token=%s", token)
    if err := sendEmail(toEmail, subject, message); err != nil {
        logger.WithError(err).Error("Failed to send verification email")
    }
}

func sendEmail(toEmail, subject, message string) error {
    from := "bouquiniste19@gmail.com"
    password := "fjnzynihbertfxye"
    smtpHost := "smtp.gmail.com"
    smtpPort := "587"

    auth := smtp.PlainAuth("", from, password, smtpHost)
    body := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", toEmail, subject, message)
    return smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, []byte(body))
}

func verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
    token := r.URL.Query().Get("token")
    if token == "" {
        http.Error(w, "Token is required", http.StatusBadRequest)
        return
    }

    var verificationToken VerificationToken
    if err := db.Where("token = ?", token).First(&verificationToken).Error; err != nil {
        http.Error(w, "Invalid or expired token", http.StatusBadRequest)
        return
    }

    if verificationToken.ExpiresAt.Before(time.Now()) {
        http.Error(w, "Token expired", http.StatusBadRequest)
        return
    }

    // Обновление статуса пользователя
    if err := db.Model(&User{}).Where("id = ?", verificationToken.UserID).Update("verified", true).Error; err != nil {
        http.Error(w, "Failed to verify email", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{"message": "Email verified successfully."})
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
