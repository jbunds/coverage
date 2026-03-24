[simple-tree]: https://github.com/psnet/simple-tree
[light theme]: ./light_theme.jpg "light theme"
[dark theme]:  ./dark_theme.jpg "dark theme"

#### Simple Web UI for Go Test Coverage

The `coverage` Go module renders an HTML file for each `*.go` source file listed in the specified Go test coverage profile file (typically created per some variation of the invocation `go test -coverprofile <filename> ./...`).

The program expects the specification of two flags with corresponding arguments: `-coverprofile` and `-path` (see [usage](#usage) below).

The generated HTML files are marked up to identify which lines are covered by tests ($\color{seagreen}{\text{green}}$), and which lines are not ($\color{red}{\text{red}}$). Each HTML file is written to the specified path (per the `-path` flag) following the same directory structure as the source from which the coverage profile file (per the `-coverprofile` flag) was created.

The program then creates a `tree.html` file which provides a navigable view of the source rendered as a directory tree within an iframe on the left, where each node is either a subdirectory (`📁 subdirectory`) or a source file (`file.go`). Clicking on a subdirectory node expands its contents, and clicking on a source file node renders the marked up source in the iframe to the right of the directory tree.

Both iframes are hosted by a parent `index.html` file, and both HTML files can be inspected in a browser, either directly via the `file://` scheme, or via an HTTP server using the `http://` scheme.

When served via HTTP, buttons are available to:

- toggle between ***light*** and ***dark*** themes
- toggle between a fully-collapsed and fully-expanded directory tree

#### User Interface

***light*** theme:

![light theme][light theme]

***dark*** theme:

![dark theme][dark theme]

#### But _Why?_

The motivation for the `coverage` module was to create a relatively minimal alternative to the default HTML interface produced by `go tool cover -html <coverage profile filename> -o <html filename>`, with a simple and intuitive UI, and with minimal JavaScript (55 lines total as of this writing, and which implements the functionality of the toggle buttons).

The CSS code was inspired by and adapted from [github.com/psnet/simple-tree][simple-tree], and it clearly still needs to be polished. But I am definitely _not_ a CSS expert, and it fulfills the required behavior as-is.

The `coverage` module may someday provide relatively lightweight GitHub Actions CI artifacts, as its output is highly-compressible plaintext (the roughly 7.2 kB of bundled image files notwithstanding). Of course, its output is directly proportional to its input.

For example, `coverage` generated ~44 MB of HTML content for [`k8s.io/kubernetes`](https://github.com/kubernetes/kubernetes), which can be compressed to ~6.5 MB via `tar czf`, or ~9.3 MB via `zip -r`. Smaller code bases can expect smaller output.

#### Usage

```
$ go get github.com/jbunds/coverage

$ go run github.com/jbunds/coverage
coverage usage:

  -coverprofile string
    	Go test coverage profile file
  -path string
    	path where HTML files will be written
```
