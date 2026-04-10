package github

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/google/go-github/v76/github"
	"golang.org/x/oauth2"

	"devbot/internal/config"
)

type Client struct {
	gh         *github.Client
	owner      string
	repo       string
	baseBranch string
	token      string
}

func NewClient(cfg config.GitHubConfig) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh:         github.NewClient(tc),
		owner:      cfg.Owner,
		repo:       cfg.Repo,
		baseBranch: cfg.BaseBranch,
		token:      cfg.Token,
	}
}

func newClientFromRepo(repo config.RepoConfig) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: repo.Token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{
		gh:         github.NewClient(tc),
		owner:      repo.Owner,
		repo:       repo.Repo,
		baseBranch: repo.BaseBranch,
		token:      repo.Token,
	}
}

// Owner returns the repository owner.
func (c *Client) Owner() string { return c.owner }

// Repo returns the repository name.
func (c *Client) Repo() string { return c.repo }

// BaseBranch returns the base branch name.
func (c *Client) BaseBranch() string { return c.baseBranch }

// ClientPool holds one GitHub client per configured repository.
type ClientPool struct {
	clients []*Client
	byKey   map[string]*Client // "owner/repo" → client
	byName  map[string]*Client // name alias → client
}

// NewClientPool creates a pool from the full GitHub config.
// cfg.Repos must already be normalised (Load() does this).
func NewClientPool(cfg config.GitHubConfig) *ClientPool {
	p := &ClientPool{
		byKey:  make(map[string]*Client),
		byName: make(map[string]*Client),
	}
	for _, r := range cfg.Repos {
		c := newClientFromRepo(r)
		p.clients = append(p.clients, c)
		key := r.Owner + "/" + r.Repo
		p.byKey[key] = c
		if r.Name != "" {
			p.byName[r.Name] = c
		}
	}
	return p
}

// Default returns the first (primary) client.
func (p *ClientPool) Default() *Client {
	if len(p.clients) == 0 {
		return nil
	}
	return p.clients[0]
}

// Get returns the client for a specific owner/repo pair.
// When both owner and repo are empty it returns the default client.
// When a non-empty pair is provided but not found in the pool it returns nil
// so callers can detect stale/mismatched task metadata rather than silently
// writing to the wrong repository.
func (p *ClientPool) Get(owner, repo string) *Client {
	if owner == "" && repo == "" {
		return p.Default()
	}
	return p.byKey[owner+"/"+repo] // nil when not found
}

// Lookup finds a client by name alias or "owner/repo" string.
// Returns Default() when ref is empty or unknown.
func (p *ClientPool) Lookup(ref string) *Client {
	if ref == "" {
		return p.Default()
	}
	if c, ok := p.byName[ref]; ok {
		return c
	}
	if c, ok := p.byKey[ref]; ok {
		return c
	}
	return nil
}

// All returns all clients in configuration order.
func (p *ClientPool) All() []*Client { return p.clients }

// IsMultiRepo reports whether more than one repository is configured.
func (p *ClientPool) IsMultiRepo() bool { return len(p.clients) > 1 }

type PR struct {
	Number int
	URL    string
	Title  string
	Body   string
}

func (c *Client) CreatePR(ctx context.Context, branch, title, body string) (*PR, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, c.owner, c.repo, &github.NewPullRequest{
		Title: github.Ptr(title),
		Head:  github.Ptr(branch),
		Base:  github.Ptr(c.baseBranch),
		Body:  github.Ptr(body),
	})
	if err != nil {
		return nil, fmt.Errorf("create PR: %w", err)
	}
	return &PR{
		Number: pr.GetNumber(),
		URL:    pr.GetHTMLURL(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
	}, nil
}

func (c *Client) GetPR(ctx context.Context, prNumber int) (*PR, error) {
	pr, _, err := c.gh.PullRequests.Get(ctx, c.owner, c.repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("get PR #%d: %w", prNumber, err)
	}
	return &PR{
		Number: pr.GetNumber(),
		URL:    pr.GetHTMLURL(),
		Title:  pr.GetTitle(),
		Body:   pr.GetBody(),
	}, nil
}

