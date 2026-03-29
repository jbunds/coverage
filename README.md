[![made-with-Go](https://img.shields.io/badge/Made%20with-Go-1f425f.svg)](https://go.dev/) &nbsp; [![Go Version](https://img.shields.io/badge/go-%20v1.26.1-blue?logo=go)](https://github.com/jbunds/coverage/blob/main/go.mod) &nbsp; [![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)](https://github.com/pre-commit/pre-commit) &nbsp; [![tests](https://github.com/jbunds/coverage/actions/workflows/ci.yml/badge.svg)](https://github.com/jbunds/coverage/actions/workflows/ci.yml) &nbsp; [![coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/jbunds/5a36403860174baeee62844ab96a77d9/raw/coverage.json)](https://github.com/jbunds/coverage/actions/workflows/ci.yml) &nbsp; [![lint](https://github.com/jbunds/coverage/actions/workflows/golangci-lint.yml/badge.svg)](https://github.com/jbunds/coverage/actions/workflows/golangci-lint.yml) &nbsp; [![MIT](https://img.shields.io/badge/license-MIT-blue.svg)](https://opensource.org/licenses/MIT)

[simple-tree]:          https://github.com/psnet/simple-tree
[light theme]:          ./screenshots/light_theme.jpg "light theme"
[dark theme]:           ./screenshots/dark_theme.jpg "dark theme"
[gwatts-gocov-action]:  https://github.com/gwatts/go-coverage-action
[gwatts-gocov-outputs]: https://github.com/gwatts/go-coverage-action/blob/main/action.yml

#### Simple Web UI for Go Test Coverage

Drop-in replacement for `go tool cover -html`.

The `coverage` Go module renders an HTML file for each `*.go` source file listed in the specified Go test coverage profile file (typically created per an invocation of `go test -coverprofile <filename> ./...`, or similar).

The program expects the specification of two flags with corresponding arguments: `-coverprofile` and `-path` (see [usage](#usage) below).

The generated HTML files are marked up to identify which lines are covered by tests ($\color{seagreen}{\text{green}}$), and which lines are not ($\color{red}{\text{red}}$). Each HTML file is written to the specified path (per the `-path` flag) following the same directory structure as the source from which the coverage profile file (per the `-coverprofile` flag) was created.

The program then creates a `tree.html` file which provides a navigable view of the source rendered as a directory tree within an iframe on the left, where each node is either a subdirectory (`📁 subdirectory`) or a source file (`file.go`). Clicking on a subdirectory node expands its contents, and clicking on a source file node renders the marked up source in the iframe to the right of the directory tree.

Both iframes are hosted by a parent `index.html` file, and both HTML files can be inspected in a browser, either directly via the `file://` scheme, or via an HTTP server using the `http://` scheme.

When served via HTTP, buttons are available to:

- toggle between ***light*** and ***dark*** themes
- toggle between a fully-collapsed and fully-expanded directory tree

---

#### User Interface

***light*** theme:

![light theme][light theme]

***dark*** theme:

![dark theme][dark theme]

---

#### GitHub Action Workflow Configuration

Example GitHub workflow configuration (the [`coverage-threshold`][gwatts-gocov-outputs] parameter is optional):

```
jobs:
  coverage: # or whatever you wish to call your workflow
    steps:
    - name: coverage report
      id:   coverage_report
      uses: actions/go-test-coverage-html-report@v1
      with:
        coverage-threshold: 50
```

All [outputs][gwatts-gocov-outputs] produced by the [`gwatts/go-coverage-action`][gwatts-gocov-action] action are available downstream via JSON decoding:

```
${{ fromJson(steps.coverage_report.outputs.all).gcov-pathname    }}
${{ fromJson(steps.coverage_report.outputs.all).report-pathname  }}
${{ fromJson(steps.coverage_report.outputs.all).coverage-pct     }}
${{ fromJson(steps.coverage_report.outputs.all).coverage-pct-1dp }}
${{ fromJson(steps.coverage_report.outputs.all).meets-threshold  }}

# etc...
```

---

#### But _Why?_

The motivation for the `coverage` module was to create a relatively minimal alternative to the default HTML interface produced by `go tool cover -html <coverage profile filename> -o <html filename>`, with a simple and intuitive UI, and with minimal JavaScript (55 lines total as of this writing, to implement the functionality of the toggle buttons).

The CSS code was inspired by and adapted from [github.com/psnet/simple-tree][simple-tree], and it clearly still needs to be polished. But I am definitely _not_ a CSS expert, and it fulfills the required behavior as-is.

---

#### CLI Usage

```
$ go get github.com/jbunds/coverage

$ go run github.com/jbunds/coverage
coverage usage:

  -coverprofile string
    	path to Go test coverage profile file
  -path string
    	path where HTML files will be written
```
