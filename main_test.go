package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type sseReader struct {
	scanner *bufio.Scanner
}
type nonBlocking struct {
	Response *http.Response
	Error    error
}

func Get(url string, ch chan nonBlocking) {
	resp, err := http.Get(url)
	ch <- nonBlocking{
		Response: resp,
		Error:    err,
	}
}

func (r *sseReader) Read(p []byte) (n int, err error) {
	if !r.scanner.Scan() {
		if r.scanner.Err() != nil {
			return 0, r.scanner.Err()
		}
		return 0, io.EOF
	}
	return copy(p, r.scanner.Bytes()), nil
}

func renderHTML(filename string, data struct {
	Question string
	Id       int
}) (string, error) {
	fileContent, err := ioutil.ReadFile(filename)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("template").Parse(string(fileContent))
	if err != nil {
		return "", err
	}

	var renderedHTML string
	buf := new(bytes.Buffer)
	err = tmpl.Execute(buf, data)
	if err != nil {
		return "", err
	}
	renderedHTML = string(buf.Bytes())
	return renderedHTML, nil
}

func TestMainPage(t *testing.T) {
	ch := make(chan Text, 1)
	type SleepTest struct {
		sleep   uint
		timeout uint
	}
	tests := []SleepTest{
		{sleep: 1, timeout: 2},
		{sleep: 2, timeout: 1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("sleep=%d, timeout=%d", tt.sleep, tt.timeout), func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("/", MakeMainHandler(ch, 0, tt.sleep))
			mux.HandleFunc("/processed", MakeSSEDependantHandler(ch, tt.sleep))

			svr := httptest.NewServer(mux)
			defer svr.Close()

			// Simulate a POST request to the main page
			formData := url.Values{}
			formData.Add("input_text", "test")
			res, err := http.PostForm(svr.URL, formData)
			if err != nil {
				t.Errorf("expected error to be nil, got %v", err)
				return
			}
			defer res.Body.Close()

			data, err := ioutil.ReadAll(res.Body)
			if err != nil {
				t.Errorf("expected error to be nil, got %v", err)
				return
			}

			expectedHTML, err := renderHTML("templates/snippet.html", struct {
				Question string
				Id       int
			}{Question: "You have entered: test", Id: 1})
			if err != nil {
				t.Fatalf("An error occurred: %s", err)
			}

			if string(data) != expectedHTML {
				t.Errorf("Expected:\n%s\nGot:\n%s", expectedHTML, string(data))
				return
			}

			// Simulate SSE events
			rch := make(chan nonBlocking, 1)
			go Get(svr.URL+"/processed", rch)

			hasTimedOut := false
			var event string
			expected := fmt.Sprintf("data: <p id='answer-0'>You have entered: test. Waited %d seconds</p>", tt.sleep)
			timeout := time.After(time.Duration(tt.timeout) * time.Second)
			select {
			case <-timeout:
				hasTimedOut = true
				return
			case nb := <-rch:
				if nb.Error != nil {
					t.Fatalf("Error Requesting /processed: %v", nb.Error)
				}
				defer nb.Response.Body.Close()
				sseReader := &sseReader{scanner: bufio.NewScanner(nb.Response.Body)}
				if sseReader.scanner.Scan() {
					event = sseReader.scanner.Text()
					if event == expected {
						return
					}
				} else {
					// Error reading SSE
					err := sseReader.scanner.Err()
					if err != nil && err != io.EOF {
						t.Errorf("Error reading SSE: %v", err)
					}
					return
				}
			}
			if tt.sleep > tt.timeout {
				if hasTimedOut == false {
					t.Errorf("Expected timeout when tt.sleep: %d> tt.timeout: %d", tt.sleep, tt.timeout)

				}
			} else {
				if event != expected {
					t.Errorf("Expected event:%s to be equal to %s", event, expected)
				}
			}

		})
	}
}
