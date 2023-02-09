# Doc Extractor

This application downloads google docs from your google account and saves them into plaintext files with a markdown-ish format. 

To run this project:
You need to have `credentials.json` downloaded from the Google API console into the same working directory as this project. The application will generate a `token.json` after the initial OAuth2 authentication.
```
go run oauth2.go
```

This will create subdirectories which corresponds to the downloaded file types (doc only currently)
```
gdoc-searcher
- doc
  - 1rJz...
  - ZSz5...
- pdf (to come)
```
You can run the application twice - it should be idempotent and not append twice to the same file.