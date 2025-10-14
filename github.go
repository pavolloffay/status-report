package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

type GitHubClient struct {
	client *github.Client
	ctx    context.Context
}

type StatusReport struct {
	Username        string
	Week            int
	Year            int
	StartDate       time.Time
	EndDate         time.Time
	PRsCreated      []PullRequestInfo
	PRReviews       []PullRequestReview
	IssuesCreated   []IssueInfo
	IssuesCommented []IssueInfo
	CommentsCount   int
}

type PullRequestReview struct {
	PullRequest PullRequestInfo
	ReviewDate  time.Time
}

type PullRequestInfo struct {
	Title       string
	URL         string
	Number      int
	CommitCount int
	CreatedAt   time.Time
}

type IssueInfo struct {
	Title     string
	URL       string
	Number    int
	CreatedAt time.Time
}

func NewGitHubClient() (*GitHubClient, error) {
	ctx := context.Background()

	// Try to get GitHub token from environment
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)

	return &GitHubClient{
		client: client,
		ctx:    ctx,
	}, nil
}

func (gc *GitHubClient) GenerateStatusReport(username string, weekStr string) (*StatusReport, error) {
	week, err := strconv.Atoi(weekStr)
	if err != nil {
		return nil, fmt.Errorf("invalid week number: %s", weekStr)
	}

	// Calculate the date range for the given week
	startDate, endDate := calculateWeekDateRange(week, time.Now().Year())

	report := &StatusReport{
		Username:  username,
		Week:      week,
		Year:      time.Now().Year(),
		StartDate: startDate,
		EndDate:   endDate,
	}

	// Fetch data from GitHub API
	if err := gc.fetchPRsCreated(report); err != nil {
		return nil, fmt.Errorf("failed to fetch PRs created: %v", err)
	}

	if err := gc.fetchPRReviews(report); err != nil {
		return nil, fmt.Errorf("failed to fetch PR reviews: %v", err)
	}

	if err := gc.fetchIssuesCreated(report); err != nil {
		return nil, fmt.Errorf("failed to fetch issues created: %v", err)
	}

	if err := gc.fetchIssuesCommented(report); err != nil {
		return nil, fmt.Errorf("failed to fetch issues commented: %v", err)
	}

	if err := gc.fetchAllComments(report); err != nil {
		return nil, fmt.Errorf("failed to fetch all comments: %v", err)
	}

	return report, nil
}

func calculateWeekDateRange(week, year int) (time.Time, time.Time) {
	// ISO 8601 week calculation
	// Week 1 is the first week with Thursday in the new year
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)

	// Find the Monday of week 1
	weekday := jan4.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysToSubtract := int(weekday) - 1
	mondayWeek1 := jan4.AddDate(0, 0, -daysToSubtract)

	// Calculate the start date of the requested week
	startDate := mondayWeek1.AddDate(0, 0, (week-1)*7)
	endDate := startDate.AddDate(0, 0, 6).Add(23*time.Hour + 59*time.Minute + 59*time.Second)

	return startDate, endDate
}

