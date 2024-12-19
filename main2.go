package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// Структура для получения данных из POST-запроса
type BuyRequest struct {
	BookID string `json:"book_id"`
}

func main() {
	http.HandleFunc("/buy", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("Received POST request on /buy")
		
		if r.Method == http.MethodPost {
			var req BuyRequest

			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				http.Error(w, "Invalid JSON format", http.StatusBadRequest)
				return
			}

		
			fmt.Printf("Book with ID %s added to the cart.\n", req.BookID)

			
			response := map[string]string{
				"status":  "success",
				"message": "Book added to the cart",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		}
	})

	
	log.Fatal(http.ListenAndServe(":8080", nil))
}
