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
		log.Printf("git ls-tree failed repoPath=%s gitObject=%s output=%s", repoPath, gitObject, string(out))
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
		log.Printf("git rev-parse --abbrev-ref HEAD failed repoPath=%s output=%s", repoPath, string(out))
		return "", err
	}
	str := string(out)

	if str == "HEAD" {
		cmd = exec.Command("git", "-C", repoPath, "rev-parse", "HEAD")
		out, err = cmd.Output()
		if err != nil {
			log.Printf("git rev-parse HEAD failed repoPath=%s output=%s", repoPath, string(out))
			return "", err
		}
		str = string(out)
	}

	return str, nil
}

func getRepo(name string, head string, includeFiles bool) (*Repo, error) {
	repoPath := path.Join(GitDir, name)

	if head == "" {
		str, err := getRepoHead(repoPath)
		if err != nil {
			return nil, err
		}
		head = str
	}

	repo := &Repo{
		Name:  name,
		Head:  head,
		Files: []string{},
	}

	if includeFiles {
		files, err := getRepoFiles(repoPath, head)
		if err != nil {
			return nil, err
		}
		repo.Files = files
	}

	return repo, nil
}

func getRepos(includesFiles bool) ([]*Repo, error) {
	dir, err := os.ReadDir(GitDir)
	if err != nil {
		return nil, err
	}

	repos := []*Repo{}
	for _, file := range dir {
		if !file.IsDir() {
			continue // skip regular files
		}

		repo, err := getRepo(file.Name(), "", includesFiles)
		if err != nil {
			return nil, err
		}

		repos = append(repos, repo)
	}

	return repos, nil
}

func main() {
	tmpl, err := template.ParseGlob("./views/*.html")
	if err != nil {
		log.Fatal(err)
	}

	router := mux.NewRouter()

	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		repos, err := getRepos(false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		repo, err := getRepo(name, "", true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
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

	router.HandleFunc("/{repo}/{head}", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		name := vars["repo"]
		head := vars["head"]

		repo, err := getRepo(name, head, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if repo == nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
