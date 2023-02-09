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

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/docs/v1"
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

// Splits a doc's body into chunks
func splitBody(doc *docs.Document) []string {
	return []string{}
}

// downloads google doc
func downloadDoc(srv *docs.Service, docId string) *docs.Document {
	doc, err := srv.Documents.Get(docId).Do()
	if err != nil {
		log.Fatalf("Unable to retrieve data from document: %v", err)
	}
	return doc
}

func extractAndSaveDoc(doc *docs.Document, docId string) error {
	r, _ := regexp.Compile("HEADING_")
	os.MkdirAll("doc", os.ModePerm)
	fileName := fmt.Sprintf("doc/%s", docId)
	f, osErr := os.Create(fileName)
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
			flushErr := w.Flush()
			if flushErr != nil {
				return flushErr
			}
		}
	}
	fmt.Printf("Finished processing doc %s", docId)
	return nil
}

func main() {
	ctx := context.Background()
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, "https://www.googleapis.com/auth/documents.readonly")
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := docs.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Docs client: %v", err)
	}

	docId := "1rJz6s712Y0s2dOAEZSz5sGkPgmFWWex5hB_W0WENKnM"
	doc := downloadDoc(srv, docId)
	err = extractAndSaveDoc(doc, docId)
	// TODO update metadata in postgres
	if err != nil {
		panic(err)
	}
}
