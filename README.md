# Aurelius

Aurelius is a web-based streaming music player with a focus on simplicity.
Stream your personal music collection to your workstation or mobile device using
only a web browser.

Features:

- Broad format support provided by FFmpeg, including video container formats
- ReplayGain
- Network traffic control via:
  - Transcoding
  - Streaming throttled to playback speed
- Play subsections of a track

## Running

### Docker

    docker build -t aurelius .
    docker run -v /path/to/media/library:/media -p 9090:9090 aurelius

Then point your browser to [http://localhost:9090](http://localhost:9090).

### Windows package

A portable `.zip` package containing native Windows binaries can be easily built
from source using Docker:

    docker build -t aurwin docker/windows
    docker run -v /path/to/aurelius:/src aurwin

This produces a package as `/path/to/aurelius/aurelius.zip`.

### Manual source build

Requirements:

- npm
- Go
- FFmpeg
- pkg-config
- GCC/Clang

Building natively on Windows is possible by installing the C dependencies with
[MSYS2](https://www.msys2.org/), but using Docker instead is recommended.

    npm install
    npm run build
    cd cmd/aurelius
    go build

## Using the command line

Command-line arguments can also be passed to `docker run`.

    $ ./aurelius -help
    Usage of ./aurelius:
    -cert string
            TLS certificate file
    -key string
            TLS key file
    -listen string
            [address][:port] at which to listen for connections (default ":9090")
    -log int
            log verbosity (0-3) (default 1)
    -media string
            path to media library root (default ".")

### ReplayGain

Supported metadata (non-exhaustive):

- Tags applied with foobar2000 to most formats
- FLAC tags applied with `metaflac --add-replay-gain`
- Ogg Vorbis tags applied with vorbisgain
- MP3 tags applied with
  [patched mp3gain](https://sourceforge.net/p/mp3gain/patches/5/) (TXXX format)

Currently unsupported:

- WAV tags applied with foobar2000
- MP3 tags applied with unpatched mp3gain (RVA2 format)

## Development

Configuration files are provided for development in Visual Studio Code and its
Remote - Containers extension. Just use
`Remote-Containers: Open Folder in Container...`.

Press Ctrl+Shift+B to show build tasks, and press F5 to run.

### Testing

Run Go tests:

    go test ./...

Run TypeScript tests:

    npm run test

Generate a TypeScript code coverage report in `coverage/lcov-report/index.html`:

    npm run coverage
