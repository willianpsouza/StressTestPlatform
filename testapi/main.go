package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var (
	names    = []string{"Alice", "Bob", "Carlos", "Diana", "Eduardo", "Fernanda", "Gabriel", "Helena"}
	cities   = []string{"São Paulo", "Rio de Janeiro", "Curitiba", "Belo Horizonte", "Porto Alegre", "Salvador", "Brasília", "Recife"}
	products = []string{"Notebook", "Smartphone", "Tablet", "Monitor", "Teclado", "Mouse", "Headset", "Webcam"}
	statuses = []string{"active", "pending", "completed", "cancelled"}
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "service": "testapi", "port": "8089"})
	})

	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		n := rand.Intn(5) + 1
		users := make([]map[string]interface{}, n)
		for i := range users {
			users[i] = map[string]interface{}{
				"id":    rand.Intn(10000),
				"name":  names[rand.Intn(len(names))],
				"email": fmt.Sprintf("user%d@test.com", rand.Intn(1000)),
				"city":  cities[rand.Intn(len(cities))],
				"age":   rand.Intn(50) + 18,
			}
		}
		writeJSON(w, users)
	})

	r.Get("/api/products", func(w http.ResponseWriter, r *http.Request) {
		n := rand.Intn(8) + 1
		items := make([]map[string]interface{}, n)
		for i := range items {
			items[i] = map[string]interface{}{
				"id":    rand.Intn(10000),
				"name":  products[rand.Intn(len(products))],
				"price": float64(rand.Intn(50000)) / 100,
				"stock": rand.Intn(200),
			}
		}
		writeJSON(w, items)
	})

	r.Get("/api/orders", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]interface{}{
			"order_id": rand.Intn(100000),
			"customer": names[rand.Intn(len(names))],
			"product":  products[rand.Intn(len(products))],
			"quantity": rand.Intn(10) + 1,
			"total":    float64(rand.Intn(100000)) / 100,
			"status":   statuses[rand.Intn(len(statuses))],
		})
	})

	r.Get("/api/slow", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Duration(rand.Intn(500)+100) * time.Millisecond)
		writeJSON(w, map[string]interface{}{
			"message":  "slow response",
			"delay_ms": rand.Intn(500) + 100,
		})
	})

	r.Post("/api/echo", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, body)
	})

	fmt.Println("Test API running on :8089")
	http.ListenAndServe(":8089", r)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
