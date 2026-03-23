[simple-tree]: https://github.com/psnet/simple-tree

#### Simple Web UI for Go Test Coverage

The `coverage` Go module renders an HTML file for each `*.go` source file listed in a given Go test coverage profile file.

The generated HTML files are marked up to identify which lines are covered by tests (green), and which lines are not (red). Each HTML file is written to the specified path following the same directory structure as the source code which was tested to generate the Go test coverage profile file.

The program then creates a `tree.html` file which mirrors the directory structure of the tested source code, and which is rendered in a very simple Web UI accessible via the `index.html` file. Both HTML files can be inspected in a browser, either directly via the `file://` scheme, or via an HTTP server using the `http://` scheme. When served via an HTTP server, buttons are available to:

- toggle between "light" and "dark" themes
- toggle between a fully-expanded and fully-collapsed directory tree

The motivation for the `coverage` Go module was to produce a relatively minimal alternative to the default HTML interface produced by `go tool cover -html <coverage profile> -o <html file>`, with a simple and intuitive UI, and with minimal JavaScript (59 lines total as of this writing).

The CSS code was inspired by and adapted from [github.com/psnet/simple-tree][simple-tree]. The CSS code clearly still needs to be polished, but I am not a CSS expert, and it fulfills the required behavior as-is.

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
