#!/usr/bin/env bash

set -e

# The purpose of this end-to-end test is to allow top-level
# structural refactoring.

# Create tmpdir for all our work
tmpdir="/tmp/deploy_static_site_e2e_tmp_$(date +%Y%m%d%H%M%S)"
mkdir "$tmpdir"

# Create www deployment directories with some initial content
mkdir -p "${tmpdir}/www/branch1"
echo "file a initial content" > "${tmpdir}/www/branch1/file_a"
echo "file e content" > "${tmpdir}/www/branch1/file_e"
mkdir "${tmpdir}/www/branch1/.well-known"
well_known_file="${tmpdir}/www/branch1/.well-known/leave_me_alone"
touch "$well_known_file"

mkdir -p "${tmpdir}/www/branch2"
echo "file f content" > "${tmpdir}/www/branch2/file_f"

# Set up local git server
mkdir "${tmpdir}/gitserver"  # Top-level git server directory
git_url="${tmpdir}/gitserver/project-repo.git"  
mkdir "${git_url}"
pushd "${git_url}" ; git init --bare ; popd

# Populate git repo
mkdir "${tmpdir}/gitclient"
pushd "${tmpdir}/gitclient"

git clone "${git_url}"
cd project-repo
touch .gitignore
git add .gitignore
git commit -m 'initial commit'
git push

git checkout -b b1
mkdir web
echo "file a content" > web/file_a
echo "file b content" > web/file_b
git add web
git commit -m 'add web content files a and b'
git push -u origin b1
git checkout -

git checkout -b b2
mkdir web
echo "file c content" > web/file_c
echo "file d content" > web/file_d
git add web
git commit -m 'add web content files c and d'
git push -u origin b2
git checkout -

popd

# Create config file
cat <<CONFIG > "${tmpdir}/config.json" 
{
    "git_url": "${git_url}",
    "port": "8033",
    "branch_configs": [
        {
            "branch": "b1",
            "target_dir": "${tmpdir}/www/branch1"
        },
        {
            "branch": "b2",
            "target_dir": "${tmpdir}/www/branch2"
        }
    ]
}
CONFIG

# start server
go build -o "${tmpdir}/deploy_static_site" main.go
"${tmpdir}/deploy_static_site" "${tmpdir}/config.json" &
PID=$!
sleep 2

# request refresh for branch 1
curl --include -X POST localhost:8033/refresh -d '{"ref":"refs/heads/b1","otherfield":"othervalue"}'
sleep 2

# stop server
kill $PID

# verify content
# - branch 1 has changed
! [ -f "${tmpdir}/www/branch1/file_e" ]
[ -f "$well_known_file" ]
[ "$(cat "${tmpdir}/www/branch1/file_a")" == "file a content" ]
# - branch 2 has not changed
! [ -f "${tmpdir}/www/branch2/file_c" ]

# Remove tmpdir
rm -rf "$tmpdir"
