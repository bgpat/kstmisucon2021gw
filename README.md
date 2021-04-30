# isucon-boilerplate

## usage

### initialize

#### create a new repository

Create a new repository from https://github.com/bgpat/isucon-boilerplate/generate.

#### add members

Invite your team members from https://github.com/$GITHUB_REPOSITORY/settings/access.
After them have confirmed, run [Update SSH Keys](https://github.com/$GITHUB_REPOSITORY/actions?query=workflow%3A%22Update+SSH+Keys%22).

#### install to the competition server

Run following commands in the each competition server:

```bash
cd /
git init
git remote add origin https://github.com/$GITHUB_REPOSITORY.git
git fetch origin master
git reset --hard FETCH_HEAD
make
```

Add deploy key and run `git push -u origin master`.

### edit config file

```bash
vim /etc/nginx/nginx.conf
```

### track source files by git

```bash
git add -f *.go
```
