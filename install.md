## Running and deploying new versions.
You can build a new version as follows
```
go build -o cosm -ldflags "-X main.version=$VERSION"
```
You can run the testsuite
```
go test -v
```
After commiting your changes, you can deploy a new version (here v0.1.2) as follows:
```
git tag v0.1.2
git push origin tag v0.1.2
```
The CI/CD pipeline in `.github/workflows` is then run and new versions are deployed for the following targets:
* cosm-linux-amd64
* cosm-linux-arm64
* cosm-darwin-amd64
* cosm-darwin-arm64