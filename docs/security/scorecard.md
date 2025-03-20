# OpenSSF Scorecard Guide

## Overview

The [OpenSSF Scorecard](https://securityscorecards.dev/) is an automated tool that assesses open source projects for security risks. It runs a series of checks against your repository to evaluate security best practices and provides a score that helps identify areas for improvement.

## How It Works

The Scorecard workflow has been set up in `.github/workflows/scorecard.yml` and runs:
- On a weekly schedule (every Tuesday at 7:20 UTC)
- When changes are pushed to the main branch
- When branch protection rules are modified

The workflow performs various security checks and:
1. Uploads results as a GitHub artifact
2. Publishes results to GitHub's code scanning dashboard
3. Publishes results to the public Scorecard API (which powers the badge in our README)

## Scorecard Checks

Scorecard evaluates your project on multiple security criteria, including:

| Check | Description |
|-------|-------------|
| **Branch Protection** | Verifies that the repository has branch protection rules enabled |
| **Code Review** | Checks if the project requires code reviews before merging code |
| **Dependencies** | Evaluates how dependencies are managed and updated |
| **Maintained** | Checks if the project is actively maintained |
| **Vulnerabilities** | Looks for unfixed vulnerabilities |
| **CI Tests** | Verifies that CI tests are run on pull requests |
| **Dangerous Workflows** | Identifies dangerous patterns in GitHub Actions workflows |
| **Binary Artifacts** | Checks for binary artifacts in the repository |
| **SAST** | Verifies if Static Application Security Testing is used |
| **Token Permissions** | Checks if GitHub workflows follow the principle of least privilege |

## Improving Your Score

To improve your Scorecard results:

1. **Enable branch protection rules** for your main branch
   - Require pull request reviews before merging
   - Require status checks to pass before merging
   - Restrict who can push to matching branches

2. **Implement dependency management**
   - Use Dependabot or similar tools to keep dependencies updated
   - Regularly audit and update dependencies

3. **Maintain the project actively**
   - Respond to issues and pull requests
   - Regularly commit to the repository

4. **Implement security scanning**
   - Add CodeQL or other SAST tools to your CI pipeline
   - Run vulnerability scanning on dependencies

5. **Follow least privilege principle**
   - Use read-only tokens when possible in GitHub Actions
   - Limit permissions to what's necessary

6. **Sign your releases and commits**
   - Use GPG to sign commits and tags
   - Verify signatures on dependencies

## Viewing Your Results

You can view your Scorecard results in several ways:
- Click the Scorecard badge in the README
- Check the GitHub Actions workflow run
- View the uploaded SARIF file in GitHub's code scanning dashboard

## Resources

- [OpenSSF Scorecard Documentation](https://github.com/ossf/scorecard/blob/main/docs/checks.md)
- [GitHub Branch Protection](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/defining-the-mergeability-of-pull-requests/about-protected-branches)
- [Dependabot Configuration](https://docs.github.com/en/code-security/dependabot/dependabot-version-updates/configuration-options-for-the-dependabot.yml-file)
- [GitHub Code Scanning](https://docs.github.com/en/code-security/code-scanning/automatically-scanning-your-code-for-vulnerabilities-and-errors/about-code-scanning)
