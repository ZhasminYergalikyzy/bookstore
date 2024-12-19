package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv" // Для преобразования строки в int

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

var db *sql.DB

// Модель пользователя
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Инициализация соединения с базой данных
func init() {
	var err error
	connStr := "user=zhasmin password=062442 dbname=bookstore sslmode=disable"
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Connected to the database")
}

// Создание нового пользователя
func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	err := json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Вставка нового пользователя в базу данных
	sqlStatement := `INSERT INTO users (name, email) VALUES ($1, $2) RETURNING id`
	err = db.QueryRow(sqlStatement, user.Name, user.Email).Scan(&user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// Получение всех пользователей
func getUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name, email FROM users")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.Name, &user.Email); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(users)
}

// Редактирование пользователя
func updateUser(w http.ResponseWriter, r *http.Request) {
	var user User
	vars := mux.Vars(r)
	id := vars["id"]

	// Преобразуем id из строки в int
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Декодируем запрос с новыми данными
	err = json.NewDecoder(r.Body).Decode(&user)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Обновление данных пользователя в базе
	sqlStatement := `UPDATE users SET name=$1, email=$2 WHERE id=$3`
	_, err = db.Exec(sqlStatement, user.Name, user.Email, idInt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Возвращаем обновленные данные
	user.ID = idInt
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(user)
}

// Удаление пользователя
func deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	// Преобразуем id из строки в int
	idInt, err := strconv.Atoi(id)
	if err != nil {
		http.Error(w, "Invalid ID", http.StatusBadRequest)
		return
	}

	// Удаляем пользователя из базы данных
	sqlStatement := `DELETE FROM users WHERE id=$1`
	_, err = db.Exec(sqlStatement, idInt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "User with ID %d was deleted", idInt)
}

func main() {
	r := mux.NewRouter()

	// Маршруты
	r.HandleFunc("/create", createUser).Methods("POST")
	r.HandleFunc("/users", getUsers).Methods("GET")
	r.HandleFunc("/users/{id}", updateUser).Methods("PUT")
	r.HandleFunc("/users/{id}", deleteUser).Methods("DELETE")

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
