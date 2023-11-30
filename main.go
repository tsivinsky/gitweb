package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
)

const (
	GitDir = "/srv/git"
)

func main() {
	tmpl, err := template.ParseGlob("./views/*.html")
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		dir, err := os.ReadDir(GitDir)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		repos := []string{}

		for _, file := range dir {
			repos = append(repos, file.Name())
		}

		type homePage struct {
			Repos []string
		}

		err = tmpl.ExecuteTemplate(w, "index.html", homePage{
			Repos: repos,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	err = http.ListenAndServe(":5000", mux)
	if err != nil {
		log.Fatal(err)
	}
}