func (c *Client) GetPRDiff(ctx context.Context, prNumber int) (string, error) {
	opts := &github.RawOptions{Type: github.Diff}
	diff, _, err := c.gh.PullRequests.GetRaw(ctx, c.owner, c.repo, prNumber, *opts)
	if err != nil {
		return "", fmt.Errorf("get PR diff #%d: %w", prNumber, err)
	}
	return diff, nil
}

func (c *Client) GetPRFiles(ctx context.Context, prNumber int) ([]*github.CommitFile, error) {
	files, _, err := c.gh.PullRequests.ListFiles(ctx, c.owner, c.repo, prNumber, nil)
	if err != nil {
		return nil, fmt.Errorf("list PR files #%d: %w", prNumber, err)
	}
	return files, nil
}

func (c *Client) DeleteBranch(ctx context.Context, branch string) error {
	_, err := c.gh.Git.DeleteRef(ctx, c.owner, c.repo, "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("delete branch %q: %w", branch, err)
	}
	return nil
}

// GetRepoArchiveURL returns a URL to download the repo's default branch as a tarball.
// Used internally for building file tree context for the AI prompt.
func (c *Client) GetRepoContents(ctx context.Context, path string) ([]*github.RepositoryContent, error) {
	_, dir, _, err := c.gh.Repositories.GetContents(ctx, c.owner, c.repo, path, &github.RepositoryContentGetOptions{
		Ref: c.baseBranch,
	})
	if err != nil {
		return nil, fmt.Errorf("get repo contents at %q: %w", path, err)
	}
	return dir, nil
}

// GetFileContent fetches the raw content of a file from the default branch.
func (c *Client) GetFileContent(ctx context.Context, filePath string) (string, error) {
	fc, _, _, err := c.gh.Repositories.GetContents(ctx, c.owner, c.repo, filePath, &github.RepositoryContentGetOptions{
		Ref: c.baseBranch,
	})
	if err != nil {
		return "", fmt.Errorf("get file content %q: %w", filePath, err)
	}
	if fc == nil {
		return "", fmt.Errorf("path %q is a directory, not a file", filePath)
	}
	return fc.GetContent()
}

// GetCloneURL returns the HTTPS clone URL for the repository.
func (c *Client) GetCloneURL() string {
	return fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git", c.token, c.owner, c.repo)
}

// CheckRepoExists verifies we can reach the repository.
func (c *Client) CheckRepoExists(ctx context.Context) error {
	_, resp, err := c.gh.Repositories.Get(ctx, c.owner, c.repo)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("repository %s/%s not found (check github.owner and github.repo in config)", c.owner, c.repo)
		}
		return fmt.Errorf("check repo: %w", err)
	}
	return nil
}

// BuildFileTree fetches the top-level and one level of subdirectories to give
// the AI agent context about the repository structure.
func (c *Client) BuildFileTree(ctx context.Context) (string, error) {
	tree, _, err := c.gh.Git.GetTree(ctx, c.owner, c.repo, c.baseBranch, true)
	if err != nil {
		return "", fmt.Errorf("get tree: %w", err)
	}

	var result string
	count := 0
	for _, entry := range tree.Entries {
		if count >= 200 {
			result += "... (truncated)\n"
			break
		}
		result += entry.GetPath() + "\n"
		count++
	}
	return result, nil
}

// DownloadArchive downloads the repo as a tarball into the provided writer.
func (c *Client) DownloadArchive(ctx context.Context, w io.Writer) error {
	url, _, err := c.gh.Repositories.GetArchiveLink(ctx, c.owner, c.repo,
		github.Tarball, &github.RepositoryContentGetOptions{Ref: c.baseBranch}, 5)
	if err != nil {
		return fmt.Errorf("get archive link: %w", err)
	}
	resp, err := http.Get(url.String())
	if err != nil {
		return fmt.Errorf("download archive: %w", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(w, resp.Body); err != nil {
		return fmt.Errorf("read archive: %w", err)
	}
	return nil
}
