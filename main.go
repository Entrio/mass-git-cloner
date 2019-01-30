package main

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
	"time"
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

	cloneJob struct {
		ID         int
		Project    string
		Repo       string
		FinalPath  string
		URL        string
		Success    bool
		FailReason *string
		TimeTaken  float64
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

	if rootDirectory != "./" {
		_ = os.MkdirAll(rootDirectory, os.ModePerm)
	}

	fmt.Println(fmt.Sprintf("Starting mass git clone. URL: %s, base repo url: %s", baseURL, baseRepoURL))
	if username != "" {
		plen := len(password)
		passmask := ""
		for i := 0; i < plen; i++ {
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

	var cloneJobs []cloneJob

	for _, project := range data.Projects {
		// need to fetch prefix for all projects and then get repos
		if repoReq, err := http.NewRequest("GET", fmt.Sprintf(baseRepoURL, project.Key), nil); err != nil {
			fmt.Println(fmt.Sprintf("There was an error forming request for project %s. Error was: %s", project.Name, err.Error()))
		} else {
			projectPath := fmt.Sprintf("%s%s", rootDirectory, project.Name)
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
									cloneJobs = append(cloneJobs, cloneJob{
										Project:   project.Name,
										Repo:      repository.Name,
										FinalPath: fmt.Sprintf("%s/%s", projectPath, repository.Slug),
										URL:       link.Href,
									})
								}
							}
						}
					}
				}
			}
		}
	}

	// Do actual cloning
	maxGoroutines := subenv.EnvI("BB_MAX_JOBS", 4)
	fmt.Println(fmt.Sprintf("Maximum clone jobs: %d", maxGoroutines))
	guard := make(chan struct{}, maxGoroutines)

	total := len(cloneJobs)
	for i := 0; i < total; i++ {
		guard <- struct{}{} // would block if guard channel is already filled
		go func() {
			clone(&cloneJobs[i], i, total)
			<-guard
		}()
	}

	var success, failure int

	for _, v := range cloneJobs {
		if v.Success {
			success++
		} else {
			failure++
		}
	}

	fmt.Println("Repository clone report:")
	fmt.Println(fmt.Sprintf("Total: %d Succeeded: %d Failed: %d", len(cloneJobs), success, failure))
	if failure > 0 {
		for _, v := range cloneJobs {
			if !v.Success {
				fmt.Println(fmt.Sprintf("[%d] Failed '%s'", v.ID, v.URL))
				if v.FailReason != nil {
					fmt.Println(fmt.Sprintf("Reason: %s", *v.FailReason))
				}
			}
		}
	}

	fmt.Println("All goroutines finished, exiting...")
}

func clone(cj *cloneJob, c, t int) {
	cj.ID = c
	start := time.Now()
	commandFull := fmt.Sprintf("git clone %s %s", cj.URL, cj.FinalPath)
	fmt.Println(fmt.Sprintf("[%d/%d] Executing command: '%s'", c, t, commandFull))
	cmd := exec.Command("git", "clone", cj.URL, cj.FinalPath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	end := time.Now()
	if err != nil {
		reason := err.Error()
		fmt.Println(fmt.Sprintf("[%d/%d] Failed to execute '%s' Error was: %s", c, t, commandFull, err.Error()))
		cj.FailReason = &reason
	} else {
		fmt.Println(fmt.Sprintf("[%d/%d] Successfully cloned %s, took %f second(s)", c, t, cj.FinalPath, end.Sub(start).Seconds()))
		cj.Success = true
		cj.TimeTaken = end.Sub(start).Seconds()
	}
}
