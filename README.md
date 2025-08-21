# Deploy static site

Simple service that listens on a port and refreshes the contents of a directory based on a remote git repo.

## To build

    GOARCH=amd64 GOOS=linux go build -o /tmp/deploy_static_site main.go

## To run

    ./deploy_static_site refresh_config.json

## Requirements

This program assumes the following:

    - The code that you want to copy lives under the ./web directory of your
      repo.
    - The only existing content under the target directory that you need to
      preserve is the .well-known directory.
    - Your server has `git` and `rsync` executables, and the user
      running the refresh program can access them in its path.

If you want to use this program and not make these assumptions, feel free to
submit a pull request.

## Config file format

    {
        "git_url": "https://github.....something.git",
        "port": "####",
        "branch_configs": [
            {
                "branch": "my_first_branch",
                "target_dir": "/var/http/site1"
            },
            {
                "branch": "my_second_branch",
                "target_dir": "/var/http/site2"
            }
        ]
    }
