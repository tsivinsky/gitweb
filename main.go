package main

import (
	"fmt"
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
	GitDir = "/home/git"
)

type Repo struct {
	Name  string
	Head  string
	Files []string
}

type Commit struct {
	Hash    string
	Message string
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

	return strings.TrimSpace(str), nil
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

func getRepoBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "branch", "--no-color")
	out, err := cmd.Output()
	if err != nil {
		log.Printf("git branch --no-color failed repoPath=%s", repoPath)
		return nil, err
	}
	str := string(out)

	branches := []string{}
	for _, line := range strings.Split(str, "\n") {
		line = strings.ReplaceAll(line, "*", "")
		line = strings.TrimSpace(line)

		branches = append(branches, line)
	}

	return branches, nil
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

		if !strings.HasSuffix(file.Name(), ".git") {
			continue // skip directories w/o .git at the end
		}

		repo, err := getRepo(file.Name(), "", includesFiles)
		if err != nil {
			return nil, err
		}

		repos = append(repos, repo)
	}

	return repos, nil
}

func getRepoCommits(repoPath string) ([]Commit, error) {
	cmd := exec.Command("git", "-C", repoPath, "log", "--oneline", "--no-color", "--no-decorate")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	str := string(out)
	str = strings.TrimSpace(str)

	commits := []Commit{}
	for _, line := range strings.Split(str, "\n") {
		parts := strings.Split(line, " ")
		hash := parts[0]
		message := parts[1:]

		commits = append(commits, Commit{
			Hash:    hash,
			Message: strings.Join(message, " "),
		})
	}

	return commits, nil
}

func createRepo(name string) (*Repo, error) {
	fullName := fmt.Sprintf("%s.git", name)

	repoPath := path.Join(GitDir, fullName)
	cmd := exec.Command("git", "init", "--bare", "--initial-branch", "master", repoPath)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	return getRepo(fullName, "", false)
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

	router.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			err = tmpl.ExecuteTemplate(w, "new.html", struct{}{})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		if r.Method == "POST" {
			name := r.FormValue("name")
			if name == "" {
				http.Error(w, "No name was provided", http.StatusBadRequest)
				return
			}

			repo, err := createRepo(name)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			u := fmt.Sprintf("http://%s/%s/%s", r.Host, repo.Name, repo.Head)
			http.Redirect(w, r, u, http.StatusTemporaryRedirect)
			return
		}

		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
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

		branches, err := getRepoBranches(path.Join(GitDir, name))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		type repoPage struct {
			Repo
			Branches []string
		}

		err = tmpl.ExecuteTemplate(w, "repo.html", repoPage{
			Repo:     *repo,
			Branches: branches,
		})
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

		branches, err := getRepoBranches(path.Join(GitDir, name))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		type repoPage struct {
			Repo
			Branches []string
		}

		err = tmpl.ExecuteTemplate(w, "repo.html", repoPage{
			Repo:     *repo,
			Branches: branches,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	router.HandleFunc("/{repo}/{head}/commits", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		vars := mux.Vars(r)
		name := vars["repo"]
		head := vars["head"]

		repo, err := getRepo(name, head, false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if repo == nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		commits, err := getRepoCommits(path.Join(GitDir, name))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		type commitsPage struct {
			Repo
			Commits []Commit
		}

		err = tmpl.ExecuteTemplate(w, "commits.html", commitsPage{
			Repo:    *repo,
			Commits: commits,
		})
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
