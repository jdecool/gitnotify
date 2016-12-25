package main

import (
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strings"

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
	return g.branchTagInfo(repoName, gitRefBranch)
}

// caches branch response
func (g *localGithub) Tags(repoName string) ([]*GitRefWithCommit, error) {
	return g.branchTagInfo(repoName, gitRefTag)
}

func (g *localGithub) DefaultBranch(repoName string) (string, error) {
	ownerRepo := strings.SplitN(repoName, "/", 2)
	repository, gr, err := g.Client().Repositories.Get(ownerRepo[0], ownerRepo[1])

	if err != nil || gr.StatusCode >= 400 {
		// gr will be in case of connection interruption
		// 401 statusCode means the token is no longer valid
		return "", err
	}
	return *repository.DefaultBranch, err
}

func (g *localGithub) branchTagInfo(repoName, option string) ([]*GitRefWithCommit, error) {

	var list []*githubApp.Branch
	var gr *githubApp.Response
	var err error

	refs := make([]*GitRefWithCommit, 0, 100)
	ownerRepo := strings.SplitN(repoName, "/", 2)
	page := 1
	for page != 0 {
		opt := &githubApp.ListOptions{Page: page, PerPage: 100}
		if option == "tags" {
			list, gr, err = g.Client().Repositories.ListBranches(ownerRepo[0], ownerRepo[1], opt)
		} else if option == "branches" {
			list, gr, err = g.Client().Repositories.ListBranches(ownerRepo[0], ownerRepo[1], opt)
		}

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

type ghSearchRepo struct {
	Items []*searchRepoItem `json:"items"`
}

// searchRepoItem is used by the interface
func (g *localGithub) SearchRepos(query string) ([]*searchRepoItem, error) {
	query = g.cleanRepoName(query)
	result, gr, err := g.Client().Search.Repositories(query, nil)

	if err != nil || gr.StatusCode >= 400 {
		return nil, err
	}

	searchResults := make([]*searchRepoItem, 0, len(result.Repositories))
	for _, r := range result.Repositories {
		searchResults = append(searchResults, &searchRepoItem{
			ID:          *r.Name,
			Name:        *r.FullName,
			Description: *r.Description,
			HomePage:    *r.Homepage,
		})
	}
	return searchResults, nil
}

func (g *localGithub) cleanRepoName(search string) string {
	search = strings.Replace(search, " ", "+", -1)
	// Add support for regular searches
	if strings.Contains(search, "/") {
		var modifiedRepoValidator = regexp.MustCompile("[\\p{L}\\d_-]+/[\\.\\p{L}\\d_-]*")
		data := modifiedRepoValidator.FindAllString(search, -1)
		d := strings.Split(data[0], "/")
		rep := fmt.Sprintf("%s+user:%s", d[1], d[0])
		search = strings.Replace(search, data[0], rep, 1)
	}
	return search
}

func (g *localGithub) SearchUsers(query string) ([]*searchUserItem, error) {
	result, gr, err := g.Client().Search.Users(query, nil)

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
	user, gr, err := g.Client().Users.Get(name)

	if err != nil || gr.StatusCode >= 400 {
		return "", err
	}
	// Organization / User
	return *user.Type, nil
}

func (g *localGithub) ReposForUser(organisation string) ([]string, error) {
	var repoList []string
	page := 1
	for page != 0 {
		opt := &githubApp.RepositoryListOptions{Sort: "created", ListOptions: githubApp.ListOptions{Page: page, PerPage: 100}}
		repositories, gr, err := g.Client().Repositories.List(organisation, opt)
		if err != nil || gr.StatusCode >= 400 {
			page = page + 1
			if page >= 100 {
				break
			}
			continue
		}
		var repos = make([]string, 0, len(repositories))
		for _, repo := range repositories {
			repos = append(repos, *repo.Name)
		}
		repoList = append(repoList, repos...)
		page = gr.NextPage
	}
	return repoList, nil

}
