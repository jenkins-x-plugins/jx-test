

# jx test

This chart garbage collects failed test resources

## Installing

- Add jx3 helm charts repo

```bash
helm repo add jx3 https://storage.googleapis.com/jenkinsxio/charts

helm repo update
```

- Install (or upgrade)

```bash
# This will install jx-test in the jx namespace (with a jx-test release name)

# Helm v3
helm upgrade --install jx-test --namespace jx jx3/jx-test
```

Look [below](#values) for the list of all available options and their corresponding description.

## Uninstalling

To uninstall the chart, simply delete the release.

```bash
# This will uninstall jx-test in the jx-test namespace (assuming a jx-test release name)

# Helm v3
helm uninstall jx-test --namespace jx
```

## Version

