package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "github":
		handleGitHubCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Status Report Tool\n\n")
	fmt.Fprintf(os.Stderr, "Usage: %s <command> [options]\n\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Commands:\n")
	fmt.Fprintf(os.Stderr, "  github    Generate GitHub status report\n\n")
	fmt.Fprintf(os.Stderr, "Environment Variables:\n")
	fmt.Fprintf(os.Stderr, "  GITHUB_TOKEN: Required GitHub personal access token\n\n")
	fmt.Fprintf(os.Stderr, "For command-specific help, use: %s <command> -h\n", os.Args[0])
}

func handleGitHubCommand() {
	githubCmd := flag.NewFlagSet("github", flag.ExitOnError)

	outputFile := githubCmd.String("output", "", "Output file path (default: status-<user>-<week>.md)")

	githubCmd.Usage = func() {
		fmt.Fprintf(os.Stderr, "GitHub Status Report\n\n")
		fmt.Fprintf(os.Stderr, "Usage: %s github [options] <username> <week-number>\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Arguments:\n")
		fmt.Fprintf(os.Stderr, "  username      GitHub username\n")
		fmt.Fprintf(os.Stderr, "  week-number   ISO 8601 week number (1-53)\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		githubCmd.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample: %s github john-doe 42\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s github -output report.md john-doe 42\n", os.Args[0])
	}

	githubCmd.Parse(os.Args[2:])

	if githubCmd.NArg() < 2 {
		githubCmd.Usage()
		os.Exit(1)
	}

	username := githubCmd.Arg(0)
	weekNumber := githubCmd.Arg(1)

	// Determine output file
	var outputFilePath string
	if *outputFile == "" {
		outputFilePath = fmt.Sprintf("status-%s-%s.md", username, weekNumber)
	} else {
		outputFilePath = *outputFile
	}

	fmt.Printf("Generating GitHub status report for user '%s', week %s...\n", username, weekNumber)

	// Create GitHub client
	client, err := NewGitHubClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nPlease ensure GITHUB_TOKEN environment variable is set.\n")
		fmt.Fprintf(os.Stderr, "You can create a token at: https://github.com/settings/tokens\n")
		os.Exit(1)
	}

	// Generate status report
	report, err := client.GenerateStatusReport(username, weekNumber)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating report: %v\n", err)
		os.Exit(1)
	}

	// Generate markdown report
	markdown := report.GenerateMarkdown()

	// Write to file
	err = writeToFile(outputFilePath, markdown)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error writing to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Status report written to: %s\n", outputFilePath)
}

func writeToFile(filePath, content string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if dir != "." {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = file.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write content: %v", err)
	}

	return nil
}

