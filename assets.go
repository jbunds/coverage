package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
)

// writeStaticFiles writes the static files required by the coverage report to the user-specified path.
func (rg *reportGenerator) writeStaticFiles() error {
	for _, file := range rg.staticFiles {
		outFile   := filepath.Join(rg.outRoot.Name(), filepath.Base(file))
		f, err    := rg.fsys.Create(outFile)
		if                                        err != nil { return fmt.Errorf("cannot create %q: %w",     outFile, err) }
		data, err := fs.ReadFile(rg.embeddedFiles, file)
		if                                        err != nil { return fmt.Errorf("cannot read %q: %w",          file, err) }
		if _, err := fmt.Fprint(f, string(data)); err != nil { return fmt.Errorf("cannot write file %q: %w", outFile, err) }
		if err := f.Close();                      err != nil { return fmt.Errorf("cannot close file %q: %w", outFile, err) }
	}

	return nil
}
