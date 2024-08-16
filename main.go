// want to have db so that is "reset" each day
// must put in name to see what other people
// have said yes
// main.go
package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

type Player struct {
	Name string
	Date string
}

type PageData struct {
	Players  []Player
	Date     string
	Message  string
	BasePath string
}

var db *sql.DB
var templates *template.Template

func main() {
	var err error
	db, err = sql.Open("sqlite3", "./basketball.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	go resetDatabaseDaily()
	createTable()

	templates = template.Must(template.ParseFiles("index.html", "players.html"))

	r := mux.NewRouter()

	// Get the base path from environment variable or use "/"
	basePath := os.Getenv("BASE_PATH")
	if basePath == "" {
		basePath = "/"
	}

	r.HandleFunc("/", homeHandler).Methods("GET")
	r.HandleFunc("/submit", submitHandler).Methods("POST")
	r.HandleFunc("/players", playersHandler).Methods("GET")

	fmt.Println("Server is running on http://localhost:8098")
	log.Fatal(http.ListenAndServe(":8098", r))
}

func createTable() {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS players (
			name TEXT,
			date TEXT,
			UNIQUE(name, date)
		)
	`)
	if err != nil {
		log.Fatal(err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	basePath := os.Getenv("BASE_PATH")
	if basePath == "" {
		basePath = "/"
	}

	data := PageData{
		BasePath: basePath,
	}

	err := templates.ExecuteTemplate(w, "index.html", data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func submitHandler(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	date := time.Now().Format("2006-01-02")

	fmt.Println("got name", name)
	// Check if the name already exists for the current date
	var exists bool
	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM players WHERE name = ? AND date = ?)", name, date).Scan(&exists)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var message string
	if exists {
		message = fmt.Sprintf("%s is already signed up for today!", name)
	} else {
		_, err = db.Exec("INSERT INTO players (name, date) VALUES (?, ?)", name, date)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		message = fmt.Sprintf("%s has been added for today's game!", name)
	}

	http.SetCookie(w, &http.Cookie{
		Name:    "player_name",
		Value:   name,
		Expires: time.Now().Add(365 * 24 * time.Hour),
	})

	playersHandlerWithMessage(w, r, message)
}

func playersHandler(w http.ResponseWriter, r *http.Request) {
	playersHandlerWithMessage(w, r, "")
}

func playersHandlerWithMessage(w http.ResponseWriter, r *http.Request, message string) {
	date := time.Now().Format("2006-01-02")
	rows, err := db.Query("SELECT name FROM players WHERE date = ?", date)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var players []Player
	for rows.Next() {
		var p Player
		err := rows.Scan(&p.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		p.Date = date
		players = append(players, p)
	}

	basePath := os.Getenv("BASE_PATH")
	if basePath == "" {
		basePath = "/"
	}

	pageData := PageData{
		Players:  players,
		Date:     date,
		Message:  message,
		BasePath: basePath,
	}

	err = templates.ExecuteTemplate(w, "players.html", pageData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func resetDatabaseDaily() {
	for {
		now := time.Now()
		next := now.AddDate(0, 0, 1)
		next = time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
		time.Sleep(time.Until(next))

		_, err := db.Exec("DELETE FROM players WHERE date < ?", time.Now().Format("2006-01-02"))
		if err != nil {
			log.Println("Error resetting database:", err)
		} else {
			log.Println("Database reset for the new day")
		}
	}
}
