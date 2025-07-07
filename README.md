# YNAB Vault

A simple Go command-line tool to backup all your YNAB (You Need A Budget) budgets by downloading each as JSON files.

## Features

* Fetch your complete list of budgets via the YNAB API
* Download each budget's detailed JSON export
* Save each file as `BudgetName_BudgetID_Timestamp.json`
* Fully configurable via CLI flags or environment variables
* Optional verbose logging for progress feedback

## Prerequisites

* [Go 1.18+](https://golang.org/dl/)
* A YNAB API access token (Bearer token)

## Installation

1. **Clone the repository:**

   ```bash
   git clone https://github.com/bad33ndj3/ynabvault.git
   ```

2. **Build and install:**

   ```bash
   cd ynabvault
   go install ./...
   ```

3. **Or install directly without cloning:**

   ```bash
   go install github.com/bad33ndj3/ynabvault@latest
   ```

You will need a YNAB API access token. Generate one in your YNAB account as described in the [official documentation](https://api.youneedabudget.com/#authentication).

## Usage

```bash
ynabvault [--token <TOKEN>] [--output <DIR>] [--url <API_URL>] [--verbose]
```

### Flags

* `--token` — YNAB API bearer token. If omitted, falls back to the `YNAB_BEARER_TOKEN` environment variable.
* `--output` —Directory to save the budget JSON files (default: `budgets`).
* `--url` — Base API URL for the budgets endpoint (default: `https://api.youneedabudget.com/v1/budgets`).
* `--verbose` — Enable verbose logging to stderr.

### Environment Variables

* `YNAB_BEARER_TOKEN` — Alternative to `--token` flag for providing the API token.

## Examples

Backup to the default `budgets` folder:

```bash
ynabvault --token YOUR_TOKEN
```

Backup with custom folder and verbose output:

```bash
ynabvault --token YOUR_TOKEN --output ./my_backups --verbose
```

Use environment variable for token:

```bash
export YNAB_BEARER_TOKEN=YOUR_TOKEN
ynabvault --verbose
```

## License

[MIT License](LICENSE)
