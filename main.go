package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/grma16021/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

func (cfg *apiConfig) middleWareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {
	godotenv.Load()

	dbURL := os.Getenv("DB_URL")

	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	var cfg = apiConfig{
		fileServerHits: atomic.Int32{},
		db:             dbQueries,
	}

	mux := http.NewServeMux()
	mux.Handle("/app/", http.StripPrefix("/app", cfg.middleWareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", handlerHealth)
	mux.HandleFunc("GET /admin/metrics", cfg.handlercount)
	mux.HandleFunc("POST /admin/reset", cfg.handlerResetcount)
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)
	mux.HandleFunc("POST /api/users", cfg.handlerRegisterUser)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

func (cfg *apiConfig) handlerRegisterUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}
	Dbuser, err := cfg.db.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error Creating user: %s", err)
		w.WriteHeader(500)
		return
	}

	apiUser := User{
		ID:        Dbuser.ID,
		CreatedAt: Dbuser.CreatedAt.Time,
		UpdatedAt: Dbuser.UpdatedAt.Time,
		Email:     Dbuser.Email,
	}

	respBody := apiUser

	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshaling json: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	w.Write(dat)

}

func handlerValidateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Test string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Test) <= 140 {

		clean := cleanString(params.Test)

		type returnVals struct {
			Cleaned_body string `json:"cleaned_body"`
		}

		respBody := returnVals{
			Cleaned_body: clean,
		}

		dat, err := json.Marshal(respBody)
		if err != nil {
			log.Printf("Error marshaling json: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write(dat)
	} else {
		type returnVals struct {
			Error string `json:"error"`
		}

		respBody := returnVals{
			Error: "Chirp is too long",
		}

		dat, err := json.Marshal(respBody)
		if err != nil {
			log.Printf("Error marshalling json: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		w.Write(dat)
	}

}

func cleanString(s string) string {
	//lowerString := strings.ToLower(s)

	splitStr := strings.Split(s, " ")

	cleanedSplit := []string{}

	for _, word := range splitStr {
		if strings.ToLower(word) == "kerfuffle" || strings.ToLower(word) == "sharbert" || strings.ToLower(word) == "fornax" {
			cleanedSplit = append(cleanedSplit, "****")
			word = "****"
		} else {
			cleanedSplit = append(cleanedSplit, word)
		}
	}
	cleanedString := strings.Join(cleanedSplit, " ")
	return cleanedString
}

func handlerHealth(res http.ResponseWriter, req *http.Request) {
	res.Header().Set("Content-Type", "text/plain; charset=utf-8")
	res.WriteHeader(http.StatusOK)
	res.Write([]byte(http.StatusText(http.StatusOK)))
}

func (cfg *apiConfig) handlercount(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/html")
	w.Write([]byte(fmt.Sprintf(`
	<html> 
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
	 	</body>
	</html> 
	`, cfg.fileServerHits.Load())))
}

func (cfg *apiConfig) handlerResetcount(w http.ResponseWriter, r *http.Request) {
	platform := os.Getenv("PLATFORM")

	if platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(http.StatusText(http.StatusForbidden)))
		return
	}
	cfg.fileServerHits.Store(0)
	err := cfg.db.DeleteUser(r.Context())
	if err != nil {
		log.Printf("Error deleting users: %s", err)
		w.WriteHeader(500)
		return
	}
}
