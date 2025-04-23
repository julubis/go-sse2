package main

import (
    "fmt"
    "log"
    "math/rand"
    "net/http"
    "time"
)

func main() {
    http.HandleFunc("/", indexHandler)
    http.HandleFunc("/events", sseHandler)

    fmt.Println("Server running on http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
    // Atur header agar sesuai dengan SSE
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*") // Opsional untuk CORS

    flusher, ok := w.(http.Flusher)
    if !ok {
        http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
        return
    }

    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-r.Context().Done():
            log.Println("Client disconnected")
            return
        case <-ticker.C:
            randomString := generateRandomString(8)
            fmt.Fprintf(w, "data: %s\n\n", randomString)
            flusher.Flush()
        }
    }
}

func generateRandomString(n int) string {
    letters := []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")
    b := make([]rune, n)
    for i := range b {
        b[i] = letters[rand.Intn(len(letters))]
    }
    return string(b)
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
    html := `
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <title>Golang SSE Demo</title>
    </head>
    <body>
        <h1>Random String from Server:</h1>
        <div id="output" style="font-family: monospace; font-size: 1.5em;"></div>

        <script>
            const output = document.getElementById("output");
            const eventSource = new EventSource("/events");

            eventSource.onmessage = function(event) {
                const para = document.createElement("p");
                para.textContent = "Received: " + event.data;
                output.appendChild(para);
            };

            eventSource.onerror = function(err) {
                console.error("EventSource failed:", err);
                eventSource.close();
            };
        </script>
    </body>
    </html>
    `
    w.Header().Set("Content-Type", "text/html")
    fmt.Fprint(w, html)
}
