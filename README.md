# Status Report Tool

A Go tool that generates GitHub status reports in markdown format with configurable output options.

## Features

The tool provides comprehensive GitHub activity tracking:

- **PRs Created**: Lists all PRs created by the user with links, titles, and commit counts
- **PR Reviews**: Lists all PR reviews done by the user with links and titles of reviewed PRs
- **Issues Created**: Lists all issues created by the user with links and titles
- **Issues Commented**: Lists all issues the user commented on
- **Comprehensive Comment Counting**: Counts comments on issues, pull requests, and commits
- **Configurable Output**: Writes to files with customizable naming

## Prerequisites

- Go 1.24 or later
- GitHub personal access token with appropriate permissions

## Setup

1. **Build the tool:**
   ```bash
   go build -o status-report
   ```

2. **Set up authentication:**

   ```bash
   export GITHUB_TOKEN="your_github_personal_access_token"
   ```
   You can create a token at: https://github.com/settings/tokens

## Usage

### Basic Command Structure
```bash
./status-report github [options] <username> <week-number>
```

### Command Options

#### Options
- `-output <file>`: Specify output file path (default: `status-<user>-<week>.md`)

#### Parameters
- `username`: The GitHub username to generate a report for
- `week-number`: ISO 8601 week number (1-53)

### Examples

**Basic usage with default output file:**
```bash
./status-report github john-doe 42
```
This creates `status-john-doe-42.md`

**Custom output file:**
```bash
./status-report github -output my-report.md john-doe 42
```

**Get help:**
```bash
./status-report github -h
```

## Output

The tool generates markdown-formatted reports with the following sections:

- **Pull Requests Created**: Lists all PRs created by the user with links, titles, and commit counts
- **Pull Request Reviews**: Lists all PRs reviewed by the user with links and titles
- **Issues Created**: Lists all issues created by the user with links and titles
- **Issues Commented**: Lists all issues the user commented on
- **Summary**: Provides totals for all activities including comprehensive comment counts

### Default File Naming
By default, output files are named: `status-<username>-<week>.md`

## Comment Counting

The tool provides comprehensive comment counting across:
- **Issue Comments**: Comments on GitHub issues
- **PR Comments**: Both review comments and general discussion comments on pull requests
- **Commit Comments**: Comments on specific commits

This gives a complete picture of the user's engagement across all GitHub activities.

## Dependencies

- `github.com/google/go-github/v57/github`: GitHub API client
- `golang.org/x/oauth2`: OAuth2 authentication for GitHub

## Rate Limiting

The tool respects GitHub API rate limits. If you encounter rate limiting issues, consider:

- Using a personal access token with higher rate limits
- Running the tool less frequently
- Implementing additional rate limiting logic if needed

## Error Handling

The tool provides clear error messages for common issues:

- Missing GitHub token
- Invalid week numbers
- GitHub API errors
- Network connectivity issues
- File writing permissions

## Extensibility

The tool focuses on comprehensive GitHub development activity tracking.

Future integrations could include GitLab, Bitbucket, Azure DevOps, Jira, etc.