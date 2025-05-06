package main

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"

	_ "image/jpeg"
	_ "image/png"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	go StartWhatsAppClient()

	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/events", sseHandler)

	fmt.Println("Server running on http://localhost:" + port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

var clients = make(map[chan string]bool)

func StartWhatsAppClient() {
	container, err := sqlstore.New("sqlite3", "file:examplestore.db?_foreign_keys=on", nil)
	if err != nil {
		panic(err)
	}

	deviceStore, err := container.GetFirstDevice()
	if err != nil {
		panic("No device found. Scan QR first.")
	}

	client := whatsmeow.NewClient(deviceStore, nil)
	client.AddEventHandler(func(evt interface{}) {
		var urlRegex = regexp.MustCompile(`https?://[^\s]+`)

		switch v := evt.(type) {
		case *events.Message:
			text := v.Message.GetConversation()
			if text == "" && v.Message.ExtendedTextMessage != nil {
				text = v.Message.ExtendedTextMessage.GetText()
			}

			links := urlRegex.FindAllString(text, -1)
			for _, link := range links {
				sendToSSEClients(link)
			}

			// Handling image qrcode
			if v.Message.ImageMessage != nil {
				data, err := client.Download(v.Message.ImageMessage)
				if err != nil {
					fmt.Println("Download error:", err)
					break
				}

				// Decode image
				img, _, err := image.Decode(bytes.NewReader(data))
				if err != nil {
					fmt.Println("Image decode error:", err)
					break
				}

				// Decode QR or barcode
				bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
				if err != nil {
					fmt.Println("Bitmap error:", err)
					break
				}

				qrReader := qrcode.NewQRCodeReader()
				result, err := qrReader.Decode(bitmap, nil)
				if err != nil {
					fmt.Println("QR decode error:", err)
					break
				}

				links := urlRegex.FindAllString(result.String(), -1)
				for _, link := range links {
					sendToSSEClients(link)
				}
			}

			// newMessage := fmt.Sprintf("Message: %s", v.Message.GetConversation())
			// sendToSSEClients(newMessage) // custom func
		}
	})

	if client.Store.ID == nil {
		// No ID stored, new login
		return
	} else {
		// Already logged in, just connect
		err = client.Connect()
		if err != nil {
			panic(err)
		}
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	client.Disconnect()
}

func sendToSSEClients(message string) {
	for client := range clients {
		client <- message
	}
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

	messageChan := make(chan string)
	clients[messageChan] = true

	defer func() {
		delete(clients, messageChan)
		close(messageChan)
	}()

	for {
		select {
		case msg := <-messageChan:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
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
        <a href="#" id="output" style="font-family: monospace; font-size: 1.5em;"></a>

        <script>
            const output = document.getElementById("output");
            const eventSource = new EventSource("/events");

            eventSource.onmessage = function(event) {
                output.innerHTML = '';
                output.innerHTML = event.data;
                output.href = event.data;
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
