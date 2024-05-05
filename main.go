package main

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var mainTmpl *template.Template

type Text struct {
	Text string
	Id   string
}

type Data struct {
	Question string
	Id       int
}

func init() {
	mainTmpl = template.Must(template.New("index.html").ParseFiles("templates/index.html"))
}

func process(text string, secs time.Duration) string {
	time.Sleep(secs * time.Second)
	if secs > 0 {
		return fmt.Sprintf("You have entered: %s. Waited %d seconds", text, secs)
	}
	return fmt.Sprintf("You have entered: %s", text)
}

func MakeMainHandler(ch chan Text, id int, sleep uint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			mainTmpl.Execute(w, nil)
		case "POST":
			if err := r.ParseForm(); err != nil {
				http.Error(w, fmt.Sprintf("Error parsing form: %v", err), http.StatusBadRequest)
				return
			}
			inputText := r.Form.Get("input_text")
			snipTmpl := template.Must(template.New("snippet.html").ParseFiles("templates/snippet.html"))
			text := process(inputText, 0)
			snipTmpl.Execute(w, Data{Question: text, Id: 1})
			go func() {
				processedText := process(inputText, time.Duration(sleep))
				fmt.Printf("processedText %s\n", processedText)
				ch <- Text{
					Text: processedText,
					Id:   strconv.Itoa(id),
				}
				id++
			}()
		default:
			http.Error(w, fmt.Sprintf("HTTP method not supported: %s", r.Method), http.StatusInternalServerError)
		}
	}
}
func MakeSSECounterHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		notify := w.(http.CloseNotifier).CloseNotify()
		i := 0
		for {
			select {
			case <-notify:
				fmt.Println("Client has closed the connection")
				return
			case <-r.Context().Done():
				fmt.Println("Client closed")
				return
			default:
				time.Sleep(1 * time.Second)
				text := "<p>counter=" + strconv.Itoa(i) + "</p>"
				fmt.Fprintf(w, "data: %s\n\n", text)
				flusher, ok := w.(http.Flusher)

				if ok {
					flusher.Flush()
				} else {
					fmt.Print("NOT FLUSHED")
				}
				i++
			}
		}
	}
}

func MakeSSEDependantHandler(ch chan Text, sleep uint) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {

		case "GET":
			timeout := time.After(time.Duration(sleep + uint(1)) * time.Second)
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")

			notify := w.(http.CloseNotifier).CloseNotify()
			select {
			case <-notify:
				fmt.Println("Client has closed the connection")
				return
			case <-r.Context().Done():
				fmt.Println("Client closed")
				return
			case text := <-ch:
				html := fmt.Sprintf("<p id='answer-%s'>%s</p>", text.Id, text.Text)
				fmt.Fprintf(w, "data: %s\n\n", html)
				flusher, ok := w.(http.Flusher)
				if ok {
					flusher.Flush()
				} else {
					fmt.Println("NOT FLUSHED")
				}
				return
			case <-timeout:
				fmt.Printf("timeout %d\n", sleep)
			}
		default:
			http.Error(w, "Method not supported", http.StatusMethodNotAllowed)
		}

	}
}
func Server(port int) error {
	ch := make(chan Text)
	defer close(ch)
	var sleep uint = 2
	http.Handle("/", MakeMainHandler(ch, 0, sleep))
	http.Handle("/counter", MakeSSECounterHandler())
	http.Handle("/processed", MakeSSEDependantHandler(ch, sleep))
	http.Handle("/dist/", http.StripPrefix("/dist/", http.FileServer(http.Dir("dist/"))))

	if err := http.ListenAndServe(strings.Join([]string{"", strconv.Itoa(port)}, ":"), nil); err != nil {
		return err
	}
	return nil

}
func main() {
	if err := Server(1313); err != nil {
		panic("Error")
	}
}
