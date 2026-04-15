[![Go Version](https://img.shields.io/badge/go-%20v1.26.1-blue?logo=go)](https://github.com/jbunds/coverage/blob/main/go.mod) &nbsp; [![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit)](https://github.com/pre-commit/pre-commit) &nbsp; [![tests](https://github.com/jbunds/coverage/actions/workflows/test-go.yml/badge.svg)](https://github.com/jbunds/coverage/actions/workflows/test-go.yml) &nbsp; [![coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/jbunds/5a36403860174baeee62844ab96a77d9/raw/coverage.json)](https://github.com/jbunds/coverage/actions/workflows/test-go.yml) &nbsp; [![lint](https://github.com/jbunds/coverage/actions/workflows/lint-go.yml/badge.svg)](https://github.com/jbunds/coverage/actions/workflows/lint-go.yml) &nbsp; [![ESLint | neostandard](https://img.shields.io/badge/ESLint-neostandard-brightgreen?style=flat)](https://github.com/neostandard/neostandard)

[simple-tree]:          https://github.com/psnet/simple-tree
[k8s]:                  https://github.com/kubernetes/kubernetes
[light theme]:          ./screenshots/light_theme.jpg "light theme"
[dark theme]:           ./screenshots/dark_theme.jpg "dark theme"
[gwatts-gocov-action]:  https://github.com/gwatts/go-coverage-action
[gwatts-gocov-outputs]: https://github.com/gwatts/go-coverage-action/blob/main/action.yml
[action]:               https://github.com/jbunds/coverage/blob/main/action.yml
[workflow]:             https://github.com/jbunds/coverage/blob/main/.github/workflows/pages.yml
[actions]:              https://docs.github.com/actions
[workflows]:            https://docs.github.com/actions/concepts/workflows-and-actions/workflows
[pages]:                https://docs.github.com/pages

#### Simple Web UI for Go Test Coverage

Drop-in replacement for `go tool cover -html`.

The `coverage` Go module renders an HTML file for each `*.go` source file listed in the specified Go test coverage profile file (typically created per an invocation of `go test -coverprofile <filename> ./...`, or similar).

The program expects the specification of three flags with corresponding arguments (see [usage](#cli-usage) below):

```
-gomod         # path to the root go.mod file
-coverprofile  # path to the Go test coverage profile file
-path          # path where HTML files will be written
```

The generated HTML files are marked up to identify which lines are covered by tests ($\color{seagreen}{\text{green}}$), and which lines are not ($\color{red}{\text{red}}$). Each HTML file is written to the specified path (per the `-path` flag) following the same directory structure as the source from which the coverage profile file (per the `-coverprofile` flag) was created.

The program then creates a `tree.html` file which provides a navigable view of the source rendered as a directory tree within an iframe on the left, where each node is either a subdirectory (`📁 <subdirectory>`) or a source file (`<source file>.go`). Clicking on a subdirectory node expands its contents, and clicking on a source file node renders the marked up source in the iframe to the right of the directory tree.

Both iframes are hosted by a parent `index.html` file, and both HTML files can be inspected in a browser, either directly via the `file://` scheme, or via an HTTP server using the `http://` scheme.

When served via HTTP, buttons are available to:

- <img height="20" style="vertical-align: middle;" alt="theme button"  src="./theme.png" > toggle between **light** and **dark** themes
- <img height="20" style="vertical-align: middle;" alt="expand button" src="./expand.png"> toggle between a fully-collapsed and fully-expanded directory tree

See also [![DeepWiki](https://img.shields.io/badge/DeepWiki-jbunds%2Fcoverage-blue.svg?logo=data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAACwAAAAyCAYAAAAnWDnqAAAAAXNSR0IArs4c6QAAA05JREFUaEPtmUtyEzEQhtWTQyQLHNak2AB7ZnyXZMEjXMGeK/AIi+QuHrMnbChYY7MIh8g01fJoopFb0uhhEqqcbWTp06/uv1saEDv4O3n3dV60RfP947Mm9/SQc0ICFQgzfc4CYZoTPAswgSJCCUJUnAAoRHOAUOcATwbmVLWdGoH//PB8mnKqScAhsD0kYP3j/Yt5LPQe2KvcXmGvRHcDnpxfL2zOYJ1mFwrryWTz0advv1Ut4CJgf5uhDuDj5eUcAUoahrdY/56ebRWeraTjMt/00Sh3UDtjgHtQNHwcRGOC98BJEAEymycmYcWwOprTgcB6VZ5JK5TAJ+fXGLBm3FDAmn6oPPjR4rKCAoJCal2eAiQp2x0vxTPB3ALO2CRkwmDy5WohzBDwSEFKRwPbknEggCPB/imwrycgxX2NzoMCHhPkDwqYMr9tRcP5qNrMZHkVnOjRMWwLCcr8ohBVb1OMjxLwGCvjTikrsBOiA6fNyCrm8V1rP93iVPpwaE+gO0SsWmPiXB+jikdf6SizrT5qKasx5j8ABbHpFTx+vFXp9EnYQmLx02h1QTTrl6eDqxLnGjporxl3NL3agEvXdT0WmEost648sQOYAeJS9Q7bfUVoMGnjo4AZdUMQku50McDcMWcBPvr0SzbTAFDfvJqwLzgxwATnCgnp4wDl6Aa+Ax283gghmj+vj7feE2KBBRMW3FzOpLOADl0Isb5587h/U4gGvkt5v60Z1VLG8BhYjbzRwyQZemwAd6cCR5/XFWLYZRIMpX39AR0tjaGGiGzLVyhse5C9RKC6ai42ppWPKiBagOvaYk8lO7DajerabOZP46Lby5wKjw1HCRx7p9sVMOWGzb/vA1hwiWc6jm3MvQDTogQkiqIhJV0nBQBTU+3okKCFDy9WwferkHjtxib7t3xIUQtHxnIwtx4mpg26/HfwVNVDb4oI9RHmx5WGelRVlrtiw43zboCLaxv46AZeB3IlTkwouebTr1y2NjSpHz68WNFjHvupy3q8TFn3Hos2IAk4Ju5dCo8B3wP7VPr/FGaKiG+T+v+TQqIrOqMTL1VdWV1DdmcbO8KXBz6esmYWYKPwDL5b5FA1a0hwapHiom0r/cKaoqr+27/XcrS5UwSMbQAAAABJRU5ErkJggg==)](https://deepwiki.com/jbunds/coverage)

---

#### User Interface

[demo.webm](https://github.com/user-attachments/assets/a9b5a7e6-7450-4846-b251-f6fc5e2ed906)

**light** theme:

![light theme][light theme]

**dark** theme:

![dark theme][dark theme]

---

#### CLI Usage

```
$ go get github.com/jbunds/coverage

$ go run github.com/jbunds/coverage
coverage usage:

  -coverprofile string
    	path to Go test coverage profile file
  -gomod string
    	path to the root go.mod file
  -path string
    	path where HTML files will be written
```

---

#### GitHub Workflow Configuration

Aside from the [CLI interface](#cli-usage) outlined above, there are two ways to incorporate the `coverage` module within [GitHub workflows][workflows]:

1. The [`jbunds/coverage@v1`][action] reusable [GitHub Action][actions] generates the test coverage report and writes the files comprising the report to `coverage-report-path`. For example:

```
- uses: jbunds/coverage@v1
  with:
    go-version:           '1.26.1'           # optional; default is '1.26.1'
    go-mod:               'go.mod'           # optional; default is 'go.mod'
    coverage-threshold:   '50'               # optional; default is '0'
    coverage-report-path: 'coverage_report'  # optional; default is 'coverage_report'
```

The [`go-version`][action], [`go-mod`][action], [`coverage-threshold`][gwatts-gocov-outputs], and [`coverage-report-path`][workflow] parameters are optional.

All [outputs][gwatts-gocov-outputs] produced by the [`gwatts/go-coverage-action`][gwatts-gocov-action] workflow step are available downstream via JSON decoding, e.g.:

```
${{ fromJson(steps.coverage_report.outputs.all).gcov-pathname    }}
${{ fromJson(steps.coverage_report.outputs.all).report-pathname  }}
${{ fromJson(steps.coverage_report.outputs.all).coverage-pct     }}
${{ fromJson(steps.coverage_report.outputs.all).coverage-pct-1dp }}
${{ fromJson(steps.coverage_report.outputs.all).meets-threshold  }}
```

etc...

2. The [`jbunds/coverage/.github/workflows/pages.yml@v1`][workflow] reusable [GitHub Workflow][workflows] generates the test coverage report and also deploys it to [GitHub Pages][pages]. For example:

```
- uses: jbunds/coverage/.github/workflows/pages.yml@v1
  with:
    go-version:           '1.26.1'           # optional: default is '1.26.1'
    go-mod:               'go.mod'           # optional; default is 'go.mod'
    coverage-threshold:   '50'               # optional; default is '0'
    coverage-report-path: 'coverage_report'  # optional; default is 'coverage_report'
```

See https://jbunds.github.io/coverage/ for an example, which is not particularly interesting since it consists of just three Go source files which all reside in the repo's root directory.

The well-known and relatively large (500k+ LoC) [Kubernetes][k8s] project was chosen for the demo to better illustrate the features and performance.

---

#### But _Why?_

The motivation for the `coverage` module was to create a relatively minimal alternative to the default HTML interface produced by `go tool cover -html <coverage profile filename> -o <html filename>`, with a simple and intuitive UI, and with minimal JavaScript (55 lines total as of this writing, to implement the functionality of the toggle buttons).

The CSS code was inspired by and adapted from [github.com/psnet/simple-tree][simple-tree], and it clearly still needs to be polished. But I am definitely _not_ a CSS expert, and it fulfills the required behavior as-is.
