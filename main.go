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
	"github.com/grma16021/chirpy/internal/auth"
	"github.com/grma16021/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileServerHits atomic.Int32
	db             *database.Queries
	secret         string
}

type User struct {
	ID           uuid.UUID `json:"id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Email        string    `json:"email"`
	Password     string    `json:"password"`
	Token        string    `json:"token"`
	RefreshToken string    `json:"refresh_token"`
	IsChirpyRed  bool      `json:"is_chirpy_red"`
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
	JWTsecret := os.Getenv("SECRET")

	db, err := sql.Open("postgres", dbURL)
	dbQueries := database.New(db)

	var cfg = apiConfig{
		fileServerHits: atomic.Int32{},
		db:             dbQueries,
		secret:         JWTsecret,
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
	mux.HandleFunc("POST /api/login", cfg.handlerLogin)
	mux.HandleFunc("POST /api/refresh", cfg.handlerRefresh)
	mux.HandleFunc("POST /api/revoke", cfg.handlerRevoke)
	mux.HandleFunc("PUT /api/users", cfg.handlerUpdateUsers)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handlerDeleteChirp)
	mux.HandleFunc("POST /api/polka/webhooks", cfg.handlerUpgardeuser)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err = server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

}

func (cfg *apiConfig) handlerUpgardeuser(w http.ResponseWriter, r *http.Request) {
	type Parameters struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}
	params := Parameters{}
	decoder := json.NewDecoder(r.Body)

	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("error decoding json: %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Event != "user.upgraded" {
		w.WriteHeader(204)
		return
	}

	userID := params.Data.UserID

	_, err = cfg.db.UpgradeUser(r.Context(), userID)
	if err != nil {
		log.Printf("error upgrading user: %s", err)
		w.WriteHeader(404)
		return
	}

	w.WriteHeader(204)

}

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, r *http.Request) {
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("error getting token: %s", err)
		w.WriteHeader(401)
		return
	}

	validToken, err := auth.ValidateJWT(bearerToken, cfg.secret)
	if err != nil {
		log.Printf("error validating token: %s", err)
		w.WriteHeader(403)
		return
	}

	chirpIDSTR := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDSTR)
	if err != nil {
		log.Printf("error parsing uuid: %s", err)
		w.WriteHeader(500)
		return
	}

	chirp, err := cfg.db.GetChirpById(r.Context(), chirpID)
	if err != nil {
		log.Printf("error getting chirp from DB: %s", err)
		w.WriteHeader(404)
		return
	}

	if validToken != chirp.UserID {
		log.Printf("error user id's do not match")
		w.WriteHeader(403)
		return
	}

	err = cfg.db.DeleteChirpByID(r.Context(), chirpID)
	if err != nil {
		log.Printf("error deleting chirp: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(204)
}

func (cfg *apiConfig) handlerUpdateUsers(w http.ResponseWriter, r *http.Request) {
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting token: %s", err)
		w.WriteHeader(401)
		return
	}

	validToken, err := auth.ValidateJWT(bearerToken, cfg.secret)
	if err != nil {
		log.Printf("Error validating JWT token %s", err)
		w.WriteHeader(401)
		return
	}

	type Parameters struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	params := Parameters{}

	decoder := json.NewDecoder(r.Body)

	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Email == "" || params.Password == "" {
		log.Printf("error user email and password is required")
		w.WriteHeader(400)
		return
	}

	hashedPass, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("error hashing password: %s", err)
		w.WriteHeader(500)
		return
	}

	updateUserParams := database.UpdateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPass,
		ID:             validToken,
	}

	updatedUser, err := cfg.db.UpdateUser(r.Context(), updateUserParams)
	if err != nil {
		log.Printf("Erro updating user: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnParams struct {
		Email       string `json:"email"`
		IsChirpyRed bool   `json:"is_chirpy_red"`
	}

	apiUser := returnParams{
		Email:       updatedUser.Email,
		IsChirpyRed: updatedUser.IsChirpyRed,
	}

	dat, err := json.Marshal(apiUser)
	if err != nil {
		log.Printf("error marshalling json")
		w.WriteHeader(500)

	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, r *http.Request) {
	bearerToken, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting token: %s", err)
		w.WriteHeader(400)
		return
	}

	_, err = cfg.db.SetRevoked(r.Context(), bearerToken)
	if err != nil {
		log.Printf("Error revoking token: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(204)
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting token In refresh endpoint:  %s", err)
		w.WriteHeader(400)
		return
	}

	DBtoken, err := cfg.db.GetToken(r.Context(), token)
	if err != nil {
		log.Printf("Error getting token from db: %s", err)
		w.WriteHeader(401)
		return
	} else if DBtoken.ExpiresAt.Before(time.Now()) {
		log.Printf("Error token expired")
		w.WriteHeader(401)
		return
	} else if DBtoken.RevokedAt.Valid != false {
		log.Printf("Error token was revoked")
		w.WriteHeader(401)
		return
	}

	user, err := cfg.db.GetUserByID(r.Context(), DBtoken.UserID)

	JWTTOKEN, err := auth.MakeJWT(user.ID, cfg.secret, time.Duration(3600)*time.Second)

	if err != nil {
		log.Printf("error creating JWT token: %s", err)
		w.WriteHeader(500)
		return
	}

	type returnParams struct {
		Token string `json:"token"`
	}

	APIReturn := returnParams{
		Token: JWTTOKEN,
	}

	dat, err := json.Marshal(APIReturn)
	if err != nil {
		log.Printf("Error marshaling json: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)

}

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Password         string `json:"password"`
		Email            string `json:"email"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.db.GetUserByEmail(r.Context(), params.Email)

	match, err := auth.CheckPasswordHash(params.Password, user.HashedPassword)
	if err != nil {
		log.Printf("error validating password: %s", err)
		w.WriteHeader(500)
		return
	}

	if match != true {
		log.Printf("passwords do not match")
		w.WriteHeader(401)
		w.Write([]byte("Incorrect email or password"))
	}

	if params.ExpiresInSeconds == 0 || params.ExpiresInSeconds >= 3600 {
		params.ExpiresInSeconds = 3600
	}

	JWTToken, err := auth.MakeJWT(user.ID, cfg.secret, time.Duration(params.ExpiresInSeconds)*time.Second)
	if err != nil {
		log.Printf("error making JWT token %s", err)
		w.WriteHeader(500)
		return
	}

	JWTRefreshToken := auth.MakeRefreshToken()
	refreshExpireTimeStamp := time.Now().Add(time.Duration(1440) * time.Hour)
	DBrefreshToken := database.CreateRefreshTokenParams{
		Token:     JWTRefreshToken,
		UserID:    user.ID,
		ExpiresAt: refreshExpireTimeStamp,
	}

	cfg.db.CreateRefreshToken(r.Context(), DBrefreshToken)

	apiUser := User{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt.Time,
		UpdatedAt:    user.UpdatedAt.Time,
		Email:        user.Email,
		Token:        JWTToken,
		RefreshToken: JWTRefreshToken,
		IsChirpyRed:  user.IsChirpyRed,
	}

	dat, err := json.Marshal(apiUser)
	if err != nil {
		log.Printf("error marshaling json: %s", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(dat)
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
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if params.Email == "" || params.Password == "" {
		w.WriteHeader(400)
		w.Write([]byte("Error email and password must be provided"))
	}

	hashedPass, err := auth.HashPassword(params.Password)
	if err != nil {
		log.Printf("Error Hashing password: %s", err)
		w.WriteHeader(500)
		return
	}

	CreateUser := database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashedPass,
	}

	Dbuser, err := cfg.db.CreateUser(r.Context(), CreateUser)
	if err != nil {
		log.Printf("Error Creating user: %s", err)
		w.WriteHeader(500)
		return
	}

	apiUser := User{
		ID:          Dbuser.ID,
		CreatedAt:   Dbuser.CreatedAt.Time,
		UpdatedAt:   Dbuser.UpdatedAt.Time,
		Email:       Dbuser.Email,
		IsChirpyRed: Dbuser.IsChirpyRed,
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

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		log.Printf("Error getting token: %s", err)
		w.WriteHeader(400)
		return
	}

	validUserID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		log.Printf("error validating token: %s", err)
		w.WriteHeader(401)
		return
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err = decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(params.Body) <= 140 {
		clean := cleanString(params.Body)

		chirpParams := database.CreateChirpParams{
			Body:   clean,
			UserID: validUserID,
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
