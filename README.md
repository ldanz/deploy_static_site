# Deploy static site

Simple service that listens on a port and refreshes the contents of a directory based on a remote git repo.

## To build

    GOARCH=amd64 GOOS=linux go build -o /tmp/deploy_static_site main.go

## To run

    ./deploy_static_site refresh_config.json

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
