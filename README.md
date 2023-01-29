# scanct

`scanct` uses information from [certificate transparency](https://certificate.transparency.dev/) logs to build a database of hostnames.
Hostnames are then filtered for common software deployments.

Currently, the following software is detected:

* **GitLab**: `scanct` clones repositories and scans for secrets using [gitleaks](https://github.com/zricethezav/gitleaks).
* **Jenkins**: `scanct` scans for open `/script` endpoints and downloads workspaces of jobs to scan for secrets with gitleaks.


## Installation

1. Install [Go](https://golang.org/doc/install) for your platform.
2. Clone the repository: `git clone https://github.com/rgwohlbold/scanct`.
3. Build the binary: `go build -o scanct cmd/scanct/main.go`.
4. Run the binary: `./scanct <options>`.

## License

`scanct` is licensed under the MIT license. See [LICENSE](LICENSE) for details.

This repository was adapted from [shhgit](https://github.com/eth0izzle/shhgit) and heavily modified, removing almost all code in the progress.
shhgit is licensed under MIT see <https://github.com/eth0izzle/shhgit/blob/master/LICENSE> for details.

## Acknowledgements

* Thanks to @lukasrad02 for the idea and for implementing the GitLab scanner
