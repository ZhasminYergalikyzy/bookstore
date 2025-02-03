package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type RequestData struct {
	Message string `json:"message"`
}

func main() {
	http.HandleFunc("/post", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var requestData map[string]interface{}

			// Декодируем JSON
			err := json.NewDecoder(r.Body).Decode(&requestData)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "fail",
					"message": "Некорректное JSON-сообщение",
				})
				return
			}

			// Проверяем наличие и тип поля "message"
			message, ok := requestData["message"].(string)
			if !ok  {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{
					"status":  "fail",
					"message": "Некорректное JSON-сообщение",
				})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]string{
				"status":  "success",
				"message": "Данные успешно приняты",
			})
			fmt.Println("Получено сообщение:", message)
		}
	})

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

	fmt.Println("Сервер запущен на порту 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
