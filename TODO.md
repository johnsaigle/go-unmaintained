# TODO

## De-slopify README and Makefile

## Fix lint violations

## Reduce code complexity; see if there are duplicated sections

## GitHub action for popular cache build does not work

The PAT is the same one that we do for regular scanning.
```
0s
Run PACKAGE_COUNT="500"
Building cache with 500 packages...
Building popular packages cache...
  Count: 500 packages
  Output: ./pkg/popular/data/popular-packages.json
  Inactive threshold: 365 days

Error building cache: failed to create GitHub client: GitHub token validation failed: GitHub token lacks necessary permissions
Error: Process completed with exit code 1.
Run PACKAGE_COUNT="500"
Building cache with 500 packages...
Building popular packages cache...
  Count: 500 packages
  Output: ./pkg/popular/data/popular-packages.json
  Inactive threshold: 365 days

Error building cache: failed to create GitHub client: GitHub token validation failed: GitHub token lacks necessary permissions
```
