package gitnotify

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	githubApp "github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type localGithub struct {
	client GitClient
}

func (*localGithub) WebsiteLink() string {
	return config.GithubURLEndPoint
}

func (*localGithub) RepoLink(repo string) string {
	return fmt.Sprintf(githubRepoEndPoint, repo)
}

func (*localGithub) TreeLink(repo, ref string) string {
	return fmt.Sprintf(githubTreeURLEndPoint, repo, ref)
}

func (*localGithub) CommitLink(repo, ref string) string {
	return fmt.Sprintf(githubCommitURLEndPoint, repo, ref)
}

func (*localGithub) CompareLink(repo, oldCommit, newCommit string) string {
	return fmt.Sprintf(githubCompareURLEndPoint, repo, oldCommit, newCommit)
}

func (g *localGithub) Client() *githubApp.Client {
	return g.client.(*githubApp.Client)
}

// Helper method to create github client
func newGithubClient(token string) *localGithub {
	if token == "" {
		return &localGithub{}
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	client := githubApp.NewClient(tc)
	var err error
	client.BaseURL, err = url.Parse(config.GithubAPIEndPoint)
	if err != nil {
		log.Println(err)
		return &localGithub{}
	}
	return &localGithub{client}
}

func (g *localGithub) BranchesWithoutRefs(repoName string) ([]string, error) {
	statCount("github.branches_without_refs")
	listBranches, err := g.Branches(repoName)
	if err != nil {
		return nil, err
	}

	branches := make([]string, 0, len(listBranches))
	for _, b := range listBranches {
		branches = append(branches, b.Name)
	}
	return branches, nil
}

// caches branch response
func (g *localGithub) Branches(repoName string) ([]*GitRefWithCommit, error) {
	statCount("github.branches")
	var list []*githubApp.Branch
	var gr *githubApp.Response
	var err error

	refs := make([]*GitRefWithCommit, 0, 100)
	ownerRepo := strings.SplitN(repoName, "/", 2)
	page := 1
	for page != 0 {
		opt := &githubApp.ListOptions{Page: page, PerPage: 100}
		start := time.Now()
		list, gr, err = g.Client().Repositories.ListBranches(context.TODO(), ownerRepo[0], ownerRepo[1], opt)
		statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
		statCount("github.api_call")

		if len(list) == 0 || err != nil || gr.StatusCode >= 400 {
			page = page + 1
			if page >= 100 {
				break
			}
			continue
		}

		for _, r := range list {
			ref := &GitRefWithCommit{
				Name:   *r.Name,
				Commit: *r.Commit.SHA,
			}
			refs = append(refs, ref)
		}
		page = gr.NextPage
	}

	return refs, nil
}

// caches branch response
// TODO - this is exact duplicate above of except for the list type format
func (g *localGithub) Tags(repoName string) ([]*GitRefWithCommit, error) {
	statCount("github.tags")
	var list []*githubApp.RepositoryTag
	var gr *githubApp.Response
	var err error

	refs := make([]*GitRefWithCommit, 0, 100)
	ownerRepo := strings.SplitN(repoName, "/", 2)
	page := 1
	for page != 0 {
		opt := &githubApp.ListOptions{Page: page, PerPage: 100}

		start := time.Now()
		list, gr, err = g.Client().Repositories.ListTags(context.TODO(), ownerRepo[0], ownerRepo[1], opt)
		statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
		statCount("github.api_call")

		if len(list) == 0 || err != nil || gr.StatusCode >= 400 {
			page = page + 1
			if page >= 100 {
				break
			}
			continue
		}

		for _, r := range list {
			ref := &GitRefWithCommit{
				Name:   *r.Name,
				Commit: *r.Commit.SHA,
			}
			refs = append(refs, ref)
		}
		page = gr.NextPage
	}

	return refs, nil
}

func (g *localGithub) DefaultBranch(repoName string) (string, error) {
	statCount("github.default_branch")
	ownerRepo := strings.SplitN(repoName, "/", 2)
	start := time.Now()
	repository, gr, err := g.Client().Repositories.Get(context.TODO(), ownerRepo[0], ownerRepo[1])
	statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
	statCount("github.api_call")

	if err != nil || gr.StatusCode >= 400 {
		// gr will be in case of connection interruption
		// 401 statusCode means the token is no longer valid
		return "", err
	}
	return *repository.DefaultBranch, err
}

type ghSearchRepo struct {
	Items []*searchRepoItem `json:"items"`
}

// NOTE: the official repo does not support complex queries
// It only supports simple queries. Github API expects data to be in "abc+code:go"
func (g *localGithub) SearchRepos(query string) ([]*searchRepoItem, error) {
	statCount("github.search_repos")
	query = g.cleanRepoName(query)
	searchRepositoryURL := fmt.Sprintf("%ssearch/repositories?page=%d&q=%s", config.GithubAPIEndPoint, 1, query)
	req, _ := http.NewRequest("GET", searchRepositoryURL, nil)
	result := new(githubApp.RepositoriesSearchResult)
	start := time.Now()
	gr, err := g.Client().Do(context.TODO(), req, result)
	statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
	statCount("github.api_call")
	if gr.StatusCode >= 400 {
		return nil, errors.New("status code > 400")
	}

	if err != nil || gr.StatusCode >= 400 {
		return nil, err
	}

	searchResults := make([]*searchRepoItem, 0, len(result.Repositories))
	for _, r := range result.Repositories {
		item := &searchRepoItem{
			ID:   *r.Name,
			Name: *r.FullName,
		}
		if r.Description != nil {
			item.Description = *r.Description
		}
		if r.Homepage != nil {
			item.HomePage = *r.Homepage
		}

		searchResults = append(searchResults, item)
	}
	return searchResults, nil
}

func (g *localGithub) cleanRepoName(search string) string {
	search = strings.Trim(search, " ")
	// Add support for regular searches
	if strings.Contains(search, "/") {
		var modifiedRepoValidator = regexp.MustCompile("[\\p{L}\\d_-]+/[\\.\\p{L}\\d_-]*")
		data := modifiedRepoValidator.FindAllString(search, -1)
		d := strings.Split(data[0], "/")
		rep := fmt.Sprintf("%s+user:%s", d[1], d[0])
		search = strings.Replace(search, data[0], rep, 1)
		search = strings.Trim(search, "/")
	}
	search = strings.Trim(search, " ")
	search = strings.Replace(search, " ", "+", -1)
	return search
}

func (g *localGithub) SearchUsers(query string) ([]*searchUserItem, error) {
	statCount("github.search_users")
	start := time.Now()
	result, gr, err := g.Client().Search.Users(context.TODO(), query, nil)
	statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
	statCount("github.api_call")

	if err != nil || gr.StatusCode >= 400 {
		return []*searchUserItem{}, err
	}

	searchResults := make([]*searchUserItem, 0, len(result.Users))
	for _, r := range result.Users {
		searchResults = append(searchResults, &searchUserItem{
			ID:    fmt.Sprintf("%d", *r.ID),
			Login: *r.Login,
			Type:  *r.Type,
		})
	}

	return searchResults, nil
}

func (g *localGithub) RemoteOrgType(name string) (string, error) {
	statCount("github.remote_org_type")
	start := time.Now()
	user, gr, err := g.Client().Users.Get(context.TODO(), name)
	statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
	statCount("github.api_call")

	if err != nil || gr.StatusCode >= 400 {
		return "", err
	}
	// Organization / User
	return *user.Type, nil
}

func (g *localGithub) ReposForUser(organisation string) ([]*searchRepoItem, error) {
	statCount("github.repos_for_user")
	var repoList []*searchRepoItem
	page := 1
	for page != 0 {
		opt := &githubApp.RepositoryListOptions{Sort: "created", ListOptions: githubApp.ListOptions{Page: page, PerPage: 100}}
		start := time.Now()
		repositories, gr, err := g.Client().Repositories.List(context.TODO(), organisation, opt)
		statValue("github.api_time", time.Since(start).Nanoseconds()/1000)
		statCount("github.api_call")
		if err != nil || gr.StatusCode >= 400 {
			page = page + 1
			if page >= 100 {
				break
			}
			continue
		}
		var repos = make([]*searchRepoItem, 0, len(repositories))
		for _, repo := range repositories {
			item := &searchRepoItem{}
			item.ID = *repo.Name
			item.Name = *repo.Name
			if repo.Homepage != nil {
				item.HomePage = *repo.Homepage
			}
			if repo.Description != nil {
				item.Description = *repo.Description
			}
			repos = append(repos, item)
		}
		repoList = append(repoList, repos...)
		page = gr.NextPage
	}

	return repoList, nil
}
