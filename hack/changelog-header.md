### Linux

```shell
curl -L https://github.com/jenkins-x-plugins/jx-test/releases/download/v{{.Version}}/jx-test-linux-amd64.tar.gz | tar xzv 
sudo mv jx-test /usr/local/bin
```

### macOS

```shell
curl -L  https://github.com/jenkins-x-plugins/jx-test/releases/download/v{{.Version}}/jx-test-darwin-amd64.tar.gz | tar xzv
sudo mv jx-test /usr/local/bin
```

