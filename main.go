package playground

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Entrio/subenv"
	"github.com/labstack/gommon/log"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
)

type (
	projectList struct {
		Size       int64     `json:"size"`
		Limit      int64     `json:"limit"`
		IsLastPage bool      `json:"isLastPage"`
		Projects   []project `json:"values"`
	}

	project struct {
		Key         string       `json:"key"`
		ID          int64        `json:"id"`
		Name        string       `json:"name"`
		Description *string      `json:"description"`
		Public      bool         `json:"public"`
		Type        string       `json:"type"`
		Links       projectLinks `json:"links"`
	}

	projectLinks struct {
		Self []struct {
			Href string `json:"href"`
		} `json:"self"`
	}

	reposlist struct {
		Size       int64  `json:"size"`
		Limit      int64  `json:"limit"`
		IsLastPage bool   `json:"isLastPage"`
		Repos      []repo `json:"values"`
	}

	repo struct {
		Slug     string  `json:"slug"`
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		State    string  `json:"state"`
		Forkable bool    `json:"forkable"`
		Project  project `json:"project"`
		Public   bool    `json:"public"`
		Links    struct {
			Clone []struct {
				Href string `json:"href"`
				Name string `json:"name"`
			} `json:"clone"`
			Self []struct {
				Href string `json:"href"`
			} `json:"self"`
		} `json:"links"`
	}
)

func main() {
	baseURL := subenv.Env("BASE_BB_URL", "")
	baseRepoURL := subenv.Env("BASE_BB_REPO_URL", "")

	if baseRepoURL == "" {
		baseRepoURL = fmt.Sprintf("%s%s/repos", baseURL, "%s")
	}

	username := subenv.Env("BB_USERNAME", "")
	password := subenv.Env("BB_PASSWORD", "")
	rootDirectory := subenv.Env("BB_GIT_BASE_FOLDER", "./")

	if baseURL == "" {
		log.Fatalf("Please specify base url where bit bucket projects can be found. Use environmental variable `BASE_BB_URL` to do so")
	}

	fmt.Println(fmt.Sprintf("Starting mass git clone. URL: %s, base repo url: %s", baseURL, baseRepoURL))
	if username != "" {
		plen := len(password)
		passmask := ""
		for i := 0; i < plen; i ++ {
			passmask += "*"
		}
		fmt.Println(fmt.Sprintf("Username: %s, password: %s", username, passmask))
	}

	client := &http.Client{}
	req, err := http.NewRequest("GET", baseURL, nil)
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to request url! Error: %s", err.Error())
	}
	projects, err := ioutil.ReadAll(res.Body)
	_ = res.Body.Close()
	if err != nil {
		log.Fatalf("Failed to read body data. Error: %s", err.Error())
	}
	//fmt.Println(fmt.Printf("%s", projects))

	var data projectList

	if err := json.Unmarshal(projects, &data); err != nil {
		log.Fatalf("Failed to unmarshal JSON: %s", err.Error())
	}

	for _, project := range data.Projects {
		// need to fetch prefix for all projects and then get repos
		if repoReq, err := http.NewRequest("GET", fmt.Sprintf(baseRepoURL, project.Key), nil); err != nil {
			fmt.Println(fmt.Sprintf("There was an error forming request for project %s. Error was: %s", project.Name, err.Error()))
		} else {
			projectPath := fmt.Sprintf("./%s", project.Name)
			if _, err := os.Stat(projectPath); os.IsNotExist(err) {
				if err := os.Mkdir(projectPath, os.ModePerm); err != nil {
					log.Fatalf("failed to create directory %s! Error: %s", projectPath, err.Error())
				}
			}
			// we are goo, lets get repos
			if username != "" {
				repoReq.SetBasicAuth(username, password)
			}
			if repoRes, err := client.Do(repoReq); err != nil {
				fmt.Println(fmt.Sprintf("There was an error getting repos for project %s. Error was: %s", project.Name, err.Error()))
			} else {
				// clone the repo

				if repos, err := ioutil.ReadAll(repoRes.Body); err != nil {
					fmt.Println(fmt.Sprintf("There was an error getting repos body for project %s. Error was: %s", project.Name, err.Error()))
				} else {
					_ = repoRes.Body.Close()

					var repo reposlist

					if err := json.Unmarshal(repos, &repo); err != nil {
						log.Fatalf("Failed to unmarshal repo JSON: %s", err.Error())
					} else {
						for _, repository := range repo.Repos {
							for _, link := range repository.Links.Clone {
								if link.Name == "ssh" {
									fmt.Println(fmt.Sprintf("Cloning repo %s (%s)", repository.Name, link.Href))
									cmd := exec.Command("git", "clone", link.Href, fmt.Sprintf("%s/%s", projectPath, repository.Slug))
									var out bytes.Buffer
									cmd.Stdout = &out
									err := cmd.Run()
									if err != nil {
										log.Fatal(err)
									}
									fmt.Println(fmt.Sprintf("Successfully cloned %s to %s", repository.Name, fmt.Sprintf("%s/%s", projectPath, repository.Slug)))
									os.Exit(0)
								}
							}
						}
					}
				}
			}
		}
	}
}
