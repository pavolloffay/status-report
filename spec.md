# Status report tool

* The tool is written in golang and creates a status report in markdown format.
* The tool is configurable via flags.
* The tool creates status report by default called status-<user>-<week>.md but the output file can be configured via a flag.

## Github sub-command

* As input takes a github username and a week number
* The links should be in a format [<user/org/>/<repository-name>#123 - title](link)
* The output is list of PRs created by the user in the given week. The output should have a link to the PR and the title of the PR plus number of commits in the PR.
* The output should list pull request reviews done by the user in the given week with a link and title of the PR.
* The output should list all issues created by the user in the given week. The output should have a link to the issue and the title of the issue.
* The output should list all issues commented by the user in the given week. 
* The output should have the number of comments done by the user in the given week. Comments on any issue, pull request or commit.