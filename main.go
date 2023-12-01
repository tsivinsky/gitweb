package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/gorilla/mux"
)

const (
	GitDir = "/srv/git"
)

type Repo struct {
	Name  string
	Head  string
	Files []string
}

func getRepoFiles(repoPath string, gitObject string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "ls-tree", gitObject, "--name-only")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	str := string(out)

	files := []string{}
	for _, line := range strings.Split(str, "\n") {
		files = append(files, line)
	}

	return files, nil
}

func getRepoHead(repoPath string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	str := string(out)

	if str == "HEAD" {
		cmd = exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		out, err = cmd.Output()
		if err != nil {
			return "", err
		}
		str = string(out)
	}

	return str, nil
}

func main() {
	tmpl, err := template.ParseGlob("./views/*.html")
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	dir, err := os.ReadDir(GitDir)
	if err != nil {
		log.Fatal(err)
	}

	repos := []*Repo{}

	for _, file := range dir {
		if !file.IsDir() {
			continue // skip regular files
		}

		repoPath := path.Join(GitDir, file.Name())

		head, err := getRepoHead(repoPath)
		if err != nil {
			log.Fatal(err)
		}

		repo := &Repo{
			Name:  file.Name(),
			Head:  head,
			Files: []string{},
		}

		repo.Files, err = getRepoFiles(repoPath, head)
		if err != nil {
			log.Fatal(err)
		}

		repos = append(repos, repo)
	}

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type homePage struct {
			Repos []*Repo
		}

		err = tmpl.ExecuteTemplate(w, "index.html", homePage{
			Repos: repos,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	router.HandleFunc("/{repo}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		name := vars["repo"]
		if name == "" {
			http.Error(w, "no repo param provided", http.StatusBadRequest)
			return
		}

		repo := new(Repo)
		for _, r := range repos {
			if r.Name == name {
				repo = r
			}
		}

		if repo == nil {
			http.Error(w, "Repo not found", http.StatusNotFound)
			return
		}

		err = tmpl.ExecuteTemplate(w, "repo.html", repo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	err = http.ListenAndServe(":5000", router)
	if err != nil {
		log.Fatal(err)
	}
}
