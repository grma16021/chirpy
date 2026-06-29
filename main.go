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

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserId    uuid.UUID `json:"user_id"`
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
	mux.HandleFunc("POST /api/users", cfg.handlerRegisterUser)
	mux.HandleFunc("POST /api/chirps", cfg.handlerPostChirp)
	mux.HandleFunc("GET /api/chirps", cfg.handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handlerGetChirp)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {

	chripIDSTR := r.PathValue("chirpID")

	chirpID, err := uuid.Parse(chripIDSTR)
	if err != nil {
		log.Printf("Error parsing id: %s", err)
		w.WriteHeader(500)
		return
	}

	chirp, err := cfg.db.GetChirpById(r.Context(), chirpID)
	if err != nil {
		log.Printf("Error fetching chirp: %s", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	apiChirp := Chirp{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt.Time,
		UpdatedAt: chirp.UpdatedAt.Time,
		Body:      chirp.Body,
		UserId:    chirp.UserID,
	}

	dat, err := json.Marshal(apiChirp)
	if err != nil {
		log.Printf("Error marshalling json: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	chirps, err := cfg.db.GetAllChrips(r.Context())
	if err != nil {
		log.Printf("Error fetching chirps: %s", err)
		w.WriteHeader(500)
		return
	}

	chirpArray := []Chirp{}

	for _, chirp := range chirps {
		apiChirp := Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt.Time,
			UpdatedAt: chirp.UpdatedAt.Time,
			Body:      chirp.Body,
			UserId:    chirp.UserID,
		}

		chirpArray = append(chirpArray, apiChirp)
	}

	respBody := chirpArray

	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("error marshaling json: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
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

func (cfg *apiConfig) handlerPostChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body    string    `json:"body"`
		User_id uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) <= 140 {
		clean := cleanString(params.Body)

		chirpParams := database.CreateChirpParams{
			Body:   clean,
			UserID: params.User_id,
		}

		DbChirp, err := cfg.db.CreateChirp(r.Context(), chirpParams)
		if err != nil {
			log.Printf("Error Creating chirp: %s", err)
			w.WriteHeader(500)
			return
		}

		apiChirp := Chirp{
			ID:        DbChirp.ID,
			CreatedAt: DbChirp.CreatedAt.Time,
			UpdatedAt: DbChirp.UpdatedAt.Time,
			Body:      DbChirp.Body,
			UserId:    DbChirp.UserID,
		}

		respBody := apiChirp

		dat, err := json.Marshal(respBody)
		if err != nil {
			log.Printf("Error marshalling json: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
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
