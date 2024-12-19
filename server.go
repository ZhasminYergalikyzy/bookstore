package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Структура для обработки JSON данных
type RequestData struct {
	Message string `json:"message"`
}

func main() {
	// Обработчик для POST запроса
	http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var requestData RequestData

			// Декодируем JSON из тела запроса
			err := json.NewDecoder(r.Body).Decode(&requestData)
			if err != nil {
				// В случае ошибки в JSON данных
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "fail",
					"message": "Некорректное JSON-сообщение",
				})
				return
			}

			// Если данные корректные, отправляем успешный ответ
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "success",
				"message": "Данные успешно приняты",
			})
			fmt.Println("Получено сообщение:", requestData.Message)
		}
	})

	// Обработчик для GET запроса
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			response := map[string]string{
				"status":  "success",
				"message": "Это GET запрос",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		}
	})

	// Запуск сервера на порту 8080
	fmt.Println("Сервер запущен на порту 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
