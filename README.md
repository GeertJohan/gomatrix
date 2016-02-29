## THIS IS A FORK of github.com/GeertJohan/gomatrix

> _This fork uses github.com/gdamore/tcell
> It offers better (nicer/richer) colors and can work on terminals that don't
> support Unicode.  For example, it works on GB18030, and EUC-JP terminals.
> If your environment is not UTF-8 compliant, the glyphs will be replaced with
> "?"'s by default.  Press "a" to see ASCII in that case._

## gomatrix
gomatrix connects to The Matrix and displays it's data streams in your terminal.

### Installation
Install from source with `go get github.com/GeertJohan/gomatrix`

### Usage
Just run `gomatrix`. Use `gomatrix --help` to view all options.

### Docker
This application is available in docker.

Build manually with `docker build -t gomatrix .` and `docker run -ti gomatrix`.

Or pull the automated build version: `docker run -ti geertjohan/gomatrix`

### License:
This project is licenced under a a Simplified BSD license. Please read the [LICENSE file](LICENSE).
