package bot

import (
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

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
	cmd := exec.Command("docker", "compose", "down")
	return cmd.Run()
}

func Status() (string, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name=moonrelay", "--format", "{{.Status}}")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
