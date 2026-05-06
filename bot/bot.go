package bot

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"encoding/json"
	"net"
	"net/http"

	tele "gopkg.in/telebot.v4"
)

func StartBot() {
	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("You did not provide a TELEGRAM_TOKEN")
	}

	log.Println("Starting bot...")

	pref := tele.Settings{
		Token:  token,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)

	}
	b.Use(func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			allowedID, _ := strconv.ParseInt(os.Getenv("TELEGRAM_CHAT_ID"), 10, 64)
			if c.Sender().ID != allowedID {
				return c.Send("Unauthorized")
			}
			return next(c)
		}
	})
	b.Handle("/start", func(c tele.Context) error {
		return c.Send("bot is alive")
	})
	b.Handle("/stop_stream", func(c tele.Context) error {
		err := StopStream()
		if err != nil {
			return c.Send("Failed to stop stream: " + err.Error())
		}
		return c.Send("Stream stopped")
	})
	b.Handle("/status", func(c tele.Context) error {
		status, err := Status()
		if err != nil {
			return c.Send("Error checking status: " + err.Error())
		}
		if status == "" {
			return c.Send("Stream is not running")
		}
		return c.Send("Stream is running: " + status)
	})

	log.Println("Bot started successfully")
	b.Start()
}

func StopStream() error {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}
	req, err := http.NewRequest("POST", "http://localhost/containers/moonrelay/stop", nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func Status() (string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/var/run/docker.sock")
			},
		},
	}
	resp, err := client.Get("http://localhost/containers/moonrelay/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var result struct {
		State struct {
			Status string `json:"Status"`
		} `json:"State"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.State.Status, nil
}
