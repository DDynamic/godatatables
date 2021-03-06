package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"os"

	"github.com/ddynamic/godatatables"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

func main() {
	env, _ := godotenv.Read("test/.env")

	if env["DATABASE_URL"] == "" {
		env["DATABASE_URL"] = "root:password@tcp(127.0.0.1:3306)/test?parseTime=true&charset=utf8mb4,utf8"
	}

	godotenv.Write(env, "test/.env")

	godotenv.Load("test/.env")

	db, err := sql.Open("mysql", os.Getenv("DATABASE_URL"))

	if err != nil {
		fmt.Println(err)
	}

	tmpl := template.Must(template.ParseFiles("test/test.html"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		godatatables.DataTables(w, r, db, "members", "", godatatables.Column{Name: "id", Display: "id"}, godatatables.Column{Name: "name", Display: "name"})
	})

	http.ListenAndServe(":8080", nil)
}
