// yaml_stream.jsonnet
local Build(mirror, go, alpine, os, arch) = {
    kind: "pipeline",
    type: "docker",
    name: "build-" + go + "-alpine" + alpine + "-" + arch,
    platform: {
        os: os,
        arch: arch
    },
    environment: {
        GOTOOLCHAIN: "local",
        CGO_ENABLED: "0",
        GOOS: os,
        GOARCH: arch,
    },
    steps: [
        {
            name: "build",
            image: "golang:" + go +"-alpine" + alpine,
            mirror: mirror,
            commands: [
                "apk update",
                "apk add --no-cache git",
                "mkdir .bin",
                "go build -tags coraza.rule.no_regex_multiline -v -pgo=auto -v -trimpath -ldflags=-buildid= -o ./.bin/go-away ./cmd/go-away",
                "go build -v -o ./.bin/test-wasm-runtime ./cmd/test-wasm-runtime",
            ],
        },
        {
            name: "check-policy-forgejo",
            image: "alpine:" + alpine,
            mirror: mirror,
            depends_on: ["build"],
            commands: [
                "./.bin/go-away --check --slog-level DEBUG --backend example.com=http://127.0.0.1:80 --policy examples/forgejo.yml --policy-snippets examples/snippets/"
            ],
        },
        {
            name: "check-policy-generic",
            image: "alpine:" + alpine,
            mirror: mirror,
            depends_on: ["build"],
            commands: [
                "./.bin/go-away --check --slog-level DEBUG --backend example.com=http://127.0.0.1:80 --policy examples/generic.yml --policy-snippets examples/snippets/"
            ],
        },
        {
            name: "check-policy-spa",
            image: "alpine:" + alpine,
            mirror: mirror,
            depends_on: ["build"],
            commands: [
                "./.bin/go-away --check --slog-level DEBUG --backend example.com=http://127.0.0.1:80 --policy examples/spa.yml --policy-snippets examples/snippets/"
            ],
        },
        {
            name: "test-wasm-success",
            image: "alpine:" + alpine,
            mirror: mirror,
            depends_on: ["build"],
            commands: [
                "./.bin/test-wasm-runtime -wasm ./embed/challenge/js-pow-sha256/runtime/runtime.wasm " +
                "-make-challenge ./embed/challenge/js-pow-sha256/test/make-challenge.json " +
                "-make-challenge-out ./embed/challenge/js-pow-sha256/test/make-challenge-out.json " +
                "-verify-challenge ./embed/challenge/js-pow-sha256/test/verify-challenge.json " +
                "-verify-challenge-out 0",
            ],
        },
        {
            name: "test-wasm-fail",
            image: "alpine:" + alpine,
            mirror: mirror,
            depends_on: ["build"],
            commands: [
                "./.bin/test-wasm-runtime -wasm ./embed/challenge/js-pow-sha256/runtime/runtime.wasm " +
                "-make-challenge ./embed/challenge/js-pow-sha256/test/make-challenge.json " +
                "-make-challenge-out ./embed/challenge/js-pow-sha256/test/make-challenge-out.json " +
                "-verify-challenge ./embed/challenge/js-pow-sha256/test/verify-challenge-fail.json " +
                "-verify-challenge-out 1",
            ],
        },
    ]
};

local alpineVersion = "3.21";
local goVersion = "1.25";

local mirror = "https://mirror.gcr.io";

[
    Build(mirror, goVersion, alpineVersion, "linux", "amd64") + {"trigger": {event: ["push", "tag"], }},
    Build(mirror, goVersion, alpineVersion, "linux", "arm64") + {"trigger": {event: ["push", "tag"], }},

    # Test PRs
    Build(mirror, goVersion, alpineVersion, "linux", "amd64") + {"name": "test-pr", "trigger": {event: ["pull_request"], }},
]