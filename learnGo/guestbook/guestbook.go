package main

import (
	"bufio"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

type Guestbook struct {
	SignatureCount int
	Signatures     []string
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func getStrings(filename string) []string {
	var lines []string
	file, err := os.Open(filename)
	if os.IsNotExist(err) {
		return nil
	}
	check(err)
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	check(scanner.Err())
	return lines
}

func viewHandler(writer http.ResponseWriter, request *http.Request) {
	wd, err := os.Getwd()
	check(err)
	filePath_view := filepath.Join(wd, "guestbook/view.html")
	filePath_signature := filepath.Join(wd, "guestbook/signatures.txt")
	signatures := getStrings(filePath_signature)
	html, err := template.ParseFiles(filePath_view)
	check(err)
	guestbook := Guestbook{
		SignatureCount: len(signatures),
		Signatures:     signatures,
	}
	err = html.Execute(writer, guestbook)
	check(err)
}

func newHandler(writer http.ResponseWriter, request *http.Request) {
	wd, err := os.Getwd()
	check(err)
	filePath_new := filepath.Join(wd, "guestbook/new.html")
	html, err := template.ParseFiles(filePath_new)
	check(err)
	err = html.Execute(writer, nil)
	check(err)
}

func createHandler(writer http.ResponseWriter, request *http.Request) {
	wd, err := os.Getwd()
	check(err)
	filePath_signature := filepath.Join(wd, "guestbook/signatures.txt")
	signature := request.FormValue("signature")
	options := os.O_WRONLY | os.O_CREATE | os.O_APPEND
	file, err := os.OpenFile(filePath_signature, options, os.FileMode(0600))
	check(err)
	_, err = fmt.Fprintln(file, signature)
	err = file.Close()
	check(err)
	http.Redirect(writer, request, "/guestbook", http.StatusFound)

}

func main() {
	http.HandleFunc("/guestbook", viewHandler)
	http.HandleFunc("/guestbook/new", newHandler)
	http.HandleFunc("/guestbook/create", createHandler)
	err := http.ListenAndServe("localhost:8080", nil)
	log.Fatal(err)
}
