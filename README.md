# scanct

`scanct` searches certificate transparency logs for known self-hosted services, hoping to find exposed credentials such as AWS keys.
See [my blog post](https://rgwohlbold.de/2023/scanct/) for a high-level overview.

Currently, the following software is detected:

* **GitLab**: `scanct` clones repositories and scans for secrets using [gitleaks](https://github.com/zricethezav/gitleaks).
* **Jenkins**: `scanct` scans for open `/script` endpoints and downloads workspaces of jobs to scan for secrets with gitleaks.

## Installation

1. Install [Go](https://golang.org/doc/install) for your platform.
2. Clone the repository: `git clone https://github.com/rgwohlbold/scanct`.
3. Build the binary: `go build -o scanct cmd/scanct/main.go`.
4. Run the binary: `./scanct <options>`.

## Usage

All flags are documented in [main.go](cmd/scanct/main.go).
scanct stores all its information in a SQLite database, `instance.db`.
This makes it resilient to restarts, as entries that have not been fully processed are retried on the next run.

## License

`scanct` is licensed under the MIT license. See [LICENSE](LICENSE) for details.

This repository was adapted from [shhgit](https://github.com/eth0izzle/shhgit) and heavily modified, removing almost all code in the progress.
shhgit is licensed under MIT see <https://github.com/eth0izzle/shhgit/blob/master/LICENSE> for details.

Thanks to Lukas Radermacher ([lukasrad02](https://github.com/lukasrad02)) and Tyron Franzke for initially implementing the GitLab scanner into `shhgit`.
