package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"text/template"

	"github.com/caarlos0/env"
	"golang.org/x/net/proxy"
	"gopkg.in/telegram-bot-api.v4"
)

type context struct {
	TgToken     string `env:"TG_TOKEN,required"`
	Template    string `env:"TEMPLATE,required"`
	ChatID      int64  `env:"CHAT_ID,required"`
	Socks5Proxy string `env:"SOCKS5_PROXY"`

	Bot              *tgbotapi.BotAPI
	CompiledTemplate *template.Template
}

type papertrailEvent struct {
	ID       int64
	Hostname string
	Program  string
	Message  string
	Severity string
	Facility string
}

type papertrailSavedSearch struct {
	ID            int64
	Name          string
	Query         string
	HTMLEditURL   string
	HTMLSearchURL string
}

type papertrailPayload struct {
	Events      []papertrailEvent
	SavedSearch papertrailSavedSearch `json:"saved_search"`
}

func (ctx *context) getBotAPI() (*tgbotapi.BotAPI, error) {
	if ctx.Socks5Proxy != "" {
		dialer, err := proxy.SOCKS5("tcp", ctx.Socks5Proxy, nil, proxy.Direct)
		if err != nil {
			log.Fatalf("can't connect to the proxy: %s", err)
		}
		httpTransport := &http.Transport{}
		httpClient := &http.Client{Transport: httpTransport}
		httpTransport.Dial = dialer.Dial

		return tgbotapi.NewBotAPIWithClient(ctx.TgToken, httpClient)
	}

	return tgbotapi.NewBotAPI(ctx.TgToken)
}

func (ctx *context) handler(w http.ResponseWriter, r *http.Request) {
	data := r.FormValue("payload")

	if data == "" {
		log.Println("Catch request without body")
		http.Error(w, "Please send a request body", 400)
		return
	}

	var payload papertrailPayload

	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 400)
	}

	buf := bytes.NewBufferString("")

	if err := ctx.CompiledTemplate.Execute(buf, payload); err != nil {
		log.Println(err)
	}

	msg := tgbotapi.NewMessage(ctx.ChatID, buf.String())

	if _, err := ctx.Bot.Send(msg); err != nil {
		log.Println(err)
		http.Error(w, "interval error", 500)
	}

	if _, err := fmt.Fprintf(w, "%+v", payload); err != nil {
		log.Println(err)
	}
}

func main() {
	ctx := context{}
	err := env.Parse(&ctx)

	if err != nil {
		log.Fatal(err)
	}

	ctx.CompiledTemplate = template.Must(template.New("message").Parse(ctx.Template))

	ctx.Bot, err = ctx.getBotAPI()

	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", ctx.handler)
	log.Println("Listening :5555")

	if err := http.ListenAndServe(":5555", nil); err != nil {
		log.Fatal(err)
	}
}
