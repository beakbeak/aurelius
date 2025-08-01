# Aurelius

![Aurelius](img/aurelius.svg)

Aurelius is a web-based streaming music player with a focus on simplicity.
Stream your personal music collection to your workstation or mobile device using
only a web browser.

Try the [live demo](https://aurelius.app/) (password: demo).

<a href="https://pkg.go.dev/github.com/beakbeak/aurelius"><img src="https://pkg.go.dev/badge/github.com/beakbeak/aurelius" alt="PkgGoDev"></a>

Features:

- Broad format support provided by FFmpeg, including video container formats
- ReplayGain
- Network traffic control via:
  - Transcoding
  - Streaming throttled to playback speed

## Running

### Docker

Pull from Docker Hub:

    docker run --rm -it -v /path/to/media/library:/media -p 9090:9090 beakbeak/aurelius

Or, build from source:

    docker build -t aurelius .
    docker run --rm -it -v /path/to/media/library:/media -p 9090:9090 aurelius

Then point your browser to [http://localhost:9090](http://localhost:9090).

Note that if using the above commands, an additional mount to `/storage` will be
needed for favorites to persist across runs.

### Windows package

A portable `.zip` package containing native Windows binaries can be easily built
from source using Docker:

    docker build -t aurwin docker/windows
    docker run -v /path/to/aurelius:/src aurwin

This produces a package as `/path/to/aurelius/aurelius.zip`.

### Manual source build

See [Building without a container](#building-without-a-container).

## Command-line arguments

Command-line arguments can also be passed to `docker run`.

    $ ./aurelius -help
    Usage of ./aurelius:
        -cert string
                TLS certificate file.
        -config string
                Path to ini file containing values for command-line flags in 'flagName = value' format.
        -dumpflags
                Print values for all command-line flags to stdout in a format compatible with -config, then exit.
        -key string
                TLS key file.
        -listen string
                [address][:port] at which to listen for connections. (default ":9090")
        -media string
                Path to media library root. (default ".")
        -noThrottle
                Don't limit streaming throughput to playback speed.
        -pass string
                Passphrase used for login. If unspecified, access will not be restricted.
            
                WARNING: Passphrases from the client will be transmitted as plain text,
                so use of HTTPS is recommended.
        -storage string
                Path to directory where persistent data (favorites, etc.) will be stored.
                It will be created if it doesn't exist. (default ".")

## Usage notes

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

### Importing favorites

Favorites are stored in a file named `favorites.m3u` in the configured
persistent storage directory (`-storage`). Each line of the file is a path
relative to the root of the media library (`-media`).

### Playing subsections of a track

It's possible to create descriptor files that represent subsections of a track.
See [documentation for pkg/fragment](https://pkg.go.dev/github.com/beakbeak/aurelius/pkg/fragment?tab=doc).

### Transcoding

Currently, streamed audio is always transcoded. For playback with no quality
loss, choose the FLAC codec under Settings > Stream Encoding.

## Development

Configuration files are provided for development in Visual Studio Code and its
Remote - Containers extension. Just use
`Remote-Containers: Open Folder in Container...`.

Press Ctrl+Shift+B to show build tasks, and press F5 to run.

### Testing

Run Go tests:

    go test -asan ./...

Run TypeScript tests:

    npm run test

Generate a TypeScript code coverage report in `coverage/lcov-report/index.html`:

    npm run coverage

### Building without a container

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

## Licensing

Aurelius is licensed under the Apache License Version 2.0. See
[LICENSE.txt](LICENSE.txt) and [NOTICE.txt](NOTICE.txt) for details.
