package main

import (
	"context"
	"crypto/rand"
	"log"
	"math/big"
	"strings"

	"encoding/json"
	"fmt"
	"io"

	"os"
	"os/signal"
	"syscall"
	"time"

	"text/template"

	"mime/multipart"
	"net/http"
	"net/smtp"

	"github.com/golang-jwt/jwt/v4"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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
	Timestamp time.Time `json:"timestamp"` 
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}
type User struct {
	ID               uint      `gorm:"primaryKey" json:"id"`
	Name             string    `json:"name"`
	Email            string    `gorm:"unique" json:"email"`
	PasswordHash     string    `json:"-"`
	Role             string    `json:"role"`
	Confirmed        bool      `json:"confirmed"`
	VerificationToken string   `gorm:"unique" json:"verification_token"`
	CreatedAt        time.Time `json:"created_at"`
}
type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

var db *gorm.DB
var err error
var logger = logrus.New()
var jwtKey = []byte("my_secret_key")

var limiter = rate.NewLimiter(1, 5) 

// OAUTH для логина
var googleOauthConfig = &oauth2.Config{
	ClientID:     "YOUR_GOOGLE_CLIENT_ID",
	ClientSecret: "YOUR_GOOGLE_CLIENT_SECRET",
	RedirectURL:  "http://localhost:8080/auth/google/callback",
	Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
	Endpoint:     google.Endpoint,
}

// func initDB() {
// 	db, err = gorm.Open(sqlite.Open("./books.db"), &gorm.Config{})
// 	if err != nil {
// 		logger.WithError(err).Fatal("failed to connect to the database")
// 	}

// 	err = db.AutoMigrate(&Book{})
// 	if err != nil {
// 		logger.WithError(err).Fatal("failed to connect to the database")
// 	}

// 	err = db.AutoMigrate(&Fantasy{})
// 	if err != nil {
// 		logger.WithError(err).Fatal("failed to connect to the database")
// 	}
// 	// Миграция для таблицы User
// 	err = db.AutoMigrate(&User{})
// 	if err != nil {
// 		logger.WithError(err).Fatal("failed to migrate User table")
// 	}
// }

