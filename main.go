package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type options struct {
	GitURL    string
	GitBranch string
	TargetDir string
}

type BranchConfig struct {
	Branch    string `json:"branch"`
	TargetDir string `json:"target_dir"`
}

type Config struct {
	GitURL        string         `json:"git_url"`
	Port          string         `json:"port"`
	BranchConfigs []BranchConfig `json:"branch_configs"`
}

func (c Config) Validate() error {
	problems := []string{}

	if c.GitURL == "" {
		problems = append(problems, "missing git_url")
	}

	if c.Port == "" {
		problems = append(problems, "missing port")
	}

	if len(c.BranchConfigs) == 0 {
		problems = append(problems, "missing branch_configs")
	}

	for i, branchConfig := range c.BranchConfigs {
		if branchConfig.Branch == "" {
			problems = append(problems, fmt.Sprintf("branch config #%d (0-indexed) missing branch", i))
		}
		if branchConfig.TargetDir == "" {
			problems = append(problems, fmt.Sprintf("branch config #%d (0-indexed) missing target_dir", i))
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, ", "))
	}
	return nil
}

func parseConfig() Config {
	if len(os.Args) < 2 {
		log.Fatalf("config file argument required")
	}
	configFile := os.Args[1]
	configContent, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatalf("error reading config file: %s", err)
	}
	config := Config{}
	err = json.Unmarshal(configContent, &config)
	if err != nil {
		log.Fatalf("error parsing config file: %s", err)
	}
	if err = config.Validate(); err != nil {
		log.Fatalf("invalid config content: %s", err)
	}
	return config
}

func refreshSite(options options) error {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	tmpDir, err := ioutil.TempDir("", "website-files")
	if err != nil {
		return err
	}
	defer os.Remove(tmpDir)

	gitCmd := exec.Command("git", "clone", "--depth", "1", "--branch", options.GitBranch, options.GitURL, tmpDir)
	gitCmd.Stdout = &stdout
	gitCmd.Stderr = &stderr

	err = gitCmd.Run()
	if err != nil {
		log.Printf("Error cloning from git: %s\n%s\n%s", err, stdout.String(), stderr.String())
		return err
	}

	stdout.Reset()
	stderr.Reset()

	rsyncCmd := exec.Command("rsync", "-c", "-r", "--delete", "--exclude=.well-known", "web/", options.TargetDir)
	rsyncCmd.Dir = tmpDir
	rsyncCmd.Stdout = &stdout
	rsyncCmd.Stderr = &stderr
	err = rsyncCmd.Run()
	if err != nil {
		log.Printf("Error syncing cloned files to target dir %s: %s\n%s\n%s", options.TargetDir, err, stdout.String(), stderr.String())
		return err
	}

	return nil
}

type rateLimit struct {
	LastRun  int64
	Interval int64
}

func newRateLimit(interval int64) rateLimit {
	return rateLimit{
		LastRun:  0,
		Interval: interval, // seconds
	}
}

func (rl *rateLimit) CanRun() bool {
	currentTime := time.Now().Unix()
	if currentTime-rl.Interval < rl.LastRun {
		log.Printf("rate limit error: last ran at %d, current time %d", rl.LastRun, currentTime)
		return false
	}
	rl.LastRun = currentTime
	return true
}

type WebhookPayload struct {
	Ref string `json:"ref"`
}

func main() {
	config := parseConfig()

	rateLimit := newRateLimit(10)

	http.HandleFunc("/refresh", func(w http.ResponseWriter, r *http.Request) {
		options := options{GitURL: config.GitURL}
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !rateLimit.CanRun() {
			// NOTE: This rate limit applies globally, not per-client. In the
			// event of a DOS attack, this should protect the rest of the
			// server, but it would interfere with the ability for a legitimate
			// client to perform a refresh.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		bodyContent, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Error reading request body: %s", err)
			return
		}

		payload := WebhookPayload{}
		if err := json.Unmarshal(bodyContent, &payload); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Error unmarshaling request payload: %s\nPayload:\n%s\n", err, string(bodyContent))
			return
		}

		// parse branch from ref
		refPrefix := "refs/heads/"
		if !strings.HasPrefix(payload.Ref, refPrefix) {
			w.WriteHeader(http.StatusBadRequest)
			log.Printf("Unexpected ref format.  Received %s; expected prefix %s.", payload.Ref, refPrefix)
			return
		}

		branch := strings.TrimPrefix(payload.Ref, refPrefix)
		options.GitBranch = branch

		// find target dir based on branch
		for _, branchConfig := range config.BranchConfigs {
			if branchConfig.Branch == branch {
				options.TargetDir = branchConfig.TargetDir
				break
			}
		}
		if options.TargetDir == "" {
			w.WriteHeader(http.StatusOK)
			log.Printf("ignoring request for branch that does not match any config: %s", branch)
			return
		}

		// refresh site
		if err := refreshSite(options); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Printf("Error refreshing site: %s", err)
			return
		}

		log.Printf("Refreshed site content in dir %s from branch %s", options.TargetDir, branch)
		w.WriteHeader(http.StatusOK)
	})

	log.Printf("Starting server listening on %s and git repo %s with branch configs %v", config.Port, config.GitURL, config.BranchConfigs)
	http.ListenAndServe(fmt.Sprintf(":%s", config.Port), nil)
}
