package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Retrieves a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Requests a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache OAuth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

// get list of gdoc file information
func getGdocList(srv *drive.Service, numDocuments int64) []*drive.File {
	fileList, err := srv.Files.List().Corpora("user").OrderBy("createdTime desc").
		PageSize(numDocuments).Q(`mimeType = 'application/vnd.google-apps.document'`).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve file list from google drive.\n%s", err)
		return []*drive.File{}
	}
	if fileList.Files != nil {
		log.Printf("Obtained %d files", len(fileList.Files))
		return fileList.Files
	} else {
		log.Println("No files returned from google drive")
		return []*drive.File{}
	}
}

// downloads google doc
func downloadDoc(srv *docs.Service, docId string) *docs.Document {
	doc, err := srv.Documents.Get(docId).Do()
	// TODO implement retries
	if err != nil {
		log.Fatalf("Unable to retrieve data from document: %v", err)
	}
	log.Printf("Downloaded doc %s %s...", doc.Title, docId[:10])
	return doc
}

// extract text content from doc and write to disk as a text file
func extractAndSaveDoc(doc *docs.Document, docId string) error {
	r, _ := regexp.Compile("HEADING_")
	os.MkdirAll("doc", os.ModePerm)
	fileName := fmt.Sprintf("doc/%s", docId)
	f, osErr := os.Create(fileName)
	defer f.Close()
	if osErr != nil {
		return osErr
	}
	w := bufio.NewWriter(f)
	for _, b := range doc.Body.Content {
		if b.Paragraph != nil && b.Paragraph.Elements != nil {
			// get heading level and prepend '#'
			if b.Paragraph.ParagraphStyle != nil {
				idx := r.FindStringIndex(b.Paragraph.ParagraphStyle.NamedStyleType)
				if idx != nil {
					headingLevel, _ := strconv.Atoi(b.Paragraph.ParagraphStyle.NamedStyleType[idx[1]:])
					for i := 0; i < headingLevel; i++ {
						w.WriteString("#") // ignore number of bytes written
					}
					w.WriteString(" ")
				}
			}
			// extract text
			for _, p := range b.Paragraph.Elements {
				if p.TextRun != nil {
					w.WriteString(p.TextRun.Content)
				}
			}
			// TODO extract (un)ordered list elements
			flushErr := w.Flush()
			if flushErr != nil {
				return flushErr
			}
		}
	}
	log.Printf("Finished processing doc %s %s...", doc.Title, docId[:10])
	return nil
}

// download, extract and save document
func downloadExtractSaveDoc(srv *docs.Service, docId string, wg *sync.WaitGroup) {
	defer wg.Done()
	doc := downloadDoc(srv, docId)
	err := extractAndSaveDoc(doc, docId)
	if err != nil {
		log.Printf("Could not extract and save doc %s %s\n%s", doc.Title, docId, err)
	}
}

func main() {

	// authentication
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, docs.DocumentsReadonlyScope, drive.DriveReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	// setup Google API clients (drive and gdocs)
	driveService, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Drive client: %v", err)
	}
	docsService, err := docs.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Docs client: %v", err)
	}

	docFiles := getGdocList(driveService, 10)

	var wg sync.WaitGroup
	wg.Add(len(docFiles))
	for _, docFile := range docFiles {
		go downloadExtractSaveDoc(docsService, docFile.Id, &wg)

		// TODO update metadata in postgres
	}
	wg.Wait()
}