func initDB() {
    dbPath := "/var/data/books.db" // Render-friendly путь

    // ✅ Создаём директорию, если её нет
    if err := os.MkdirAll("/var/data", os.ModePerm); err != nil {
        log.Fatal("❌ Ошибка при создании папки /var/data:", err)
    }

    // ✅ Проверяем, существует ли файл базы
    if _, err := os.Stat(dbPath); os.IsNotExist(err) {
        fmt.Println("📂 База данных не найдена, создаём новую:", dbPath)
    }

    // ✅ Подключаемся к базе
    var err error
    db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
    if err != nil {
        log.Fatal("❌ Ошибка подключения к базе данных:", err)
    }

    fmt.Println("📡 Подключено к базе данных:", dbPath)

    // ✅ Выполняем миграции
    if err := db.AutoMigrate(&Book{}, &Fantasy{}, &User{}); err != nil {
        log.Fatal("❌ Ошибка миграции базы данных:", err)
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
		"test":   "logging",
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
	
	mux.HandleFunc("/fantasy", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.ServeFile(w, r, "fantasy.html")
		} else if r.Method == http.MethodPost {
			getFantasyBooks(w, r) 
		}
	})
	mux.HandleFunc("/bouquiniste", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "bouquiniste.html")
	})
	mux.HandleFunc("/account", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "account.html")
	})
	
	mux.Handle("/api/profile", authMiddleware(http.HandlerFunc(profileHandler)))

	mux.Handle("/admin", roleMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "profile.html")
	}), "admin"))
	
	mux.HandleFunc("/check_country", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "Country check success!"})
	})
	


	mux.HandleFunc("/books", getBooks)                 
	mux.HandleFunc("/books/add", addBook)              
	mux.HandleFunc("/books/update", updateBook)       
	mux.HandleFunc("/books/delete", deleteBook)       
	mux.HandleFunc("/books/search", getBookByID)       
	mux.HandleFunc("/send-message", handleSendMessage) 
	mux.HandleFunc("/register", registerHandler)
	mux.HandleFunc("/verify", verifyEmailHandler)
	mux.HandleFunc("/login", loginHandler)


	rateLimitedMux := rateLimitMiddleware(mux)

	port := os.Getenv("PORT") // Берём порт из Render
	if port == "" {
    	port = "8080" // По умолчанию 8080, если Render не передал порт
	}

	srv := &http.Server{
    	Addr:    ":" + port,
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
	<-quit
	logger.Info("Server is shutting down...")
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
	// Если книги не найдены
	if len(books) == 0 {
		http.Error(w, "No books match the filter criteria", http.StatusNotFound)
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
	// Проверяем, есть ли книга с таким же названием
	var existingBook Book
	if err := db.Where("title = ?", book.Title).First(&existingBook).Error; err == nil {
		http.Error(w, "A book with this title already exists", http.StatusConflict)
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
	password := "fjnzynihbertfxye"    // Пароль от email
	smtpHost := "smtp.gmail.com"      // SMTP-сервер
	smtpPort := "587"                 // Порт SMTP

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

//------------ РЕГИСТРАЦИЯ И АВТОРИЗАЦИЯ

func registerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Запрос на /register:", r.Method)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name     string `json:"name"`
		Email    string `json:"email"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		fmt.Println("Ошибка декодирования JSON:", err)
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Role != "admin" {
        req.Role = "user" // По умолчанию все пользователи - user
    }

	fmt.Println("Полученные данные:", req)

	if req.Name == "" || req.Email == "" || req.Password == "" {
		fmt.Println("Ошибка: не все поля заполнены")
		http.Error(w, "All fields are required", http.StatusBadRequest)
		return
	}

	var user User
	err := db.Where("email = ?", req.Email).First(&user).Error

	if err == nil {
		// Email уже есть в БД
		fmt.Println("⚠ Email уже зарегистрирован:", req.Email)

		if !user.Confirmed {
			user.VerificationToken = generateVerificationCode()
			db.Model(&user).Update("verification_token", user.VerificationToken)
		
			fmt.Println("📩 Повторная отправка верификационного письма:", req.Email, "Код:", user.VerificationToken)
			go sendVerificationEmail(user.Email, user.VerificationToken)
		
			json.NewEncoder(w).Encode(map[string]string{"message": "User already exists. Verification email resent."})
			return
		}
		

		// Если email подтверждён, просто возвращаем сообщение
		json.NewEncoder(w).Encode(map[string]string{"message": "User already exists and is verified."})
		return
	}

	// Если email не найден, создаём нового пользователя
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("Ошибка хеширования пароля:", err)
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	verificationToken := generateVerificationCode()

	user = User{
		Name:             req.Name,
		Email:            req.Email,
		PasswordHash:     string(passwordHash),
		Role:             req.Role, // Сохраняем роль
		Confirmed:        false,
		VerificationToken: verificationToken,
		CreatedAt:        time.Now(),
	}

	fmt.Println("🛠 Сохраняем пользователя в БД:", user) // Логируем перед сохранением

	if err := db.Create(&user).Error; err != nil {
		fmt.Println("Ошибка создания пользователя:", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	fmt.Println("Пользователь добавлен!")

	fmt.Println("📩 Отправка письма верификации для:", req.Email, "Код:", verificationToken)
	go sendVerificationEmail(req.Email, verificationToken)

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"message": "User registered. Check your email for verification link."})
}


func verifyEmailHandler(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Token is required", http.StatusBadRequest)
		return
	}

	var user User
	if err := db.Where("verification_token = ?", token).First(&user).Error; err != nil {
		http.Error(w, "Invalid or expired token", http.StatusBadRequest)
		return
	}

	user.Confirmed = true
	user.VerificationToken = ""
	db.Save(&user)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Email verified successfully. You can now log in.")
}

func generateVerificationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	code := 1000 + int(n.Int64()) // Гарантируем 4 цифры
	return fmt.Sprintf("%04d", code) // Преобразуем в строку с ведущими нулями, если нужно
}

func sendVerificationEmail(to, token string) {
	
	from := "bouquiniste19@gmail.com" // почта и пароль
	password := "fjnzynihbertfxye" 
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	subject := "Email Verification"
	link := fmt.Sprintf("http://localhost:8080/verify?token=%s", token)
	message := fmt.Sprintf("Click the link to verify your email: %s", link)

	auth := smtp.PlainAuth("", from, password, smtpHost)
	msg := []byte("Subject: " + subject + "\r\n" + "\r\n" + message)

	fmt.Println("Отправка письма на:", to)

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, msg)
	if err != nil {
		fmt.Println("Ошибка отправки Email:", err)
	} else {
		fmt.Println("Письмо успешно отправлено!")
	}
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }

    var user User
    err := db.Where("email = ?", req.Email).First(&user).Error
    if err != nil {
        http.Error(w, "User not found", http.StatusUnauthorized)
        return
    }

    if !user.Confirmed {
        http.Error(w, "Email not verified", http.StatusForbidden)
        return
    }

    if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
        http.Error(w, "Invalid password", http.StatusUnauthorized)
        return
    }

    token, err := generateJWT(user)
    if err != nil {
        http.Error(w, "Failed to generate token", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// func loginHandler(w http.ResponseWriter, r *http.Request) {
//     if r.Method != http.MethodPost {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusMethodNotAllowed)
//         json.NewEncoder(w).Encode(map[string]string{"error": "Method not allowed"})
//         return
//     }

//     var req struct {
//         Email    string `json:"email"`
//         Password string `json:"password"`
//     }

//     if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusBadRequest)
//         json.NewEncoder(w).Encode(map[string]string{"error": "Invalid request"})
//         return
//     }

//     var user User
//     err := db.Where("email = ?", req.Email).First(&user).Error
//     if err != nil {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusUnauthorized)
//         json.NewEncoder(w).Encode(map[string]string{"error": "User not found"})
//         return
//     }

//     if !user.Confirmed {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusForbidden)
//         json.NewEncoder(w).Encode(map[string]string{"error": "Email not verified"})
//         return
//     }

//     if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusUnauthorized)
//         json.NewEncoder(w).Encode(map[string]string{"error": "Invalid password"})
//         return
//     }

//     token, err := generateJWT(user)
//     if err != nil {
//         w.Header().Set("Content-Type", "application/json")
//         w.WriteHeader(http.StatusInternalServerError)
//         json.NewEncoder(w).Encode(map[string]string{"error": "Failed to generate token"})
//         return
//     }

//     w.Header().Set("Content-Type", "application/json")
//     json.NewEncoder(w).Encode(map[string]string{"token": token})
// }


func generateJWT(user User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour) // Токен действует 24 часа
	claims := &Claims{
		Email: user.Email,
		Role:  user.Role, // Роль пользователя (admin или user)
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime), // Время истечения
		},
	}

	fmt.Println("🛠 Генерируем токен для:", user.Email, "Роль:", user.Role)

	// Генерируем токен
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey) // jwtKey — секретный ключ
}



func profileHandler(w http.ResponseWriter, r *http.Request) {
    user := r.Context().Value("user").(*Claims)
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]string{
        "message": "Welcome, " + user.Email,
        "role":    user.Role,
    })
}


func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        tokenStr := r.Header.Get("Authorization")
        fmt.Println("🔍 Получен заголовок Authorization:", tokenStr) // ЛОГ ДЛЯ ОТЛАДКИ

        if tokenStr == "" {
            http.Error(w, "Unauthorized: No token", http.StatusUnauthorized)
            return
        }

        // Убираем "Bearer " из строки токена
        parts := strings.Split(tokenStr, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            fmt.Println("❌ Неправильный формат заголовка Authorization")
            http.Error(w, "Unauthorized: Invalid token format", http.StatusUnauthorized)
            return
        }
        tokenStr = parts[1]

        claims := &Claims{}
        token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
            return jwtKey, nil
        })

        if err != nil || !token.Valid {
            fmt.Println("❌ Ошибка парсинга токена:", err) // ЛОГ ОШИБКИ
            http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
            return
        }

        fmt.Println("✅ Токен успешно распознан. Пользователь:", claims.Email) // ЛОГ УСПЕХА
        ctx := context.WithValue(r.Context(), "user", claims)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}


func roleMiddleware(next http.Handler, requiredRole string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		role := r.Header.Get("Role") // Заголовок с ролью пользователя
		if role != requiredRole {
			http.Error(w, "Forbidden: Access is denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
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
