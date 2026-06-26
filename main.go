package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type apiConfig struct {
	fileServerHits atomic.Int32
}

func (cfg *apiConfig) middleWareMetricsInc(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func main() {

	var cfg = &apiConfig{}

	mux := http.NewServeMux()
	mux.Handle("/app/", http.StripPrefix("/app", cfg.middleWareMetricsInc(http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /api/healthz", handlerHealth)
	mux.HandleFunc("GET /admin/metrics", cfg.handlercount)
	mux.HandleFunc("POST /admin/reset", cfg.handlerResetcount)
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal(err)
	}

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
	cfg.fileServerHits.Store(0)
}
