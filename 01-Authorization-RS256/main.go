package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	auth0 "github.com/auth0-community/go-auth0"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	jose "gopkg.in/square/go-jose.v2"
	jwt "gopkg.in/square/go-jose.v2/jwt"
)

var JWKS_URI string
var AUTH0_API_ISSUER string
var AUTH0_API_AUDIENCE []string

type Response struct {
	Message string `json:"message"`
}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	JWKS_URI = os.Getenv("JWKS_URI")
	AUTH0_API_ISSUER = os.Getenv("AUTH0_API_ISSUER")
	AUTH0_API_AUDIENCE = []string{os.Getenv("AUTH0_API_AUDIENCE")}

	fmt.Println("JWKS_URI: ", JWKS_URI)
	fmt.Println("AUTH0_API_ISSUER: ", AUTH0_API_ISSUER)
	fmt.Println("AUTH0_API_AUDIENCE: ", AUTH0_API_AUDIENCE)

	r := mux.NewRouter()

	// This route is always accessible
	r.Handle("/api/public", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Response{
			Message: "Hello from a public endpoint! You don't need to be authenticated to see this.",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}))

	// This route is only accessible if the user has a valid access_token with the read:messages scope
	// We are wrapping the checkJwt middleware around the handler function which will check for a
	// valid token and scope.
	r.Handle("/api/private", checkJwt(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Response{
			Message: "Hello from a private endpoint! You need to be authenticated and have a scope of read:messages to see this.",
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)

	})))

	http.ListenAndServe(":3001", r)
	fmt.Println("Listening on http://localhost:3001")
}

func checkJwt(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		client := auth0.NewJWKClient(auth0.JWKClientOptions{URI: JWKS_URI})
		audience := AUTH0_API_AUDIENCE

		configuration := auth0.NewConfiguration(client, audience, AUTH0_API_ISSUER, jose.RS256)
		validator := auth0.NewValidator(configuration)

		token, err := validator.ValidateRequest(r)

		if err != nil {
			fmt.Println("Token is not valid or missing token")

			response := Response{
				Message: "Missing or invalid token.",
			}

			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(response)

		} else {
			// Ensure the token has the correct scope
			result := checkScope(r, validator, token)
			if result == true {
				// If the token is valid and we have the right scope, we'll pass through the middleware
				h.ServeHTTP(w, r)
			} else {
				response := Response{
					Message: "You do not have the read:messages scope.",
				}
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(response)

			}
		}
	})
}

func checkScope(r *http.Request, validator *auth0.JWTValidator, token *jwt.JSONWebToken) bool {
	claims := map[string]interface{}{}
	err := validator.Claims(r, token, &claims)

	if err != nil {
		fmt.Println(err)
		return false
	}

	if strings.Contains(claims["scope"].(string), "read:messages") {
		return true
	} else {
		return false
	}
}
