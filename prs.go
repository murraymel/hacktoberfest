package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/markbates/goth"
	"github.com/pkg/errors"
)

// PR is a pull request against any repo GitHub. The Valid field is set based
// on the Repo's presence in the orgs or projects maps.
type PR struct {
	Title string
	Date  time.Time
	URL   string
	Repo  Repo
	Valid bool
}

func prs(w http.ResponseWriter, r *http.Request) {
	u, ok := findUser(r)
	if !ok {
		http.Error(w, "you are not logged in", http.StatusUnauthorized)
		return
	}

	prs, err := fetchPRs(u)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	for i, pr := range prs {
		prs[i].Valid = orgs[pr.Repo.Owner] || projects[pr.Repo.Owner+"/"+pr.Repo.Name]
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(prs); err != nil {
		log.Println(err)
	}
}

func fetchPRs(u goth.User) ([]PR, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/search/issues", nil)
	if err != nil {
		return nil, errors.Wrap(err, "could not build request")
	}

	q := fmt.Sprintf(
		"author:%s type:pr created:2016-10-01T00:00:00-12:00..2016-10-31T23:59:59-12:00",
		u.NickName,
	)
	vals := req.URL.Query()
	vals.Add("q", q)
	req.URL.RawQuery = vals.Encode()

	// Use their access token so it counts against their rate limit
	req.Header.Add("Authorization", "token "+u.AccessToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "could not execute request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, errors.Wrapf(err, "status was %d, not 200", resp.StatusCode)
	}

	var data struct {
		Items []struct {
			Title     string    `json:"title"`
			CreatedAt time.Time `json:"created_at"`
			URL       string    `json:"url"`
			RepoURL   string    `json:"repository_url"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, errors.Wrap(err, "could not decode json")
	}

	prs := []PR{}
	for _, item := range data.Items {
		pr := PR{
			Title: item.Title,
			Date:  item.CreatedAt,
			URL:   item.URL,
		}

		pr.Repo, err = repoFromURL(item.RepoURL)
		if err != nil {
			return nil, errors.Wrapf(err, "could not identify repo from %s", item.RepoURL)
		}

		// ¯\_(ツ)_/¯
		if u.NickName == `kentonh` && pr.Repo.Owner == u.NickName {
			continue
		}

		prs = append(prs, pr)
	}

	return prs, nil
}