func (gc *GitHubClient) fetchPRsCreated(report *StatusReport) error {
	// Search for PRs created by the user in the specified time range
	query := fmt.Sprintf("author:%s type:pr created:%s..%s",
		report.Username,
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allPRs []PullRequestInfo

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opts)
		if err != nil {
			return fmt.Errorf("failed to search for PRs: %v", err)
		}

		for _, issue := range result.Issues {
			// Get additional PR information including commit count
			prInfo, err := gc.getPRInfo(issue)
			if err != nil {
				// Log error but continue with other PRs
				issueNum := "unknown"
				if issue.Number != nil {
					issueNum = fmt.Sprintf("%d", *issue.Number)
				}
				fmt.Fprintf(os.Stderr, "Warning: failed to get PR info for #%s: %v\n", issueNum, err)
				continue
			}
			allPRs = append(allPRs, prInfo)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	report.PRsCreated = allPRs
	return nil
}

func (gc *GitHubClient) getPRInfo(issue *github.Issue) (PullRequestInfo, error) {
	// Check basic required fields
	if issue.Number == nil || issue.Title == nil || issue.HTMLURL == nil {
		return PullRequestInfo{}, fmt.Errorf("missing basic required fields")
	}

	// Extract owner and repo from HTML URL since Repository object may be nil in search results
	// URL format: https://github.com/owner/repo/pull/123
	owner, repo, err := gc.parseRepoFromURL(*issue.HTMLURL)
	if err != nil {
		return PullRequestInfo{}, fmt.Errorf("failed to parse repository info from URL: %v", err)
	}

	// Get commit count
	commits, _, err := gc.client.PullRequests.ListCommits(gc.ctx, owner, repo, *issue.Number, nil)
	if err != nil {
		return PullRequestInfo{}, fmt.Errorf("failed to get PR commits: %v", err)
	}

	return PullRequestInfo{
		Title:       *issue.Title,
		URL:         *issue.HTMLURL,
		Number:      *issue.Number,
		CommitCount: len(commits),
		CreatedAt:   issue.CreatedAt.Time,
	}, nil
}

// parseRepoFromURL extracts owner and repo name from GitHub URL
func (gc *GitHubClient) parseRepoFromURL(url string) (string, string, error) {
	// Expected formats:
	// https://github.com/owner/repo/pull/123
	// https://github.com/owner/repo/issues/123
	parts := strings.Split(url, "/")
	if len(parts) < 5 || parts[2] != "github.com" {
		return "", "", fmt.Errorf("invalid GitHub URL format: %s", url)
	}

	owner := parts[3]
	repo := parts[4]

	if owner == "" || repo == "" {
		return "", "", fmt.Errorf("empty owner or repo in URL: %s", url)
	}

	return owner, repo, nil
}

// formatLinkWithRepo formats a link according to the spec: [<user/org/>/<repository-name>#123 - title](link)
func formatLinkWithRepo(url, title string, number int) (string, error) {
	owner, repo, err := (&GitHubClient{}).parseRepoFromURL(url)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("[%s/%s#%d - %s](%s)", owner, repo, number, title, url), nil
}

func (gc *GitHubClient) fetchPRReviews(report *StatusReport) error {
	// Search for PRs that the user has reviewed in the specified time range
	query := fmt.Sprintf("type:pr reviewed-by:%s updated:%s..%s",
		report.Username,
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allReviews []PullRequestReview

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opts)
		if err != nil {
			return fmt.Errorf("failed to search for reviewed PRs: %v", err)
		}

		for _, issue := range result.Issues {
			// Get reviews by this user on this PR within the time range
			reviews, err := gc.getUserReviewsInTimeRange(issue, report.Username, report.StartDate, report.EndDate)
			if err != nil {
				issueNum := "unknown"
				if issue.Number != nil {
					issueNum = fmt.Sprintf("%d", *issue.Number)
				}
				fmt.Fprintf(os.Stderr, "Warning: failed to get reviews for PR #%s: %v\n", issueNum, err)
				continue
			}
			allReviews = append(allReviews, reviews...)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	report.PRReviews = allReviews
	return nil
}

func (gc *GitHubClient) getUserReviewsInTimeRange(issue *github.Issue, username string, startDate, endDate time.Time) ([]PullRequestReview, error) {
	// Check basic required fields
	if issue.Number == nil || issue.HTMLURL == nil || issue.Title == nil {
		return nil, fmt.Errorf("missing basic required fields")
	}

	// Extract owner and repo from HTML URL
	owner, repo, err := gc.parseRepoFromURL(*issue.HTMLURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse repository info from URL: %v", err)
	}

	// Get all reviews for this PR
	reviews, _, err := gc.client.PullRequests.ListReviews(gc.ctx, owner, repo, *issue.Number, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get PR reviews: %v", err)
	}

	var userReviews []PullRequestReview
	for _, review := range reviews {
		if review.User != nil && review.User.Login != nil && *review.User.Login == username {
			if review.SubmittedAt != nil {
				reviewTime := *review.SubmittedAt
				if reviewTime.After(startDate) && reviewTime.Before(endDate) {
					// Get commit count for this PR
					commits, _, err := gc.client.PullRequests.ListCommits(gc.ctx, owner, repo, *issue.Number, nil)
					commitCount := 0
					if err == nil {
						commitCount = len(commits)
					}

					prInfo := PullRequestInfo{
						Title:       *issue.Title,
						URL:         *issue.HTMLURL,
						Number:      *issue.Number,
						CommitCount: commitCount,
						CreatedAt:   issue.CreatedAt.Time,
					}

					userReviews = append(userReviews, PullRequestReview{
						PullRequest: prInfo,
						ReviewDate:  reviewTime.Time,
					})
				}
			}
		}
	}

	return userReviews, nil
}

func (gc *GitHubClient) fetchIssuesCreated(report *StatusReport) error {
	// Search for issues created by the user in the specified time range
	query := fmt.Sprintf("author:%s type:issue created:%s..%s",
		report.Username,
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "created",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var allIssues []IssueInfo

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opts)
		if err != nil {
			return fmt.Errorf("failed to search for issues created: %v", err)
		}

		for _, issue := range result.Issues {
			// Check for nil fields
			if issue.Title == nil || issue.HTMLURL == nil || issue.Number == nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping issue with missing required fields\n")
				continue
			}

			issueInfo := IssueInfo{
				Title:     *issue.Title,
				URL:       *issue.HTMLURL,
				Number:    *issue.Number,
				CreatedAt: issue.CreatedAt.Time,
			}
			allIssues = append(allIssues, issueInfo)
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	report.IssuesCreated = allIssues
	return nil
}

func (gc *GitHubClient) fetchIssuesCommented(report *StatusReport) error {
	// Search for issues that the user has commented on in the specified time range
	query := fmt.Sprintf("commenter:%s type:issue updated:%s..%s",
		report.Username,
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	var commentedIssues []IssueInfo
	totalComments := 0

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opts)
		if err != nil {
			return fmt.Errorf("failed to search for commented issues: %v", err)
		}

		for _, issue := range result.Issues {
			// Check if the user has commented on this issue in the time range
			hasComments, commentCount, err := gc.checkUserCommentsInTimeRange(issue, report.Username, report.StartDate, report.EndDate)
			if err != nil {
				issueNum := "unknown"
				if issue.Number != nil {
					issueNum = fmt.Sprintf("%d", *issue.Number)
				}
				fmt.Fprintf(os.Stderr, "Warning: failed to check comments for issue #%s: %v\n", issueNum, err)
				continue
			}

			if hasComments {
				// Check for nil fields
				if issue.Title == nil || issue.HTMLURL == nil || issue.Number == nil {
					fmt.Fprintf(os.Stderr, "Warning: skipping commented issue with missing required fields\n")
					continue
				}

				issueInfo := IssueInfo{
					Title:     *issue.Title,
					URL:       *issue.HTMLURL,
					Number:    *issue.Number,
					CreatedAt: issue.CreatedAt.Time,
				}
				commentedIssues = append(commentedIssues, issueInfo)
				totalComments += commentCount
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	report.IssuesCommented = commentedIssues
	report.CommentsCount = totalComments
	return nil
}

func (gc *GitHubClient) checkUserCommentsInTimeRange(issue *github.Issue, username string, startDate, endDate time.Time) (bool, int, error) {
	// Check basic required fields
	if issue.Number == nil || issue.HTMLURL == nil {
		return false, 0, fmt.Errorf("missing basic required fields")
	}

	// Extract owner and repo from HTML URL
	owner, repo, err := gc.parseRepoFromURL(*issue.HTMLURL)
	if err != nil {
		return false, 0, fmt.Errorf("failed to parse repository info from URL: %v", err)
	}

	// Get all comments for this issue
	comments, _, err := gc.client.Issues.ListComments(gc.ctx, owner, repo, *issue.Number, nil)
	if err != nil {
		return false, 0, fmt.Errorf("failed to get issue comments: %v", err)
	}

	commentCount := 0
	for _, comment := range comments {
		if comment.User != nil && comment.User.Login != nil && *comment.User.Login == username {
			if comment.CreatedAt != nil {
				commentTime := *comment.CreatedAt
				if commentTime.After(startDate) && commentTime.Before(endDate) {
					commentCount++
				}
			}
		}
	}

	return commentCount > 0, commentCount, nil
}

func (gc *GitHubClient) fetchAllComments(report *StatusReport) error {
	totalComments := 0

	// Count issue comments (already done in fetchIssuesCommented)
	issueComments := report.CommentsCount

	// Count PR comments
	prComments, err := gc.countPRComments(report.Username, report.StartDate, report.EndDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to count PR comments: %v\n", err)
	} else {
		totalComments += prComments
	}

	// Count commit comments
	commitComments, err := gc.countCommitComments(report.Username, report.StartDate, report.EndDate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to count commit comments: %v\n", err)
	} else {
		totalComments += commitComments
	}

	// Update total comments count (issue comments + PR comments + commit comments)
	report.CommentsCount = issueComments + totalComments

	return nil
}

func (gc *GitHubClient) countPRComments(username string, startDate, endDate time.Time) (int, error) {
	// Search for PRs that the user has commented on
	query := fmt.Sprintf("commenter:%s type:pr updated:%s..%s",
		username,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "updated",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	totalComments := 0

	for {
		result, resp, err := gc.client.Search.Issues(gc.ctx, query, opts)
		if err != nil {
			return 0, fmt.Errorf("failed to search for commented PRs: %v", err)
		}

		for _, issue := range result.Issues {
			if issue.Number == nil || issue.HTMLURL == nil {
				continue
			}

			owner, repo, err := gc.parseRepoFromURL(*issue.HTMLURL)
			if err != nil {
				continue
			}

			// Count PR review comments
			reviewComments, _, err := gc.client.PullRequests.ListComments(gc.ctx, owner, repo, *issue.Number, nil)
			if err == nil {
				for _, comment := range reviewComments {
					if comment.User != nil && comment.User.Login != nil && *comment.User.Login == username {
						if comment.CreatedAt != nil {
							commentTime := *comment.CreatedAt
							if commentTime.After(startDate) && commentTime.Before(endDate) {
								totalComments++
							}
						}
					}
				}
			}

			// Count issue comments on PRs (general PR comments)
			issueComments, _, err := gc.client.Issues.ListComments(gc.ctx, owner, repo, *issue.Number, nil)
			if err == nil {
				for _, comment := range issueComments {
					if comment.User != nil && comment.User.Login != nil && *comment.User.Login == username {
						if comment.CreatedAt != nil {
							commentTime := *comment.CreatedAt
							if commentTime.After(startDate) && commentTime.Before(endDate) {
								totalComments++
							}
						}
					}
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return totalComments, nil
}

func (gc *GitHubClient) countCommitComments(username string, startDate, endDate time.Time) (int, error) {
	// This is more complex as we need to search through repositories the user has access to
	// For now, we'll search for recent commits by the user and check for comments on those
	query := fmt.Sprintf("author:%s committer-date:%s..%s",
		username,
		startDate.Format("2006-01-02"),
		endDate.Format("2006-01-02"))

	opts := &github.SearchOptions{
		Sort:  "committer-date",
		Order: "desc",
		ListOptions: github.ListOptions{
			PerPage: 100,
		},
	}

	totalComments := 0

	for {
		result, resp, err := gc.client.Search.Commits(gc.ctx, query, opts)
		if err != nil {
			// Commit search may have restrictions, so we'll just return 0 if it fails
			fmt.Fprintf(os.Stderr, "Warning: commit search failed: %v\n", err)
			return 0, nil
		}

		for _, commit := range result.Commits {
			if commit.Repository == nil || commit.Repository.Owner == nil ||
				commit.Repository.Owner.Login == nil || commit.Repository.Name == nil ||
				commit.SHA == nil {
				continue
			}

			owner := *commit.Repository.Owner.Login
			repo := *commit.Repository.Name
			sha := *commit.SHA

			// Get comments for this commit
			comments, _, err := gc.client.Repositories.ListCommitComments(gc.ctx, owner, repo, sha, nil)
			if err != nil {
				continue
			}

			for _, comment := range comments {
				if comment.User != nil && comment.User.Login != nil && *comment.User.Login == username {
					if comment.CreatedAt != nil {
						commentTime := *comment.CreatedAt
						if commentTime.After(startDate) && commentTime.Before(endDate) {
							totalComments++
						}
					}
				}
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return totalComments, nil
}

func (report *StatusReport) GenerateMarkdown() string {
	var md strings.Builder

	// Title
	md.WriteString(fmt.Sprintf("# GitHub Status Report - Week %d, %d\n\n", report.Week, report.Year))
	md.WriteString(fmt.Sprintf("**User:** %s  \n", report.Username))
	md.WriteString(fmt.Sprintf("**Period:** %s - %s\n\n",
		report.StartDate.Format("2006-01-02"),
		report.EndDate.Format("2006-01-02")))

	// Pull Requests Created
	md.WriteString("## Pull Requests Created\n\n")
	if len(report.PRsCreated) == 0 {
		md.WriteString("No pull requests created in this period.\n\n")
	} else {
		for _, pr := range report.PRsCreated {
			formattedLink, err := formatLinkWithRepo(pr.URL, pr.Title, pr.Number)
			if err != nil {
				// Fallback to old format if parsing fails
				formattedLink = fmt.Sprintf("[#%d - %s](%s)", pr.Number, pr.Title, pr.URL)
			}
			md.WriteString(fmt.Sprintf("- %s (%d commits)\n", formattedLink, pr.CommitCount))
		}
		md.WriteString(fmt.Sprintf("\n**Total PRs Created:** %d\n\n", len(report.PRsCreated)))
	}

	// Pull Request Reviews
	md.WriteString("## Pull Request Reviews\n\n")
	if len(report.PRReviews) == 0 {
		md.WriteString("No pull request reviews done in this period.\n\n")
	} else {
		for _, review := range report.PRReviews {
			formattedLink, err := formatLinkWithRepo(review.PullRequest.URL, review.PullRequest.Title, review.PullRequest.Number)
			if err != nil {
				// Fallback to old format if parsing fails
				formattedLink = fmt.Sprintf("[#%d - %s](%s)", review.PullRequest.Number, review.PullRequest.Title, review.PullRequest.URL)
			}
			md.WriteString(fmt.Sprintf("- %s\n", formattedLink))
		}
		md.WriteString(fmt.Sprintf("\n**Total PR Reviews:** %d\n\n", len(report.PRReviews)))
	}

	// Issues Created
	md.WriteString("## Issues Created\n\n")
	if len(report.IssuesCreated) == 0 {
		md.WriteString("No issues created in this period.\n\n")
	} else {
		for _, issue := range report.IssuesCreated {
			formattedLink, err := formatLinkWithRepo(issue.URL, issue.Title, issue.Number)
			if err != nil {
				// Fallback to old format if parsing fails
				formattedLink = fmt.Sprintf("[#%d - %s](%s)", issue.Number, issue.Title, issue.URL)
			}
			md.WriteString(fmt.Sprintf("- %s\n", formattedLink))
		}
		md.WriteString(fmt.Sprintf("\n**Total Issues Created:** %d\n\n", len(report.IssuesCreated)))
	}

	// Issues Commented
	md.WriteString("## Issues Commented\n\n")
	if len(report.IssuesCommented) == 0 {
		md.WriteString("No issues commented in this period.\n\n")
	} else {
		for _, issue := range report.IssuesCommented {
			formattedLink, err := formatLinkWithRepo(issue.URL, issue.Title, issue.Number)
			if err != nil {
				// Fallback to old format if parsing fails
				formattedLink = fmt.Sprintf("[#%d - %s](%s)", issue.Number, issue.Title, issue.URL)
			}
			md.WriteString(fmt.Sprintf("- %s\n", formattedLink))
		}
		md.WriteString(fmt.Sprintf("\n**Total Issues Commented:** %d  \n", len(report.IssuesCommented)))
		md.WriteString(fmt.Sprintf("**Total Comments Made:** %d\n\n", report.CommentsCount))
	}

	// Summary
	md.WriteString("## Summary\n\n")
	md.WriteString(fmt.Sprintf("- **PRs Created:** %d\n", len(report.PRsCreated)))
	md.WriteString(fmt.Sprintf("- **PR Reviews:** %d\n", len(report.PRReviews)))
	md.WriteString(fmt.Sprintf("- **Issues Created:** %d\n", len(report.IssuesCreated)))
	md.WriteString(fmt.Sprintf("- **Issues Commented:** %d\n", len(report.IssuesCommented)))
	md.WriteString(fmt.Sprintf("- **Total Comments:** %d\n", report.CommentsCount))

	return md.String()
}
