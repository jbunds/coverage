[simple-tree]: https://github.com/psnet/simple-tree

#### Simple Web UI for Go Test Coverage

The `coverage` Go module renders an HTML file for each `*.go` source file listed in a given Go test coverage profile file.

The generated HTML files are marked up to identify which lines are covered by tests (`$\color{seagreen}{\text{green}}$`), and which lines are not (`$\color{red}{\text{red}}$`). Each HTML file is written to the specified path following the same directory structure as the source code which was tested to generate the Go test coverage profile file.

The main program then creates a `tree.html` file which mirrors the directory structure of the tested source code, and which is rendered in a very simple Web UI accessible via the `index.html` file. Both HTML files can be inspected in a browser, either directly via the `file://` scheme, or via an HTTP server using the `http://` scheme. When served via HTTP, buttons are available to:

- toggle between ***light*** and ***dark*** themes
- toggle between a fully-collapsed and fully-expanded directory tree

#### But Why?

The motivation for the `coverage` Go module was to create a relatively minimal alternative to the default HTML interface produced by `go tool cover -html <coverage profile> -o <html file>`, with a simple and intuitive UI, and with minimal JavaScript (59 lines total as of this writing, used to implement the functionality of the toggle buttons).

The CSS code was inspired by and adapted from [github.com/psnet/simple-tree][simple-tree], and it _clearly_ still needs to be polished. But I am definitly _not_ a CSS expert, and it fulfills the required behavior as-is.

The `coverage` module may someday provide relatively lightweight GitHub Actions CI artifacts, as its output is highly-compressible plaintext (the roughly 7.2 kB of bundled image files notwithstanding).

#### Usage

```
$ go build

$ ./coverage -h
coverage usage:

  -coverprofile string
    	Go test coverage profile file
  -path string
    	path where HTML files will be written
```
